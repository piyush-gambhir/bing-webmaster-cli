package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/build"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/update"
	"github.com/spf13/cobra"
)

type app struct {
	format, profile, site, apiKey, accessToken string
	timeout                                    time.Duration
	noInput, quiet, verbose, readOnly          bool
	dryRun, yes, raw                           bool
	in                                         io.Reader
	out, errOut                                io.Writer
	// transport replaces the network for tests; nil uses the default transport.
	transport http.RoundTripper
	// openBrowser replaces the system browser for tests.
	openBrowser func(string) error
	// exePath and stderrTTY replace the running executable's path and stderr
	// terminal detection for tests.
	exePath   string
	stderrTTY func() bool
	// updateCheck carries the release check from PersistentPreRun to
	// PersistentPostRun; checks tracks its background request, and
	// checkStarted is set when this run sent it.
	updateCheck  chan update.Cache
	checks       sync.WaitGroup
	checkStarted bool
}

func envBool(name string) bool { s := os.Getenv(name); return s == "1" || strings.EqualFold(s, "true") }

func NewRoot(in io.Reader, out, errOut io.Writer) *cobra.Command {
	return newRoot(&app{in: in, out: out, errOut: errOut})
}

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   "bwt",
		Short: "Bing Webmaster Tools from your terminal",
		Long: "Read and manage Bing Webmaster Tools data: search performance, crawl health, URL and sitemap\n" +
			"submission, IndexNow, keyword research, and site settings.\n" +
			"Data goes to stdout and diagnostics to stderr. --read-only blocks every state change.",
		SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !output.Valid(a.format) {
				return fmt.Errorf("unsupported output %q; use table, json, yaml, or csv", a.format)
			}
			if a.timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			if a.readOnly && cmd.Annotations["mutates"] == "true" {
				return fmt.Errorf("%s changes Bing state and is blocked by --read-only", cmd.CommandPath())
			}
			if a.readOnly && cmd.Annotations["writes-local"] == "true" {
				return fmt.Errorf("%s changes local state and is blocked by --read-only", cmd.CommandPath())
			}
			if cmd.Annotations["experimental"] == "true" {
				a.info("Experimental: Bing documents site moves, but GetSiteMoves returned HTTP 404 in a live check on 2026-10-04. Check the result in the Bing Webmaster Tools dashboard.")
			}
			a.startUpdateCheck(cmd)
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) { a.printUpdateNotice() },
	}
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.errOut)
	f := root.PersistentFlags()
	f.StringVarP(&a.format, "output", "o", "table", "Output format: table, json, yaml, csv")
	f.StringVar(&a.profile, "profile", "", "Named profile (or BWT_PROFILE)")
	f.StringVarP(&a.site, "site", "s", "", "Site URL exactly as in sites list, or a bare host (or BWT_SITE)")
	f.StringVar(&a.apiKey, "api-key", "", "API key override (prefer BWT_API_KEY or auth login)")
	f.StringVar(&a.accessToken, "access-token", "", "Bearer token override (prefer BWT_ACCESS_TOKEN)")
	f.DurationVar(&a.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	f.BoolVar(&a.noInput, "no-input", envBool("BWT_NO_INPUT"), "Never prompt or open a browser")
	f.BoolVarP(&a.quiet, "quiet", "q", envBool("BWT_QUIET"), "Suppress informational stderr output")
	f.BoolVarP(&a.verbose, "verbose", "v", envBool("BWT_VERBOSE"), "Log method, redacted URL, and status to stderr")
	f.BoolVar(&a.readOnly, "read-only", envBool("BWT_READ_ONLY"), "Block remote writes, local credential changes, and self-update")
	f.BoolVar(&a.dryRun, "dry-run", false, "For writes: print the requests and send nothing")
	f.BoolVarP(&a.yes, "yes", "y", false, "Confirm destructive commands and updates without a prompt")
	f.BoolVar(&a.raw, "raw", false, "Print Bing's wire JSON without date conversion")

	root.AddCommand(a.authCmd(), a.configCmd(), a.sitesCmd(), a.statsCmd(), a.crawlCmd(), a.urlCmd(),
		a.linksCmd(), a.connectedPagesCmd(), a.sitemapsCmd(), a.submitCmd(), a.quotaCmd(), a.indexnowCmd(),
		a.keywordsCmd(), a.fetchCmd(), a.usersCmd(), a.paramsCmd(), a.blockCmd(), a.previewBlocksCmd(),
		a.geoCmd(), a.deepLinkBlocksCmd(), a.experimentalCmd(), a.apiCmd(), a.doctorCmd(), a.updateCmd())
	login := a.loginCmd()
	login.Short = "Alias for auth login"
	root.AddCommand(login)
	status := a.statusCmd()
	status.Short = "Alias for auth status"
	root.AddCommand(status)
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print build information", Long: "Prints the version, commit, and build date. latest and update_available come from the last\nrelease check (see bwt update --help) and appear only when one is cached; version never uses the network.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		info := map[string]any{"version": build.Version, "commit": build.Commit, "date": build.Date}
		if dir, err := config.Dir(); err == nil {
			if c := update.ReadCache(dir); c.LatestVersion != "" {
				info["latest"] = c.LatestVersion
				info["update_available"] = update.Newer(c.LatestVersion, build.Version)
			}
		}
		return a.print(info)
	}})
	root.AddCommand(&cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate shell completion script", Args: cobra.ExactArgs(1), ValidArgs: []string{"bash", "zsh", "fish", "powershell"}, RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(a.out)
		case "zsh":
			return root.GenZshCompletion(a.out)
		case "fish":
			return root.GenFishCompletion(a.out, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(a.out)
		default:
			return fmt.Errorf("unsupported shell %q", args[0])
		}
	}})
	root.CompletionOptions.DisableDefaultCmd = true
	return root
}

// Run executes the CLI and returns the process exit code. Errors go to stderr,
// structured in JSON and YAML modes, with credentials redacted.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	if runtime.GOOS == "windows" {
		// A Windows self-update leaves the replaced bwt.exe.old behind.
		if exe, err := os.Executable(); err == nil {
			if exe, err = filepath.EvalSymlinks(exe); err == nil {
				update.RemoveLeftover(exe)
			}
		}
	}
	return execute(ctx, &app{in: in, out: out, errOut: errOut}, args)
}

func execute(ctx context.Context, a *app, args []string) int {
	errOut := a.errOut
	root := newRoot(a)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		message := err.Error()
		for _, secret := range []string{a.apiKey, a.accessToken, os.Getenv("BWT_API_KEY"), os.Getenv("BWT_ACCESS_TOKEN")} {
			if len(secret) >= 4 {
				message = strings.ReplaceAll(message, secret, "[REDACTED]")
			}
		}
		payload := map[string]any{"error": message}
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			payload["method"] = apiErr.Method
			payload["http_status"] = apiErr.HTTPStatus
			if apiErr.Name != "" {
				payload["api_error_code"] = apiErr.Code
				payload["api_error_name"] = apiErr.Name
			}
			if apiErr.RetryAfter != "" {
				payload["retry_after"] = apiErr.RetryAfter
			}
		}
		if a.format == "json" || a.format == "yaml" {
			_ = output.Print(errOut, a.format, payload)
		} else {
			fmt.Fprintln(errOut, "Error:", message)
		}
		return 1
	}
	return 0
}

func (a *app) print(data any, cols ...output.Col) error {
	return output.Print(a.out, a.format, data, cols...)
}

func (a *app) info(format string, args ...any) {
	if !a.quiet {
		fmt.Fprintf(a.errOut, format+"\n", args...)
	}
}
