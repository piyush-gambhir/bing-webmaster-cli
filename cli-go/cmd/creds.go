package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
	"golang.org/x/term"
)

const keychainService = "bing-webmaster-cli"

type creds struct {
	kind    string
	source  string
	profile string
	apiKey  string
	token   func(context.Context) (string, error)
}

func (a *app) load() (*config.Config, string, error) {
	path, err := config.Path()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func (a *app) store() (secrets.Store, error) {
	path, err := config.SecretsPath()
	if err != nil {
		return secrets.Store{}, err
	}
	cfgPath, err := config.Path()
	if err != nil {
		return secrets.Store{}, err
	}
	def, _ := config.DefaultPath()
	return secrets.Store{Service: secrets.ServiceFor(keychainService, cfgPath, def), FilePath: path}, nil
}

func (a *app) httpClient() *http.Client {
	h := &http.Client{Timeout: a.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if a.transport != nil {
		h.Transport = a.transport
	}
	return h
}

func (a *app) webClient() *http.Client {
	h := &http.Client{Timeout: a.timeout}
	if a.transport != nil {
		h.Transport = a.transport
	}
	return h
}

func (a *app) resolve() (*creds, error) {
	if a.accessToken != "" {
		tok := strings.TrimSpace(a.accessToken)
		return &creds{kind: "access_token", source: "flag", token: func(context.Context) (string, error) { return tok, nil }}, nil
	}
	if env := strings.TrimSpace(os.Getenv("BWT_ACCESS_TOKEN")); env != "" {
		return &creds{kind: "access_token", source: "environment", token: func(context.Context) (string, error) { return env, nil }}, nil
	}
	if a.apiKey != "" {
		return &creds{kind: "api_key", source: "flag", apiKey: strings.TrimSpace(a.apiKey)}, nil
	}
	if env := strings.TrimSpace(os.Getenv("BWT_API_KEY")); env != "" {
		return &creds{kind: "api_key", source: "environment", apiKey: env}, nil
	}
	cfg, path, err := a.load()
	if err != nil {
		return nil, err
	}
	name := cfg.ProfileName(a.profile)
	if name == "" {
		return nil, errors.New("no Bing credentials; run bwt auth login (or set BWT_API_KEY)")
	}
	return a.profileCreds(cfg, path, name)
}

func (a *app) profileCreds(cfg *config.Config, path, name string) (*creds, error) {
	p, ok := cfg.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found; run bwt auth login --profile %s", name, name)
	}
	store, err := a.store()
	if err != nil {
		return nil, err
	}
	switch p.Auth {
	case config.AuthAPIKey:
		key, err := store.Load(secrets.Backend(p.TokenStore), auth.SecretKey(name, "api_key"))
		if errors.Is(err, secrets.ErrNotFound) {
			return nil, fmt.Errorf("profile %q has no saved API key; run bwt auth login --profile %s", name, name)
		}
		if err != nil {
			return nil, err
		}
		return &creds{kind: "api_key", source: "profile:" + name, profile: name, apiKey: key}, nil
	case "oauth":
		return nil, fmt.Errorf("profile %q used browser login, which bwt no longer supports; run bwt auth login --profile %s with your API key", name, name)
	default:
		return nil, fmt.Errorf("profile %q has no Bing credentials; run bwt auth login --profile %s", name, name)
	}
}

func bearerBase() string {
	if os.Getenv("BWT_OAUTH_API_HOST") == "ssl.bing.com" {
		return client.KeyBase
	}
	return client.BearerBase
}

func (a *app) api() (*client.Client, error) {
	cr, err := a.resolve()
	if err != nil {
		return nil, err
	}
	return a.clientFor(cr), nil
}

func (a *app) clientFor(cr *creds) *client.Client {
	c := client.New(a.timeout)
	if a.transport != nil {
		c.HTTP.Transport = a.transport
	}
	c.ReadOnly, c.DryRun = a.readOnly, a.dryRun
	if a.verbose {
		c.Log = a.errOut
	}
	if cr.kind == "api_key" {
		c.Base, c.APIKey = client.KeyBase, cr.apiKey
	} else {
		c.Base, c.Token = bearerBase(), cr.token
	}
	return c
}

func (a *app) siteURL(ctx context.Context, c *client.Client) (string, error) {
	s := strings.TrimSpace(a.site)
	if s == "" {
		s = strings.TrimSpace(os.Getenv("BWT_SITE"))
	}
	if s == "" {
		if cfg, _, err := a.load(); err == nil {
			if p, ok := cfg.Profiles[cfg.ProfileName(a.profile)]; ok {
				s = p.Site
			}
		}
	}
	if s == "" {
		return "", errors.New("no site selected; pass --site, set BWT_SITE, or run bwt sites use SITE")
	}
	if strings.Contains(s, "://") {
		return s, nil
	}
	return a.matchSite(ctx, c, s)
}

func (a *app) matchSite(ctx context.Context, c *client.Client, host string) (string, error) {
	data, err := c.Call(ctx, "GetUserSites", map[string]any{})
	if err != nil {
		return "", err
	}
	host = strings.ToLower(strings.TrimSuffix(host, "/"))
	var matches []string
	for _, item := range asList(data) {
		siteURL, _ := item["Url"].(string)
		u, err := url.Parse(siteURL)
		if err != nil {
			continue
		}
		h := strings.ToLower(u.Host)
		if h == host || h == "www."+host {
			matches = append(matches, siteURL)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no site in your account matches %q; see bwt sites list", host)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q matches several sites (%s); pass the exact URL", host, strings.Join(matches, ", "))
	}
}

func (a *app) confirm(prompt string) error {
	if a.yes || a.dryRun {
		return nil
	}
	f, ok := a.in.(*os.File)
	if a.noInput || !ok || !term.IsTerminal(int(f.Fd())) {
		return fmt.Errorf("%s; rerun with --yes to confirm", prompt)
	}
	fmt.Fprintf(a.errOut, "%s. Continue? [y/N] ", prompt)
	line, _ := bufio.NewReader(a.in).ReadString('\n')
	if answer := strings.ToLower(strings.TrimSpace(line)); answer != "y" && answer != "yes" {
		return errors.New("cancelled")
	}
	return nil
}

func asList(v any) []map[string]any {
	items, _ := v.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
