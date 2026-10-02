package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/zalando/go-keyring"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

// fake records requests and answers them from a handler.
type fake struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	handle   func(r *http.Request, body string) *http.Response
}

func (f *fake) transport() http.RoundTripper {
	return roundTrip(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
		}
		f.mu.Lock()
		f.requests = append(f.requests, r)
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		if f.handle == nil {
			return nil, errors.New("unexpected network request to " + r.URL.Host + r.URL.Path)
		}
		return f.handle(r, body), nil
	})
}

func (f *fake) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.requests) }

func isolate(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("BWT_CONFIG", filepath.Join(dir, "config.yaml"))
	for _, name := range []string{"BWT_API_KEY", "BWT_ACCESS_TOKEN", "BWT_PROFILE", "BWT_SITE", "BWT_NO_INPUT", "BWT_QUIET", "BWT_VERBOSE",
		"BWT_READ_ONLY", "BWT_CLIENT_ID", "BWT_CLIENT_SECRET", "BWT_INDEXNOW_KEY", "BWT_OAUTH_API_HOST"} {
		t.Setenv(name, "")
	}
}

type result struct {
	err         error
	out, errOut string
}

func run(t *testing.T, f *fake, input string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	a := &app{in: strings.NewReader(input), out: &out, errOut: &errOut}
	if f != nil {
		a.transport = f.transport()
	} else {
		a.transport = (&fake{}).transport()
	}
	root := newRoot(a)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return result{err, out.String(), errOut.String()}
}

const sitesBody = `{"d":[{"__type":"Site:#x","Url":"https://www.example.com/","IsVerified":true,"AuthenticationCode":"AUTHCODE"}]}`

func bingHandler(t *testing.T, routes map[string]string) func(*http.Request, string) *http.Response {
	return func(r *http.Request, body string) *http.Response {
		method := filepath.Base(r.URL.Path)
		if b, ok := routes[method]; ok {
			return reply(200, b)
		}
		t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
		return reply(500, "{}")
	}
}

func TestAPIKeyLoginLifecycle(t *testing.T) {
	isolate(t)
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		if r.URL.Query().Get("apikey") != "secret-api-key" || r.URL.Host != "ssl.bing.com" {
			t.Errorf("bad credential use: %s", r.URL.Redacted())
		}
		return reply(200, sitesBody)
	}}
	res := run(t, f, "secret-api-key\n", "auth", "login", "--with-api-key", "--no-input", "-o", "json")
	if res.err != nil || strings.Contains(res.out+res.errOut, "secret-api-key") {
		t.Fatalf("login: %v %s %s", res.err, res.out, res.errOut)
	}
	var login map[string]any
	_ = json.Unmarshal([]byte(res.out), &login)
	if login["site"] != "https://www.example.com/" || login["token_store"] != "keychain" || login["verified"] != true {
		t.Fatalf("login result %v", login)
	}
	if v, _ := keyring.Get(testService(t), "default:api_key"); v != "secret-api-key" {
		t.Fatal("key not in keychain")
	}
	before := f.count()
	res = run(t, f, "", "auth", "status", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, `"source": "profile:default"`) || f.count() != before {
		t.Fatalf("status must be local: %v %s calls=%d", res.err, res.out, f.count()-before)
	}
	cfg, _ := os.ReadFile(os.Getenv("BWT_CONFIG"))
	if strings.Contains(string(cfg), "secret-api-key") {
		t.Fatal("secret written to config")
	}
	res = run(t, f, "", "auth", "logout")
	if res.err != nil || !strings.Contains(res.errOut, "no token revocation endpoint") {
		t.Fatalf("logout: %v %s", res.err, res.errOut)
	}
	if _, err := keyring.Get(testService(t), "default:api_key"); err == nil {
		t.Fatal("key survived logout")
	}
	if res = run(t, f, "", "auth", "status"); res.err == nil {
		t.Fatal("status succeeded after logout")
	}
	// The site survives logout for the next login.
	if cfg, _ := os.ReadFile(os.Getenv("BWT_CONFIG")); !strings.Contains(string(cfg), "https://www.example.com/") {
		t.Fatal("site lost on logout")
	}
}

func TestKeychainUnavailableNeedsExplicitInsecureStorage(t *testing.T) {
	isolate(t)
	keyring.MockInitWithError(errors.New("no secret service"))
	f := &fake{handle: func(r *http.Request, body string) *http.Response { return reply(200, sitesBody) }}
	res := run(t, f, "k-123456\n", "auth", "login", "--with-api-key", "--no-input")
	if res.err == nil || !strings.Contains(res.err.Error(), "--insecure-storage") {
		t.Fatalf("expected refusal: %v", res.err)
	}
	res = run(t, f, "k-123456\n", "auth", "login", "--with-api-key", "--no-input", "--insecure-storage", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, `"token_store": "file"`) {
		t.Fatalf("insecure storage: %v %s", res.err, res.out)
	}
	secretsFile := filepath.Join(filepath.Dir(os.Getenv("BWT_CONFIG")), "secrets.yaml")
	info, err := os.Stat(secretsFile)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("secrets file: %v %v", info, err)
	}
	// Commands read the key back from the file backend.
	res = run(t, f, "", "sites", "list", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, "www.example.com") {
		t.Fatalf("sites list: %v %s", res.err, res.out)
	}
}

func TestOAuthLoginAndBearerUse(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_CLIENT_ID", "cid")
	t.Setenv("BWT_CLIENT_SECRET", "csecret")
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		switch {
		case r.URL.Path == "/webmasters/oauth/token":
			form, _ := url.ParseQuery(body)
			if form.Get("client_secret") != "csecret" || form.Get("code") != "code-1" || form.Get("code_verifier") == "" {
				t.Errorf("token form %v", form)
			}
			return reply(200, `{"access_token":"access-1","token_type":"bearer","expires_in":3599,"refresh_token":"refresh-1"}`)
		case r.URL.Host == "www.bing.com" && strings.HasSuffix(r.URL.Path, "/GetUserSites"):
			if r.Header.Get("Authorization") != "Bearer access-1" {
				t.Errorf("auth header %q", r.Header.Get("Authorization"))
			}
			return reply(200, `{"d":[{"Url":"https://a.example/"},{"Url":"https://b.example/"}]}`)
		}
		t.Errorf("unexpected %s", r.URL)
		return reply(500, "{}")
	}}
	var out, errOut bytes.Buffer
	a := &app{in: strings.NewReader(""), out: &out, errOut: &errOut, transport: f.transport()}
	a.openBrowser = func(authURL string) error {
		u, _ := url.Parse(authURL)
		q := u.Query()
		if q.Get("redirect_uri") != "http://127.0.0.1:47619/callback" || q.Get("scope") != "Webmaster.manage" {
			t.Errorf("authorize URL %s", authURL)
		}
		go func() {
			if res, err := http.Get(q.Get("redirect_uri") + "?code=code-1&state=" + url.QueryEscape(q.Get("state"))); err == nil {
				res.Body.Close()
			}
		}()
		return nil
	}
	root := newRoot(a)
	root.SetArgs([]string{"auth", "login", "-o", "json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("login: %v %s", err, errOut.String())
	}
	if strings.Contains(out.String()+errOut.String(), "refresh-1") || strings.Contains(out.String()+errOut.String(), "csecret") {
		t.Fatal("secret printed")
	}
	if !strings.Contains(out.String(), `"sites": 2`) || strings.Contains(out.String(), `"site":`) {
		t.Fatalf("two sites without a terminal must not pick a default: %s", out.String())
	}
	cfg, _ := os.ReadFile(os.Getenv("BWT_CONFIG"))
	if !strings.Contains(string(cfg), "client_id: cid") || strings.Contains(string(cfg), "refresh-1") {
		t.Fatalf("config: %s", cfg)
	}
	// A later command uses the cached access token without another token call.
	before := f.count()
	if res := run(t, f, "", "sites", "list"); res.err != nil || f.count() != before+1 {
		t.Fatalf("sites list: %v calls=%d", res.err, f.count()-before)
	}
}

func TestBrowserLoginRefusedWithoutInput(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_CLIENT_ID", "cid")
	t.Setenv("BWT_CLIENT_SECRET", "s")
	res := run(t, nil, "", "auth", "login", "--no-input")
	if res.err == nil || !strings.Contains(res.err.Error(), "--with-api-key") {
		t.Fatalf("%v", res.err)
	}
	t.Setenv("BWT_CLIENT_ID", "")
	if res := run(t, nil, "", "auth", "login"); res.err == nil || !strings.Contains(res.err.Error(), "no built-in OAuth client") {
		t.Fatalf("missing client: %v", res.err)
	}
}

func TestReadOnlyBlocksWritesBeforeNetwork(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	t.Setenv("BWT_SITE", "https://www.example.com/")
	writes := [][]string{
		{"sites", "add", "https://x.example/"}, {"sites", "verify", "https://x.example/"}, {"sites", "remove", "https://x.example/"},
		{"submit", "urls", "https://www.example.com/a"}, {"sitemaps", "submit", "https://www.example.com/s.xml"},
		{"indexnow", "submit", "https://www.example.com/a", "--key", "abcdef12"}, {"block", "add", "https://www.example.com/a", "--request", "cache-only"},
		{"crawl", "set-settings", "--boost", "on"}, {"users", "add", "x@example.com", "--role", "read-only"}, {"fetch", "request", "https://www.example.com/a"},
		{"params", "add", "utm"}, {"preview-blocks", "add", "https://www.example.com/a", "--reason", "1"},
		{"experimental", "site-move", "submit", "--from", "https://a/", "--to", "https://b/", "--scope", "host", "--type", "global"},
		{"auth", "login", "--with-api-key"}, {"auth", "logout"}, {"sites", "use", "https://www.example.com/"}, {"auth", "use", "x"},
		{"api", "SubmitUrl", "--data", `{"siteUrl":"s","url":"u"}`}, {"api", "Unknown", "--http", "GET"}, {"update"},
		{"indexnow", "key", "generate", "--host", "example.com"},
	}
	for _, args := range writes {
		f := &fake{}
		res := run(t, f, "", append(args, "--read-only")...)
		if res.err == nil || !strings.Contains(res.err.Error(), "read-only") || f.count() != 0 {
			t.Errorf("%v: err=%v calls=%d", args, res.err, f.count())
		}
	}
	// Reads, including the POST read GetChildrenUrlInfo, stay allowed.
	f := &fake{handle: func(r *http.Request, body string) *http.Response { return reply(200, `{"d":[]}`) }}
	for _, args := range [][]string{{"url", "children", "https://www.example.com/"}, {"api", "GetUserSites"}, {"crawl", "issues"}} {
		if res := run(t, f, "", append(args, "--read-only")...); res.err != nil {
			t.Errorf("%v blocked: %v", args, res.err)
		}
	}
	if res := run(t, nil, "", "indexnow", "key", "generate", "--host", "example.com", "--no-save", "--read-only"); res.err != nil {
		t.Errorf("--no-save must work read-only: %v", res.err)
	}
}

func TestDryRunAndBatching(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	t.Setenv("BWT_SITE", "https://www.example.com/")
	path := filepath.Join(t.TempDir(), "urls.txt")
	var b strings.Builder
	for i := 0; i < 1001; i++ {
		fmt.Fprintf(&b, "https://www.example.com/p%d\n", i)
	}
	b.WriteString("# comment\n\nhttps://www.example.com/p0\n")
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	res := run(t, &fake{}, "", "submit", "urls", "--file", path, "--dry-run", "-o", "json")
	if res.err != nil {
		t.Fatal(res.err)
	}
	var plan struct {
		Requests []struct {
			Method string
			Body   map[string]any
		}
	}
	_ = json.Unmarshal([]byte(res.out), &plan)
	if len(plan.Requests) != 3 || len(plan.Requests[0].Body["urlList"].([]any)) != 500 || len(plan.Requests[2].Body["urlList"].([]any)) != 1 {
		t.Fatalf("plan: %s", res.out)
	}
	if strings.Contains(res.out, "k-123456") {
		t.Fatal("dry run printed the key")
	}
	calls := 0
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		calls++
		if calls == 2 {
			return reply(400, `{"ErrorCode":4,"Message":"ThrottleUser"}`)
		}
		return reply(200, `{"d":null}`)
	}}
	res = run(t, f, "", "submit", "urls", "--file", path, "-o", "json")
	if res.err == nil || !strings.Contains(res.err.Error(), "500 of 1001") || calls != 2 || !strings.Contains(res.out, `"accepted": 500`) {
		t.Fatalf("partial failure: %v calls=%d %s", res.err, calls, res.out)
	}
}

func TestStatsEnvelopeAndLocalFilters(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	body := `{"d":[{"Query":"shoes","Date":"\/Date(1788246000000-0700)\/","Clicks":5,"Impressions":100,"AvgClickPosition":31,"AvgImpressionPosition":55},
		{"Query":"boots","Date":"\/Date(1788850800000-0700)\/","Clicks":9,"Impressions":0,"AvgClickPosition":12,"AvgImpressionPosition":14}]}`
	f := &fake{handle: bingHandler(t, map[string]string{"GetQueryStats": body})}
	res := run(t, f, "", "stats", "queries", "-s", "https://www.example.com/", "--since", "2026-09-05", "-o", "json")
	if res.err != nil {
		t.Fatal(res.err)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(res.out), &env)
	rows := env["rows"].([]any)
	if env["filtered_locally"] != true || env["update_cadence"] != "weekly" || len(rows) != 1 || rows[0].(map[string]any)["Date"] != "2026-09-08T00:00:00-07:00" {
		t.Fatalf("envelope: %s", res.out)
	}
	if _, has := rows[0].(map[string]any)["ctr"]; has {
		t.Fatal("JSON rows must stay as Bing returned them")
	}
	res = run(t, f, "", "stats", "queries", "-s", "https://www.example.com/")
	if res.err != nil || !strings.Contains(res.out, "avg_impression_position") || !strings.Contains(res.out, "0.05") {
		t.Fatalf("table: %v %s", res.err, res.out)
	}
	if res = run(t, f, "", "stats", "queries", "-s", "https://www.example.com/", "--raw", "--limit", "1"); res.err == nil {
		t.Fatal("--raw with local filters accepted")
	}
}

func TestIndexNowFromSitemap(t *testing.T) {
	isolate(t)
	res := run(t, nil, "", "indexnow", "key", "generate", "--host", "www.example.com", "-o", "json")
	if res.err != nil {
		t.Fatal(res.err)
	}
	var gen map[string]any
	_ = json.Unmarshal([]byte(res.out), &gen)
	key := gen["key"].(string)
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://www.example.com/new</loc><lastmod>2026-10-02</lastmod></url><url><loc>https://www.example.com/old</loc><lastmod>2025-01-01</lastmod></url></urlset>`
	var posted map[string]any
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		switch r.URL.Host {
		case "www.example.com":
			return reply(200, sitemap)
		case "api.indexnow.org":
			_ = json.Unmarshal([]byte(body), &posted)
			return reply(200, "")
		}
		t.Errorf("unexpected %s", r.URL)
		return reply(500, "")
	}}
	res = run(t, f, "", "indexnow", "submit", "--sitemap", "https://www.example.com/sitemap.xml", "--changed-since", "2026-10-01", "-o", "json")
	if res.err != nil {
		t.Fatalf("%v %s", res.err, res.errOut)
	}
	if posted["key"] != key || len(posted["urlList"].([]any)) != 1 || posted["keyLocation"] != "https://www.example.com/"+key+".txt" {
		t.Fatalf("posted %v", posted)
	}
	f.handle = func(r *http.Request, body string) *http.Response { return reply(422, "") }
	if res = run(t, f, "", "indexnow", "submit", "https://www.example.com/x"); res.err == nil || !strings.Contains(res.err.Error(), "422") {
		t.Fatalf("422: %v", res.err)
	}
	if res = run(t, nil, "", "indexnow", "submit", "https://other.example/x"); res.err == nil || !strings.Contains(res.err.Error(), "no IndexNow key for other.example") {
		t.Fatalf("missing key: %v", res.err)
	}
}

func TestValidationBeforeNetwork(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	t.Setenv("BWT_SITE", "https://www.example.com/")
	for _, args := range [][]string{
		{"stats", "queries", "--since", "09/01/2026"}, {"stats", "queries", "--sort", "nope"}, {"submit", "urls", "not-a-url"},
		{"submit", "urls", "https://x/", "--batch-size", "501"}, {"crawl", "set-settings", "--rate", "1,2,3"},
		{"indexnow", "submit", "https://x/", "--endpoint", "http://insecure/"}, {"keywords", "get", "q", "--start", "2026-09-10", "--end", "2026-09-01"},
		{"sites", "list", "-o", "xml"}, {"url", "children", "https://x/", "--http", "9xx"}, {"block", "add", "https://x/", "--request", "forever"},
		{"users", "add", "x@example.com", "--role", "owner"}, {"params", "add", "bad name"}, {"stats", "page-queries", "not-a-url"},
		{"experimental", "geo", "add", "--url", "https://x/", "--country", "usa"}, {"api", "GetDeepLink"}, {"stats", "summary", "--days", "0"},
	} {
		f := &fake{}
		if res := run(t, f, "", args...); res.err == nil || f.count() != 0 {
			t.Errorf("%v: err=%v calls=%d", args, res.err, f.count())
		}
	}
}

func TestStructuredErrorsAndRedaction(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "super-secret-key")
	var out, errOut bytes.Buffer
	// A transport error that echoes the request URL, as net/http does, must not leak the key.
	leaky := roundTrip(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: lookup failed for " + r.URL.String())
	})
	a := &app{in: strings.NewReader(""), out: &out, errOut: &errOut, transport: leaky}
	code := execute(context.Background(), a, []string{"sites", "list", "-o", "json", "--verbose"})
	if code == 0 || strings.Contains(errOut.String(), "super-secret-key") || !json.Valid(lastJSON(errOut.String())) || out.Len() != 0 {
		t.Fatalf("code=%d err=%s", code, errOut.String())
	}
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		return reply(400, `{"ErrorCode":14,"Message":"NotAuthorized"}`)
	}}
	res := run(t, f, "", "sitemaps", "list", "-s", "https://www.example.com/")
	if res.err == nil || !strings.Contains(res.err.Error(), "NotAuthorized") || !strings.Contains(res.err.Error(), "matches bwt sites list exactly") {
		t.Fatalf("fault: %v", res.err)
	}
}

// lastJSON returns the JSON document at the end of stderr (after verbose log lines).
func lastJSON(s string) []byte {
	if i := strings.Index(s, "{"); i >= 0 {
		return []byte(s[i:])
	}
	return nil
}

func TestBareHostAndLookups(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k-123456")
	var removed string
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		switch filepath.Base(r.URL.Path) {
		case "GetUserSites":
			return reply(200, sitesBody)
		case "GetSiteRoles":
			return reply(200, `{"d":[{"Email":"a@example.com","Site":"https://www.example.com/","Role":1},{"Email":"a@example.com","Site":"https://blog.example.com/","Role":2}]}`)
		case "RemoveSiteRole":
			removed = body
			return reply(200, `{"d":null}`)
		case "GetFeeds":
			if r.URL.Query().Get("siteUrl") != "https://www.example.com/" {
				t.Errorf("bare host not resolved: %s", r.URL.Query().Get("siteUrl"))
			}
			return reply(200, `{"d":[]}`)
		}
		return reply(500, "{}")
	}}
	if res := run(t, f, "", "sitemaps", "list", "-s", "example.com"); res.err != nil {
		t.Fatal(res.err)
	}
	res := run(t, f, "", "users", "remove", "a@example.com", "-s", "https://www.example.com/", "--yes")
	if res.err == nil || !strings.Contains(res.err.Error(), "2 roles") {
		t.Fatalf("ambiguous: %v", res.err)
	}
	res = run(t, f, "", "users", "remove", "a@example.com", "-s", "https://www.example.com/", "--delegated-url", "https://blog.example.com/")
	if res.err == nil || !strings.Contains(res.err.Error(), "--yes") {
		t.Fatalf("destructive without confirmation: %v", res.err)
	}
	res = run(t, f, "", "users", "remove", "a@example.com", "-s", "https://www.example.com/", "--delegated-url", "https://blog.example.com/", "--yes")
	if res.err != nil || !strings.Contains(removed, `"Site":"https://blog.example.com/"`) {
		t.Fatalf("remove: %v %s", res.err, removed)
	}
}

// testService is the keychain service the CLI uses for the test's config path.
func testService(t *testing.T) string {
	t.Helper()
	st, err := (&app{}).store()
	if err != nil {
		t.Fatal(err)
	}
	return st.Service
}
