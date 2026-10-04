package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
	"github.com/spf13/cobra"
)

// APICoverage maps each Bing method to the runnable commands that call it.
func APICoverage(root *cobra.Command) map[string][]string {
	m := map[string][]string{}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() && c.Annotations["ops"] != "" {
			for _, op := range strings.Split(c.Annotations["ops"], ",") {
				m[op] = append(m[op], c.CommandPath())
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
	for k := range m {
		sort.Strings(m[k])
	}
	return m
}

var beyondAPI = [][3]string{
	{"IndexNow protocol", "`bwt indexnow key generate`, `bwt indexnow key check`, `bwt indexnow submit`", "Needs no Bing credentials; the key is hosted on the site. Submissions reach every participating engine."},
	{"SOAP and POX (XML) protocols", "Never implemented", "Retired by Microsoft on 2026-08-31. The CLI uses the JSON endpoints only."},
	{"AI Performance and citation reports", "Not available", "Dashboard only (public preview February 2026, expanded June 2026); no public API method."},
	{"Recommendations and Top Insights", "Not available", "Dashboard only; no public API method."},
	{"Site Scan", "Not available", "Dashboard audit workflow; no public API method."},
	{"Backlink comparison", "Not available", "Dashboard only. `bwt links` covers the legacy GetLinkCounts and GetUrlLinks methods."},
	{"IndexNow Insights", "Not available", "Dashboard only; no public API method."},
	{"Bing Search APIs (Web Search API)", "Out of scope", "A separate product, retired 2025-08-11; it does not affect the Webmaster API."},
}

// APICoverageMarkdown renders docs/api-coverage.md from the registry and the command tree.
func APICoverageMarkdown(root *cobra.Command) string {
	cov := APICoverage(root)
	counts := map[registry.Status]int{}
	for _, op := range registry.Ops {
		counts[op.Status]++
	}
	var b strings.Builder
	b.WriteString("# Bing Webmaster API coverage\n\n")
	b.WriteString("Generated from the operation registry and the command tree by `make docs`. A test fails when this file\n")
	b.WriteString("is out of date, when a documented method has no command, or when a command's read/write\n")
	b.WriteString("classification disagrees with the registry.\n\n")
	fmt.Fprintf(&b, "The registry is checked against the pinned documentation snapshot described in [compatibility.md](compatibility.md):\n"+
		"%d methods, of which %d are implemented, %d are implemented as experimental, and %d are never implemented.\n\n",
		len(registry.Ops), counts[registry.Implemented], counts[registry.Experimental], counts[registry.Obsolete])
	b.WriteString("Effect is behavior, not HTTP verb: `read` methods are allowed under `--read-only`; `write` methods are blocked.\n")
	b.WriteString("Experimental commands print a notice on every run: Bing still documents them, but they failed a live check.\n\n")
	var groups []string
	seen := map[string]bool{}
	for _, op := range registry.Ops {
		if !seen[op.Group] {
			seen[op.Group] = true
			groups = append(groups, op.Group)
		}
	}
	for _, g := range groups {
		fmt.Fprintf(&b, "## %s\n\n| Method | HTTP | Effect | Status | Command | Notes |\n| --- | --- | --- | --- | --- | --- |\n", g)
		for _, op := range registry.Ops {
			if op.Group != g {
				continue
			}
			command := "Never implemented"
			if cmds := cov[op.Name]; len(cmds) > 0 {
				quoted := make([]string, len(cmds))
				for i, c := range cmds {
					quoted[i] = "`" + c + "`"
				}
				command = strings.Join(quoted, ", ")
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n", op.Name, op.HTTP, op.Effect, op.Status, command, op.Note)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Beyond the Bing Webmaster API\n\n| Feature | Handled by | Notes |\n| --- | --- | --- |\n")
	for _, row := range beyondAPI {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", row[0], row[1], row[2])
	}
	return b.String()
}
