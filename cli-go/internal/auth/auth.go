// Package auth implements Bing Webmaster OAuth 2.0 login and token refresh.
// Release builds embed a built-in OAuth client through -ldflags; the values are
// empty in source so forks never reuse it silently.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/oauthflow"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
	"golang.org/x/oauth2"
)

// Built-in OAuth client, injected at build time:
//
//	-X github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth.ClientID=...
//	-X github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth.ClientSecret=...
var (
	ClientID     string
	ClientSecret string
)

// Endpoints from Bing's OAuth guide; variables so tests can point them at fakes.
var (
	AuthorizeURL = "https://www.bing.com/webmasters/oauth/authorize"
	TokenURL     = "https://www.bing.com/webmasters/oauth/token"
)

const (
	// RedirectPort is registered with the built-in client; Bing requires an exact match.
	RedirectPort = 47619
	RedirectPath = "/callback"
	ScopeManage  = "Webmaster.manage"
	ScopeRead    = "Webmaster.read"
)

// Builtin returns the built-in client, with BWT_CLIENT_ID and BWT_CLIENT_SECRET
// taking precedence over the values compiled into the binary.
func Builtin() (id, secret string, ok bool) {
	id, secret = ClientID, ClientSecret
	if env := os.Getenv("BWT_CLIENT_ID"); env != "" {
		id, secret = env, os.Getenv("BWT_CLIENT_SECRET")
	}
	return id, secret, id != "" && secret != ""
}

// RedirectURI is the loopback callback for a port.
func RedirectURI(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, RedirectPath)
}

func oauthConfig(id, secret, redirect, scope string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: id, ClientSecret: secret, RedirectURL: redirect, Scopes: []string{scope},
		// Explicit style: x/oauth2 would otherwise probe with Basic auth and
		// silently retry with form parameters. Bing documents form parameters.
		Endpoint: oauth2.Endpoint{AuthURL: AuthorizeURL, TokenURL: TokenURL, AuthStyle: oauth2.AuthStyleInParams},
	}
}

type LoginOptions struct {
	ClientID, ClientSecret, Scope string
	Port                          int
	OpenBrowser                   func(string) error
	Log                           io.Writer
	HTTP                          *http.Client
	Timeout                       time.Duration
}

// Login runs the browser flow: it binds the registered loopback port before
// opening the browser, requires a matching state, sends PKCE parameters, and
// exchanges the code once.
func Login(ctx context.Context, o LoginOptions) (*oauth2.Token, error) {
	if o.Port == 0 {
		o.Port = RedirectPort
	}
	verifier := oauth2.GenerateVerifier()
	var conf *oauth2.Config
	res, err := oauthflow.Run(ctx, oauthflow.Options{
		Port: o.Port, Path: RedirectPath, Log: o.Log, OpenBrowser: o.OpenBrowser, Timeout: o.Timeout,
		AuthURL: func(redirect, state string) string {
			conf = oauthConfig(o.ClientID, o.ClientSecret, redirect, o.Scope)
			return conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
		},
	})
	if err != nil {
		return nil, err
	}
	tok, err := conf.Exchange(withHTTP(ctx, o.HTTP), res.Code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, describe("exchange the authorization code", err)
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("Bing returned no refresh token, so the login could not be saved")
	}
	return tok, nil
}

func withHTTP(ctx context.Context, h *http.Client) context.Context {
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return context.WithValue(ctx, oauth2.HTTPClient, h)
}

// describe turns token endpoint failures into a message without echoing response bodies.
func describe(action string, err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		status := 0
		if re.Response != nil {
			status = re.Response.StatusCode
		}
		switch re.ErrorCode {
		case "invalid_grant":
			return fmt.Errorf("Bing rejected the saved login (invalid_grant: expired, revoked, or issued to another client); run bwt auth login")
		case "invalid_client":
			return fmt.Errorf("Bing rejected the OAuth client credentials (invalid_client)")
		case "":
			return fmt.Errorf("could not %s: Bing token endpoint returned HTTP %d", action, status)
		default:
			return fmt.Errorf("could not %s: %s (HTTP %d)", action, re.ErrorCode, status)
		}
	}
	return fmt.Errorf("could not %s: %w", action, err)
}

// Stored is the long-lived keychain payload for an OAuth profile. The access
// token is cached in a separate item (see cachedToken) so each item stays under
// Windows Credential Manager's 2,560-byte limit.
type Stored struct {
	RefreshToken string `json:"refresh_token"`
	// ClientSecret is kept only for custom (bring-your-own) OAuth clients.
	ClientSecret string `json:"client_secret,omitempty"`
}

type cachedToken struct {
	AccessToken string    `json:"access_token"`
	Expiry      time.Time `json:"expiry"`
}

func (c *cachedToken) valid() bool {
	return c != nil && c.AccessToken != "" && time.Until(c.Expiry) > time.Minute
}

// SecretKey names a profile's secret in the store: kind is oauth, access, or api_key.
func SecretKey(profile, kind string) string { return profile + ":" + kind }

func Encode(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func decode(raw string) (*Stored, error) {
	var s Stored
	if err := json.Unmarshal([]byte(raw), &s); err != nil || s.RefreshToken == "" {
		return nil, errors.New("saved OAuth login is unreadable; run bwt auth login")
	}
	return &s, nil
}

// Session hands out access tokens for one OAuth profile, refreshing them under
// a per-profile cross-process lock so concurrent commands do not race.
type Session struct {
	Profile    string
	ConfigPath string
	Store      secrets.Store
	ReadOnly   bool
	HTTP       *http.Client

	mu     sync.Mutex
	cached *cachedToken
}

// LockProfile takes the per-profile cross-process lock that token refresh uses,
// so login and logout cannot interleave with a refresh in another process.
func LockProfile(ctx context.Context, configPath, profile string) (func(), error) {
	return config.Lock(ctx, profileLockPath(configPath, profile))
}

func profileLockPath(configPath, profile string) string {
	return filepath.Join(filepath.Dir(configPath), "locks", profile+".lock")
}

func (s *Session) lockPath() string { return profileLockPath(s.ConfigPath, s.Profile) }

func (s *Session) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached.valid() {
		return s.cached.AccessToken, nil
	}
	unlock, err := config.Lock(ctx, s.lockPath())
	if err != nil {
		return "", err
	}
	defer unlock()
	// Re-read under the lock: another process may have refreshed, logged in
	// again, or logged out since this command started.
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return "", err
	}
	p, ok := cfg.Profiles[s.Profile]
	if !ok || p.Auth != config.AuthOAuth {
		return "", fmt.Errorf("profile %q is no longer logged in with OAuth; run bwt auth login", s.Profile)
	}
	backend := secrets.Backend(p.TokenStore)
	if raw, err := s.Store.Load(backend, SecretKey(s.Profile, "access")); err == nil {
		var c cachedToken
		if json.Unmarshal([]byte(raw), &c) == nil && c.valid() {
			s.cached = &c
			return c.AccessToken, nil
		}
	}
	raw, err := s.Store.Load(backend, SecretKey(s.Profile, "oauth"))
	if errors.Is(err, secrets.ErrNotFound) {
		return "", fmt.Errorf("no saved OAuth login for profile %q; run bwt auth login", s.Profile)
	}
	if err != nil {
		return "", err
	}
	stored, err := decode(raw)
	if err != nil {
		return "", err
	}
	id, secret := p.ClientID, stored.ClientSecret
	if p.Client != config.ClientCustom {
		builtinID, builtinSecret, ok := Builtin()
		if !ok || builtinID != p.ClientID {
			return "", fmt.Errorf("profile %q was authorized with a different OAuth client than this build provides; run bwt auth login", s.Profile)
		}
		secret = builtinSecret
	}
	if id == "" || secret == "" {
		return "", fmt.Errorf("the OAuth client for profile %q is incomplete; run bwt auth login", s.Profile)
	}
	conf := oauthConfig(id, secret, "", p.Scope)
	tok, err := conf.TokenSource(withHTTP(ctx, s.HTTP), &oauth2.Token{RefreshToken: stored.RefreshToken}).Token()
	if err != nil {
		return "", describe("refresh the access token", err)
	}
	fresh := &cachedToken{AccessToken: tok.AccessToken, Expiry: tok.Expiry}
	// Under --read-only the refreshed token stays in memory only.
	if !s.ReadOnly {
		if tok.RefreshToken != "" && tok.RefreshToken != stored.RefreshToken {
			stored.RefreshToken = tok.RefreshToken
			payload, err := Encode(stored)
			if err != nil {
				return "", err
			}
			if _, err := s.Store.Save(ctx, SecretKey(s.Profile, "oauth"), payload, backend, false); err != nil {
				return "", fmt.Errorf("Bing rotated the refresh token but it could not be saved: %w", err)
			}
		}
		// The access-token cache is an optimization; failing to write it only
		// means the next command refreshes again.
		if payload, err := Encode(fresh); err == nil {
			_, _ = s.Store.Save(ctx, SecretKey(s.Profile, "access"), payload, backend, false)
		}
	}
	s.cached = fresh
	return fresh.AccessToken, nil
}
