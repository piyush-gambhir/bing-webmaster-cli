# Bing Webmaster API coverage

Generated from the operation registry and the command tree by `make docs`. A test fails when this file
is out of date, when a documented method has no command, or when a command's read/write
classification disagrees with the registry.

The registry is checked against the pinned documentation snapshot described in [compatibility.md](compatibility.md):
62 methods, of which 50 are implemented, 9 are implemented as experimental, and 3 are never implemented.

Effect is behavior, not HTTP verb: `read` methods are allowed under `--read-only`; `write` methods are blocked.
Experimental commands print a notice on every run because their current behavior is unverified.

## Sites

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetUserSites` | GET | read | implemented | `bwt sites list`, `bwt users add` |  |
| `AddSite` | POST | write | implemented | `bwt sites add` |  |
| `VerifySite` | POST | write | implemented | `bwt sites verify` | changes verification state |
| `RemoveSite` | POST | write | implemented | `bwt sites remove` |  |

## Users and roles

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetSiteRoles` | GET | read | implemented | `bwt users list`, `bwt users remove` |  |
| `AddSiteRoles` | POST | write | implemented | `bwt users add` |  |
| `RemoveSiteRole` | POST | write | implemented | `bwt users remove` |  |

## URL and content submission

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `SubmitUrl` | POST | write | implemented | `bwt submit urls` |  |
| `SubmitUrlBatch` | POST | write | implemented | `bwt submit urls` | 500 URLs per request |
| `GetUrlSubmissionQuota` | GET | read | implemented | `bwt quota`, `bwt submit urls` |  |
| `SubmitContent` | POST | write | experimental | `bwt experimental submit-content` | docs conflict on whether OAuth is required; content can be indexed despite robots.txt |
| `GetContentSubmissionQuota` | GET | read | implemented | `bwt quota` |  |

## Sitemaps and feeds

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetFeeds` | GET | read | implemented | `bwt sitemaps list` |  |
| `GetFeedDetails` | GET | read | implemented | `bwt sitemaps get` |  |
| `SubmitFeed` | POST | write | implemented | `bwt sitemaps submit` |  |
| `RemoveFeed` | POST | write | implemented | `bwt sitemaps remove` |  |

## Search performance

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetQueryStats` | GET | read | implemented | `bwt stats queries` | top queries; updated weekly |
| `GetPageStats` | GET | read | implemented | `bwt stats pages` | top pages; updated weekly |
| `GetQueryPageStats` | GET | read | implemented | `bwt stats query-pages` |  |
| `GetPageQueryStats` | GET | read | implemented | `bwt stats page-queries` |  |
| `GetQueryPageDetailStats` | GET | read | implemented | `bwt stats detail` |  |
| `GetQueryTrafficStats` | GET | read | implemented | `bwt stats query-traffic` | updated daily |
| `GetRankAndTrafficStats` | GET | read | implemented | `bwt stats summary`, `bwt stats traffic` | updated daily |

## Crawl

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetCrawlStats` | GET | read | implemented | `bwt crawl stats` |  |
| `GetCrawlIssues` | GET | read | implemented | `bwt crawl issues` |  |
| `GetCrawlSettings` | GET | read | implemented | `bwt crawl set-settings`, `bwt crawl settings` |  |
| `SaveCrawlSettings` | POST | write | implemented | `bwt crawl set-settings` |  |

## URL information

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetUrlInfo` | GET | read | implemented | `bwt url info` |  |
| `GetUrlTrafficInfo` | GET | read | implemented | `bwt url traffic` |  |
| `GetChildrenUrlInfo` | POST | read | implemented | `bwt url children` | a read that uses POST |
| `GetChildrenUrlTrafficInfo` | GET | read | implemented | `bwt url children-traffic` |  |

## Links

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetLinkCounts` | GET | read | implemented | `bwt links counts` |  |
| `GetUrlLinks` | GET | read | implemented | `bwt links list` |  |
| `GetConnectedPages` | GET | read | implemented | `bwt connected-pages list` |  |
| `AddConnectedPage` | POST | write | implemented | `bwt connected-pages add` |  |

## Keyword research

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetKeyword` | GET | read | implemented | `bwt keywords get` |  |
| `GetKeywordStats` | GET | read | implemented | `bwt keywords stats` |  |
| `GetRelatedKeywords` | GET | read | implemented | `bwt keywords related` |  |

## Query parameters

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetQueryParameters` | GET | read | implemented | `bwt params list` |  |
| `AddQueryParameter` | POST | write | implemented | `bwt params add` |  |
| `RemoveQueryParameter` | POST | write | implemented | `bwt params remove` |  |
| `EnableDisableQueryParameter` | POST | write | implemented | `bwt params disable`, `bwt params enable` |  |

## Blocked URLs

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetBlockedUrls` | GET | read | implemented | `bwt block list`, `bwt block remove` |  |
| `AddBlockedUrl` | POST | write | implemented | `bwt block add` |  |
| `RemoveBlockedUrl` | POST | write | implemented | `bwt block remove` |  |

## Page preview blocks

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetActivePagePreviewBlocks` | GET | read | implemented | `bwt preview-blocks list` |  |
| `AddPagePreviewBlock` | POST | write | implemented | `bwt preview-blocks add` |  |
| `RemovePagePreviewBlock` | POST | write | implemented | `bwt preview-blocks remove` |  |

## Deep links

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetDeepLinkBlocks` | GET | read | experimental | `bwt experimental deeplink-blocks list` | current behavior unverified |
| `AddDeepLinkBlock` | POST | write | experimental | `bwt experimental deeplink-blocks add` | current behavior unverified |
| `RemoveDeepLinkBlock` | POST | write | experimental | `bwt experimental deeplink-blocks remove` | current behavior unverified |
| `GetDeepLink` | GET | read | obsolete | Never implemented | marked Obsolete by Microsoft; use GetDeepLinkBlocks |
| `GetDeepLinkAlgoUrls` | GET | read | obsolete | Never implemented | marked Obsolete by Microsoft; no longer used |
| `UpdateDeepLink` | POST | write | obsolete | Never implemented | marked Obsolete by Microsoft; use deep-link blocking |

## Geo-targeting

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetCountryRegionSettings` | GET | read | experimental | `bwt experimental geo list` | current behavior unverified |
| `AddCountryRegionSettings` | POST | write | experimental | `bwt experimental geo add` | current behavior unverified |
| `RemoveCountryRegionSettings` | POST | write | experimental | `bwt experimental geo remove` | current behavior unverified |

## Site moves

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `GetSiteMoves` | GET | read | experimental | `bwt experimental site-move list` | community reports HTTP 404 |
| `SubmitSiteMove` | POST | write | experimental | `bwt experimental site-move submit` | current behavior unverified |

## Fetch as Bingbot

| Method | HTTP | Effect | Status | Command | Notes |
| --- | --- | --- | --- | --- | --- |
| `FetchUrl` | POST | write | implemented | `bwt fetch request` | schedules a Bingbot fetch |
| `GetFetchedUrls` | GET | read | implemented | `bwt fetch list` |  |
| `GetFetchedUrlDetails` | GET | read | implemented | `bwt fetch get` |  |

## Beyond the Bing Webmaster API

| Feature | Handled by | Notes |
| --- | --- | --- |
| IndexNow protocol | `bwt indexnow key generate`, `bwt indexnow key check`, `bwt indexnow submit` | Needs no Bing credentials; the key is hosted on the site. Submissions reach every participating engine. |
| SOAP and POX (XML) protocols | Never implemented | Retired by Microsoft on 2026-08-31. The CLI uses the JSON endpoints only. |
| AI Performance and citation reports | Not available | Dashboard only (public preview February 2026, expanded June 2026); no public API method. |
| Recommendations and Top Insights | Not available | Dashboard only; no public API method. |
| Site Scan | Not available | Dashboard audit workflow; no public API method. |
| Backlink comparison | Not available | Dashboard only. `bwt links` covers the legacy GetLinkCounts and GetUrlLinks methods. |
| IndexNow Insights | Not available | Dashboard only; no public API method. |
| Bing Search APIs (Web Search API) | Out of scope | A separate product, retired 2025-08-11; it does not affect the Webmaster API. |
