package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/indexnow"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) indexnowCmd() *cobra.Command {
	c := &cobra.Command{Use: "indexnow", Short: "Notify Bing and other engines of new or changed URLs with IndexNow",
		Long: "IndexNow needs no Bing login: you host a key file on your site and submit URLs with the key.\n" +
			"A submission to one participating endpoint is shared with the other IndexNow engines.\n" +
			"Keys are public by design (served at https://host/KEY.txt), so the CLI keeps them in the config file."}
	key := &cobra.Command{Use: "key", Short: "Create and check the IndexNow key for a host"}
	key.AddCommand(a.indexnowGenerateCmd(), a.indexnowCheckCmd())
	c.AddCommand(key, a.indexnowSubmitCmd())
	return c
}

// defaultHost derives a host from --site, BWT_SITE, or the profile's site without any network call.
func (a *app) defaultHost() string {
	s := a.site
	if s == "" {
		s = os.Getenv("BWT_SITE")
	}
	if s == "" {
		if cfg, _, err := a.load(); err == nil {
			s = cfg.Profiles[cfg.ProfileName(a.profile)].Site
		}
	}
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		return strings.ToLower(strings.TrimSuffix(s, "/"))
	}
	if u, err := url.Parse(s); err == nil {
		return strings.ToLower(u.Host)
	}
	return ""
}

func (a *app) profileForWrite(cfg *config.Config) string {
	if name := cfg.ProfileName(a.profile); name != "" {
		return name
	}
	return "default"
}

func (a *app) indexnowGenerateCmd() *cobra.Command {
	var host string
	var noSave bool
	c := &cobra.Command{Use: "generate", Short: "Create a new key and save it for a host", Args: cobra.NoArgs,
		Annotations: map[string]string{"writes-local": "conditional"},
		Long: "Creates a 32-character hexadecimal key and shows where to host it. The key is saved in the selected\n" +
			"profile unless --no-save. Upload a text file named KEY.txt containing only the key to the site root,\n" +
			"then run bwt indexnow key check."}
	c.Flags().StringVar(&host, "host", "", "Host the key is for (default: the selected site's host)")
	c.Flags().BoolVar(&noSave, "no-save", false, "Print the key without saving it")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if host == "" {
			host = a.defaultHost()
		}
		if host == "" {
			return fmt.Errorf("pass --host or select a site")
		}
		host = strings.ToLower(host)
		if !noSave && a.readOnly {
			return fmt.Errorf("saving the key changes local state and is blocked by --read-only; use --no-save")
		}
		k, err := indexnow.GenerateKey()
		if err != nil {
			return err
		}
		location := indexnow.DefaultKeyLocation(host, k)
		result := map[string]any{"host": host, "key": k, "key_location": location, "file_name": k + ".txt", "file_content": k, "saved": !noSave}
		if !noSave {
			path, err := config.Path()
			if err != nil {
				return err
			}
			var profile string
			if err := config.Update(cmd.Context(), path, func(cfg *config.Config) error {
				profile = a.profileForWrite(cfg)
				p := cfg.Profiles[profile]
				if p.IndexNow == nil {
					p.IndexNow = map[string]config.IndexNowKey{}
				}
				p.IndexNow[host] = config.IndexNowKey{Key: k, KeyLocation: location}
				cfg.Profiles[profile] = p
				if cfg.CurrentProfile == "" {
					cfg.CurrentProfile = profile
				}
				return nil
			}); err != nil {
				return err
			}
			result["profile"] = profile
		}
		a.info("Upload %s to your site so it is served at %s, then run bwt indexnow key check --host %s", k+".txt", location, host)
		return a.print(result)
	}
	return c
}

// keyFor finds the key and key location for a host: --key, then BWT_INDEXNOW_KEY, then the profile.
func (a *app) keyFor(host, flagKey, flagLocation string) (string, string, error) {
	k, location := flagKey, flagLocation
	if k == "" {
		k = os.Getenv("BWT_INDEXNOW_KEY")
	}
	if k == "" {
		cfg, _, err := a.load()
		if err != nil {
			return "", "", err
		}
		saved, ok := cfg.Profiles[cfg.ProfileName(a.profile)].IndexNow[host]
		if !ok {
			return "", "", fmt.Errorf("no IndexNow key for %s; run bwt indexnow key generate --host %s or pass --key", host, host)
		}
		k = saved.Key
		if location == "" {
			location = saved.KeyLocation
		}
	}
	if !indexnow.ValidKey(k) {
		return "", "", fmt.Errorf("IndexNow keys are 8-128 letters, digits, or hyphens")
	}
	if location != "" {
		if err := requireURL("--key-location", location); err != nil {
			return "", "", err
		}
		if u, _ := url.Parse(location); !strings.EqualFold(u.Host, host) {
			return "", "", fmt.Errorf("the key location must be on %s", host)
		}
	}
	return k, location, nil
}

func (a *app) indexnowCheckCmd() *cobra.Command {
	var host, flagKey, flagLocation string
	c := &cobra.Command{Use: "check", Short: "Fetch the key file and confirm it matches", Args: cobra.NoArgs}
	c.Flags().StringVar(&host, "host", "", "Host to check (default: the selected site's host)")
	c.Flags().StringVar(&flagKey, "key", "", "Key to check (default: the saved key)")
	c.Flags().StringVar(&flagLocation, "key-location", "", "Key file URL (default: https://host/KEY.txt)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if host == "" {
			host = a.defaultHost()
		}
		if host == "" {
			return fmt.Errorf("pass --host or select a site")
		}
		host = strings.ToLower(host)
		k, location, err := a.keyFor(host, flagKey, flagLocation)
		if err != nil {
			return err
		}
		if location == "" {
			location = indexnow.DefaultKeyLocation(host, k)
		}
		if err := indexnow.CheckKey(cmd.Context(), a.httpClient(), location, k); err != nil {
			return err
		}
		return a.print(map[string]any{"host": host, "key_location": location, "valid": true})
	}
	return c
}

func (a *app) indexnowSubmitCmd() *cobra.Command {
	var file, sitemap, changedSince, flagKey, flagLocation, endpoint string
	c := &cobra.Command{Use: "submit [URL...]", Short: "Submit new, changed, or deleted URLs with IndexNow",
		Annotations: map[string]string{"mutates": "true"},
		Long: "Groups URLs by host and sends up to 10,000 per request, one request at a time. Stops at the first\n" +
			"rejected batch. 200 and 202 mean received (202: key validation pending); neither means indexed.",
		Example: "  bwt indexnow submit https://example.com/new-post\n" +
			"  bwt indexnow submit --sitemap https://example.com/sitemap.xml --changed-since 2026-10-01\n" +
			"  bwt indexnow submit --file changed.txt --endpoint bing"}
	c.Flags().StringVar(&file, "file", "", "Read URLs from a file, one per line (- for stdin)")
	c.Flags().StringVar(&sitemap, "sitemap", "", "Read URLs from a sitemap or sitemap index")
	c.Flags().StringVar(&changedSince, "changed-since", "", "With --sitemap: only URLs whose lastmod is on or after YYYY-MM-DD")
	c.Flags().StringVar(&flagKey, "key", "", "IndexNow key for every host (default: saved key per host, or BWT_INDEXNOW_KEY)")
	c.Flags().StringVar(&flagLocation, "key-location", "", "Key file URL (single host only)")
	c.Flags().StringVar(&endpoint, "endpoint", "indexnow", "indexnow (api.indexnow.org), bing, or an https URL")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		target, ok := indexnow.Endpoints[endpoint]
		if !ok {
			u, err := url.Parse(endpoint)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return fmt.Errorf("--endpoint must be indexnow, bing, or an https URL")
			}
			target = endpoint
		}
		since, err := parseDay("changed-since", changedSince)
		if err != nil {
			return err
		}
		if changedSince != "" && sitemap == "" {
			return fmt.Errorf("--changed-since needs --sitemap")
		}
		var urls []string
		if len(args) > 0 || file != "" {
			list, err := a.readURLs(args, file)
			if err != nil {
				return err
			}
			urls = append(urls, list...)
		}
		if sitemap != "" {
			entries, err := indexnow.ReadSitemap(cmd.Context(), a.webClient(), sitemap)
			if err != nil {
				return err
			}
			if since.IsZero() {
				for _, e := range entries {
					urls = append(urls, strings.TrimSpace(e.Loc))
				}
			} else {
				kept, missing := indexnow.ChangedSince(entries, since)
				if missing > 0 {
					a.info("%d sitemap URLs have no usable lastmod and were skipped", missing)
				}
				urls = append(urls, kept...)
			}
		}
		if len(urls) == 0 {
			return fmt.Errorf("no URLs to submit; pass URLs, --file, or --sitemap")
		}
		batches, err := indexnow.Plan(urls, indexnow.MaxBatch)
		if err != nil {
			return err
		}
		hosts := map[string]bool{}
		for _, b := range batches {
			hosts[b.Host] = true
		}
		if flagLocation != "" && len(hosts) > 1 {
			return fmt.Errorf("--key-location applies to one host; these URLs span %d hosts", len(hosts))
		}
		keys := map[string][2]string{}
		for host := range hosts {
			k, location, err := a.keyFor(host, flagKey, flagLocation)
			if err != nil {
				return err
			}
			keys[host] = [2]string{k, location}
		}
		if a.dryRun {
			planned := make([]map[string]any, 0, len(batches))
			for _, b := range batches {
				planned = append(planned, map[string]any{"endpoint": target, "host": b.Host, "count": len(b.URLs), "key_location": keys[b.Host][1]})
			}
			return output.View(a.out, a.format, map[string]any{"dry_run": true, "batches": planned}, planned)
		}
		var results []indexnow.Result
		var failure error
		for _, b := range batches {
			res, err := indexnow.Submit(cmd.Context(), a.httpClient(), target, b, keys[b.Host][0], keys[b.Host][1])
			if err != nil {
				failure = err
				break
			}
			results = append(results, res)
			if !res.OK {
				failure = fmt.Errorf("IndexNow rejected the batch for %s: HTTP %d, %s", b.Host, res.Status, res.Meaning)
				break
			}
		}
		rows := make([]map[string]any, len(results))
		for i, r := range results {
			rows[i] = map[string]any{"host": r.Host, "count": r.Count, "http_status": r.Status, "meaning": r.Meaning}
		}
		report := map[string]any{"endpoint": target, "urls": len(urls), "batches": results, "note": "Received is not indexed."}
		if err := output.View(a.out, a.format, report, rows, cols("host", "host", "count", "count", "http_status", "http_status", "meaning", "meaning")...); err != nil {
			return err
		}
		return failure
	}
	return c
}
