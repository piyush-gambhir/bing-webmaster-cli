package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/spf13/cobra"
)

var keywordCols = cols("query", "Query", "impressions", "Impressions", "broad_impressions", "BroadImpressions")

func (a *app) keywordsCmd() *cobra.Command {
	c := &cobra.Command{Use: "keywords", Short: "Bing keyword research (market data, not your site's performance)",
		Long: "Keyword research reports Bing search demand for a query in a market. It is not filtered to your site.\n" +
			"Bing does not document the accepted date syntax for these methods; the CLI sends YYYY-MM-DD."}
	c.AddCommand(a.keywordCmd("get QUERY", "Impressions for one query over a date range", "GetKeyword", true, keywordCols),
		a.keywordCmd("stats QUERY", "Weekly impression history for one query", "GetKeywordStats", false,
			cols("date", "Date", "query", "Query", "impressions", "Impressions", "broad_impressions", "BroadImpressions")),
		a.keywordCmd("related QUERY", "Related queries with impressions over a date range", "GetRelatedKeywords", true, keywordCols))
	return c
}

func (a *app) keywordCmd(use, short, method string, dated bool, columns []output.Col) *cobra.Command {
	var country, language, start, end string
	spec := opSpec{use: use, short: short, op: method, args: []string{"q"}, cols: columns}
	spec.setup = func(c *cobra.Command) func() (map[string]any, error) {
		c.Flags().StringVar(&country, "country", "us", "Market country code, such as us or gb")
		c.Flags().StringVar(&language, "language", "en-US", "Market language, such as en-US")
		if dated {
			c.Flags().StringVar(&start, "start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
			c.Flags().StringVar(&end, "end", "", "End date YYYY-MM-DD (default: today, UTC)")
		}
		return func() (map[string]any, error) {
			if strings.TrimSpace(country) == "" || strings.TrimSpace(language) == "" {
				return nil, fmt.Errorf("--country and --language must not be empty")
			}
			params := map[string]any{"country": country, "language": language}
			if !dated {
				return params, nil
			}
			endDay, err := parseDay("end", end)
			if err != nil {
				return nil, err
			}
			if endDay.IsZero() {
				now := time.Now().UTC()
				endDay = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			}
			startDay, err := parseDay("start", start)
			if err != nil {
				return nil, err
			}
			if startDay.IsZero() {
				startDay = endDay.AddDate(0, 0, -30)
			}
			if startDay.After(endDay) {
				return nil, fmt.Errorf("--start must not be after --end")
			}
			a.info("Keyword data for %s to %s (%s, %s)", startDay.Format("2006-01-02"), endDay.Format("2006-01-02"), country, language)
			params["startDate"], params["endDate"] = startDay, endDay
			return params, nil
		}
	}
	return a.opCmd(spec)
}
