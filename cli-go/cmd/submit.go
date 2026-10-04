package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/spf13/cobra"
)

const acceptedNote = "Accepted means Bing received the URLs; it does not mean they are indexed."

var feedCols = cols("url", "Url", "type", "Type", "status", "Status", "submitted", "Submitted", "last_crawled", "LastCrawled",
	"urls", "UrlCount", "size", "FileSize", "compressed", "Compressed")

func (a *app) sitemapsCmd() *cobra.Command {
	c := &cobra.Command{Use: "sitemaps", Short: "Sitemaps and feeds submitted to Bing"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List submitted sitemaps and feeds", op: "GetFeeds", cols: feedCols}),
		a.opCmd(opSpec{use: "get URL", short: "Show the sitemaps inside a sitemap index", op: "GetFeedDetails", args: []string{"feedUrl"}, cols: feedCols}),
		a.opCmd(opSpec{use: "submit URL", short: "Submit a sitemap or feed URL", op: "SubmitFeed", args: []string{"feedUrl"},
			long: "Supports XML sitemaps, sitemap indexes, RSS 2.0, Atom 0.3/1.0, and text feeds."}),
		a.opCmd(opSpec{use: "remove URL", short: "Remove a sitemap or feed", op: "RemoveFeed", args: []string{"feedUrl"},
			confirm: "This removes the sitemap %s from Bing Webmaster Tools"}),
	)
	return c
}

// readURLs collects URLs from arguments and an optional file ("-" for stdin),
// skipping blank lines and # comments, validating, and removing duplicates.
func (a *app) readURLs(args []string, file string) ([]string, error) {
	list := append([]string{}, args...)
	if file != "" {
		r := a.in
		if file != "-" {
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			r = f
		}
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				list = append(list, line)
			}
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, u := range list {
		if err := requireURL("URL", u); err != nil {
			return nil, err
		}
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no URLs given; pass URLs as arguments or with --file")
	}
	return out, nil
}

func (a *app) submitCmd() *cobra.Command {
	c := &cobra.Command{Use: "submit", Short: "Submit URLs to Bing (prefer indexnow submit for new or changed URLs)"}
	var file string
	var checkQuota bool
	var batchSize int
	urls := &cobra.Command{Use: "urls [URL...]", Short: "Submit URLs with Bing's URL Submission API",
		Long: "Submits URLs for crawling. One URL uses SubmitUrl; more are sent in batches of up to 500 with\n" +
			"SubmitUrlBatch, one after another. On a failure the command reports the batches Bing accepted and\n" +
			"stops; it never resubmits automatically because repeated submissions spend quota.\n" + acceptedNote,
		Example: "  bwt submit urls https://example.com/new-page\n  bwt submit urls --file urls.txt --check-quota"}
	annotate(urls, "SubmitUrl", "SubmitUrlBatch", "GetUrlSubmissionQuota")
	urls.Flags().StringVar(&file, "file", "", "Read URLs from a file, one per line (- for stdin)")
	urls.Flags().BoolVar(&checkQuota, "check-quota", false, "Read the URL submission quota first")
	urls.Flags().IntVar(&batchSize, "batch-size", 500, "URLs per batch (1-500)")
	urls.RunE = func(cmd *cobra.Command, args []string) error {
		if batchSize < 1 || batchSize > 500 {
			return fmt.Errorf("--batch-size must be between 1 and 500")
		}
		list, err := a.readURLs(args, file)
		if err != nil {
			return err
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		report := map[string]any{"site": site, "requested": len(list), "note": acceptedNote}
		if checkQuota {
			quota, err := cl.Call(cmd.Context(), "GetUrlSubmissionQuota", map[string]any{"siteUrl": site})
			if err != nil {
				return err
			}
			report["quota_before"] = quota
		}
		var batches []map[string]any
		accepted := 0
		var failure error
		for i := 0; i < len(list); i += batchSize {
			chunk := list[i:min(i+batchSize, len(list))]
			var err error
			if len(list) == 1 {
				_, err = cl.Call(cmd.Context(), "SubmitUrl", map[string]any{"siteUrl": site, "url": chunk[0]})
			} else {
				_, err = cl.Call(cmd.Context(), "SubmitUrlBatch", map[string]any{"siteUrl": site, "urlList": chunk})
			}
			entry := map[string]any{"batch": len(batches) + 1, "count": len(chunk)}
			if err != nil {
				entry["status"] = "failed"
				batches = append(batches, entry)
				failure = err
				break
			}
			entry["status"] = "accepted by Bing"
			accepted += len(chunk)
			batches = append(batches, entry)
		}
		if len(cl.Planned) > 0 {
			return a.printPlanned(cl.Planned)
		}
		report["accepted"] = accepted
		report["batches"] = batches
		if failure != nil {
			report["not_sent"] = len(list) - accepted - batches[len(batches)-1]["count"].(int)
			if err := output.View(a.out, a.format, report, batches, cols("batch", "batch", "count", "count", "status", "status")...); err != nil {
				return err
			}
			return fmt.Errorf("stopped after a failed batch (%d of %d URLs accepted): %w", accepted, len(list), failure)
		}
		return output.View(a.out, a.format, report, batches, cols("batch", "batch", "count", "count", "status", "status")...)
	}
	c.AddCommand(urls, a.submitContentCmd())
	return c
}

func (a *app) quotaCmd() *cobra.Command {
	c := &cobra.Command{Use: "quota", Short: "Remaining URL and content submission quota", Args: cobra.NoArgs,
		Long: "Shows Bing's daily and monthly counters exactly as returned. Quotas are site-specific; nothing is assumed."}
	annotate(c, "GetUrlSubmissionQuota", "GetContentSubmissionQuota")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		urlQuota, err := cl.Call(cmd.Context(), "GetUrlSubmissionQuota", map[string]any{"siteUrl": site})
		if err != nil {
			return err
		}
		result := map[string]any{"site": site, "url_submission": urlQuota}
		rows := []map[string]any{quotaRow("url submission", urlQuota)}
		contentQuota, err := cl.Call(cmd.Context(), "GetContentSubmissionQuota", map[string]any{"siteUrl": site})
		if err != nil {
			result["content_submission_error"] = err.Error()
		} else {
			result["content_submission"] = contentQuota
			rows = append(rows, quotaRow("content submission", contentQuota))
		}
		return output.View(a.out, a.format, result, rows, cols("kind", "kind", "daily", "daily", "monthly", "monthly")...)
	}
	return c
}

func quotaRow(kind string, v any) map[string]any {
	m, _ := v.(map[string]any)
	return map[string]any{"kind": kind, "daily": m["DailyQuota"], "monthly": m["MonthlyQuota"]}
}

func (a *app) fetchCmd() *cobra.Command {
	c := &cobra.Command{Use: "fetch", Short: "Fetch as Bingbot: request a fetch and read the results"}
	get := a.opCmd(opSpec{use: "get URL", short: "Show a fetched URL's status, headers, and document", op: "GetFetchedUrlDetails", args: []string{"url"},
		cols: cols("url", "Url", "date", "Date", "status", "Status"),
		long: "The table shows the status; use -o json for the response headers and document."})
	c.AddCommand(
		a.opCmd(opSpec{use: "request URL", short: "Ask Bingbot to fetch a URL", op: "FetchUrl", args: []string{"url"},
			long: "Schedules a Bingbot fetch. Check the result later with bwt fetch list and bwt fetch get."}),
		a.opCmd(opSpec{use: "list", short: "List fetch requests and whether they completed", op: "GetFetchedUrls",
			cols: cols("url", "Url", "date", "Date", "fetched", "Fetched", "expired", "Expired")}),
		get,
	)
	return c
}

// convert applies date conversion unless --raw.
func (a *app) convert(v any) any {
	if a.raw {
		return v
	}
	return client.Clean(v)
}
