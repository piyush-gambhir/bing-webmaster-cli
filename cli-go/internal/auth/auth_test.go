package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func fakeEndpoints(t *testing.T) {
	t.Helper()
	oldA, oldT := AuthorizeURL, TokenURL
	AuthorizeURL, TokenURL = "https://bing.test/webmasters/oauth/authorize", "https://bing.test/webmasters/oauth/token"
	t.Cleanup(func() { AuthorizeURL, TokenURL = oldA, oldT })
}

func TestLoginSendsPKCEStateAndSecretInForm(t *testing.T) {
	fakeEndpoints(t)
	var requests int32
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&requests, 1)
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		f := r.PostForm
		if _, _, hasBasic := r.BasicAuth(); hasBasic || f.Get("client_secret") != "sec" || f.Get("client_id") != "cid" || f.Get("grant_type") != "authorization_code" || f.Get("code") != "the-code" || f.Get("code_verifier") == "" || !strings.HasPrefix(f.Get("redirect_uri"), "http://127.0.0.1:") {
			t.Fatalf("bad token request: %v basic=%v", f, hasBasic)
		}
		return jsonResponse(200, `{"access_token":"at","token_type":"bearer","expires_in":3599,"refresh_token":"rt"}`), nil
	})}
	browser := func(authURL string) error {
		u, _ := url.Parse(authURL)
		q := u.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("scope") != ScopeManage || q.Get("client_id") != "cid" || q.Get("state") == "" || q.Get("response_type") != "code" {
			t.Errorf("bad authorize URL %s", authURL)
		}
		go func() {
			res, err := http.Get(q.Get("redirect_uri") + "?code=the-code&state=" + url.QueryEscape(q.Get("state")))
			if err == nil {
				res.Body.Close()
			}
		}()
		return nil
	}
	tok, err := Login(context.Background(), LoginOptions{ClientID: "cid", ClientSecret: "sec", Scope: ScopeManage, Port: 47998, OpenBrowser: browser, HTTP: httpClient, Timeout: 5 * time.Second})
	if err != nil || tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Fatalf("%+v %v", tok, err)
	}
	if requests != 1 {
		t.Fatalf("token requests: %d", requests)
	}
}

func TestLoginTokenFailureIsOneRequestAndNoBody(t *testing.T) {
	fakeEndpoints(t)
	var requests int32
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&requests, 1)
		return jsonResponse(400, `{"error":"invalid_client","error_description":"leaky detail"}`), nil
	})}
	browser := func(authURL string) error {
		u, _ := url.Parse(authURL)
		q := u.Query()
		go func() {
			if res, err := http.Get(q.Get("redirect_uri") + "?code=c&state=" + url.QueryEscape(q.Get("state"))); err == nil {
				res.Body.Close()
			}
		}()
		return nil
	}
	_, err := Login(context.Background(), LoginOptions{ClientID: "cid", ClientSecret: "sec", Scope: ScopeManage, Port: 47997, OpenBrowser: browser, HTTP: httpClient, Timeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "invalid_client") || strings.Contains(err.Error(), "leaky") {
		t.Fatalf("error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("token endpoint called %d times; AuthStyle probing must be off", requests)
	}
}

// setup creates an OAuth profile whose access token has expired, stored in the
// file backend so concurrent tests do not share go-keyring's unsynchronized mock.
func setup(t *testing.T, clientID string) (string, secrets.Store) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	store := secrets.Store{Service: "bwt-test", FilePath: filepath.Join(dir, "secrets.yaml")}
	if err := config.Update(context.Background(), cfgPath, func(c *config.Config) error {
		c.Profiles["p"] = config.Profile{Auth: config.AuthOAuth, Client: config.ClientBuiltin, ClientID: clientID, Scope: ScopeManage, TokenStore: string(secrets.File)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	payload, _ := Encode(Stored{RefreshToken: "rt"})
	if _, err := store.Save(context.Background(), SecretKey("p", "oauth"), payload, secrets.File, false); err != nil {
		t.Fatal(err)
	}
	expired, _ := Encode(cachedToken{AccessToken: "old", Expiry: time.Now().Add(-time.Hour)})
	if _, err := store.Save(context.Background(), SecretKey("p", "access"), expired, secrets.File, false); err != nil {
		t.Fatal(err)
	}
	return cfgPath, store
}

func TestConcurrentRefreshHappensOnce(t *testing.T) {
	fakeEndpoints(t)
	t.Setenv("BWT_CLIENT_ID", "cid")
	t.Setenv("BWT_CLIENT_SECRET", "sec")
	cfgPath, store := setup(t, "cid")
	var refreshes int32
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&refreshes, 1)
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != "rt" || r.PostForm.Get("client_secret") != "sec" {
			t.Errorf("bad refresh form %v", r.PostForm)
		}
		time.Sleep(50 * time.Millisecond)
		return jsonResponse(200, `{"access_token":"new`+string(rune('0'+n))+`","token_type":"bearer","expires_in":3599}`), nil
	})}
	var wg sync.WaitGroup
	tokens := make([]string, 2)
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := &Session{Profile: "p", ConfigPath: cfgPath, Store: store, HTTP: httpClient}
			tok, err := s.Token(context.Background())
			if err != nil {
				t.Error(err)
			}
			tokens[i] = tok
		}(i)
	}
	wg.Wait()
	if refreshes != 1 || tokens[0] != "new1" || tokens[1] != "new1" {
		t.Fatalf("refreshes=%d tokens=%v", refreshes, tokens)
	}
	// The refresh token survives a response that omits it.
	raw, _ := store.Load(secrets.File, SecretKey("p", "oauth"))
	var stored Stored
	_ = json.Unmarshal([]byte(raw), &stored)
	if stored.RefreshToken != "rt" {
		t.Fatalf("refresh token lost: %+v", stored)
	}
}

func TestReadOnlyRefreshIsNotPersisted(t *testing.T) {
	fakeEndpoints(t)
	t.Setenv("BWT_CLIENT_ID", "cid")
	t.Setenv("BWT_CLIENT_SECRET", "sec")
	cfgPath, store := setup(t, "cid")
	httpClient := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"access_token":"mem","token_type":"bearer","expires_in":3599,"refresh_token":"rotated"}`), nil
	})}
	s := &Session{Profile: "p", ConfigPath: cfgPath, Store: store, HTTP: httpClient, ReadOnly: true}
	if tok, err := s.Token(context.Background()); err != nil || tok != "mem" {
		t.Fatalf("%q %v", tok, err)
	}
	raw, _ := store.Load(secrets.File, SecretKey("p", "oauth"))
	if strings.Contains(raw, "rotated") {
		t.Fatal("read-only run persisted a rotated refresh token")
	}
	access, _ := store.Load(secrets.File, SecretKey("p", "access"))
	if strings.Contains(access, "mem") {
		t.Fatal("read-only run persisted the access token")
	}
}

func TestRefreshErrors(t *testing.T) {
	fakeEndpoints(t)
	t.Setenv("BWT_CLIENT_ID", "different")
	t.Setenv("BWT_CLIENT_SECRET", "sec")
	cfgPath, store := setup(t, "cid")
	s := &Session{Profile: "p", ConfigPath: cfgPath, Store: store, HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		t.Fatal("refreshed with the wrong client")
		return nil, nil
	})}}
	if _, err := s.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "different OAuth client") {
		t.Fatalf("client mismatch: %v", err)
	}
	t.Setenv("BWT_CLIENT_ID", "cid")
	s = &Session{Profile: "p", ConfigPath: cfgPath, Store: store, HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(400, `{"error":"invalid_grant"}`), nil
	})}}
	if _, err := s.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "bwt auth login") {
		t.Fatalf("invalid_grant: %v", err)
	}
	// A profile logged out meanwhile is not resurrected.
	_ = config.Update(context.Background(), cfgPath, func(c *config.Config) error { delete(c.Profiles, "p"); return nil })
	if _, err := s.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "no longer logged in") {
		t.Fatalf("deleted profile: %v", err)
	}
}

func TestBuiltinPrefersEnvironment(t *testing.T) {
	oldID, oldSecret := ClientID, ClientSecret
	t.Cleanup(func() { ClientID, ClientSecret = oldID, oldSecret })
	ClientID, ClientSecret = "baked", "baked-secret"
	t.Setenv("BWT_CLIENT_ID", "")
	if id, _, ok := Builtin(); !ok || id != "baked" {
		t.Fatal("ldflags client not used")
	}
	t.Setenv("BWT_CLIENT_ID", "env")
	t.Setenv("BWT_CLIENT_SECRET", "env-secret")
	if id, secret, ok := Builtin(); !ok || id != "env" || secret != "env-secret" {
		t.Fatal("env override not used")
	}
	ClientID, ClientSecret = "", ""
	t.Setenv("BWT_CLIENT_ID", "")
	if _, _, ok := Builtin(); ok {
		t.Fatal("empty build reported a client")
	}
}
