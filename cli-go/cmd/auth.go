package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// webmasterHome is where the API key lives: Settings > API Access > API Key.
const webmasterHome = "https://www.bing.com/webmasters"

const revokeNote = "Logging out removes the key from this machine only. To invalidate it, regenerate it in Bing Webmaster Tools > Settings > API Access."

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

// terminal reports whether input is an interactive terminal.
func (a *app) terminal() bool {
	f, ok := a.in.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// readSecret reads one secret from a hidden prompt, or from stdin when it is not a terminal.
func (a *app) readSecret(prompt string) (string, error) {
	if a.terminal() {
		if a.noInput {
			return "", fmt.Errorf("%s needs input; pipe it on stdin with --no-input", strings.ToLower(prompt))
		}
		fd := int(a.in.(*os.File).Fd())
		fmt.Fprint(a.errOut, prompt+": ")
		b, err := term.ReadPassword(fd)
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

// showAPIKeySteps tells an interactive user where the key is and opens Bing
// Webmaster Tools; the URL is printed too in case no browser opens.
func (a *app) showAPIKeySteps() {
	fmt.Fprintf(a.errOut, "Log in with your Bing Webmaster Tools API key (one key covers all your sites):\n"+
		"  1. Opening %s\n"+
		"  2. Go to Settings (gear icon) > API Access > API Key, then Generate or copy your key.\n"+
		"  3. Paste it below. Input is hidden.\n", webmasterHome)
	open := a.openBrowser
	if open == nil {
		open = openURL
	}
	_ = open(webmasterHome)
}

// plural renders "1 site" or "3 sites".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func openURL(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}

func (a *app) loginCmd() *cobra.Command {
	var withAPIKey, insecure, noVerify bool
	c := &cobra.Command{Use: "login", Short: "Save your Bing Webmaster API key and pick a default site", Args: cobra.NoArgs,
		Annotations: map[string]string{"writes-local": "true", "interactive": "true"},
		Long: "Logs in with your Bing Webmaster Tools API key, which covers every site in your account. In a terminal it\n" +
			"opens Bing Webmaster Tools, shows where the key is (Settings > API Access > API Key), and reads it from a\n" +
			"hidden prompt; piped input is read from stdin. The key is checked with one GetUserSites call before anything\n" +
			"is saved, then stored in the OS keychain, and a default site is chosen. Without an OS keychain, login stops\n" +
			"unless you pass --insecure-storage (a 0600 plaintext file). Bing's OAuth registration rejects loopback\n" +
			"redirect URIs, so there is no browser OAuth login.",
		Example: "  bwt auth login\n  bwt auth login --profile client-b\n  printf '%s' \"$KEY\" | bwt auth login --no-input --profile ci\n" +
			"  BWT_API_KEY=... bwt sites list    # use a key without saving it"}
	c.Flags().BoolVar(&withAPIKey, "with-api-key", false, "Accepted for compatibility; login always uses an API key")
	_ = c.Flags().MarkHidden("with-api-key")
	c.Flags().BoolVar(&insecure, "insecure-storage", false, "Store the key in a 0600 plaintext file instead of the OS keychain")
	c.Flags().BoolVar(&noVerify, "no-verify", false, "Save without checking the key with Bing (for a key that is not active yet)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
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
		if a.terminal() && !a.noInput {
			a.showAPIKeySteps()
		}
		key, err := a.readSecret("Bing Webmaster API key")
		if err != nil {
			return err
		}
		if key == "" || strings.ContainsAny(key, " \t\r\n") {
			return fmt.Errorf("the API key must be nonempty and contain no whitespace")
		}
		// Check the key before saving anything, so a typo never lands in a profile.
		var sites []string
		if !noVerify {
			data, err := a.clientFor(&creds{kind: "api_key", apiKey: key}).Call(ctx, "GetUserSites", map[string]any{})
			if err != nil {
				return fmt.Errorf("Bing did not accept this key, so nothing was saved: %w\n"+
					"A newly generated key can take about 30 minutes to start working; try again later, or pass --no-verify to save it now", err)
			}
			for _, s := range asList(data) {
				if u, ok := s["Url"].(string); ok {
					sites = append(sites, u)
				}
			}
			sort.Strings(sites)
		}
		// Choose a default site before taking the lock, since it may prompt.
		chosen, picked := cfg.Profiles[name].Site, false
		switch {
		case noVerify, chosen != "" && slices.Contains(sites, chosen):
		case len(sites) == 1:
			chosen = sites[0]
		case len(sites) > 1:
			chosen = a.pick(sites)
			picked = chosen != ""
		default:
			chosen = ""
		}
		prefer := secrets.Keychain
		if insecure {
			prefer = secrets.File
		}
		unlock, err := auth.LockProfile(ctx, path, name)
		if err != nil {
			return err
		}
		defer unlock()
		// Re-read under the lock: another process may have changed this profile
		// (its storage backend, site, or IndexNow keys) meanwhile.
		fresh, err := config.Load(path)
		if err != nil {
			return err
		}
		previous := fresh.Profiles[name]
		saved, err := store.Save(ctx, auth.SecretKey(name, "api_key"), key, prefer, false)
		if err != nil {
			return fmt.Errorf("%w\nNo OS keychain is available. Rerun with --insecure-storage to keep the key in a 0600 file, or set BWT_API_KEY without saving", err)
		}
		// Delete what the profile held before: v0.1.1 OAuth tokens, and the old key
		// when the backend changed. The new key is saved, so a failure only warns.
		backends := []secrets.Backend{saved.Backend}
		if old := secrets.Backend(previous.TokenStore); old != "" && old != saved.Backend {
			backends = append(backends, old)
		}
		var leftovers []string
		for _, b := range backends {
			kinds := auth.LegacyKinds
			if b != saved.Backend {
				kinds = append([]string{"api_key"}, kinds...)
			}
			for _, kind := range kinds {
				if err := store.Delete(ctx, b, auth.SecretKey(name, kind)); err != nil {
					leftovers = append(leftovers, fmt.Sprintf("%s (%s): %v", auth.SecretKey(name, kind), b, err))
				}
			}
		}
		if len(leftovers) > 0 {
			fmt.Fprintf(a.errOut, "Could not delete old credentials, so remove them by hand: %s\n", strings.Join(leftovers, "; "))
		}
		switch {
		case noVerify:
			chosen = previous.Site // unchecked key: keep whatever site the profile has now
		case !picked && previous.Site != "" && slices.Contains(sites, previous.Site):
			chosen = previous.Site // another process may have run `bwt sites use` meanwhile
		}
		profile := config.Profile{Auth: config.AuthAPIKey, TokenStore: string(saved.Backend), Site: chosen, IndexNow: previous.IndexNow}
		if err := config.Update(ctx, path, func(c *config.Config) error {
			c.Profiles[name] = profile
			c.CurrentProfile = name
			return nil
		}); err != nil {
			return err
		}
		result := map[string]any{"profile": name, "auth": profile.Auth, "token_store": profile.TokenStore, "saved": true, "verified": !noVerify}
		if !noVerify {
			result["sites"] = len(sites)
		}
		if chosen != "" {
			result["site"] = chosen
		}
		switch {
		case noVerify:
			a.info("Saved the key for profile %q without checking it.", name)
		case chosen != "":
			a.info("Logged in (profile %q). Found %s; default site is %s (change with: bwt sites use SITE).", name, plural(len(sites), "site"), chosen)
		default:
			a.info("Logged in (profile %q). Found %s; choose a default with: bwt sites use SITE", name, plural(len(sites), "site"))
		}
		return a.print(result)
	}
	return c
}

// pick shows a numbered list on stderr; it returns "" without a terminal.
func (a *app) pick(options []string) string {
	if a.noInput || !a.terminal() {
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
			result["profile"], result["token_store"] = cr.profile, cfg.Profiles[cr.profile].TokenStore
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
	return &cobra.Command{Use: "logout", Short: "Delete the profile's saved API key (keeps its site and IndexNow keys)", Args: cobra.NoArgs,
		Annotations: map[string]string{"writes-local": "true"},
		Long:        "Deletes the saved API key from the keychain or secrets file. " + revokeNote,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := a.load()
			if err != nil {
				return err
			}
			name := cfg.ProfileName(a.profile)
			if _, ok := cfg.Profiles[name]; name == "" || !ok {
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
			// Re-read under the lock so we delete from the backend that holds the key now.
			if cfg, err = config.Load(path); err != nil {
				return err
			}
			p, ok := cfg.Profiles[name]
			if !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			if p.TokenStore != "" {
				// Also delete v0.1.1 OAuth tokens. Fail before forgetting the backend,
				// or nothing would know where the leftovers live.
				for _, kind := range append([]string{"api_key"}, auth.LegacyKinds...) {
					if err := store.Delete(cmd.Context(), secrets.Backend(p.TokenStore), auth.SecretKey(name, kind)); err != nil {
						return fmt.Errorf("could not delete the saved %s credential: %w", kind, err)
					}
				}
			}
			if err := config.Update(cmd.Context(), path, func(c *config.Config) error {
				q := c.Profiles[name]
				q.Auth, q.TokenStore = "", ""
				c.Profiles[name] = q
				return nil
			}); err != nil {
				return err
			}
			a.info(revokeNote)
			return a.print(map[string]any{"profile": name, "logged_out": true})
		}}
}
