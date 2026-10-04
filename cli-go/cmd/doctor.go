package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/auth"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/secrets"
	"github.com/spf13/cobra"
)

func (a *app) doctorCmd() *cobra.Command {
	var verify bool
	c := &cobra.Command{Use: "doctor", Short: "Check configuration, credentials, site selection, and PATH", Args: cobra.NoArgs,
		Long: "Runs local checks without network access; --verify adds one GetUserSites call. Exits nonzero when a check fails."}
	c.Flags().BoolVar(&verify, "verify", false, "Also check access with one GetUserSites call")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		var rows []map[string]any
		add := func(check, status, detail string) {
			rows = append(rows, map[string]any{"check": check, "status": status, "detail": detail})
		}
		cfg, path, err := a.load()
		if err != nil {
			add("config", "fail", err.Error())
		} else if info, err := os.Stat(path); err != nil {
			add("config", "warn", path+" does not exist yet; run bwt auth login")
		} else if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			add("config", "warn", fmt.Sprintf("%s is mode %v; expected 0600", path, info.Mode().Perm()))
		} else {
			add("config", "ok", path)
		}
		cr, credErr := a.resolve()
		if credErr != nil {
			add("credentials", "fail", credErr.Error())
		} else {
			add("credentials", "ok", cr.kind+" from "+cr.source)
			if cr.profile != "" && cfg != nil {
				p := cfg.Profiles[cr.profile]
				store, err := a.store()
				if err == nil {
					_, err = store.Load(secrets.Backend(p.TokenStore), auth.SecretKey(cr.profile, "api_key"))
				}
				switch {
				case err == nil:
					add("secret storage", "ok", "readable from "+p.TokenStore)
				case errors.Is(err, secrets.ErrNotFound):
					add("secret storage", "fail", "saved secret missing; run bwt auth login")
				default:
					add("secret storage", "fail", err.Error())
				}
				if p.TokenStore == string(secrets.File) {
					add("secret storage", "warn", "secrets are in a plaintext 0600 file (--insecure-storage)")
				}
			}
		}
		if cfg != nil {
			if site := a.defaultSiteValue(cfg); site != "" {
				add("site", "ok", site)
			} else {
				add("site", "warn", "no default site; run bwt sites use SITE")
			}
		}
		add(pathCheck())
		if verify && credErr == nil {
			sites, err := a.clientFor(cr).Call(cmd.Context(), "GetUserSites", map[string]any{})
			if err != nil {
				add("bing access", "fail", err.Error())
			} else {
				add("bing access", "ok", fmt.Sprintf("%d sites", len(asList(sites))))
			}
		}
		if err := a.print(rows, cols("check", "check", "status", "status", "detail", "detail")...); err != nil {
			return err
		}
		for _, r := range rows {
			if r["status"] == "fail" {
				return errors.New("one or more checks failed")
			}
		}
		return nil
	}
	return c
}

// pathCheck finds every bwt executable on PATH; another tool (Bitcoin Wallet
// Tracker) also installs a bwt binary.
func pathCheck() (string, string, string) {
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	name := "bwt"
	if runtime.GOOS == "windows" {
		name = "bwt.exe"
	}
	var found []string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		real, _ := filepath.EvalSymlinks(p)
		if !seen[real] {
			seen[real] = true
			found = append(found, p)
		}
	}
	switch {
	case len(found) == 0:
		return "path", "warn", "bwt is not on PATH"
	case len(found) > 1:
		return "path", "warn", "several bwt executables on PATH (another tool also uses the name): " + strings.Join(found, ", ")
	default:
		if real, _ := filepath.EvalSymlinks(found[0]); self != "" && real != self {
			return "path", "warn", "the bwt on PATH (" + found[0] + ") is not this executable"
		}
		return "path", "ok", found[0]
	}
}
