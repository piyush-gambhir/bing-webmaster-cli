package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/stats"
	"github.com/spf13/cobra"
)

var urlInfoCols = cols("url", "Url", "is_page", "IsPage", "http_status", "HttpStatus", "last_crawled", "LastCrawledDate",
	"discovered", "DiscoveryDate", "size", "DocumentSize", "anchors", "AnchorCount", "children", "TotalChildUrlCount")

var urlTrafficCols = cols("url", "Url", "is_page", "IsPage", "clicks", "Clicks", "impressions", "Impressions")

func (a *app) urlCmd() *cobra.Command {
	c := &cobra.Command{Use: "url", Short: "What Bing knows about a URL and the URLs under it",
		Long: "Bing's URL data has no explicit indexed verdict. IsPage and HttpStatus describe crawling, not indexing."}
	c.AddCommand(
		a.opCmd(opSpec{use: "info URL", short: "Crawl details for one URL", op: "GetUrlInfo", args: []string{"url"},
			long: "Shows last crawl, discovery date, HTTP status, size, anchors, and child count. A zero status or IsPage=false is not an indexing verdict."}),
		a.opCmd(opSpec{use: "traffic URL", short: "Clicks and impressions for one URL (window undocumented)", op: "GetUrlTrafficInfo", args: []string{"url"}}),
		a.childrenCmd(),
		a.childrenTrafficCmd(),
	)
	return c
}

type pager struct {
	page, maxPages int
	all            bool
	// limit is the largest page number the method's parameter type allows.
	limit int
}

func (p *pager) flags(c *cobra.Command) {
	c.Flags().IntVar(&p.page, "page", 0, "Page to fetch, starting at 0")
	c.Flags().BoolVar(&p.all, "all", false, "Fetch pages until the end (bounded by --max-pages)")
	c.Flags().IntVar(&p.maxPages, "max-pages", 100, "Upper bound on pages fetched with --all")
}

func (p *pager) validate(limit int) error {
	p.limit = limit
	if p.page < 0 || p.page > limit {
		return fmt.Errorf("--page must be between 0 and %d", limit)
	}
	if p.maxPages < 1 || p.maxPages > 10000 {
		return fmt.Errorf("--max-pages must be between 1 and 10000")
	}
	return nil
}

// walk fetches pages starting at p.page. fetch returns the page's rows and, when
// known, the total page count. It stops at an empty page, at totalPages, at
// --max-pages, at the method's largest page number, or when a page repeats the
// previous one.
func (p *pager) walk(ctx context.Context, fetch func(context.Context, int) ([]map[string]any, int, error)) (rows []map[string]any, fetched int, complete bool, err error) {
	prev := ""
	for page := p.page; ; page++ {
		if p.limit > 0 && page > p.limit {
			return rows, fetched, false, nil
		}
		items, total, err := fetch(ctx, page)
		if err != nil {
			return nil, fetched, false, err
		}
		fetched++
		if len(items) == 0 {
			return rows, fetched, true, nil
		}
		// Compare whole pages: consecutive pages can legitimately share a first row.
		b, _ := json.Marshal(items)
		if string(b) == prev {
			return rows, fetched, false, fmt.Errorf("page %d repeated the previous page; stopping", page)
		}
		prev = string(b)
		rows = append(rows, items...)
		if total > 0 && page+1 >= total {
			return rows, fetched, true, nil
		}
		if !p.all {
			return rows, fetched, false, nil
		}
		if fetched >= p.maxPages {
			return rows, fetched, false, nil
		}
	}
}

func (a *app) pagedOutput(site, method string, rows []map[string]any, fetched int, complete bool, columns []output.Col) error {
	if !complete {
		a.info("More pages may exist; use --all or a higher --page.")
	}
	envelope := map[string]any{"site": site, "method": method, "pages_fetched": fetched, "complete": complete, "row_count": len(rows), "rows": rows}
	return output.View(a.out, a.format, envelope, rows, columns...)
}

func (a *app) childrenCmd() *cobra.Command {
	var p pager
	var crawlDate, discovered, docFlags, httpCodes string
	c := &cobra.Command{Use: "children URL", Short: "URLs under a URL, with crawl details and filters", Args: cobra.ExactArgs(1),
		Long: "Lists URLs below URL. Pages start at 0; --all continues until Bing returns an empty page.\nThis is a read even though Bing implements it with POST."}
	annotate(c, "GetChildrenUrlInfo")
	p.flags(c)
	c.Flags().StringVar(&crawlDate, "crawl-date", "any", "Crawled within: any, last-week, last-two-weeks, last-three-weeks")
	c.Flags().StringVar(&discovered, "discovered", "any", "Discovered within: any, last-week, last-month")
	c.Flags().StringVar(&docFlags, "doc-flags", "", "Comma list: blocked-by-robots, malware")
	c.Flags().StringVar(&httpCodes, "http", "", "Comma list: 2xx, 3xx, 301, 302, 4xx, 5xx, other")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if err := requireURL("URL", args[0]); err != nil {
			return err
		}
		if err := p.validate(65535); err != nil {
			return err
		}
		crawl, err := choice("crawl-date", crawlDate, map[string]int{"any": 0, "last-week": 1, "last-two-weeks": 2, "last-three-weeks": 4})
		if err != nil {
			return err
		}
		disc, err := choice("discovered", discovered, map[string]int{"any": 0, "last-week": 1, "last-month": 2})
		if err != nil {
			return err
		}
		doc, err := flagSet("doc-flags", docFlags, map[string]int{"blocked-by-robots": 1, "malware": 2})
		if err != nil {
			return err
		}
		codes, err := flagSet("http", httpCodes, map[string]int{"2xx": 1, "3xx": 2, "301": 4, "302": 8, "4xx": 16, "5xx": 32, "other": 64})
		if err != nil {
			return err
		}
		filter := map[string]any{"CrawlDateFilter": crawl, "DiscoveredDateFilter": disc, "DocFlagsFilters": doc, "HttpCodeFilters": codes}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		rows, fetched, complete, err := p.walk(cmd.Context(), func(ctx context.Context, page int) ([]map[string]any, int, error) {
			res, err := cl.Call(ctx, "GetChildrenUrlInfo", map[string]any{"siteUrl": site, "url": args[0], "page": page, "filterProperties": filter})
			return a.rowsOf(res), 0, err
		})
		if err != nil {
			return err
		}
		return a.pagedOutput(site, "GetChildrenUrlInfo", rows, fetched, complete, urlInfoCols)
	}
	return c
}

func (a *app) childrenTrafficCmd() *cobra.Command {
	var p pager
	c := &cobra.Command{Use: "children-traffic URL", Short: "Clicks and impressions for URLs under a URL", Args: cobra.ExactArgs(1)}
	annotate(c, "GetChildrenUrlTrafficInfo")
	p.flags(c)
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if err := requireURL("URL", args[0]); err != nil {
			return err
		}
		if err := p.validate(65535); err != nil {
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
		rows, fetched, complete, err := p.walk(cmd.Context(), func(ctx context.Context, page int) ([]map[string]any, int, error) {
			res, err := cl.Call(ctx, "GetChildrenUrlTrafficInfo", map[string]any{"siteUrl": site, "url": args[0], "page": page})
			return a.rowsOf(res), 0, err
		})
		if err != nil {
			return err
		}
		return a.pagedOutput(site, "GetChildrenUrlTrafficInfo", rows, fetched, complete, urlTrafficCols)
	}
	return c
}

// rowsOf converts a list result's dates (unless --raw) and returns its rows.
func (a *app) rowsOf(v any) []map[string]any {
	if !a.raw {
		v = client.ConvertDates(v)
	}
	return asList(v)
}

func flagSet(flag, value string, options map[string]int) (int, error) {
	total := 0
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := choice(flag, part, options)
		if err != nil {
			return 0, err
		}
		total |= v
	}
	return total, nil
}

func (a *app) linksCmd() *cobra.Command {
	c := &cobra.Command{Use: "links", Short: "Inbound links Bing has found",
		Long: "Bing's legacy link methods can return empty results for sites that have links; empty is not proof of none."}
	c.AddCommand(a.linkPagesCmd("counts", "Pages on your site with inbound link counts", "GetLinkCounts", "Links", cols("url", "Url", "inbound_links", "Count")),
		a.linkPagesCmd("list URL", "Inbound links to one page, with anchor text", "GetUrlLinks", "Details", cols("source_url", "Url", "anchor_text", "AnchorText")))
	return c
}

func (a *app) linkPagesCmd(use, short, method, listField string, columns []output.Col) *cobra.Command {
	var p pager
	nargs := 0
	if strings.Contains(use, " ") {
		nargs = 1
	}
	c := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(nargs),
		Long: "Pages start at 0. --all follows Bing's TotalPages (bounded by --max-pages)."}
	annotate(c, method)
	p.flags(c)
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if nargs == 1 {
			if err := requireURL("URL", args[0]); err != nil {
				return err
			}
		}
		if err := p.validate(32767); err != nil {
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
		rows, fetched, complete, err := p.walk(cmd.Context(), func(ctx context.Context, page int) ([]map[string]any, int, error) {
			params := map[string]any{"siteUrl": site, "page": page}
			if nargs == 1 {
				params["link"] = args[0]
			}
			res, err := cl.Call(ctx, method, params)
			if err != nil {
				return nil, 0, err
			}
			m, _ := res.(map[string]any)
			total, _ := stats.Num(m["TotalPages"])
			if !a.raw {
				return asList(client.ConvertDates(m[listField])), int(total), nil
			}
			return asList(m[listField]), int(total), nil
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			a.info("Bing returned no links. Its legacy link methods can return empty results even when links exist.")
		}
		return a.pagedOutput(site, method, rows, fetched, complete, columns)
	}
	return c
}

func (a *app) connectedPagesCmd() *cobra.Command {
	c := &cobra.Command{Use: "connected-pages", Short: "Pages (such as social profiles) connected to your site"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List connected pages", op: "GetConnectedPages"}),
		a.opCmd(opSpec{use: "add URL", short: "Connect a page to your site", op: "AddConnectedPage", args: []string{"masterUrl"}}),
	)
	return c
}
