package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/spf13/cobra"
)

var (
	geoTypes   = map[string]string{"0": "page", "1": "directory", "2": "domain", "3": "subdomain", "Page": "page", "Directory": "directory", "Domain": "domain", "Subdomain": "subdomain"}
	moveScopes = map[string]string{"0": "domain", "1": "host", "2": "directory", "Domain": "domain", "Host": "host", "Directory": "directory"}
	moveTypes  = map[string]string{"0": "local", "1": "global", "Local": "local", "Global": "global"}
	countryRe  = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

func (a *app) experimentalCmd() *cobra.Command {
	c := &cobra.Command{Use: "experimental", Short: "Documented Bing methods that did not work in live checks",
		Long: "These methods are still in Bing's documentation, but a live check on 2026-10-04 failed (GetSiteMoves\n" +
			"returned HTTP 404). Each run prints a notice. Confirm results in the Bing Webmaster Tools dashboard."}
	c.AddCommand(a.siteMoveCmd())
	return c
}

func (a *app) geoCmd() *cobra.Command {
	c := &cobra.Command{Use: "geo", Short: "Country or region targeting for pages, directories, or hosts"}
	var url, country, typ string
	settings := func() (map[string]any, error) {
		if err := requireURL("--url", url); err != nil {
			return nil, err
		}
		if !countryRe.MatchString(country) {
			return nil, fmt.Errorf("--country must be a two-letter ISO code")
		}
		t, err := choice("type", typ, map[string]int{"page": 0, "directory": 1, "domain": 2, "subdomain": 3})
		if err != nil {
			return nil, err
		}
		// Bing accepts lowercase country codes only (us); US is rejected as InvalidParameter.
		return map[string]any{"settings": map[string]any{"Url": url, "TwoLetterIsoCountryCode": strings.ToLower(country), "Type": t}}, nil
	}
	flags := func(c *cobra.Command) func() (map[string]any, error) {
		c.Flags().StringVar(&url, "url", "", "Page, directory, or host URL (required)")
		c.Flags().StringVar(&country, "country", "", "Two-letter ISO country code (required)")
		c.Flags().StringVar(&typ, "type", "page", "page, directory, domain, or subdomain")
		return settings
	}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List targeting settings", op: "GetCountryRegionSettings",
			cols: []output.Col{col("url", "Url"), col("country", "TwoLetterIsoCountryCode"), enumCol("type", "Type", geoTypes), col("date", "Date")}}),
		a.opCmd(opSpec{use: "add", short: "Target a URL to a country or region", op: "AddCountryRegionSettings", setup: flags}),
		a.opCmd(opSpec{use: "remove", short: "Remove a targeting setting", op: "RemoveCountryRegionSettings", setup: flags}),
	)
	return c
}

func (a *app) siteMoveCmd() *cobra.Command {
	var from, to, scope, typ string
	c := &cobra.Command{Use: "site-move", Short: "Site moves (Bing's site move tool)"}
	submit := a.opCmd(opSpec{use: "submit", short: "Tell Bing that content moved to a new location", op: "SubmitSiteMove",
		confirm: "This tells Bing that %s moved",
		setup: func(c *cobra.Command) func() (map[string]any, error) {
			c.Flags().StringVar(&from, "from", "", "Source URL (required)")
			c.Flags().StringVar(&to, "to", "", "Target URL (required)")
			c.Flags().StringVar(&scope, "scope", "", "domain, host, or directory (required)")
			c.Flags().StringVar(&typ, "type", "", "local or global (required)")
			return func() (map[string]any, error) {
				if err := requireURL("--from", from); err != nil {
					return nil, err
				}
				if err := requireURL("--to", to); err != nil {
					return nil, err
				}
				s, err := choice("scope", scope, map[string]int{"domain": 0, "host": 1, "directory": 2})
				if err != nil {
					return nil, err
				}
				t, err := choice("type", typ, map[string]int{"local": 0, "global": 1})
				if err != nil {
					return nil, err
				}
				return map[string]any{"settings": map[string]any{"SourceUrl": from, "TargetUrl": to, "MoveScope": s, "MoveType": t}}, nil
			}
		}})
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List submitted site moves", op: "GetSiteMoves",
			cols: []output.Col{col("source", "SourceUrl"), col("target", "TargetUrl"), enumCol("scope", "MoveScope", moveScopes), enumCol("type", "MoveType", moveTypes), col("date", "Date")}}),
		submit,
	)
	return c
}

func (a *app) deepLinkBlocksCmd() *cobra.Command {
	var market, searchURL, deepLinkURL string
	flags := func(c *cobra.Command) func() (map[string]any, error) {
		c.Flags().StringVar(&market, "market", "", "Market, such as en-US (required)")
		c.Flags().StringVar(&searchURL, "search-url", "", "Result URL the deep link appears under (required)")
		c.Flags().StringVar(&deepLinkURL, "deep-link-url", "", "Deep link URL to block (required)")
		return func() (map[string]any, error) {
			if market == "" {
				return nil, fmt.Errorf("--market is required")
			}
			if err := requireURL("--search-url", searchURL); err != nil {
				return nil, err
			}
			if err := requireURL("--deep-link-url", deepLinkURL); err != nil {
				return nil, err
			}
			// Bing accepts lowercase market codes only (en-us); en-US is rejected as InvalidParameter.
			return map[string]any{"market": strings.ToLower(market), "searchUrl": searchURL, "deepLinkUrl": deepLinkURL}, nil
		}
	}
	c := &cobra.Command{Use: "deeplink-blocks", Short: "Block deep links (sitelinks) shown under a result"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List deep-link blocks", op: "GetDeepLinkBlocks",
			cols: cols("deep_link", "DeepLinkUrl", "search_url", "SearchUrl", "market", "Market", "submitted", "SubmitDate", "expires", "ExpiryDate")}),
		a.opCmd(opSpec{use: "add", short: "Block a deep link", op: "AddDeepLinkBlock", setup: flags}),
		a.opCmd(opSpec{use: "remove", short: "Remove a deep-link block", op: "RemoveDeepLinkBlock", setup: flags}),
	)
	return c
}

const maxContent = 10 << 20

func (a *app) submitContentCmd() *cobra.Command {
	var file, structured, serving string
	return a.opCmd(opSpec{use: "content URL", short: "Push a page's full HTTP response to Bing (content submission)", op: "SubmitContent",
		args: []string{"url"},
		long: "Sends a complete HTTP response (status line, headers, blank line, body) for URL, base64-encoded, up to 10 MB.\n" +
			"Works with an API key. Submitted content may be indexed even if robots.txt disallows it (NOINDEX is\n" +
			"honored). Capture a response with: curl -s --http1.1 -i https://example.com/page -o page.http",
		example: "  curl -s --http1.1 -i https://www.example.com/ -o home.http\n  bwt submit content https://www.example.com/ --file home.http",
		confirm: "Bing may index the submitted content for %s even if robots.txt disallows it",
		setup: func(c *cobra.Command) func() (map[string]any, error) {
			c.Flags().StringVar(&file, "file", "", "File with the full HTTP response (required)")
			c.Flags().StringVar(&structured, "structured-data", "", "File with structured data, normally JSON-LD")
			c.Flags().StringVar(&serving, "dynamic-serving", "none", "none, pc, mobile, amp, tablet, or nonvisual")
			return func() (map[string]any, error) {
				msg, err := os.ReadFile(file)
				if err != nil {
					return nil, fmt.Errorf("--file: %w", err)
				}
				if len(msg) > maxContent {
					return nil, fmt.Errorf("--file exceeds Bing's 10 MB limit")
				}
				if !bytes.HasPrefix(msg, []byte("HTTP/")) || !bytes.Contains(msg, []byte("\r\n\r\n")) {
					return nil, fmt.Errorf("--file must be a full HTTP response: a status line such as HTTP/1.1 200 OK, CRLF headers, and a blank CRLF line")
				}
				sd := ""
				if structured != "" {
					b, err := os.ReadFile(structured)
					if err != nil {
						return nil, fmt.Errorf("--structured-data: %w", err)
					}
					sd = base64.StdEncoding.EncodeToString(b)
				}
				ds, err := choice("dynamic-serving", serving, map[string]int{"none": 0, "pc": 1, "mobile": 2, "amp": 3, "tablet": 4, "nonvisual": 5})
				if err != nil {
					return nil, err
				}
				return map[string]any{"httpMessage": base64.StdEncoding.EncodeToString(msg), "structuredData": sd, "dynamicServing": ds}, nil
			}
		}})
}
