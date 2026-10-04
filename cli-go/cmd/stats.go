package cmd

import (
	"fmt"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/stats"
	"github.com/spf13/cobra"
)

const statsLong = "Bing returns its own top rows with no date range, row limit, or paging on the server.\n" +
	"--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.\n" +
	"Positions are shown as Bing returns them (their scale is undocumented)."

func ctrCol() output.Col {
	return output.Col{Header: "ctr", Value: func(r map[string]any) any { return stats.CTR(r) }}
}

// queryStatsCols serve QueryStats results; label names what the Query field holds.
func queryStatsCols(label string) []output.Col {
	return []output.Col{col(label, "Query"), col("date", "Date"), col("clicks", "Clicks"), col("impressions", "Impressions"), ctrCol(),
		col("avg_click_position", "AvgClickPosition"), col("avg_impression_position", "AvgImpressionPosition")}
}

var detailCols = []output.Col{col("date", "Date"), col("clicks", "Clicks"), col("impressions", "Impressions"), col("position", "Position")}

func trafficCols() []output.Col {
	return []output.Col{col("date", "Date"), col("clicks", "Clicks"), col("impressions", "Impressions"), ctrCol()}
}

func (a *app) statsCmd() *cobra.Command {
	c := &cobra.Command{Use: "stats", Short: "Search performance: Bing's top queries, pages, and traffic",
		Long: statsLong}
	c.AddCommand(
		a.statsQuery("traffic", "Site clicks and impressions by day", "GetRankAndTrafficStats", nil, trafficCols(), "daily"),
		a.statsQuery("queries", "Top queries", "GetQueryStats", nil, queryStatsCols("query"), "weekly"),
		a.statsQuery("pages", "Top pages", "GetPageStats", nil, queryStatsCols("page"), "weekly"),
		a.statsQuery("query-pages QUERY", "Pages that rank for a query", "GetQueryPageStats", []string{"query"}, queryStatsCols("page"), "weekly"),
		a.statsQuery("page-queries URL", "Queries that lead to a page", "GetPageQueryStats", []string{"page"}, queryStatsCols("query"), "weekly"),
		a.statsQuery("detail QUERY URL", "Position detail for one query and page", "GetQueryPageDetailStats", []string{"query", "page"}, detailCols, "weekly"),
		a.statsQuery("query-traffic QUERY", "Traffic history for one query (top queries only)", "GetQueryTrafficStats", []string{"query"}, trafficCols(), "daily"),
		a.statsSummary(),
	)
	return c
}

func parseDay(flag, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("--%s must be YYYY-MM-DD", flag)
	}
	return t, nil
}

func (a *app) statsQuery(use, short, method string, args []string, columns []output.Col, cadence string) *cobra.Command {
	var since, until, sortBy string
	var limit int
	var ascending bool
	c := &cobra.Command{Use: use, Short: short, Long: statsLong + "\nBing updates this data " + cadence + ".", Args: cobra.ExactArgs(len(args))}
	annotate(c, method)
	c.Flags().StringVar(&since, "since", "", "Keep rows on or after this day (YYYY-MM-DD, local filter)")
	c.Flags().StringVar(&until, "until", "", "Keep rows on or before this day (YYYY-MM-DD, local filter)")
	c.Flags().IntVar(&limit, "limit", 0, "Keep at most N rows after sorting (local)")
	c.Flags().StringVar(&sortBy, "sort", "", "Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)")
	c.Flags().BoolVar(&ascending, "asc", false, "Sort ascending")
	c.RunE = func(cmd *cobra.Command, posArgs []string) error {
		f := stats.Filter{Limit: limit, Sort: sortBy, Ascending: ascending}
		var err error
		if f.Since, err = parseDay("since", since); err != nil {
			return err
		}
		if f.Until, err = parseDay("until", until); err != nil {
			return err
		}
		if limit < 0 {
			return fmt.Errorf("--limit must be positive")
		}
		if _, ok := stats.SortKeys[sortBy]; sortBy != "" && !ok {
			return fmt.Errorf("unknown --sort %q", sortBy)
		}
		if a.raw && f.Active() {
			return fmt.Errorf("--raw prints Bing's response unchanged and cannot be combined with local filters")
		}
		params := map[string]any{}
		for i, name := range args {
			if name == "page" {
				if err := requireURL("URL", posArgs[i]); err != nil {
					return err
				}
			}
			params[name] = posArgs[i]
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		params["siteUrl"] = site
		result, err := cl.Call(cmd.Context(), method, params)
		if err != nil {
			return err
		}
		if a.raw {
			return a.print(result, columns...)
		}
		rows, err := stats.Apply(asList(client.Clean(result)), f)
		if err != nil {
			return err
		}
		if len(rows) == 0 && !f.Active() {
			a.info("Bing has no performance data for this site yet; new or low-traffic sites can take days to show data.")
		}
		envelope := map[string]any{"site": site, "method": method, "update_cadence": cadence,
			"filtered_locally": f.Active(), "row_count": len(rows), "rows": rows}
		return output.View(a.out, a.format, envelope, rows, columns...)
	}
	return c
}

func (a *app) statsSummary() *cobra.Command {
	var days int
	var compare string
	c := &cobra.Command{Use: "summary", Short: "Totals over the latest N days of site traffic, computed locally",
		Long: "Sums GetRankAndTrafficStats over the N calendar days ending at the latest day Bing returned.\n" +
			"CTR is recomputed from the sums. --compare previous adds the preceding N days; percentage changes\n" +
			"are null when the earlier value is zero, and the CTR change is in percentage points.",
		Args: cobra.NoArgs}
	annotate(c, "GetRankAndTrafficStats")
	c.Flags().IntVar(&days, "days", 28, "Window length in days")
	c.Flags().StringVar(&compare, "compare", "", "Compare with: previous")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if days < 1 || days > 1000 {
			return fmt.Errorf("--days must be between 1 and 1000")
		}
		if compare != "" && compare != "previous" {
			return fmt.Errorf("--compare supports only previous")
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		result, err := cl.Call(cmd.Context(), "GetRankAndTrafficStats", map[string]any{"siteUrl": site})
		if err != nil {
			return err
		}
		rows := asList(client.Clean(result))
		envelope := map[string]any{"site": site, "method": "GetRankAndTrafficStats", "computed_locally": true, "window_days": days}
		end, ok := stats.Latest(rows)
		if !ok {
			envelope["current"] = nil
			envelope["note"] = "Bing returned no dated traffic rows"
			return a.print(envelope)
		}
		current := stats.Window(rows, end, days)
		envelope["current"] = current
		table := []map[string]any{periodRow("current", current)}
		if compare == "previous" {
			previous := stats.Window(rows, end.AddDate(0, 0, -days), days)
			envelope["previous"] = previous
			envelope["change"] = stats.Compare(current, previous)
			table = append(table, periodRow("previous", previous))
		}
		if current.DaysWithData < days {
			envelope["note"] = fmt.Sprintf("only %d of %d days have data", current.DaysWithData, days)
		}
		return output.View(a.out, a.format, envelope, table,
			cols("period", "period", "start", "start", "end", "end", "days_with_data", "days_with_data", "clicks", "clicks", "impressions", "impressions", "ctr", "ctr")...)
	}
	return c
}

func periodRow(name string, p stats.Period) map[string]any {
	return map[string]any{"period": name, "start": p.Start, "end": p.End, "days_with_data": p.DaysWithData,
		"clicks": p.Clicks, "impressions": p.Impressions, "ctr": p.CTR}
}
