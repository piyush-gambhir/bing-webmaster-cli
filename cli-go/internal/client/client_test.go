package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func testClient(f roundTrip) *Client {
	c := New(time.Second)
	c.Base = KeyBase
	c.APIKey = "secret-key+/="
	c.HTTP.Transport = f
	return c
}

func TestMSDates(t *testing.T) {
	for in, want := range map[string]string{
		"/Date(1315349995284-0700)/":   "2011-09-06T15:59:55.284-07:00",
		"/Date(1315349995284)/":        "2011-09-06T22:59:55.284Z",
		"/Date(-62135568000000-0800)/": "0001-01-01T00:00:00-08:00",
		"/Date(0+0530)/":               "1970-01-01T05:30:00+05:30",
	} {
		got, ok := ParseMSDate(in)
		if !ok || got.Format(time.RFC3339Nano) != want {
			t.Errorf("%s: got %v %v, want %s", in, got.Format(time.RFC3339Nano), ok, want)
		}
	}
	if _, ok := ParseMSDate("2011-09-06"); ok {
		t.Fatal("parsed a plain date")
	}
	// Escaped slashes decode to the same string before conversion.
	var v any
	if err := json.Unmarshal([]byte(`{"d":[{"Date":"\/Date(0)\/","Query":"x"}]}`), &v); err != nil {
		t.Fatal(err)
	}
	conv := ConvertDates(v).(map[string]any)["d"].([]any)[0].(map[string]any)
	if conv["Date"] != "1970-01-01T00:00:00Z" || conv["Query"] != "x" {
		t.Fatalf("convert: %#v", conv)
	}
	if FormatMSDate(time.UnixMilli(1500).UTC()) != "/Date(1500+0000)/" {
		t.Fatal(FormatMSDate(time.UnixMilli(1500)))
	}
}

func TestGetEncodingAndEnvelope(t *testing.T) {
	c := testClient(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if r.Method != "GET" || r.URL.Path != "/webmaster/api.svc/json/GetSiteRoles" || q.Get("siteUrl") != "https://example.com/" || q.Get("includeAllSubdomains") != "false" || q.Get("apikey") != "secret-key+/=" {
			t.Fatalf("bad request %s %s", r.Method, r.URL)
		}
		return respond(200, `{"d":[{"__type":"SiteRoles:#x","Email":"a@b.c","Role":2,"Big":9007199254740993}]}`), nil
	})
	var log bytes.Buffer
	c.Log = &log
	got, err := c.Call(context.Background(), "GetSiteRoles", map[string]any{"siteUrl": "https://example.com/", "includeAllSubdomains": false})
	if err != nil {
		t.Fatal(err)
	}
	row := got.([]any)[0].(map[string]any)
	if row["__type"] != "SiteRoles:#x" || row["Big"].(json.Number).String() != "9007199254740993" {
		t.Fatalf("lost fields: %#v", row)
	}
	if strings.Contains(log.String(), "secret-key") || !strings.Contains(log.String(), "GetSiteRoles") {
		t.Fatalf("log: %s", log.String())
	}
}

func TestPostBodyAndNullAndBoolResults(t *testing.T) {
	calls := 0
	c := testClient(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch calls {
		case 1:
			list := body["urlList"].([]any)
			if r.Header.Get("Content-Type") != "application/json; charset=utf-8" || len(list) != 2 || body["siteUrl"] != "https://example.com/" {
				t.Fatalf("bad body %#v", body)
			}
			return respond(200, `{"d":null}`), nil
		default:
			return respond(200, `{"d":false}`), nil
		}
	})
	got, err := c.Call(context.Background(), "SubmitUrlBatch", map[string]any{"siteUrl": "https://example.com/", "urlList": []string{"https://example.com/a", "https://example.com/b"}})
	if err != nil || got != nil {
		t.Fatalf("null result: %v %v", got, err)
	}
	got, err = c.Call(context.Background(), "VerifySite", map[string]any{"siteUrl": "https://example.com/"})
	if err != nil || got != false {
		t.Fatalf("bool result: %#v %v", got, err)
	}
	// Request-body dates use the Microsoft format.
	if encodeBody(map[string]any{"Date": time.UnixMilli(0)}).(map[string]any)["Date"] != "/Date(0+0000)/" {
		t.Fatal("body date encoding")
	}
}

func TestFaultsUnder400And200(t *testing.T) {
	for _, status := range []int{400, 200} {
		c := testClient(func(r *http.Request) (*http.Response, error) {
			return respond(status, `{"ErrorCode":3,"Message":"InvalidApiKey secret-key+/="}`), nil
		})
		_, err := c.Call(context.Background(), "GetUserSites", map[string]any{})
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != 3 || apiErr.Name != "InvalidApiKey" || apiErr.HTTPStatus != status {
			t.Fatalf("status %d: %#v %v", status, apiErr, err)
		}
		if strings.Contains(err.Error(), "secret-key") || !strings.Contains(err.Error(), "30 minutes") {
			t.Fatalf("message: %v", err)
		}
	}
	c := testClient(func(r *http.Request) (*http.Response, error) {
		res := respond(429, "<html>slow down secret-key+/=</html>")
		res.Header.Set("Retry-After", "60")
		return res, nil
	})
	_, err := c.Call(context.Background(), "GetUserSites", map[string]any{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != "60" || apiErr.Code != -1 || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("429: %v", err)
	}
	c = testClient(func(r *http.Request) (*http.Response, error) { return respond(200, "<html>oops</html>"), nil })
	if _, err := c.Call(context.Background(), "GetUserSites", map[string]any{}); err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Fatalf("html: %v", err)
	}
}

func TestTransportErrorRedactsKey(t *testing.T) {
	c := testClient(nil)
	c.HTTP.Transport = nil
	c.Base = "http://127.0.0.1:1/webmaster/api.svc/json"
	_, err := c.Call(context.Background(), "GetUserSites", map[string]any{})
	if err == nil || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "secret-key%2B") {
		t.Fatalf("transport error leaked key: %v", err)
	}
}

func TestReadOnlyAndDryRunSendNothing(t *testing.T) {
	c := testClient(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request sent")
		return nil, nil
	})
	c.ReadOnly = true
	if _, err := c.Call(context.Background(), "SubmitUrl", map[string]any{"siteUrl": "s", "url": "u"}); err == nil || !strings.Contains(err.Error(), "--read-only") {
		t.Fatalf("read-only: %v", err)
	}
	c.ReadOnly = false
	c.DryRun = true
	c.APIKey = ""
	if _, err := c.Call(context.Background(), "SubmitUrl", map[string]any{"siteUrl": "s", "url": "u"}); err != nil {
		t.Fatal(err)
	}
	if len(c.Planned) != 1 || c.Planned[0].Method != "SubmitUrl" || c.Planned[0].Body.(map[string]any)["url"] != "u" {
		t.Fatalf("planned: %#v", c.Planned)
	}
}

func TestParamValidationAndObsolete(t *testing.T) {
	c := testClient(func(r *http.Request) (*http.Response, error) { t.Fatal("sent"); return nil, nil })
	if _, err := c.Call(context.Background(), "GetFeeds", map[string]any{}); err == nil {
		t.Fatal("missing param accepted")
	}
	if _, err := c.Call(context.Background(), "GetFeeds", map[string]any{"siteUrl": "s", "extra": 1}); err == nil {
		t.Fatal("extra param accepted")
	}
	if _, err := c.Call(context.Background(), "GetDeepLink", map[string]any{"siteUrl": "s", "url": "u"}); err == nil {
		t.Fatal("obsolete method called")
	}
	if _, err := c.Do(context.Background(), "Bad/Name", "GET", registry.Read, nil); err == nil {
		t.Fatal("bad method name accepted")
	}
}

func TestBearerAndRedirects(t *testing.T) {
	c := New(time.Second)
	c.Base = BearerBase
	c.Token = func(context.Context) (string, error) { return "tok", nil }
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Query().Has("apikey") || r.URL.Host != "www.bing.com" {
			t.Fatalf("bad bearer request %s %v", r.URL, r.Header)
		}
		res := respond(302, "")
		res.Header.Set("Location", "https://evil.example/")
		return res, nil
	})
	if _, err := c.Call(context.Background(), "GetUserSites", map[string]any{}); err == nil {
		t.Fatal("redirect treated as success")
	}
}

func TestBearerTokenRedactedFromDiagnostics(t *testing.T) {
	for _, body := range []string{
		`{"ErrorCode":14,"Message":"NotAuthorized for bearer-secret-123"}`,
		"<html>token bearer-secret-123 rejected</html>",
	} {
		c := New(time.Second)
		c.Base = BearerBase
		c.Token = func(context.Context) (string, error) { return "bearer-secret-123", nil }
		c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { return respond(400, body), nil })
		_, err := c.Call(context.Background(), "GetUserSites", map[string]any{})
		if err == nil || strings.Contains(err.Error(), "bearer-secret-123") {
			t.Fatalf("bearer token leaked: %v", err)
		}
	}
}
