package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDomainFormIsAcceptedOnlyWhereBingDocumentsIt(t *testing.T) {
	for _, ok := range []string{"domain:example.com", "https://example.com/a"} {
		if err := requireURLOrDomain("URL", ok); err != nil {
			t.Fatalf("%s rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"domain:", "domain:example", "domain:example.com/path", "example.com"} {
		if err := requireURLOrDomain("URL", bad); err == nil {
			t.Fatalf("%s accepted", bad)
		}
	}
	if err := requireURL("URL", "domain:example.com"); err == nil {
		t.Fatal("plain URL check accepted domain: form")
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "site") != "1 site" || plural(3, "site") != "3 sites" || plural(0, "site") != "0 sites" {
		t.Fatal(plural(1, "site"), plural(3, "site"))
	}
}

func TestLowercaseCodesAndDryRunTable(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k")
	t.Setenv("BWT_SITE", "https://www.example.com/")
	res := run(t, nil, "", "geo", "add", "--url", "https://www.example.com/p", "--country", "US", "--dry-run", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, `"TwoLetterIsoCountryCode": "us"`) {
		t.Fatalf("geo add must send a lowercase country (Bing rejects US): %v %s", res.err, res.out)
	}
	res = run(t, nil, "", "deeplink-blocks", "add", "--market", "en-US", "--search-url", "https://www.example.com/",
		"--deep-link-url", "https://www.example.com/p", "--dry-run", "-o", "json")
	if res.err != nil || !strings.Contains(res.out, `"market": "en-us"`) {
		t.Fatalf("deep-link block must send a lowercase market (Bing rejects en-US): %v %s", res.err, res.out)
	}
	res = run(t, nil, "", "sitemaps", "submit", "https://www.example.com/sitemap.xml", "--dry-run")
	if res.err != nil || !strings.Contains(res.out, "SubmitFeed") || strings.Contains(res.out, "{") || !strings.Contains(res.errOut, "nothing was sent") {
		t.Fatalf("dry-run table: %v out=%q err=%q", res.err, res.out, res.errOut)
	}
}

func TestEmptyPagedTableSaysNoResults(t *testing.T) {
	isolate(t)
	t.Setenv("BWT_API_KEY", "k")
	t.Setenv("BWT_SITE", "https://www.example.com/")
	f := &fake{handle: func(r *http.Request, body string) *http.Response {
		return reply(200, `{"d":{"Links":[],"TotalPages":0}}`)
	}}
	res := run(t, f, "", "links", "counts")
	if res.err != nil || !strings.Contains(res.out, "No results.") || strings.Contains(res.out, "pages_fetched") {
		t.Fatalf("empty links table: %v %q", res.err, res.out)
	}
}

func TestAPIMethodsListsTheRegistry(t *testing.T) {
	isolate(t)
	res := run(t, nil, "", "api", "methods", "-o", "json")
	var rows []map[string]any
	if res.err != nil || json.Unmarshal([]byte(res.out), &rows) != nil || len(rows) != 62 {
		t.Fatalf("api methods: %v rows=%d", res.err, len(rows))
	}
	for _, r := range rows {
		if r["method"] == "SubmitContent" && (r["status"] != "implemented" || !strings.Contains(strings.Join(toStrings(r["commands"]), ","), "bwt submit content")) {
			t.Fatalf("SubmitContent row: %v", r)
		}
	}
}

func toStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.(string))
	}
	return out
}
