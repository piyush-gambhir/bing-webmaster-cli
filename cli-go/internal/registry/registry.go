// Package registry lists every method of the Bing Webmaster JSON API with its
// HTTP verb, parameters, and effect. Commands, --read-only enforcement, the raw
// `api` command, and the coverage tests all read this table, so the effect of a
// call is decided once, by behavior rather than by HTTP verb.
package registry

import "sort"

type Effect string

const (
	Read  Effect = "read"
	Write Effect = "write"
)

type Status string

const (
	Implemented  Status = "implemented"
	Experimental Status = "experimental"
	Obsolete     Status = "obsolete"
)

// Param is one documented method parameter. Type uses the wire shape:
// string, bool, int16, uint16, int32, datetime, string[], or a data contract name.
type Param struct {
	Name string
	Type string
}

type Op struct {
	Name    string
	HTTP    string
	Params  []Param
	Returns string
	Effect  Effect
	Status  Status
	Group   string
	Note    string
}

func p(name, typ string) Param { return Param{Name: name, Type: typ} }

var site = p("siteUrl", "string")

// Ops is every method of IWebmasterApi (62), grouped as in the documentation.
var Ops = []Op{
	{Name: "GetUserSites", HTTP: "GET", Returns: "Site[]", Effect: Read, Status: Implemented, Group: "Sites"},
	{Name: "AddSite", HTTP: "POST", Params: []Param{site}, Returns: "void", Effect: Write, Status: Implemented, Group: "Sites"},
	{Name: "VerifySite", HTTP: "POST", Params: []Param{site}, Returns: "bool", Effect: Write, Status: Implemented, Group: "Sites", Note: "changes verification state"},
	{Name: "RemoveSite", HTTP: "POST", Params: []Param{site}, Returns: "void", Effect: Write, Status: Implemented, Group: "Sites"},

	{Name: "GetSiteRoles", HTTP: "GET", Params: []Param{site, p("includeAllSubdomains", "bool")}, Returns: "SiteRoles[]", Effect: Read, Status: Implemented, Group: "Users and roles"},
	{Name: "AddSiteRoles", HTTP: "POST", Params: []Param{site, p("delegatedUrl", "string"), p("userEmail", "string"), p("authenticationCode", "string"), p("isAdministrator", "bool"), p("isReadOnly", "bool")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Users and roles"},
	{Name: "RemoveSiteRole", HTTP: "POST", Params: []Param{site, p("siteRole", "SiteRoles")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Users and roles"},

	{Name: "SubmitUrl", HTTP: "POST", Params: []Param{site, p("url", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "URL and content submission"},
	{Name: "SubmitUrlBatch", HTTP: "POST", Params: []Param{site, p("urlList", "string[]")}, Returns: "void", Effect: Write, Status: Implemented, Group: "URL and content submission", Note: "500 URLs per request"},
	{Name: "GetUrlSubmissionQuota", HTTP: "GET", Params: []Param{site}, Returns: "UrlSubmissionQuota", Effect: Read, Status: Implemented, Group: "URL and content submission"},
	{Name: "SubmitContent", HTTP: "POST", Params: []Param{site, p("url", "string"), p("httpMessage", "string"), p("structuredData", "string"), p("dynamicServing", "int32")}, Returns: "void", Effect: Write, Status: Implemented, Group: "URL and content submission", Note: "works with an API key (verified 2026-10-04); content can be indexed despite robots.txt"},
	{Name: "GetContentSubmissionQuota", HTTP: "GET", Params: []Param{site}, Returns: "ContentSubmissionQuota", Effect: Read, Status: Implemented, Group: "URL and content submission"},

	{Name: "GetFeeds", HTTP: "GET", Params: []Param{site}, Returns: "Feed[]", Effect: Read, Status: Implemented, Group: "Sitemaps and feeds"},
	{Name: "GetFeedDetails", HTTP: "GET", Params: []Param{site, p("feedUrl", "string")}, Returns: "Feed[]", Effect: Read, Status: Implemented, Group: "Sitemaps and feeds"},
	{Name: "SubmitFeed", HTTP: "POST", Params: []Param{site, p("feedUrl", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Sitemaps and feeds"},
	{Name: "RemoveFeed", HTTP: "POST", Params: []Param{site, p("feedUrl", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Sitemaps and feeds"},

	{Name: "GetQueryStats", HTTP: "GET", Params: []Param{site}, Returns: "QueryStats[]", Effect: Read, Status: Implemented, Group: "Search performance", Note: "top queries; updated weekly"},
	{Name: "GetPageStats", HTTP: "GET", Params: []Param{site}, Returns: "QueryStats[]", Effect: Read, Status: Implemented, Group: "Search performance", Note: "top pages; updated weekly"},
	{Name: "GetQueryPageStats", HTTP: "GET", Params: []Param{site, p("query", "string")}, Returns: "QueryStats[]", Effect: Read, Status: Implemented, Group: "Search performance"},
	{Name: "GetPageQueryStats", HTTP: "GET", Params: []Param{site, p("page", "string")}, Returns: "QueryStats[]", Effect: Read, Status: Implemented, Group: "Search performance"},
	{Name: "GetQueryPageDetailStats", HTTP: "GET", Params: []Param{site, p("query", "string"), p("page", "string")}, Returns: "DetailedQueryStats[]", Effect: Read, Status: Implemented, Group: "Search performance"},
	{Name: "GetQueryTrafficStats", HTTP: "GET", Params: []Param{site, p("query", "string")}, Returns: "RankAndTrafficStats[]", Effect: Read, Status: Implemented, Group: "Search performance", Note: "updated daily"},
	{Name: "GetRankAndTrafficStats", HTTP: "GET", Params: []Param{site}, Returns: "RankAndTrafficStats[]", Effect: Read, Status: Implemented, Group: "Search performance", Note: "updated daily"},

	{Name: "GetCrawlStats", HTTP: "GET", Params: []Param{site}, Returns: "CrawlStats[]", Effect: Read, Status: Implemented, Group: "Crawl"},
	{Name: "GetCrawlIssues", HTTP: "GET", Params: []Param{site}, Returns: "UrlWithCrawlIssues[]", Effect: Read, Status: Implemented, Group: "Crawl"},
	{Name: "GetCrawlSettings", HTTP: "GET", Params: []Param{site}, Returns: "CrawlSettings", Effect: Read, Status: Implemented, Group: "Crawl"},
	{Name: "SaveCrawlSettings", HTTP: "POST", Params: []Param{site, p("crawlSettings", "CrawlSettings")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Crawl"},

	{Name: "GetUrlInfo", HTTP: "GET", Params: []Param{site, p("url", "string")}, Returns: "UrlInfo", Effect: Read, Status: Implemented, Group: "URL information"},
	{Name: "GetUrlTrafficInfo", HTTP: "GET", Params: []Param{site, p("url", "string")}, Returns: "UrlTrafficInfo", Effect: Read, Status: Implemented, Group: "URL information"},
	{Name: "GetChildrenUrlInfo", HTTP: "POST", Params: []Param{site, p("url", "string"), p("page", "uint16"), p("filterProperties", "FilterProperties")}, Returns: "UrlInfo[]", Effect: Read, Status: Implemented, Group: "URL information", Note: "a read that uses POST"},
	{Name: "GetChildrenUrlTrafficInfo", HTTP: "GET", Params: []Param{site, p("url", "string"), p("page", "uint16")}, Returns: "UrlTrafficInfo[]", Effect: Read, Status: Implemented, Group: "URL information"},

	{Name: "GetLinkCounts", HTTP: "GET", Params: []Param{site, p("page", "int16")}, Returns: "LinkCounts", Effect: Read, Status: Implemented, Group: "Links"},
	{Name: "GetUrlLinks", HTTP: "GET", Params: []Param{site, p("link", "string"), p("page", "int16")}, Returns: "LinkDetails", Effect: Read, Status: Implemented, Group: "Links"},
	{Name: "GetConnectedPages", HTTP: "GET", Params: []Param{site}, Returns: "ConnectedSite[]", Effect: Read, Status: Implemented, Group: "Links"},
	{Name: "AddConnectedPage", HTTP: "POST", Params: []Param{site, p("masterUrl", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Links"},

	{Name: "GetKeyword", HTTP: "GET", Params: []Param{p("q", "string"), p("country", "string"), p("language", "string"), p("startDate", "datetime"), p("endDate", "datetime")}, Returns: "Keyword", Effect: Read, Status: Implemented, Group: "Keyword research"},
	{Name: "GetKeywordStats", HTTP: "GET", Params: []Param{p("q", "string"), p("country", "string"), p("language", "string")}, Returns: "KeywordStats[]", Effect: Read, Status: Implemented, Group: "Keyword research"},
	{Name: "GetRelatedKeywords", HTTP: "GET", Params: []Param{p("q", "string"), p("country", "string"), p("language", "string"), p("startDate", "datetime"), p("endDate", "datetime")}, Returns: "Keyword[]", Effect: Read, Status: Implemented, Group: "Keyword research"},

	{Name: "GetQueryParameters", HTTP: "GET", Params: []Param{site}, Returns: "QueryParameter[]", Effect: Read, Status: Implemented, Group: "Query parameters"},
	{Name: "AddQueryParameter", HTTP: "POST", Params: []Param{site, p("queryParameter", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Query parameters"},
	{Name: "RemoveQueryParameter", HTTP: "POST", Params: []Param{site, p("queryParameter", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Query parameters"},
	{Name: "EnableDisableQueryParameter", HTTP: "POST", Params: []Param{site, p("queryParameter", "string"), p("isEnabled", "bool")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Query parameters"},

	{Name: "GetBlockedUrls", HTTP: "GET", Params: []Param{site}, Returns: "BlockedUrl[]", Effect: Read, Status: Implemented, Group: "Blocked URLs"},
	{Name: "AddBlockedUrl", HTTP: "POST", Params: []Param{site, p("blockedUrl", "BlockedUrl")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Blocked URLs"},
	{Name: "RemoveBlockedUrl", HTTP: "POST", Params: []Param{site, p("blockedUrl", "BlockedUrl")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Blocked URLs"},

	{Name: "GetActivePagePreviewBlocks", HTTP: "GET", Params: []Param{site}, Returns: "PagePreview[]", Effect: Read, Status: Implemented, Group: "Page preview blocks"},
	{Name: "AddPagePreviewBlock", HTTP: "POST", Params: []Param{site, p("url", "string"), p("reason", "BlockReason")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Page preview blocks"},
	{Name: "RemovePagePreviewBlock", HTTP: "POST", Params: []Param{site, p("url", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Page preview blocks"},

	{Name: "GetDeepLinkBlocks", HTTP: "GET", Params: []Param{site}, Returns: "DeepLinkBlock[]", Effect: Read, Status: Implemented, Group: "Deep links", Note: "verified 2026-10-04"},
	{Name: "AddDeepLinkBlock", HTTP: "POST", Params: []Param{site, p("market", "string"), p("searchUrl", "string"), p("deepLinkUrl", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Deep links", Note: "verified 2026-10-04; market must be lowercase (en-us)"},
	{Name: "RemoveDeepLinkBlock", HTTP: "POST", Params: []Param{site, p("market", "string"), p("searchUrl", "string"), p("deepLinkUrl", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Deep links", Note: "verified 2026-10-04; market must be lowercase (en-us)"},
	{Name: "GetDeepLink", HTTP: "GET", Params: []Param{site, p("url", "string")}, Returns: "DeepLink[]", Effect: Read, Status: Obsolete, Group: "Deep links", Note: "marked Obsolete by Microsoft; use GetDeepLinkBlocks"},
	{Name: "GetDeepLinkAlgoUrls", HTTP: "GET", Params: []Param{site}, Returns: "DeepLinkAlgoUrl[]", Effect: Read, Status: Obsolete, Group: "Deep links", Note: "marked Obsolete by Microsoft; no longer used"},
	{Name: "UpdateDeepLink", HTTP: "POST", Params: []Param{site, p("algoUrl", "string"), p("deepLink", "string"), p("weight", "DeepLinkWeight")}, Returns: "void", Effect: Write, Status: Obsolete, Group: "Deep links", Note: "marked Obsolete by Microsoft; use deep-link blocking"},

	{Name: "GetCountryRegionSettings", HTTP: "GET", Params: []Param{site}, Returns: "CountryRegionSettings[]", Effect: Read, Status: Implemented, Group: "Geo-targeting", Note: "verified 2026-10-04"},
	{Name: "AddCountryRegionSettings", HTTP: "POST", Params: []Param{site, p("settings", "CountryRegionSettings")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Geo-targeting", Note: "verified 2026-10-04; country code must be lowercase (us)"},
	{Name: "RemoveCountryRegionSettings", HTTP: "POST", Params: []Param{site, p("settings", "CountryRegionSettings")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Geo-targeting", Note: "verified 2026-10-04; country code must be lowercase (us)"},

	{Name: "GetSiteMoves", HTTP: "GET", Params: []Param{site}, Returns: "SiteMoveSettings[]", Effect: Read, Status: Experimental, Group: "Site moves", Note: "returned HTTP 404 in a live check (2026-10-04)"},
	{Name: "SubmitSiteMove", HTTP: "POST", Params: []Param{site, p("settings", "SiteMoveSettings")}, Returns: "void", Effect: Write, Status: Experimental, Group: "Site moves", Note: "current behavior unverified"},

	{Name: "FetchUrl", HTTP: "POST", Params: []Param{site, p("url", "string")}, Returns: "void", Effect: Write, Status: Implemented, Group: "Fetch as Bingbot", Note: "schedules a Bingbot fetch"},
	{Name: "GetFetchedUrls", HTTP: "GET", Params: []Param{site}, Returns: "FetchedUrl[]", Effect: Read, Status: Implemented, Group: "Fetch as Bingbot"},
	{Name: "GetFetchedUrlDetails", HTTP: "GET", Params: []Param{site, p("url", "string")}, Returns: "FetchedUrlDetails", Effect: Read, Status: Implemented, Group: "Fetch as Bingbot"},
}

var byName = func() map[string]Op {
	m := make(map[string]Op, len(Ops))
	for _, op := range Ops {
		m[op.Name] = op
	}
	return m
}()

// Lookup returns the operation with this exact Bing method name.
func Lookup(name string) (Op, bool) {
	op, ok := byName[name]
	return op, ok
}

// Names returns all method names, sorted.
func Names() []string {
	names := make([]string, 0, len(Ops))
	for _, op := range Ops {
		names = append(names, op.Name)
	}
	sort.Strings(names)
	return names
}
