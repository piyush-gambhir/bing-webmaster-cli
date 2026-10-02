package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const revokeNote = "Bing has no token revocation endpoint. To revoke access, remove the app or regenerate the API key under Bing Webmaster Tools > Settings > API Access."

func (a *app) authCmd() *cobra.Command {
	c := &cobra.Command{Use: "auth", Short: "Log in, inspect credentials, and manage profiles"}
	c.AddCommand(a.loginCmd(), a.statusCmd(), a.listProfilesCmd(), a.useProfileCmd(), a.logoutCmd())
	return c
}

func (a *app) configCmd() *cobra.Command {
	c := &cobra.Command{Use: "config", Short: "Inspect configuration and select profiles"}
	show := a.statusCmd()
	show.Use, show.Short = "show", "Show the active credential source without revealing secrets"
	list := a.listProfilesCmd()
	list.Use, list.Aliases = "list-profiles", nil
	use := a.useProfileCmd()
	use.Use, use.Aliases = "use-profile NAME", nil
	c.AddCommand(show, list, use)
	return c
}

func (a *app) profileName(cfg *config.Config) (string, error) {
	name := a.profile
	if name == "" {
		name = os.Getenv("BWT_PROFILE")
	}
	if name == "" {
		name = cfg.CurrentProfile
	}
	if name == "" {
		name = "default"
	}
	return name, config.ValidName(name)
}

// readSecret reads one secret from a hidden prompt, or from stdin when it is not a terminal.
func (a *app) readSecret(prompt string) (string, error) {
	f, isFile := a.in.(*os.File)
	if isFile && term.IsTerminal(int(f.Fd())) {
		if a.noInput {
			return "", fmt.Errorf("%s needs input; pipe it on stdin with --no-input", strings.ToLower(prompt))
		}
		fmt.Fprint(a.errOut, prompt+": ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.errOut)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	b, err := io.ReadAll(io.LimitReader(a.in, 65537))
	if err != nil {
		return "", err
	}
	if len(b) > 65536 {
		return "", fmt.Errorf("input exceeds 64 KiB")
	}
	return strings.TrimSpace(string(b)), nil
}

func (a *app) loginCmd() *cobra.Command {
	var withAPIKey, insecure, noVerify, secretStdin bool
	var scope, clientID string
	var port int
	c := &cobra.Command{Use: "login", Short: "Log in with your browser (or an API key) and pick a default site", Args: cobra.NoArgs,
		Annotations: map[string]string{"writes-local": "true", "interactive": "true"},
		Long: "Opens Bing's consent page in your browser using the built-in OAuth client, saves the login in the OS\n" +
			"keychain, and sets a default site. --with-api-key saves an API key instead (Bing Webmaster Tools >\n" +
			"Settings > API Access > API Key), read from a hidden prompt or from stdin when piped.\n" +
			"Without an OS keychain, login stops unless you pass --insecure-storage (a 0600 plaintext file).",
		Example: "  bwt auth login\n  bwt auth login --with-api-key\n  printf '%s' \"$KEY\" | bwt auth login --with-api-key --no-input --profile ci\n" +
			"  bwt auth login --client-id ID --client-secret-stdin --redirect-port 8400 < secret.txt"}
	c.Flags().BoolVar(&withAPIKey, "with-api-key", false, "Save an API key instead of using the browser")
	c.Flags().BoolVar(&insecure, "insecure-storage", false, "Store secrets in a 0600 plaintext file instead of the OS keychain")
	c.Flags().BoolVar(&noVerify, "no-verify", false, "Skip the GetUserSites check after saving")
	c.Flags().StringVar(&scope, "scope", "manage", "OAuth scope: manage (read and write) or read")
	c.Flags().StringVar(&clientID, "client-id", "", "Use your own OAuth client instead of the built-in one")
	c.Flags().BoolVar(&secretStdin, "client-secret-stdin", false, "Read your OAuth client secret from stdin (with --client-id)")
	c.Flags().IntVar(&port, "redirect-port", 0, "Loopback port registered with your own client (with --client-id)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, path, err := a.load()
		if err != nil {
			return err
		}
		name, err := a.profileName(cfg)
		if err != nil {
			return err
		}
		store, err := a.store()
		if err != nil {
			return err
		}
		prefer := secrets.Keychain
		if insecure {
			prefer = secrets.File
		}
		save := func(kind, value string) (secrets.Backend, error) {
			saved, err := store.Save(cmd.Context(), auth.SecretKey(name, kind), value, prefer, false)
			if err != nil {
				return "", fmt.Errorf("%w\nNo OS keychain is available. Rerun with --insecure-storage to keep the secret in a 0600 file, or use BWT_API_KEY / BWT_ACCESS_TOKEN without saving", err)
			}
			return saved.Backend, nil
		}
		profile := cfg.Profiles[name]
		previousStore := secrets.Backend(profile.TokenStore)
		var backend secrets.Backend
		// Persistence runs under the refresh lock (taken after any browser step,
		// so a slow login never blocks other commands).
		var unlock func()
		lock := func() error {
			var err error
			if unlock, err = auth.LockProfile(cmd.Context(), path, name); err != nil {
				return err
			}
			// Re-read under the lock: another process may have changed this profile
			// (its storage backend, site, or IndexNow keys) while we waited.
			fresh, err := config.Load(path)
			if err != nil {
				return err
			}
			profile = fresh.Profiles[name]
			previousStore = secrets.Backend(profile.TokenStore)
			return nil
		}
		defer func() {
			if unlock != nil {
				unlock()
			}
		}()
		if withAPIKey {
			if clientID != "" || secretStdin || port != 0 {
				return fmt.Errorf("--client-id, --client-secret-stdin, and --redirect-port apply to browser login, not --with-api-key")
			}
			key, err := a.readSecret("Bing Webmaster API key")
			if err != nil {
				return err
			}
			if key == "" || strings.ContainsAny(key, " \t\r\n") {
				return fmt.Errorf("the API key must be nonempty and contain no whitespace")
			}
			if err := lock(); err != nil {
				return err
			}
			if backend, err = save("api_key", key); err != nil {
				return err
			}
			profile = config.Profile{Auth: config.AuthAPIKey, TokenStore: string(backend), Site: profile.Site, IndexNow: profile.IndexNow}
		} else {
			if a.noInput {
				return fmt.Errorf("browser login needs interaction; use --with-api-key with the key on stdin, or set BWT_API_KEY")
			}
			oauthScope := auth.ScopeManage
			switch scope {
			case "manage":
			case "read":
				oauthScope = auth.ScopeRead
			default:
				return fmt.Errorf("--scope must be manage or read")
			}
			id, secret, client := "", "", config.ClientBuiltin
			loginPort := auth.RedirectPort
			if clientID != "" {
				if !secretStdin {
					return fmt.Errorf("--client-id needs --client-secret-stdin (the secret is never accepted as a flag)")
				}
				if port < 1 || port > 65535 {
					return fmt.Errorf("--redirect-port is required with --client-id and must match the redirect URI you registered")
				}
				if secret, err = a.readSecret("OAuth client secret"); err != nil || secret == "" {
					return fmt.Errorf("could not read the client secret from stdin")
				}
				id, client, loginPort = clientID, config.ClientCustom, port
			} else {
				if secretStdin || port != 0 {
					return fmt.Errorf("--client-secret-stdin and --redirect-port apply only with --client-id")
				}
				var ok bool
				if id, secret, ok = auth.Builtin(); !ok {
					return fmt.Errorf("this build has no built-in OAuth client; use bwt auth login --with-api-key, set BWT_CLIENT_ID and BWT_CLIENT_SECRET, or pass --client-id")
				}
			}
			tok, err := auth.Login(cmd.Context(), auth.LoginOptions{ClientID: id, ClientSecret: secret, Scope: oauthScope, Port: loginPort,
				OpenBrowser: a.openBrowser, Log: a.errOut, HTTP: a.httpClient()})
			if err != nil {
				if strings.Contains(err.Error(), "cannot listen") {
					return fmt.Errorf("%w\nThe registered callback port is busy. Close the other login, or use bwt auth login --with-api-key", err)
				}
				return err
			}
			if err := lock(); err != nil {
				return err
			}
			stored := auth.Stored{RefreshToken: tok.RefreshToken}
			if client == config.ClientCustom {
				stored.ClientSecret = secret
			}
			payload, err := auth.Encode(stored)
			if err != nil {
				return err
			}
			// Clear any cached access token from a previous login before saving the
			// new credential: if clearing fails nothing has changed, and a failed
			// cache write below forces a refresh instead of reusing the old account.
			for _, b := range []secrets.Backend{previousStore, prefer} {
				if b != "" {
					if err := store.Delete(cmd.Context(), b, auth.SecretKey(name, "access")); err != nil {
						return fmt.Errorf("could not clear the previous cached access token: %w", err)
					}
				}
			}
			if backend, err = save("oauth", payload); err != nil {
				return err
			}
			if access, err := auth.Encode(map[string]any{"access_token": tok.AccessToken, "expiry": tok.Expiry}); err == nil {
				_, _ = store.Save(cmd.Context(), auth.SecretKey(name, "access"), access, backend, false)
			}
			profile = config.Profile{Auth: config.AuthOAuth, Client: client, ClientID: id, Scope: oauthScope, TokenStore: string(backend),
				Site: profile.Site, IndexNow: profile.IndexNow}
			if client == config.ClientCustom {
				profile.RedirectPort = loginPort
			}
		}
		// Remove the other credential kind so a profile never mixes them.
		stale := []string{"oauth", "access"}
		if !withAPIKey {
			stale = []string{"api_key"}
		}
		for _, kind := range stale {
			for _, b := range []secrets.Backend{previousStore, backend} {
				if b != "" {
					_ = store.Delete(cmd.Context(), b, auth.SecretKey(name, kind))
				}
			}
		}
		if previousStore != "" && previousStore != backend {
			for _, kind := range []string{"oauth", "access", "api_key"} {
				_ = store.Delete(cmd.Context(), previousStore, auth.SecretKey(name, kind))
			}
		}
		if err := config.Update(cmd.Context(), path, func(c *config.Config) error {
			c.Profiles[name] = profile
			c.CurrentProfile = name
			return nil
		}); err != nil {
			return err
		}
		// Release before the first API call, which refreshes under the same lock.
		unlock()
		unlock = nil
		result := map[string]any{"profile": name, "auth": profile.Auth, "token_store": profile.TokenStore, "saved": true}
		if noVerify {
			return a.print(result)
		}
		site, count, err := a.pickDefaultSite(cmd.Context(), name, path)
		if err != nil {
			result["verified"] = false
			_ = a.print(result)
			if withAPIKey {
				return fmt.Errorf("saved, but Bing rejected the key (a new key can take about 30 minutes to activate): %w", err)
			}
			return fmt.Errorf("saved, but the first call to Bing failed: %w", err)
		}
		result["verified"], result["sites"] = true, count
		if site != "" {
			result["site"] = site
		}
		return a.print(result)
	}
	return c
}

// pickDefaultSite lists the account's sites and sets the profile's default site
// when it is unambiguous or the user picks one interactively.
func (a *app) pickDefaultSite(ctx context.Context, name, path string) (string, int, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return "", 0, err
	}
	cr, err := a.profileCreds(cfg, path, name)
	if err != nil {
		return "", 0, err
	}
	data, err := a.clientFor(cr).Call(ctx, "GetUserSites", map[string]any{})
	if err != nil {
		return "", 0, err
	}
	var urls []string
	for _, s := range asList(data) {
		if u, ok := s["Url"].(string); ok {
			urls = append(urls, u)
		}
	}
	sort.Strings(urls)
	current := cfg.Profiles[name].Site
	chosen := ""
	switch {
	case current != "":
		a.info("Found %d sites. Default site stays %s.", len(urls), current)
		return current, len(urls), nil
	case len(urls) == 1:
		chosen = urls[0]
	case len(urls) > 1:
		chosen = a.pick(urls)
	}
	if chosen == "" {
		a.info("Found %d sites. Choose a default with: bwt sites use SITE", len(urls))
		return "", len(urls), nil
	}
	if err := config.Update(ctx, path, func(c *config.Config) error {
		p := c.Profiles[name]
		p.Site = chosen
		c.Profiles[name] = p
		return nil
	}); err != nil {
		return "", len(urls), err
	}
	a.info("Found %d sites. Default site set to %s (change with: bwt sites use SITE).", len(urls), chosen)
	return chosen, len(urls), nil
}

// pick shows a numbered list on stderr; it returns "" without a terminal.
func (a *app) pick(options []string) string {
	f, ok := a.in.(*os.File)
	if a.noInput || !ok || !term.IsTerminal(int(f.Fd())) {
		return ""
	}
	for i, o := range options {
		fmt.Fprintf(a.errOut, "  %d) %s\n", i+1, o)
	}
	fmt.Fprint(a.errOut, "Default site number (Enter to skip): ")
	line, _ := bufio.NewReader(a.in).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(options) {
		return ""
	}
	return options[n-1]
}

func (a *app) statusCmd() *cobra.Command {
	var verify bool
	c := &cobra.Command{Use: "status", Short: "Show the active credential source without revealing secrets", Args: cobra.NoArgs,
		Long: "Local only unless --verify, which makes one GetUserSites call."}
	c.Flags().BoolVar(&verify, "verify", false, "Check access with one GetUserSites call")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cr, err := a.resolve()
		if err != nil {
			return err
		}
		cfg, path, err := a.load()
		if err != nil {
			return err
		}
		result := map[string]any{"configured": true, "auth": cr.kind, "source": cr.source, "config_file": path}
		if cr.profile != "" {
			p := cfg.Profiles[cr.profile]
			result["profile"], result["token_store"] = cr.profile, p.TokenStore
			if p.Auth == config.AuthOAuth {
				result["client"], result["scope"] = p.Client, p.Scope
			}
		}
		if site := a.defaultSiteValue(cfg); site != "" {
			result["site"] = site
		}
		if verify {
			sites, err := a.clientFor(cr).Call(cmd.Context(), "GetUserSites", map[string]any{})
			if err != nil {
				return err
			}
			result["verified"], result["sites"] = true, len(asList(sites))
		}
		return a.print(result)
	}
	return c
}

func (a *app) defaultSiteValue(cfg *config.Config) string {
	if a.site != "" {
		return a.site
	}
	if env := os.Getenv("BWT_SITE"); env != "" {
		return env
	}
	return cfg.Profiles[cfg.ProfileName(a.profile)].Site
}

func (a *app) listProfilesCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Aliases: []string{"list-profiles"}, Short: "List saved profiles without secrets", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.load()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Profiles))
			for n := range cfg.Profiles {
				names = append(names, n)
			}
			sort.Strings(names)
			rows := make([]map[string]any, 0, len(names))
			for _, n := range names {
				p := cfg.Profiles[n]
				hosts := make([]string, 0, len(p.IndexNow))
				for h := range p.IndexNow {
					hosts = append(hosts, h)
				}
				sort.Strings(hosts)
				rows = append(rows, map[string]any{"profile": n, "current": n == cfg.CurrentProfile, "auth": p.Auth, "token_store": p.TokenStore,
					"site": p.Site, "indexnow_hosts": strings.Join(hosts, ",")})
			}
			return a.print(rows, cols("profile", "profile", "current", "current", "auth", "auth", "site", "site", "token_store", "token_store", "indexnow_hosts", "indexnow_hosts")...)
		}}
}

func (a *app) useProfileCmd() *cobra.Command {
	return &cobra.Command{Use: "use NAME", Aliases: []string{"use-profile"}, Short: "Set the default profile", Args: cobra.ExactArgs(1),
		Annotations: map[string]string{"writes-local": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			if err := config.Update(cmd.Context(), path, func(c *config.Config) error {
				if _, ok := c.Profiles[args[0]]; !ok {
					return fmt.Errorf("profile %q not found", args[0])
				}
				c.CurrentProfile = args[0]
				return nil
			}); err != nil {
				return err
			}
			return a.print(map[string]string{"current_profile": args[0]})
		}}
}

func (a *app) logoutCmd() *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "Delete the profile's saved credentials (keeps its site and IndexNow keys)", Args: cobra.NoArgs,
		Annotations: map[string]string{"writes-local": "true"},
		Long:        "Deletes the saved OAuth login or API key from the keychain or secrets file. " + revokeNote,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := a.load()
			if err != nil {
				return err
			}
			name := cfg.ProfileName(a.profile)
			p, ok := cfg.Profiles[name]
			if name == "" || !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			store, err := a.store()
			if err != nil {
				return err
			}
			unlock, err := auth.LockProfile(cmd.Context(), path, name)
			if err != nil {
				return err
			}
			defer unlock()
			// Re-read under the lock so we delete from the backend that holds the
			// credentials now, not the one seen before waiting.
			if cfg, err = config.Load(path); err != nil {
				return err
			}
			if p, ok = cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			var failures []string
			if p.TokenStore != "" {
				for _, kind := range []string{"oauth", "access", "api_key"} {
					if err := store.Delete(cmd.Context(), secrets.Backend(p.TokenStore), auth.SecretKey(name, kind)); err != nil {
						failures = append(failures, err.Error())
					}
				}
			}
			if len(failures) > 0 {
				return errors.New("could not delete saved credentials: " + strings.Join(failures, "; "))
			}
			if err := config.Update(cmd.Context(), path, func(c *config.Config) error {
				q := c.Profiles[name]
				q.Auth, q.Client, q.ClientID, q.RedirectPort, q.Scope, q.TokenStore = "", "", "", 0, "", ""
				c.Profiles[name] = q
				return nil
			}); err != nil {
				return err
			}
			a.info(revokeNote)
			return a.print(map[string]any{"profile": name, "logged_out": true})
		}}
}
