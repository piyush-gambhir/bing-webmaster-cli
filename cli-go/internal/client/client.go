// Package client calls the Bing Webmaster JSON API. It unwraps the {"d": ...}
// envelope, detects faults even under HTTP 200, keeps API keys out of every log
// line and error, and refuses state-changing calls in read-only mode.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
)

const (
	// KeyBase is the documented host for API-key calls.
	KeyBase = "https://ssl.bing.com/webmaster/api.svc/json"
	// BearerBase is the host Bing's OAuth guide uses for bearer-token calls.
	BearerBase  = "https://www.bing.com/webmaster/api.svc/json"
	maxResponse = 32 << 20
)

// Planned is a write that --dry-run printed instead of sending.
type Planned struct {
	Method string `json:"method" yaml:"method"`
	HTTP   string `json:"http" yaml:"http"`
	URL    string `json:"url" yaml:"url"`
	Body   any    `json:"body,omitempty" yaml:"body,omitempty"`
}

type Client struct {
	HTTP   *http.Client
	Base   string
	APIKey string
	// Token returns a bearer token; it is called only when a request is sent.
	Token    func(context.Context) (string, error)
	Log      io.Writer
	ReadOnly bool
	DryRun   bool
	Planned  []Planned

	// bearer is the token sent on the current request, kept so diagnostics can
	// redact it like the API key.
	bearer string
}

// New returns a client that never follows redirects, so credentials cannot be
// forwarded to another host.
func New(timeout time.Duration) *Client {
	return &Client{HTTP: &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

var methodName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// Call invokes a documented method with exactly its documented parameters.
func (c *Client) Call(ctx context.Context, method string, params map[string]any) (any, error) {
	op, ok := registry.Lookup(method)
	if !ok {
		return nil, fmt.Errorf("unknown Bing method %q", method)
	}
	if op.Status == registry.Obsolete {
		return nil, fmt.Errorf("%s is marked obsolete by Microsoft and is not supported", method)
	}
	known := map[string]bool{}
	for _, prm := range op.Params {
		known[prm.Name] = true
		if _, ok := params[prm.Name]; !ok {
			return nil, fmt.Errorf("%s requires parameter %s", method, prm.Name)
		}
	}
	for name := range params {
		if !known[name] {
			return nil, fmt.Errorf("%s has no parameter %s", method, name)
		}
	}
	return c.Do(ctx, method, op.HTTP, op.Effect, params)
}

// Do sends one request. The raw `api` command uses it directly so methods outside
// the registry still get the same transport, redaction, and fault handling.
func (c *Client) Do(ctx context.Context, method, verb string, effect registry.Effect, params map[string]any) (any, error) {
	if !methodName.MatchString(method) {
		return nil, fmt.Errorf("invalid method name %q", method)
	}
	if verb != http.MethodGet && verb != http.MethodPost {
		return nil, fmt.Errorf("unsupported HTTP method %q; Bing uses GET or POST", verb)
	}
	if effect == registry.Write && c.ReadOnly {
		return nil, fmt.Errorf("%s changes Bing state and is blocked by --read-only", method)
	}
	endpoint := strings.TrimRight(c.Base, "/") + "/" + method
	q := url.Values{}
	var body []byte
	var shown any
	if verb == http.MethodGet {
		for k, v := range params {
			q.Set(k, encodeQuery(v))
		}
	} else {
		encoded := encodeBody(params)
		if encoded == nil {
			encoded = map[string]any{}
		}
		b, err := json.Marshal(encoded)
		if err != nil {
			return nil, err
		}
		body, shown = b, encoded
	}
	visible := endpoint
	if len(q) > 0 {
		visible += "?" + q.Encode()
	}
	if c.DryRun && effect == registry.Write {
		c.Planned = append(c.Planned, Planned{Method: method, HTTP: verb, URL: visible, Body: shown})
		return nil, nil
	}
	if c.APIKey == "" && c.Token == nil {
		return nil, errors.New("no Bing credentials; run bwt auth login or set BWT_API_KEY")
	}
	if c.APIKey != "" {
		q.Set("apikey", c.APIKey)
	}
	full := endpoint
	if len(q) > 0 {
		full += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, verb, full, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New(c.redact(err.Error()))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bwt")
	if verb == http.MethodPost {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if c.Token != nil && c.APIKey == "" {
		token, err := c.Token(ctx)
		if err != nil {
			return nil, err
		}
		c.bearer = token
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.Log != nil {
		fmt.Fprintf(c.Log, "%s %s\n", verb, visible)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bing %s request failed: %s", method, c.redact(err.Error()))
	}
	defer res.Body.Close()
	if c.Log != nil {
		fmt.Fprintf(c.Log, "HTTP %d\n", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read Bing %s response: %s", method, c.redact(err.Error()))
	}
	if len(b) > maxResponse {
		return nil, fmt.Errorf("Bing %s response exceeds 32 MiB", method)
	}
	return c.decode(method, res, b)
}

func (c *Client) decode(method string, res *http.Response, b []byte) (any, error) {
	retryAfter := res.Header.Get("Retry-After")
	var data any
	var decodeErr error
	if len(bytes.TrimSpace(b)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		decodeErr = dec.Decode(&data)
		if decodeErr == nil && dec.Decode(new(any)) != io.EOF {
			decodeErr = errors.New("trailing content")
		}
	}
	// Faults can arrive under HTTP 400 (documented) or 200 (observed).
	if m, ok := data.(map[string]any); ok && decodeErr == nil {
		if _, hasD := m["d"]; !hasD {
			if code, ok := faultCode(m["ErrorCode"]); ok {
				msg, _ := m["Message"].(string)
				e := newFault(method, res.StatusCode, code, clean(c.redact(msg), 300))
				e.RetryAfter = retryAfter
				return nil, e
			}
		}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		e := &APIError{Method: method, HTTPStatus: res.StatusCode, Code: -1, RetryAfter: retryAfter}
		switch res.StatusCode {
		case http.StatusUnauthorized:
			e.Message = "unauthorized: the access token expired or was revoked, or the API key is invalid; run bwt auth login"
		case http.StatusForbidden:
			e.Message = "forbidden: the credential cannot access this site or method"
		case http.StatusNotFound:
			e.Message = "not found: Bing returned 404 (some legacy methods are no longer served)"
		case http.StatusTooManyRequests:
			e.Message = "rate limited; no automatic retry was attempted"
		default:
			e.Message = http.StatusText(res.StatusCode)
		}
		if decodeErr != nil && len(b) > 0 {
			e.Message += "; response: " + clean(c.redact(string(b)), 200)
		}
		return nil, e
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Bing %s returned non-JSON content (HTTP %d): %s", method, res.StatusCode, clean(c.redact(string(b)), 200))
	}
	if m, ok := data.(map[string]any); ok {
		if d, ok := m["d"]; ok {
			return d, nil
		}
	}
	return data, nil
}

func faultCode(v any) (int, bool) {
	switch x := v.(type) {
	case json.Number:
		n, err := strconv.Atoi(x.String())
		return n, err == nil
	case string:
		for code, name := range ErrorNames {
			if strings.EqualFold(name, x) {
				return code, true
			}
		}
	}
	return 0, false
}

// redact removes the API key and the bearer token, in plain and URL-encoded
// form, from s.
func (c *Client) redact(s string) string {
	for _, secret := range []string{c.APIKey, c.bearer} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
			s = strings.ReplaceAll(s, url.QueryEscape(secret), "[REDACTED]")
		}
	}
	return s
}

func encodeQuery(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case time.Time:
		// Date-only form; Bing does not document the accepted DateTime syntax for
		// query parameters, and ISO dates parse under WCF's query converter.
		return x.Format("2006-01-02")
	default:
		return fmt.Sprint(x)
	}
}

func encodeBody(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = encodeBody(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = encodeBody(val)
		}
		return out
	case time.Time:
		return FormatMSDate(x)
	default:
		return v
	}
}
