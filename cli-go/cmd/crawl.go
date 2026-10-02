package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/stats"
	"github.com/spf13/cobra"
)

// crawlIssueFlags is Bing's UrlWithCrawlIssues.CrawlIssues bitmask.
var crawlIssueFlags = []struct {
	bit  int
	name string
}{{1, "301"}, {2, "302"}, {4, "4xx"}, {8, "5xx"}, {16, "blocked by robots.txt"}, {32, "contains malware"},
	{64, "important URL blocked by robots.txt"}, {128, "DNS errors"}, {256, "timeout errors"}}

// issueLabels decodes the bitmask; WCF may also send the flag names as text.
func issueLabels(v any) []string {
	if s, ok := v.(string); ok && s != "" {
		if _, err := strconv.Atoi(s); err != nil {
			return strings.Split(strings.ReplaceAll(s, " ", ""), ",")
		}
	}
	n, ok := stats.Num(v)
	if !ok {
		return nil
	}
	labels := []string{}
	for _, f := range crawlIssueFlags {
		if int(n)&f.bit != 0 {
			labels = append(labels, f.name)
		}
	}
	return labels
}

func (a *app) crawlCmd() *cobra.Command {
	c := &cobra.Command{Use: "crawl", Short: "Crawl statistics, crawl issues, and crawl settings"}
	c.AddCommand(
		a.opCmd(opSpec{use: "stats", short: "Daily crawl statistics", op: "GetCrawlStats",
			long: "Daily counts of crawled pages, pages in the index, crawl errors, and HTTP status classes. Bing updates this daily.",
			cols: cols("date", "Date", "crawled_pages", "CrawledPages", "in_index", "InIndex", "crawl_errors", "CrawlErrors", "in_links", "InLinks",
				"2xx", "Code2xx", "301", "Code301", "302", "Code302", "4xx", "Code4xx", "5xx", "Code5xx", "robots_blocked", "BlockedByRobotsTxt",
				"dns_failures", "DnsFailures", "timeouts", "ConnectionTimeout", "malware", "ContainsMalware", "other_codes", "AllOtherCodes")}),
		a.crawlIssuesCmd(),
		a.opCmd(opSpec{use: "settings", short: "Show crawl rate and crawl boost settings", op: "GetCrawlSettings",
			long: "CrawlRate has 24 hourly values. Bing does not document their valid range or the timezone of the hours."}),
		a.crawlSetCmd(),
	)
	return c
}

func (a *app) crawlIssuesCmd() *cobra.Command {
	c := &cobra.Command{Use: "issues", Short: "URLs with crawl issues, with the issue bitmask decoded", Args: cobra.NoArgs,
		Long: "Lists URLs Bing had trouble crawling. issue_labels is decoded locally from the Issues bitmask.\nFixed issues can stay listed for several days, and an empty list does not prove the site has none."}
	annotate(c, "GetCrawlIssues")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		result, err := cl.Call(cmd.Context(), "GetCrawlIssues", map[string]any{"siteUrl": site})
		if err != nil || a.raw {
			if err == nil {
				err = a.print(result)
			}
			return err
		}
		rows := asList(client.ConvertDates(result))
		for _, r := range rows {
			r["issue_labels"] = issueLabels(r["Issues"])
		}
		if len(rows) == 0 {
			a.info("Bing returned no crawl issues. Its legacy endpoints can return empty lists even when issues exist; check the dashboard before treating the site as healthy.")
		}
		return output.View(a.out, a.format, rows, rows,
			col("url", "Url"), col("http_code", "HttpCode"),
			output.Col{Header: "issues", Value: func(r map[string]any) any { return strings.Join(issueLabels(r["Issues"]), "; ") }},
			col("in_links", "InLinks"))
	}
	return c
}

func (a *app) crawlSetCmd() *cobra.Command {
	var rate, boost string
	c := &cobra.Command{Use: "set-settings", Short: "Change hourly crawl rate or crawl boost", Args: cobra.NoArgs,
		Long: "Reads the current crawl settings, applies your changes, and saves the full object.\n" +
			"--rate takes 24 comma-separated hourly values (Bing does not document the valid range or timezone).",
		Example: "  bwt crawl set-settings --rate 5,5,5,5,5,5,5,5,5,5,5,5,5,5,5,5,6,6,6,5,4,3,3,4\n  bwt crawl set-settings --boost on"}
	annotate(c, "GetCrawlSettings", "SaveCrawlSettings")
	c.Flags().StringVar(&rate, "rate", "", "24 comma-separated hourly crawl-rate values")
	c.Flags().StringVar(&boost, "boost", "", "Crawl boost: on or off")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		var rates []any
		if rate != "" {
			parts := strings.Split(rate, ",")
			if len(parts) != 24 {
				return fmt.Errorf("--rate needs 24 values, got %d", len(parts))
			}
			for _, part := range parts {
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil || n < 0 || n > 255 {
					return fmt.Errorf("--rate values must be integers from 0 to 255, got %q", part)
				}
				rates = append(rates, n)
			}
		}
		if boost != "" && boost != "on" && boost != "off" {
			return fmt.Errorf("--boost must be on or off")
		}
		if rate == "" && boost == "" {
			return fmt.Errorf("nothing to change; pass --rate or --boost")
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		current, err := cl.Call(cmd.Context(), "GetCrawlSettings", map[string]any{"siteUrl": site})
		if err != nil {
			return err
		}
		settings, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("Bing returned unexpected crawl settings")
		}
		if rates != nil {
			settings["CrawlRate"] = rates
		}
		if boost != "" {
			if boost == "on" && settings["CrawlBoostAvailable"] == false {
				a.info("Bing reports crawl boost as unavailable for this site; it may reject the change.")
			}
			settings["CrawlBoostEnabled"] = boost == "on"
		}
		params := map[string]any{"siteUrl": site, "crawlSettings": settings}
		result, err := cl.Call(cmd.Context(), "SaveCrawlSettings", params)
		if err != nil {
			return err
		}
		return a.emit(cl, "SaveCrawlSettings", params, result, nil)
	}
	return c
}
