# Bing Webmaster Tools API: research

Researched on 2026-10-03 by gpt-6.1-sol (Codex CLI, live web search, read-only) and curated by Claude.
Every claim links to its source, and community sources are labeled as such. No authenticated calls were
made, so behavior marked unverified needs a credentialed smoke test during the build.

Re-checked by Claude against primary sources on 2026-10-03:

- OAuth: authorize at `https://www.bing.com/webmasters/oauth/authorize`, token at
  `https://www.bing.com/webmasters/oauth/token` (form-encoded, `client_secret` required for both code
  exchange and refresh), scopes `Webmaster.read` and `Webmaster.manage`, redirect URI must match the
  registered one exactly, no PKCE documented, refresh tokens "valid until the user revokes access",
  example `expires_in` 3599. ([OAuth guide](https://learn.microsoft.com/en-us/bingwebmaster/oauth2))
- "Legacy SOAP and POX APIs will be retired on August 31, 2026." JSON calls use
  `https://ssl.bing.com/webmaster/api.svc/json/METHOD_NAME?apikey=API_KEY`.
  ([protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols))

Where this research recommends something different from [PLAN.md](PLAN.md) (binary name, OAuth timing),
PLAN.md records the decision and why.

Verified against a live account on 2026-10-04 (details in [docs/compatibility.md](docs/compatibility.md)):

- Bing's OAuth client registration rejects loopback redirect URIs (`http://127.0.0.1:47619/callback`,
  `http://localhost:47619/callback`), so OAuth cannot serve a CLI; the CLI logs in with an API key.
- `SubmitContent` works with an API key, resolving the documentation conflict noted below.
- `AddCountryRegionSettings` needs a lowercase country code and `AddDeepLinkBlock` a lowercase market;
  uppercase values return `InvalidParameter`.
- `GetUrlInfo`, `GetUrlTrafficInfo`, and the children methods return `UnknownError` for a site without crawl
  data; `GetSiteMoves` returns HTTP 404.

## 1. Summary

- **Build against JSON over HTTPS.** Microsoft announced retirement of the legacy SOAP and POX/XML services on **2026-08-31**. Current documentation retains the JSON endpoint family at `https://ssl.bing.com/webmaster/api.svc/json/{Method}`. [API overview](https://learn.microsoft.com/en-us/bingwebmaster/), [protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols).
- **The documented `IWebmasterApi` surface contains 62 methods.** Three carry explicit obsolete annotations: `GetDeepLink`, `GetDeepLinkAlgoUrls`, and `UpdateDeepLink`. Documentation presence does not establish that every remaining legacy feature works today. [Interface](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet).
- **Support API keys first, OAuth later.** One API key belongs to a user and covers that user’s verified sites. Microsoft recommends OAuth generally, but its documented flow requires a client secret and does not document a public-client, PKCE, or device-code flow. API keys fit this suite’s local profiles and unattended use more directly. [Access](https://learn.microsoft.com/en-us/bingwebmaster/getting-access), [OAuth](https://learn.microsoft.com/en-us/bingwebmaster/oauth2).
- **Handle WCF-style JSON deliberately.** Successful results use a `d` wrapper, successful writes can return `{"d":null}`, dates use Microsoft’s `/Date(...)/` format, and documented faults are unwrapped objects containing `ErrorCode` and `Message`. [Examples and faults](https://learn.microsoft.com/en-us/bingwebmaster/getting-started), [JSON serialization](https://learn.microsoft.com/en-us/dotnet/framework/wcf/feature-details/stand-alone-json-serialization).
- **Search performance is substantially less queryable than GSC.** The seven performance methods have no server-side date-range, row-limit, or pagination parameters. Several are described as updated weekly; site and query traffic are described as updated daily. Exact bucket semantics, current API retention, and top-row limits remain insufficiently documented. [Performance methods](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet), [GSC comparison](https://developers.google.com/webmaster-tools/v1/searchanalytics/query).
- **Treat submission quotas as live site-specific values.** The historical announcement says up to 10,000 URLs/day and no monthly quota, but the API still exposes both daily and monthly counters. A 2026 community observation reports both counters being enforced on one new site. Do not hard-code age tiers or universal quota values. [Official announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/), [community observation](https://pickuma.com/for-dev/bing-webmaster-api-100-url-daily-quota-in-practice/).
- **Implement IndexNow in v0.1.** It uses a separate key hosted on the website, needs no Bing account credentials, supports 10,000 URLs per batch, and is Microsoft’s recommended submission route. Keep Bing’s own URL submission commands for compatibility. [IndexNow documentation](https://www.indexnow.org/documentation), [FAQ](https://www.indexnow.org/faq), [Bing submission guidance](https://www.bing.com/webmasters/help/URL-Submission-62f2860b).
- **New dashboard features are not automatically API features.** AI Performance and citation reporting arrived in 2026, but I found no corresponding public `IWebmasterApi` methods. Recommendations, Site Scan, and modern backlink comparisons likewise lack matching methods in this interface. [AI announcement](https://blogs.bing.com/webmaster/2026/2/Introducing-AI-Performance-in-Bing-Webmaster-Tools-Public-Preview/), [interface](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet).
- **Classify operations by effect, not HTTP verb.** `GetChildrenUrlInfo` is a read operation implemented with POST. Conversely, IndexNow’s single-URL GET causes a submission and must be blocked by `--read-only`. [Children URL method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurlinfo?view=bing-webmaster-dotnet), [IndexNow](https://www.indexnow.org/documentation).
- **Recommend `bingwm`.** `bing` collides with a Debian/Ubuntu bandwidth tool; `bwt` collides with Bitcoin Wallet Tracker. No exact `bingwm` entry was found in the checked Homebrew catalogs or common-package searches. Existing Go tools already cover much of the API, so this CLI should differentiate through suite consistency, explicit network behavior, reliable error handling, and restrained dependencies. [Debian `bing`](https://packages.debian.org/bing), [Bitcoin `bwt`](https://github.com/bwt-dev/bwt), [Go Bing CLI](https://github.com/klixpert-io/bing-cli).

## 2. API surface

### Endpoint and permission notation

For every method below, the JSON URL is:

```text
https://ssl.bing.com/webmaster/api.svc/json/{Method}
```

API-key authentication adds `?apikey={key}`. GET parameters go in the query string; POST parameters go in a JSON object using the documented parameter names. The OAuth example instead uses `https://www.bing.com/webmaster/api.svc/json/SubmitUrl` with a bearer header. Host equivalence should be verified before standardizing OAuth on `ssl.bing.com`. [Protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols), [OAuth](https://learn.microsoft.com/en-us/bingwebmaster/oauth2).

Notation:

- `s` means `siteUrl: string`.
- Other parameters are strings unless explicitly typed.
- `T[]` means the documented `List<T>`.
- `void` normally produces `{"d":null}` in JSON.
- **R**: read operation; OAuth `Webmaster.read` or `Webmaster.manage`, or API key.
- **W**: write operation; OAuth `Webmaster.manage`, or API key.
- **A**: account-level operation.
- **S**: requires access to the specified site; verified ownership or delegated access normally applies.
- **Admin**: user-management operation requiring administrative authority.

Microsoft documents the two broad OAuth scopes, but **does not publish a complete method-by-method scope and site-role matrix**. The access classifications below follow those scope definitions and the documented operation effects. Site roles remain an additional authorization boundary. Read-only users can view reports; read/write users can change most settings; administrators can manage users. [OAuth scopes](https://learn.microsoft.com/en-us/bingwebmaster/oauth2), [site user permissions](https://www2.bing.com/webmasters/help/how-to-add-users-to-your-site-account-d5d00364).

### Sites and verification

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetUserSites](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getusersites?view=bing-webmaster-dotnet) | GET `/GetUserSites` | None | `Site[]` | R, A |
| [AddSite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addsite?view=bing-webmaster-dotnet) | POST `/AddSite` | `s` | `void` | W, A |
| [RemoveSite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removesite?view=bing-webmaster-dotnet) | POST `/RemoveSite` | `s` | `void` | W, S |
| [VerifySite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.verifysite?view=bing-webmaster-dotnet) | POST `/VerifySite` | `s` | `bool` | W; site already added, may be unverified |

### Roles and users

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetSiteRoles](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getsiteroles?view=bing-webmaster-dotnet) | GET `/GetSiteRoles` | `s`, `includeAllSubdomains: bool` | `SiteRoles[]` | R, S; exact role-list visibility undocumented |
| [AddSiteRoles](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addsiteroles?view=bing-webmaster-dotnet) | POST `/AddSiteRoles` | `s`, `delegatedUrl`, `userEmail`, `authenticationCode`, `isAdministrator: bool`, `isReadOnly: bool` | `void` | W, S, Admin |
| [RemoveSiteRole](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removesiterole?view=bing-webmaster-dotnet) | POST `/RemoveSiteRole` | `s`, `siteRole: SiteRoles` | `void` | W, S, Admin |

### URL and content submission

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [SubmitUrl](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submiturl?view=bing-webmaster-dotnet) | POST `/SubmitUrl` | `s`, `url` | `void` | W, S |
| [SubmitUrlBatch](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submiturlbatch?view=bing-webmaster-dotnet) | POST `/SubmitUrlBatch` | `s`, `urlList: string[]` | `void` | W, S |
| [GetUrlSubmissionQuota](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturlsubmissionquota?view=bing-webmaster-dotnet) | GET `/GetUrlSubmissionQuota` | `s` | `UrlSubmissionQuota` | R, S |
| [SubmitContent](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitcontent?view=bing-webmaster-dotnet) | POST `/SubmitContent` | `s`, `url`, `httpMessage`, `structuredData`, `dynamicServing: int32` | `void` | W, S; auth contradiction discussed below |
| [GetContentSubmissionQuota](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcontentsubmissionquota?view=bing-webmaster-dotnet) | GET `/GetContentSubmissionQuota` | `s` | `ContentSubmissionQuota` | R, S |

### Feeds and sitemaps

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetFeeds](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getfeeds?view=bing-webmaster-dotnet) | GET `/GetFeeds` | `s` | `Feed[]` | R, S |
| [GetFeedDetails](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getfeeddetails?view=bing-webmaster-dotnet) | GET `/GetFeedDetails` | `s`, `feedUrl` | `Feed[]` | R, S |
| [SubmitFeed](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitfeed?view=bing-webmaster-dotnet) | POST `/SubmitFeed` | `s`, `feedUrl` | `void` | W, S |
| [RemoveFeed](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removefeed?view=bing-webmaster-dotnet) | POST `/RemoveFeed` | `s`, `feedUrl` | `void` | W, S |

### Search performance

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetQueryStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getquerystats?view=bing-webmaster-dotnet) | GET `/GetQueryStats` | `s` | `QueryStats[]` | R, S |
| [GetPageStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getpagestats?view=bing-webmaster-dotnet) | GET `/GetPageStats` | `s` | `QueryStats[]` | R, S |
| [GetQueryPageStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getquerypagestats?view=bing-webmaster-dotnet) | GET `/GetQueryPageStats` | `s`, `query` | `QueryStats[]` | R, S |
| [GetPageQueryStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getpagequerystats?view=bing-webmaster-dotnet) | GET `/GetPageQueryStats` | `s`, `page` (page URL) | `QueryStats[]` | R, S |
| [GetQueryPageDetailStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getquerypagedetailstats?view=bing-webmaster-dotnet) | GET `/GetQueryPageDetailStats` | `s`, `query`, `page` (page URL) | `DetailedQueryStats[]` | R, S |
| [GetQueryTrafficStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getquerytrafficstats?view=bing-webmaster-dotnet) | GET `/GetQueryTrafficStats` | `s`, `query` | `RankAndTrafficStats[]` | R, S |
| [GetRankAndTrafficStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getrankandtrafficstats?view=bing-webmaster-dotnet) | GET `/GetRankAndTrafficStats` | `s` | `RankAndTrafficStats[]` | R, S |

### Crawl statistics, issues, and settings

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetCrawlStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcrawlstats?view=bing-webmaster-dotnet) | GET `/GetCrawlStats` | `s` | `CrawlStats[]` | R, S |
| [GetCrawlIssues](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcrawlissues?view=bing-webmaster-dotnet) | GET `/GetCrawlIssues` | `s` | `UrlWithCrawlIssues[]` | R, S |
| [GetCrawlSettings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcrawlsettings?view=bing-webmaster-dotnet) | GET `/GetCrawlSettings` | `s` | `CrawlSettings` | R, S |
| [SaveCrawlSettings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.savecrawlsettings?view=bing-webmaster-dotnet) | POST `/SaveCrawlSettings` | `s`, `crawlSettings: CrawlSettings` | `void` | W, S |

### URL information and traffic

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetUrlInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturlinfo?view=bing-webmaster-dotnet) | GET `/GetUrlInfo` | `s`, `url` | `UrlInfo` | R, S |
| [GetUrlTrafficInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturltrafficinfo?view=bing-webmaster-dotnet) | GET `/GetUrlTrafficInfo` | `s`, `url` | `UrlTrafficInfo` | R, S |
| [GetChildrenUrlInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurlinfo?view=bing-webmaster-dotnet) | **POST** `/GetChildrenUrlInfo` | `s`, `url`, `page: uint16`, `filterProperties: FilterProperties` | `UrlInfo[]` | **R**, S |
| [GetChildrenUrlTrafficInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurltrafficinfo?view=bing-webmaster-dotnet) | GET `/GetChildrenUrlTrafficInfo` | `s`, `url`, `page: uint16` | `UrlTrafficInfo[]` | R, S |

### Links, backlinks, and connected pages

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetLinkCounts](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getlinkcounts?view=bing-webmaster-dotnet) | GET `/GetLinkCounts` | `s`, `page: int16` | `LinkCounts` | R, S |
| [GetUrlLinks](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturllinks?view=bing-webmaster-dotnet) | GET `/GetUrlLinks` | `s`, `link` (target URL), `page: int16` | `LinkDetails` | R, S |
| [GetConnectedPages](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getconnectedpages?view=bing-webmaster-dotnet) | GET `/GetConnectedPages` | `s` | `ConnectedSite[]` | R, S |
| [AddConnectedPage](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addconnectedpage?view=bing-webmaster-dotnet) | POST `/AddConnectedPage` | `s`, `masterUrl` | `void` | W, S |

### Keyword research

These methods are market keyword research, distinct from a particular site’s performance.

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetKeyword](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getkeyword?view=bing-webmaster-dotnet) | GET `/GetKeyword` | `q`, `country`, `language`, `startDate: DateTime`, `endDate: DateTime` | `Keyword` | R, A |
| [GetKeywordStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getkeywordstats?view=bing-webmaster-dotnet) | GET `/GetKeywordStats` | `q`, `country`, `language` | `KeywordStats[]` | R, A |
| [GetRelatedKeywords](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getrelatedkeywords?view=bing-webmaster-dotnet) | GET `/GetRelatedKeywords` | `q`, `country`, `language`, `startDate: DateTime`, `endDate: DateTime` | `Keyword[]` | R, A |

### Query-parameter handling

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetQueryParameters](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getqueryparameters?view=bing-webmaster-dotnet) | GET `/GetQueryParameters` | `s` | `QueryParameter[]` | R, S |
| [AddQueryParameter](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addqueryparameter?view=bing-webmaster-dotnet) | POST `/AddQueryParameter` | `s`, `queryParameter` | `void` | W, S |
| [RemoveQueryParameter](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removequeryparameter?view=bing-webmaster-dotnet) | POST `/RemoveQueryParameter` | `s`, `queryParameter` | `void` | W, S |
| [EnableDisableQueryParameter](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.enabledisablequeryparameter?view=bing-webmaster-dotnet) | POST `/EnableDisableQueryParameter` | `s`, `queryParameter`, `isEnabled: bool` | `void` | W, S |

### Blocked URLs

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetBlockedUrls](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getblockedurls?view=bing-webmaster-dotnet) | GET `/GetBlockedUrls` | `s` | `BlockedUrl[]` | R, S |
| [AddBlockedUrl](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addblockedurl?view=bing-webmaster-dotnet) | POST `/AddBlockedUrl` | `s`, `blockedUrl: BlockedUrl` | `void` | W, S |
| [RemoveBlockedUrl](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removeblockedurl?view=bing-webmaster-dotnet) | POST `/RemoveBlockedUrl` | `s`, `blockedUrl: BlockedUrl` | `void` | W, S |

### Deep links and page-preview blocks

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetDeepLinkBlocks](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getdeeplinkblocks?view=bing-webmaster-dotnet) | GET `/GetDeepLinkBlocks` | `s` | `DeepLinkBlock[]` | R, S |
| [AddDeepLinkBlock](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.adddeeplinkblock?view=bing-webmaster-dotnet) | POST `/AddDeepLinkBlock` | `s`, `market`, `searchUrl`, `deepLinkUrl` | `void` | W, S |
| [RemoveDeepLinkBlock](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removedeeplinkblock?view=bing-webmaster-dotnet) | POST `/RemoveDeepLinkBlock` | `s`, `market`, `searchUrl`, `deepLinkUrl` | `void` | W, S |
| [GetDeepLink](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getdeeplink?view=bing-webmaster-dotnet) | GET `/GetDeepLink` | `s`, `url` | `DeepLink[]` | R, S; **obsolete** |
| [GetDeepLinkAlgoUrls](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getdeeplinkalgourls?view=bing-webmaster-dotnet) | GET `/GetDeepLinkAlgoUrls` | `s` | `DeepLinkAlgoUrl[]` | R, S; **obsolete** |
| [UpdateDeepLink](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.updatedeeplink?view=bing-webmaster-dotnet) | POST `/UpdateDeepLink` | `s`, `algoUrl`, `deepLink`, `weight: DeepLinkWeight` | `void` | W, S; **obsolete** |
| [GetActivePagePreviewBlocks](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getactivepagepreviewblocks?view=bing-webmaster-dotnet) | GET `/GetActivePagePreviewBlocks` | `s` | `PagePreview[]` | R, S |
| [AddPagePreviewBlock](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addpagepreviewblock?view=bing-webmaster-dotnet) | POST `/AddPagePreviewBlock` | `s`, `url`, `reason: BlockReason` | `void` | W, S |
| [RemovePagePreviewBlock](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removepagepreviewblock?view=bing-webmaster-dotnet) | POST `/RemovePagePreviewBlock` | `s`, `url` | `void` | W, S |

Some return types here live in the shared data-contract namespace rather than `Microsoft.Bing.Webmaster.Api.Interfaces`; this is a .NET organization detail, not a different HTTP service.

### Geo-targeting and site moves

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [GetCountryRegionSettings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcountryregionsettings?view=bing-webmaster-dotnet) | GET `/GetCountryRegionSettings` | `s` | `CountryRegionSettings[]` | R, S; current operation unverified |
| [AddCountryRegionSettings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addcountryregionsettings?view=bing-webmaster-dotnet) | POST `/AddCountryRegionSettings` | `s`, `settings: CountryRegionSettings` | `void` | W, S; current operation unverified |
| [RemoveCountryRegionSettings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.removecountryregionsettings?view=bing-webmaster-dotnet) | POST `/RemoveCountryRegionSettings` | `s`, `settings: CountryRegionSettings` | `void` | W, S; current operation unverified |
| [GetSiteMoves](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getsitemoves?view=bing-webmaster-dotnet) | GET `/GetSiteMoves` | `s` | `SiteMoveSettings[]` | R, S; community reports HTTP 404 |
| [SubmitSiteMove](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitsitemove?view=bing-webmaster-dotnet) | POST `/SubmitSiteMove` | `s`, `settings: SiteMoveSettings` | `void` | W, S; current operation unverified |

### Fetch as Bingbot

| Method | HTTP and JSON path | Parameters | Returns | Effect / permission |
|---|---|---|---|---|
| [FetchUrl](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.fetchurl?view=bing-webmaster-dotnet) | POST `/FetchUrl` | `s`, `url` | `void` | W, S |
| [GetFetchedUrls](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getfetchedurls?view=bing-webmaster-dotnet) | GET `/GetFetchedUrls` | `s` | `FetchedUrl[]` | R, S |
| [GetFetchedUrlDetails](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getfetchedurldetails?view=bing-webmaster-dotnet) | GET `/GetFetchedUrlDetails` | `s`, `url` | `FetchedUrlDetails` | R, S |

### Important method details

#### Sites and verification

`Site` contains `Url`, `IsVerified`, `AuthenticationCode`, and `DnsVerificationCode`. `AddSite` is documented as harmless when the site is already associated with the user. `VerifySite` returns a boolean; `false` must remain a meaningful result rather than a JSON-decoding failure. It can also fault when the site URL is invalid or not associated with the account. [Site contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.site?view=bing-webmaster-dotnet), [AddSite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.addsite?view=bing-webmaster-dotnet), [VerifySite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.verifysite?view=bing-webmaster-dotnet).

Use returned site URLs as identifiers. Verification options include a root `BingSiteAuth.xml` file, an `msvalidate.01` meta tag, DNS CNAME verification, Domain Connect, and importing verified GSC properties. The interface has no GSC-import operation. [Verification help](https://www.bing.com/webmasters/help/add-and-verify-site-12184f8b).

#### URL submission

```json
{
  "siteUrl": "https://example.com/",
  "urlList": [
    "https://example.com/a",
    "https://example.com/b"
  ]
}
```

`SubmitUrlBatch` allows **up to 500 URLs per request**. It returns no per-URL result list. A successful response therefore confirms the batch request, not individual indexing. [Batch method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submiturlbatch?view=bing-webmaster-dotnet).

Both quota contracts contain `DailyQuota` and `MonthlyQuota`; neither exposes a reset timestamp. [URL quota contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.urlsubmissionquota?view=bing-webmaster-dotnet), [content quota contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.contentsubmissionquota?view=bing-webmaster-dotnet).

#### Content submission

`httpMessage` is the base64 encoding of a complete HTTP response: status line, CRLF-terminated headers, an empty CRLF line, and optional body. `structuredData` is base64-encoded structured data, normally JSON-LD, or an empty string. `dynamicServing` values are:

| Value | Meaning |
|---:|---|
| 0 | None |
| 1 | PC/laptop |
| 2 | Mobile |
| 3 | AMP |
| 4 | Tablet |
| 5 | Nonvisual browser |

The method accepts up to **10 MB uncompressed content per request** and supports HTTP gzip compression. Its signature specifies an integer, although an example incorrectly shows `"dynamicServing":"0"` as a string. [SubmitContent](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitcontent?view=bing-webmaster-dotnet).

Content submission can behave differently from crawling: official help says submitted content may be indexed despite a robots.txt disallow, while `NOINDEX` is honored. This deserves explicit command documentation before implementing content submission. [Content submission FAQ](https://www4.bing.com/webmasters/help/url-submission-62f2860b).

#### Feeds and sitemaps

The feed API supports XML sitemaps, sitemap indexes, RSS 2.0, Atom 0.3/1.0, and text feeds. `GetFeedDetails` returns feeds contained in a sitemap index. `Feed` includes `Url`, `Type`, `Status`, `Submitted`, `LastCrawled`, `UrlCount`, `FileSize`, and `Compressed`. No paging parameter is documented. [SubmitFeed](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitfeed?view=bing-webmaster-dotnet), [GetFeedDetails](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getfeeddetails?view=bing-webmaster-dotnet), [Feed](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.feed?view=bing-webmaster-dotnet).

#### URL inspection and pagination

`UrlInfo` contains:

```text
Url, IsPage, AnchorCount, DiscoveryDate, LastCrawledDate,
DocumentSize, HttpStatus, TotalChildUrlCount
```

It does **not** contain a modern explicit `IsIndexed` field. Avoid presenting `IsPage` or a zero HTTP status as a definitive indexing verdict. `UrlTrafficInfo` contains `Url`, `IsPage`, `Clicks`, and `Impressions`, without dates or a documented measurement window. [UrlInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.urlinfo?view=bing-webmaster-dotnet), [UrlTrafficInfo](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.urltrafficinfo?view=bing-webmaster-dotnet).

Children methods use **zero-based pages**. The official enumeration example advances until an empty list. Link methods also start at page zero, but return `TotalPages`. Their page types differ: children use unsigned 16-bit integers; links use signed 16-bit integers. No caller-controlled page size is documented. [Children example](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurlinfo?view=bing-webmaster-dotnet), [link-count example](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getlinkcounts?view=bing-webmaster-dotnet), [URL links](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturllinks?view=bing-webmaster-dotnet).

`LinkCounts` contains `Links` with `Url` and `Count`. `LinkDetails` contains `Details` with `Url` and `AnchorText`. Both expose `TotalPages`. [LinkCounts](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.linkcounts?view=bing-webmaster-dotnet), [LinkDetails](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.linkdetails?view=bing-webmaster-dotnet), [LinkDetail](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.linkdetail?view=bing-webmaster-dotnet).

#### Crawl issue and filter enums

`UrlWithCrawlIssues` contains `Url`, `HttpCode`, `InLinks`, and an `Issues` bitmask:

| Flag | Value |
|---|---:|
| None | 0 |
| Code301 | 1 |
| Code302 | 2 |
| Code4xx | 4 |
| Code5xx | 8 |
| BlockedByRobotsTxt | 16 |
| ContainsMalware | 32 |
| ImportantUrlBlockedByRobotsTxt | 64 |
| DnsErrors | 128 |
| TimeOutErrors | 256 |

There is no dedicated `noindex` flag in this enum. [Issue contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.urlwithcrawlissues?view=bing-webmaster-dotnet), [issue enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.urlwithcrawlissues.crawlissues?view=bing-webmaster-dotnet).

`FilterProperties` accepts four numeric enum fields:

| Field | Documented values |
|---|---|
| `CrawlDateFilter` | Any=0, LastWeek=1, LastTwoWeeks=2, LastThreeWeeks=4 |
| `DiscoveredDateFilter` | Any=0, LastWeek=1, LastMonth=2 |
| `DocFlagsFilters` | Any=0, IsBlockedByRobotsTxt=1, IsMalware=2 |
| `HttpCodeFilters` | Any=0, Code2xx=1, Code3xx=2, Code301=4, Code302=8, Code4xx=16, Code5xx=32, AllOthers=64 |

The last two are flag sets. Preserve the nonsequential `LastThreeWeeks=4` value. [FilterProperties](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.filterproperties?view=bing-webmaster-dotnet), [crawl date](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.crawldatefilter?view=bing-webmaster-dotnet), [discovery date](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.discovereddatefilter?view=bing-webmaster-dotnet), [document flags](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.docflagsfilters?view=bing-webmaster-dotnet), [HTTP flags](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.httpcodefilters?view=bing-webmaster-dotnet).

`CrawlSettings` currently documents `CrawlBoostAvailable`, `CrawlBoostEnabled`, and `CrawlRate: byte[]`. Older method examples also show `AjaxEnabled` and a 24-element numeric array. **Go’s ordinary `[]byte` JSON encoding produces base64**, so matching the documented JSON array requires a custom representation. Valid rate ranges and the hour timezone need verification. [Contract](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.crawlsettings?view=bing-webmaster-dotnet), [save example](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.savecrawlsettings?view=bing-webmaster-dotnet).

#### Other write contracts

| Contract | Relevant fields and enums |
|---|---|
| `SiteRoles` | `Site`, `VerificationSite`, `Email`, delegation metadata, `Date`, `Expired`, `Role`; Administrator=0, ReadOnly=1, ReadWrite=2 |
| `BlockedUrl` | `Url`, `Date`, `DaysToExpire`, `EntityType`, `RequestType`; Page=0, Directory=1; CacheOnly=0, FullRemoval=1 |
| `QueryParameter` | `Parameter`, `IsEnabled`, `Date`, `Source` |
| `CountryRegionSettings` | `Url`, `Date`, `TwoLetterIsoCountryCode`, `Type`; Page=0, Directory=1, Domain=2, Subdomain=3 |
| `SiteMoveSettings` | `SourceUrl`, `TargetUrl`, `Date`, `MoveScope`, `MoveType`; Domain=0, Host=1, Directory=2; Local=0, Global=1 |
| `FetchedUrl` | `Url`, `Date`, `Fetched`, `Expired` |
| `FetchedUrlDetails` | `Url`, `Date`, `Status`, `Headers`, `Document` |

Sources: [roles](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.siteroles?view=bing-webmaster-dotnet), [role enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.siteroles.userrole?view=bing-webmaster-dotnet), [blocked URL](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.blockedurl?view=bing-webmaster-dotnet), [entity enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.blockedurl.blockedurlentitytype?view=bing-webmaster-dotnet), [request enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.blockedurl.blockedurlrequesttype?view=bing-webmaster-dotnet), [query parameter](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.queryparameter?view=bing-webmaster-dotnet), [geo settings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.countryregionsettings?view=bing-webmaster-dotnet), [geo enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.countryregionsettings.countryregionsettingstype?view=bing-webmaster-dotnet), [move settings](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.sitemovesettings?view=bing-webmaster-dotnet), [move scope](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.sitemovesettings.scope?view=bing-webmaster-dotnet), [move type](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.sitemovesettings.type?view=bing-webmaster-dotnet), [fetched URL](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.fetchedurl?view=bing-webmaster-dotnet), [fetch details](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.fetchedurldetails?view=bing-webmaster-dotnet).

### Deprecated or questionable methods

| Surface | Evidence | Recommended treatment |
|---|---|---|
| SOAP and POX/XML | Official retirement date: 2026-08-31 | Never implement |
| `GetDeepLink` | Official obsolete annotation directs callers to `GetDeepLinkBlocks` | Never implement |
| `GetDeepLinkAlgoUrls` | Official obsolete annotation says no longer used | Never implement |
| `UpdateDeepLink` | Official obsolete annotation points toward deep-link blocking | Never implement |
| `GetSiteMoves` | Community compatibility report records HTTP 404 | Defer until credentialed verification |
| Deep-link blocks, preview blocks, geo-targeting, query-parameter settings | Still documented, but current operational support is insufficiently established | Later, explicitly experimental |
| URL/content submission | Current help says still supported, with possible future deprecation | Keep URL submission; defer content submission |

Official references are linked in the method tables. Community runtime evidence: [PrintingPress compatibility notes](https://printingpress.dev/library/developer-tools/bing-webmaster). Current submission status: [Bing help](https://www.bing.com/webmasters/help/URL-Submission-62f2860b).

## 3. Authentication options

### API key

**End-user setup**

1. Sign into Bing Webmaster Tools.
2. Add and verify the relevant website.
3. Open **Settings → API Access → API Key**.
4. Accept the terms if prompted and generate a key.
5. Store it in a CLI profile or provide it through an environment variable.

Only **one API key per user** is documented. It is user-level, not site-level, and works across that user’s verified sites. Requests pass it as the `apikey` query parameter, including POST requests. Rotation requires deleting the current key and generating another, invalidating every application using the old key. No fixed expiration period is documented. [Access guide](https://learn.microsoft.com/en-us/bingwebmaster/getting-access), [request formats](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols).

An official content-submission setup article warns that a newly generated key can take approximately **30 minutes** to become effective. Treat this as a possible propagation delay, not a reason for automatic retries. [Setup guide](https://blogs.bing.com/webmaster/2021/5/Easy-set-up-guide-for-Bing%E2%80%99s-Content-Submission-API-%28Beta%29/).

| Advantages | Limitations |
|---|---|
| Simple local and CI setup | Broad user-level credential |
| No app registration or browser callback | No separate read-only API key documented |
| Fits flag → env → profile precedence | Query-string placement requires explicit redaction |
| No refresh-token machinery | Rotation affects all consumers |

**Recommendation:** API key in v0.1, with `--api-key`, `BINGWM_API_KEY`, and named profiles. Prefer interactive secret entry or environment variables over shell-history-visible flags.

### OAuth 2.0

Microsoft recommends OAuth. Registration happens in **Bing Webmaster Tools → Settings → API Access → OAuth Client**, providing a client name and redirect URI to obtain a client ID and secret.

| Step | Contract |
|---|---|
| Authorize | GET `https://www.bing.com/webmasters/oauth/authorize` |
| Authorization parameters | `client_id`, `response_type=code`, `redirect_uri`, `scope` |
| Token exchange | POST `https://www.bing.com/webmasters/oauth/token`, form encoded |
| Exchange fields | `client_id`, `client_secret`, `code`, `grant_type=authorization_code`, `redirect_uri` |
| API authentication | `Authorization: Bearer {access_token}` |
| Refresh fields | `client_id`, `client_secret`, `refresh_token`, `grant_type=refresh_token` |
| Scopes | `Webmaster.read`, `Webmaster.manage` |
| Authorization-code lifetime | 5 minutes |
| Access-token lifetime | Returned `expires_in`; example is 3,599 seconds |
| Refresh-token lifetime | Documented as valid until access is revoked |

Redirect URI matching is exact. Documentation inconsistencies include lowercase scope examples, a refresh table incorrectly listing `authorization_code`, and a refresh example omitting `/oauth` from the path. [OAuth guide](https://learn.microsoft.com/en-us/bingwebmaster/oauth2).

**Recommendation:** add OAuth later using user-supplied app credentials. It offers scoped delegation, but introduces registration, callback, refresh, and secure-token-storage work. Do not embed a shared client secret in an open-source binary. Verify scope casing, callback support, and host compatibility first. Refresh only during an explicit command invocation, with atomic profile updates.

### Content-submission auth contradiction

Current help states OAuth is required for content submission, but the same page includes an API-key request. The method reference and the official setup article, updated in November 2025, also demonstrate API keys. This is an unresolved documentation contradiction, not sufficient evidence that API keys have been disabled for that method. [Current help](https://www.bing.com/webmasters/help/URL-Submission-62f2860b), [method reference](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitcontent?view=bing-webmaster-dotnet), [setup article](https://blogs.bing.com/webmaster/2021/5/Easy-set-up-guide-for-Bing%E2%80%99s-Content-Submission-API-%28Beta%29/).

## 4. Quotas, limits, errors

### Documented limits and unresolved quota policy

| Item | Number or policy | Interpretation and source |
|---|---|---|
| Bing URL submission batch | **500 URLs/request** | API batch limit, independent of remaining quota. [Method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submiturlbatch?view=bing-webmaster-dotnet) |
| Adaptive URL submission | **Up to 10,000 URLs/day** | Historical official maximum; allocation depends on verified age, impressions, and other signals. Not every site gets 10,000. [Announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/) |
| Historical monthly policy | **No monthly quota**, according to 2019 announcement | Conflicts with surviving monthly counter and a 2026 community observation. Do not discard `MonthlyQuota`. [Announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/) |
| Old pre-2019 quota | **10/day, 50/month** | Superseded historical policy, not a current default. [Announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/) |
| Manual portal submission | **Up to 10,000/domain/day**, reset at **midnight UTC** | Current help explicitly describes the manual portal flow. Do not infer a universal API reset guarantee from it. [Help](https://www.bing.com/webmasters/help/URL-Submission-62f2860b) |
| Portal submission history | Latest **1,000** URLs | Display limit, not API batch size or quota. [Help](https://www.bing.com/webmasters/help/URL-Submission-62f2860b) |
| Quota allocation | Domain-level, not separate subdomain allocations | Documented submission policy. [Announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/) |
| Content submission payload | **10 MB uncompressed/request** | Gzip supported. Exact decimal-versus-binary interpretation is unspecified. [Method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submitcontent?view=bing-webmaster-dotnet) |
| Content submission count | Site-specific daily/monthly counters | No universal numeric allowance confirmed. Query `GetContentSubmissionQuota`. [Method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcontentsubmissionquota?view=bing-webmaster-dotnet) |
| IndexNow batch | **10,000 URLs/request** | Separate protocol and credentials. [Documentation](https://www.indexnow.org/documentation) |
| General BWT API rate | **No current numeric rate confirmed** | No authoritative requests/second, burst, or daily-call allowance found |
| Site-count maximum | **No current number confirmed** | `TooManySites` exists, but the error enum supplies no limit |
| Performance row limits and retention | **No current numeric API contract confirmed** | No paging or row-limit request parameters |

The age-based quota illustration in the 2019 article is historical and explicitly subject to change. I could not verify a current 2026 age-tier table, so none is presented as current policy.

**Community discrepancy:** Pickuma reports a site verified on 2026-08-18 receiving `DailyQuota=100` and `MonthlyQuota=1300`, with both counters decreasing during subsequent submissions. It also reports repeated successful submissions consuming quota. This is a single-site observation, not a universal quota announcement. [Observation](https://pickuma.com/for-dev/bing-webmaster-api-100-url-daily-quota-in-practice/).

### JSON success and error envelopes

Examples of valid successes:

```json
{"d":[{"Url":"https://example.com/","IsVerified":true}]}
```

```json
{"d":false}
```

```json
{"d":null}
```

Objects can include `__type` metadata. The client should tolerate it and preserve unknown fields. [GetUserSites](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getusersites?view=bing-webmaster-dotnet), [VerifySite](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.verifysite?view=bing-webmaster-dotnet), [SubmitUrl](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.submiturl?view=bing-webmaster-dotnet).

The documented JSON fault is **HTTP 400**, without a `d` wrapper:

```json
{"ErrorCode":3,"Message":"InvalidApiKey"}
```

Do not map every failure to HTTP 401, and do not assume throttling only arrives as HTTP 429. [Fault documentation](https://learn.microsoft.com/en-us/bingwebmaster/getting-started).

### Complete `ApiErrorCode` enum

| Code | Name | Code | Name |
|---:|---|---:|---|
| 0 | None | 9 | TooManySites |
| 1 | InternalError | 10 | UserNotFound |
| 2 | UnknownError | 11 | NotFound |
| 3 | InvalidApiKey | 12 | AlreadyExists |
| 4 | ThrottleUser | 13 | NotAllowed |
| 5 | ThrottleHost | 14 | NotAuthorized |
| 6 | UserBlocked | 15 | UnexpectedState |
| 7 | InvalidUrl | 16 | Deprecated |
| 8 | InvalidParameter | | |

[Official enum](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.apierrorcode?view=bing-webmaster-dotnet).

**Throttling:** codes 4 and 5 identify user and host throttling. I found no documented `Retry-After` guarantee or equivalent reset header. Preserve it when present, but do not invent one.

**Defensive handling:** recognize fault-shaped bodies even under HTTP 200, because community observations report this behavior. Handle non-JSON responses, HTTP 404, 429, and 5xx separately while retaining status and sanitized diagnostics. These are implementation recommendations, not a claim that Microsoft documents each status for every method. [Community fault observation](https://pickuma.com/for-dev/bing-webmaster-api-100-url-daily-quota-in-practice/).

### IndexNow protocol

Supported endpoints include:

```text
https://api.indexnow.org/indexnow
https://www.bing.com/indexnow
```

Use one participating endpoint; submissions are shared with other participating engines. An IndexNow key is distinct from the Bing Webmaster API key and requires no Bing account. The FAQ permits **8 to 128 characters**, using letters, digits, and hyphens. The protocol page calls it “hexadecimal” but also permits that wider alphabet; generate a hexadecimal key to satisfy both readings. [FAQ](https://www.indexnow.org/faq), [protocol](https://www.indexnow.org/documentation).

**Single URL:** GET with `url`, `key`, and optional `keyLocation`.

**Batch:** POST JSON:

```json
{
  "host": "example.com",
  "key": "0123456789abcdef0123456789abcdef",
  "keyLocation": "https://example.com/0123456789abcdef0123456789abcdef.txt",
  "urlList": [
    "https://example.com/a",
    "https://example.com/b"
  ]
}
```

Batch maximum: **10,000 URLs**. URLs may mix HTTP and HTTPS but must belong to the specified host. [Protocol](https://www.indexnow.org/documentation).

Host a UTF-8 text file containing the key at `/{key}.txt`, preferably at the root. A custom `keyLocation` must be on the same host. A file within `/catalog/` only authorizes that directory subtree under the protocol’s stated location rules. [Protocol](https://www.indexnow.org/documentation).

| Status | Meaning |
|---:|---|
| 200 | Submission received successfully |
| 202 | Received, key validation pending |
| 400 | Invalid request format |
| 403 | Key invalid or cannot be verified |
| 422 | Host/key/URL constraints violated |
| 429 | Too many requests, potentially spam |

Acceptance does not guarantee indexing. No fixed numeric daily IndexNow quota was confirmed. [Protocol](https://www.indexnow.org/documentation).

IndexNow notifies engines about added, changed, or deleted URLs. Bing’s `SubmitUrl` APIs are authenticated Bing-only submission methods. Neither replaces sitemaps. **Implement IndexNow in v0.1**, use its own config fields, and classify both GET and POST submissions as writes. [FAQ](https://www.indexnow.org/faq), [Bing guidance](https://www.bing.com/webmasters/help/URL-Submission-62f2860b).

## 5. Data semantics and gotchas

### Search performance methods

| Method | Result meaning | Documented update cadence | Server filtering |
|---|---|---|---|
| `GetQueryStats` | Top queries and their statistics | Weekly | Site only |
| `GetPageStats` | Top pages and their statistics | Weekly | Site only |
| `GetQueryPageStats` | Pages associated with a query | Weekly | Exact query argument |
| `GetPageQueryStats` | Queries associated with a page | Weekly | Page URL argument |
| `GetQueryPageDetailStats` | Position detail for query/page pair | Weekly | Query and page |
| `GetQueryTrafficStats` | Traffic history for a query | Daily | Query; only top-query history is saved |
| `GetRankAndTrafficStats` | Site traffic history | Daily | Site only |

Sources: the individual method references linked in Section 2.

**Granularity caveat:** the documentation’s weekly-versus-daily wording primarily establishes update cadence. It does not fully define whether every returned `Date` identifies a day, week start, week end, or another aggregation boundary. Preserve each row and timestamp. Do not automatically interpolate weekly rows into daily data.

**No server-side date ranges:** none of these seven signatures accepts `startDate`, `endDate`, `rowLimit`, a cursor, or page number. Microsoft’s own example filters `GetRankAndTrafficStats` results locally. CLI date flags must explicitly describe local filtering of available results. [Official example](https://learn.microsoft.com/en-us/bingwebmaster/getting-started).

**Response fields:**

| Type | Fields |
|---|---|
| `QueryStats` | `Query`, `Date`, `Clicks`, `Impressions`, `AvgClickPosition`, `AvgImpressionPosition` |
| `DetailedQueryStats` | `Date`, `Clicks`, `Impressions`, `Position` |
| `RankAndTrafficStats` | `Date`, `Clicks`, `Impressions` |

For page-oriented methods returning `QueryStats`, the reused `Query` field can contain a page URL. Label it as “Page” in tables while preserving the wire field in machine output. [QueryStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.querystats?view=bing-webmaster-dotnet), [DetailedQueryStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.detailedquerystats?view=bing-webmaster-dotnet), [RankAndTrafficStats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.rankandtrafficstats?view=bing-webmaster-dotnet).

**Positions:** the official average-position properties are integers. Community integrations divide them by ten, but the primary property documentation does not explain that scale. Preserve raw values initially and verify against a live site before exposing normalized position values. [Official property](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.querystats.avgclickposition?view=bing-webmaster-dotnet), [community example](https://gist.github.com/MartijnSch/4f1696f2c539740222818cb8d5076e24).

**Retention:** the dashboard gained **16 months** of search performance history in October 2024. The announcement does not establish the same retention for these API methods. Older help and community statements about shorter retention should not be promoted into a current API guarantee. [Dashboard announcement](https://blogs.bing.com/webmaster/2024/10/Bing-Webmaster-Tools-Extends-Search-Performance-Data-to-16-Months/).

**Coverage:** the rank-and-traffic method documents inclusion of multiple search verticals from March 2023. Bing’s September 2023 explanation distinguishes aggregate traffic across surfaces from keyword/page detail that remained web-specific. The API has no documented vertical filter, and aggregate chat traffic is not equivalent to the new citation report. [Method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getrankandtrafficstats?view=bing-webmaster-dotnet), [performance explanation](https://blogs.bing.com/webmaster/2023/9/Unlocking-insights-with-the-new-Bing-Webmaster-Tools-Performance-Report/).

**Completeness:** top-query/page lists should not be summed and presented as exhaustive site totals. Current documentation provides no clear top-N count, privacy threshold, anonymization rule, or sampling guarantee. Country and language parameters in keyword research are not performance-report filters.

### Brief contrast with Google Search Console

| Capability | Bing Webmaster performance API | GSC Search Analytics API |
|---|---|---|
| Date selection | No range parameters | Required inclusive start/end dates |
| Timezone contract | Insufficiently specified | Dates use Pacific Time |
| Dimensions | Fixed endpoint-specific shapes | Date/hour, query, page, device, country, search appearance |
| Filters | Query/page selectors | Dimension filters, including regex |
| Paging | None on performance methods | `startRow` and `rowLimit` |
| Request row limit | Undocumented | 1 to 25,000; default 1,000 |
| Exhaustiveness | Top data, unspecified caps | Also does not guarantee every row |

[GSC official contract](https://developers.google.com/webmaster-tools/v1/searchanalytics/query).

### Dates and timezones

Microsoft JSON dates have this form:

```text
/Date(1315349995284-0700)/
```

The number represents milliseconds since the Unix epoch; the optional signed offset reflects local-time information. Negative millisecond values are valid, including sentinel dates in examples. Escaped slashes may appear on the wire. Do not add the offset a second time to the epoch instant. [WCF serialization](https://learn.microsoft.com/en-us/dotnet/framework/wcf/feature-details/stand-alone-json-serialization), [Bing example](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurlinfo?view=bing-webmaster-dotnet).

For typed dates inside POST data contracts, use the documented Microsoft JSON representation. For keyword GET `DateTime` query parameters, the current method references do not clearly establish an accepted lexical format. Date-only formats used by community clients are evidence of client behavior, not a definitive vendor contract.

Bing examples use `-0700` and `-0800`, but that is insufficient to declare every report globally Pacific Time. Machine output should preserve raw dates; table formatting should retain the supplied offset rather than silently applying the computer’s timezone.

### Site identifiers and verification

- Preserve the exact `GetUserSites[].Url` value for subsequent requests. Samples vary in scheme and trailing slash, so blanket normalization to HTTPS or adding/removing a slash is risky.
- The portal supports domain and branch properties. The API continues to use a string `siteUrl`; no GSC-style `sc-domain:` identifier is documented.
- Some URL-information methods explicitly accept `domain:example.com` for their **`url` argument**. This does not establish that it is valid as `siteUrl`.
- Adding a site and proving ownership are separate steps. Read scopes do not establish access to arbitrary sites.

Sources: [site list](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getusersites?view=bing-webmaster-dotnet), [verification help](https://www.bing.com/webmasters/help/add-and-verify-site-12184f8b), [URL info](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.geturlinfo?view=bing-webmaster-dotnet), [children URL info](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getchildrenurlinfo?view=bing-webmaster-dotnet).

### Freshness and dashboard differences

`GetCrawlStats` is described as updated daily. Fixed crawl issues may remain listed for several days. Older official help advises allowing approximately 48 hours for initial report processing, but it is not a current SLA for every API method. [Crawl stats](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcrawlstats?view=bing-webmaster-dotnet), [crawl issues](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getcrawlissues?view=bing-webmaster-dotnet), [report help](https://www2.bing.com/webmasters/help/refreshed-webmaster-tools-7c7d2533).

Differences between API and UI can result from unequal retention, freshness, top-row coverage, report scope, and search-surface inclusion. Do not promise reconciliation without checking the same site, dates, and report definitions.

### Known problems and documentation defects

| Finding | Evidence level | CLI implication |
|---|---|---|
| `GetSiteMoves` returns HTTP 404; obsolete deep-link reads return code 16 | Community compatibility testing | Preserve status/code; do not expose as stable functionality. [PrintingPress](https://printingpress.dev/library/developer-tools/bing-webmaster) |
| Empty link results despite nonzero crawl `InLinks`; empty issues despite aggregate crawl errors; zero URL HTTP status | Community site-level testing, September 2026 | Empty or zero-valued legacy results cannot establish that a site is healthy. [API observations](https://github.com/stufently/bing-webmaster-ai-cli-mcp/blob/main/docs/api-surface.md) |
| HTTP 200 fault envelopes and quota consumption on repeated submissions | Community observation | Inspect response shape; no automatic retry. [Pickuma](https://pickuma.com/for-dev/bing-webmaster-api-100-url-daily-quota-in-practice/) |
| Clicks exceeding impressions | Community report, June 2026 | Preserve raw values; do not clamp CTR to 100%. [Report](https://note.com/honeymarron_dev/n/n8dfd69eda0fd?hl=en) |
| `GetQueryPageDetailStats` example differs from `DetailedQueryStats.Position` contract | Official documentation inconsistency | Decode defensively; test both documented shapes. [Method](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getquerypagedetailstats?view=bing-webmaster-dotnet) |
| `GetRelatedKeywords` and `GetBlockedUrls` contain copied examples for unrelated methods | Official documentation defects | Follow signatures and operation names rather than copying examples verbatim. [Keywords](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getrelatedkeywords?view=bing-webmaster-dotnet), [blocks](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi.getblockedurls?view=bing-webmaster-dotnet) |
| Some GET examples inconsistently quote string parameters | Official example inconsistency | Verify string serialization; avoid adding undocumented quote characters globally |

These observations do not prove that the same behavior affects every account.

## 6. Recent changes, 2024-2026

| Date | Change | API-design consequence |
|---|---|---|
| **2024-03-03** | IndexNow Insights introduced richer submission diagnostics, including latest-URL status and issues. [Announcement](https://blogs.bing.com/webmaster/2024/3/Optimize-your-Impact-with-IndexNow-Insights/) | No matching public interface method found |
| **2024-03-05** | Top Insights introduced prioritized visibility and SEO guidance. [Announcement](https://blogs.bing.com/webmaster/2024/3/What%E2%80%99s-New-Top-Insights-for-Maximum-Visibility-and-Performance/) | Dashboard feature |
| **2024-10-16** | Recommendations evolved and expanded Insights. [Announcement](https://blogs.bing.com/webmaster/2024/10/Introducing-Recommendations-Bing-Webmaster-Tools-Next-Evolution-in-Analytics-and-SEO/) | No documented recommendations endpoint found |
| **2024-10-16** | Search Performance history extended to 16 months. [Announcement](https://blogs.bing.com/webmaster/2024/10/Bing-Webmaster-Tools-Extends-Search-Performance-Data-to-16-Months/) | UI retention confirmed; API parity unconfirmed |
| **2025-03-18** | Search Performance gained easier period comparisons and time filtering. [Announcement](https://blogs.bing.com/webmaster/2025/3/Supercharge-Your-Search-Performance-with-Bing-Webmaster-Tools/) | Existing API signatures still lack date-range parameters |
| **2025-05-16**, effective **2025-08-11** | Microsoft announced retirement of the separate Bing Search APIs. [Lifecycle notice](https://learn.microsoft.com/en-us/lifecycle/announcements/bing-search-api-retirement) | This notice concerns Bing Search APIs, not the Webmaster API |
| **2025-10-15** | Bing added `data-nosnippet` HTML support. [Announcement](https://blogs.bing.com/webmaster/2025/10/Bing-Introduces-Support-for-the-data-nosnippet-HTML-Attribute/) | Content directive; not a new Webmaster API method or replacement contract for preview blocks |
| **November 2025**, day unspecified | Existing URL/content submission articles were updated to recommend IndexNow while retaining direct submission support. [URL article](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/), [content article](https://blogs.bing.com/webmaster/2021/5/Easy-set-up-guide-for-Bing%E2%80%99s-Content-Submission-API-%28Beta%29/) | Prioritize IndexNow; no dated direct-submission retirement established |
| **2026-02-10** | AI Performance entered public preview for citations across Copilot, Bing AI summaries, and selected partner experiences. Includes citation activity, cited pages, and grounding-query samples. [Announcement](https://blogs.bing.com/webmaster/2026/2/Introducing-AI-Performance-in-Bing-Webmaster-Tools-Public-Preview/) | No public citation endpoint announced in the reviewed material |
| **2026-06-16** | AI visibility reporting expanded with Intents, Topics, Citation Share, and Compare. [Announcement](https://blogs.bing.com/search/2026/6/New-AI-Visibility-Insights-in-Bing-Webmaster-Tools-Intents-Topics-Citation-Share-Compare/) | Citation share is a citation-visibility measure, not search CTR or ranking |
| **2026-08-07 / 2026-08-10** documentation updates; retirement **2026-08-31** | API overview and protocol docs announced SOAP/POX retirement. [Overview](https://learn.microsoft.com/en-us/bingwebmaster/), [protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols) | JSON-only new implementation |

**Search API retirement distinction:** the lifecycle notice does not announce Webmaster API retirement. Continued Webmaster documentation and its separate August 2026 protocol migration notice support the conclusion that the Webmaster JSON API is a different service.

**Site Scan and backlinks:** Site Scan remains documented as a portal audit workflow. The reviewed public interface has no start-scan, scan-result, recommendations, competitor-backlink-comparison, or AI-performance methods. Its old link methods should not be presented as equivalent to the full modern dashboard. [Site Scan help](https://www.bing.com/webmasters/help/site-scan-623520c9), [interface](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet).

I found no authoritative 2024-2026 announcement establishing a new general API rate limit, a new current age-tier quota schedule, or a new OAuth policy.

## 7. Existing tools

Versions below are the release or source versions visible during research. Coverage descriptions are project claims unless explicitly identified as inspected behavior.

### Official Microsoft offerings

| Offering | Current evidence | Coverage / gap |
|---|---|---|
| .NET API contracts and generated service-reference examples | Microsoft documents `Microsoft.Bing.Webmaster.Api.Interfaces.dll`; examples generate a SOAP client from WSDL. [Reference](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces?view=bing-webmaster-dotnet), [getting started](https://learn.microsoft.com/en-us/bingwebmaster/getting-started) | Useful contract documentation; legacy SOAP examples are unsuitable for the new CLI |
| Bing URL Submission WordPress plugin | Official Microsoft repository; no GitHub release version established. [Repository](https://github.com/microsoft/bing-wordpress-url-submission-plugin) | Automatic and manual URL submission, API-key setup, submission history; not an analytics CLI |
| Official CLI, Go SDK, or Webmaster MCP server | None found in the reviewed Microsoft documentation, repositories, and searches. [Microsoft MCP catalog](https://github.com/microsoft/mcp) | Negative search finding, not proof that no unpublished or future tool exists |

### Community tools and libraries

| Project | Language / version seen | Coverage | Gaps or differences relevant to this suite |
|---|---|---|---|
| [klixpert-io/bing-cli](https://github.com/klixpert-io/bing-cli) | Go, [v0.2.0, 2026-08-09](https://github.com/klixpert-io/bing-cli/releases/tag/v0.2.0) | Claims all 62 methods, agent skill, multiple output formats, caching, batching, retries | No OAuth or IndexNow advertised; inclusion of obsolete methods; cache/retry behavior differs from suite conventions |
| [ncosentino/bing-webmaster-mcp](https://github.com/ncosentino/bing-webmaster-mcp) | Go and C#, [v0.1.0, 2026-08-05](https://github.com/ncosentino/bing-webmaster-mcp/releases/tag/v0.1.0) | Broad tool surface, API key, IndexNow, native binaries, stdio/HTTP hosting | MCP server rather than noun/verb CLI; Go API implementation resides in an internal package, not a generally importable SDK |
| [merj/bing-webmaster-tools](https://github.com/merj/bing-webmaster-tools) | Python, [v1.2.0, 2025-04-26](https://pypi.org/project/bing-webmaster-tools/) | Async client with broad Webmaster services | Library rather than suite CLI; its rate-limit defaults and retry policy are client choices, not Bing limits |
| [zizzfizzix/mcp-server-bwt](https://github.com/zizzfizzix/mcp-server-bwt) | Python, [v0.2.0, 2026-09-25](https://pypi.org/project/mcp-server-bwt/) | Broad method wrappers, caching, tool-level pagination | Includes legacy methods; tool pagination over downloaded results must not be mistaken for server pagination |
| [isiahw1/mcp-server-bing-webmaster](https://github.com/isiahw1/mcp-server-bing-webmaster) | Node launcher/Python server, [source v1.0.2](https://raw.githubusercontent.com/isiahw1/mcp-server-bing-webmaster/main/package.json) | Broad API-key-based tools | MCP workflow; no OAuth or IndexNow advertised in reviewed material |
| [urdigitalau/mcp-integrations](https://github.com/urdigitalau/mcp-integrations) | TypeScript, [`@urdigital/mcp-server-bing-webmaster` source v0.1.3](https://raw.githubusercontent.com/urdigitalau/mcp-integrations/main/packages/bing-webmaster/package.json) | Smaller set covering common traffic, query, crawl, URL, sitemap, and keyword tasks | Partial surface; OAuth listed as future work |
| [NmadeleiDev/bing_webmaster_cli](https://github.com/NmadeleiDev/bing_webmaster_cli) | Python, [v0.1.3, 2026-03-26](https://pypi.org/project/bing-webmaster-cli/) | `bwm` binary; sites, traffic, URL checks/submission, local date filtering | Limited method coverage; its “index check” interpretation should not replace analysis of the official `UrlInfo` contract |
| [stufently/bing-webmaster-ai-cli-mcp](https://github.com/stufently/bing-webmaster-ai-cli-mcp) | Python, [source v0.1.0](https://raw.githubusercontent.com/stufently/bing-webmaster-ai-cli-mcp/main/pyproject.toml) | `bing-wm` binary, CLI/MCP, supported-method inventory excluding three obsolete methods, IndexNow, audit and plan/apply workflows | Different configuration and workflow model; useful compatibility observations, not an official contract |
| [PrintingPress Bing Webmaster tool](https://printingpress.dev/library/developer-tools/bing-webmaster) | Go-based CLI; standalone version not confirmed | Broad API coverage plus local analytics workflows; documents excluded obsolete/erroring methods | Platform-specific packaging and local storage; not a minimal client library |
| [seo-meow/bing-webmaster-api](https://github.com/seo-meow/bing-webmaster-api) | Rust, [source v1.0.2](https://raw.githubusercontent.com/seo-meow/bing-webmaster-api/master/Cargo.toml) | Async Rust API library | Not a Go dependency; published-registry version and complete current coverage not independently established |
| [webjeyros/bing-webmaster-api](https://packagist.org/packages/webjeyros/bing-webmaster-api) | PHP, `dev-master`; latest visible package commit from 2020 | Older API wrapper | No current tagged release established; legacy maintenance risk |

**Go conclusion:** useful implementations exist, but I did not find an official, maintained, standalone Go SDK with a current JSON contract and the suite’s desired behavior. A small standard-library client is preferable to importing another CLI’s internal code.

**What this CLI should do differently**

- Make network calls predictable and explicit.
- Preserve raw evidence instead of claiming definitive indexing or health from incomplete fields.
- Exclude obsolete methods.
- Separate real server pagination from local filtering.
- Support IndexNow alongside analytics.
- Apply consistent profiles, read-only enforcement, output formats, errors, and release tooling.

## 8. Binary name check

| Candidate | Checked collisions | Assessment |
|---|---|---|
| `bwt` | [Bitcoin Wallet Tracker](https://github.com/bwt-dev/bwt) uses this name and executable | Reject: exact established executable collision |
| `bing` | Debian and Ubuntu package a bandwidth tester named `bing`. [Debian](https://packages.debian.org/bing), [Ubuntu](https://packages.ubuntu.com/noble/arm64/net/bing) | Reject: exact system-package collision and overly broad product name |
| `bing-webmaster` | Crowded project/MCP namespace; PrintingPress uses this catalog identifier, with a longer CLI executable name | Viable descriptive fallback, but long and less distinctive |
| `bingwm` | No exact established executable or common-package collision found in the checked searches | Recommended |

The checked [Homebrew core catalog](https://formulae.brew.sh/formula/) and [cask catalog](https://formulae.brew.sh/cask/) contained no exact entries for these four names. Related entries such as `bing-wallpaper` are not exact collisions. Third-party taps and every Linux distribution were not exhaustively checked.

Nearby community executables are **`bwm`** and **`bing-wm`**, which are distinct but could cause verbal confusion. [Python CLI](https://pypi.org/project/bing-webmaster-cli/), [CLI/MCP source](https://raw.githubusercontent.com/stufently/bing-webmaster-ai-cli-mcp/main/pyproject.toml).

**Recommendation:** binary `bingwm`, repository `piyush-gambhir/bing-webmaster-cli`, environment prefix `BINGWM_`, and a distinct config directory such as `$XDG_CONFIG_HOME/bingwm-cli/config.yaml`.

## 9. Recommendations for the CLI (moved)

The report's CLI recommendations were folded into [PLAN.md](PLAN.md), which records each decision and its reason.

## 10. Open questions / unverified

These should be resolved through a small, explicit credentialed compatibility exercise before declaring full API support:

1. **OAuth:** exact scope casing, PKCE/public-client/device-flow support, localhost callback acceptance, and bearer-token support on the canonical `ssl.bing.com` host.
2. **Performance:** exact row bucket boundaries, timezone, current retention, top-N limits, average-position scaling, and current vertical coverage of query/page methods.
3. **Request serialization:** accepted GET `DateTime` forms and inconsistent quoted-string examples.
4. **Quotas:** current verified-age tiers, monthly enforcement policy, API reset timing, and duplicate-submission accounting.
5. **Content submission:** API-key acceptance versus the current OAuth-required statement.
6. **Legacy operations:** actual availability of site moves, geo-targeting, preview/deep-link blocks, connected pages, and query-parameter settings.
7. **Crawl settings:** valid hourly-rate range, timezone, and supported fields beyond the current contract.
8. **Undocumented limits:** general rate/burst limits, page sizes, site-count maximum, retry headers, and keyword date-range/row limits.
9. **Modern reports:** whether Microsoft offers any separately documented public API for AI Performance, Site Scan, Recommendations, or modern backlink comparisons. None was found in this research.
10. **Registry completeness:** some community versions were confirmed from source rather than publication registries; third-party Homebrew taps and all distribution packages were not exhaustively searched.

No authenticated runtime behavior is claimed as personally verified.

## 11. Sources

The 62 individual method references and relevant contract/enum pages are linked beside their entries in Section 2. The following numbered index covers the principal guides, announcements, comparisons, and implementation references.

1. [Bing Webmaster API overview](https://learn.microsoft.com/en-us/bingwebmaster/): current service overview and SOAP/POX retirement notice.
2. [Supported API protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols): canonical JSON paths and retirement date.
3. [IWebmasterApi](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet): complete documented method inventory.
4. [Getting access](https://learn.microsoft.com/en-us/bingwebmaster/getting-access): user-level API keys, setup, rotation, and OAuth recommendation.
5. [OAuth guide](https://learn.microsoft.com/en-us/bingwebmaster/oauth2): registration, authorization/token endpoints, scopes, refresh, and lifetimes.
6. [Getting started](https://learn.microsoft.com/en-us/bingwebmaster/getting-started): legacy SOAP examples, local date filtering, JSON faults and HTTP 400.
7. [ApiErrorCode](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.apierrorcode?view=bing-webmaster-dotnet): complete numeric error enum.
8. [WCF JSON serialization](https://learn.microsoft.com/en-us/dotnet/framework/wcf/feature-details/stand-alone-json-serialization): Microsoft JSON dates and data-contract serialization.
9. [Add and verify a site](https://www.bing.com/webmasters/help/add-and-verify-site-12184f8b): property and verification workflows.
10. [Site user permissions](https://www2.bing.com/webmasters/help/how-to-add-users-to-your-site-account-d5d00364): administrator, read/write, and read-only roles.
11. [Adaptive submission announcement](https://blogs.bing.com/webmaster/2019/1/bingbot-Series-Get-your-content-indexed-fast-by-now-submitting-up-to-10%2C000-URLs-per-day-to-Bing/): historical quota policy and November 2025 IndexNow update.
12. [Current URL submission help](https://www.bing.com/webmasters/help/URL-Submission-62f2860b): continued support, manual limits, and content-auth contradiction.
13. [Content submission setup](https://blogs.bing.com/webmaster/2021/5/Easy-set-up-guide-for-Bing%E2%80%99s-Content-Submission-API-%28Beta%29/): API-key setup and November 2025 support statement.
14. [Content submission FAQ](https://www4.bing.com/webmasters/help/url-submission-62f2860b): content size, robots/noindex behavior, and submission caveats.
15. [IndexNow documentation](https://www.indexnow.org/documentation): wire protocol, key location rules, batches, and statuses.
16. [IndexNow FAQ](https://www.indexnow.org/faq): participating endpoints, key setup, rotation, and sitemap relationship.
17. [Bing IndexNow](https://www.bing.com/indexnow): Microsoft’s official protocol landing page.
18. [2023 performance report explanation](https://blogs.bing.com/webmaster/2023/9/Unlocking-insights-with-the-new-Bing-Webmaster-Tools-Performance-Report/): search-surface and report-scope distinctions.
19. [GSC Search Analytics query](https://developers.google.com/webmaster-tools/v1/searchanalytics/query): date, dimension, filtering, paging, and row-limit comparison.
20. [IndexNow Insights, March 2024](https://blogs.bing.com/webmaster/2024/3/Optimize-your-Impact-with-IndexNow-Insights/): dashboard submission diagnostics.
21. [Top Insights, March 2024](https://blogs.bing.com/webmaster/2024/3/What%E2%80%99s-New-Top-Insights-for-Maximum-Visibility-and-Performance/): prioritized dashboard insights.
22. [Recommendations, October 2024](https://blogs.bing.com/webmaster/2024/10/Introducing-Recommendations-Bing-Webmaster-Tools-Next-Evolution-in-Analytics-and-SEO/): recommendations rollout.
23. [16-month history, October 2024](https://blogs.bing.com/webmaster/2024/10/Bing-Webmaster-Tools-Extends-Search-Performance-Data-to-16-Months/): dashboard retention extension.
24. [Performance changes, March 2025](https://blogs.bing.com/webmaster/2025/3/Supercharge-Your-Search-Performance-with-Bing-Webmaster-Tools/): dashboard comparison and filtering changes.
25. [Bing Search API retirement](https://learn.microsoft.com/en-us/lifecycle/announcements/bing-search-api-retirement): separate product retirement on 2025-08-11.
26. [data-nosnippet, October 2025](https://blogs.bing.com/webmaster/2025/10/Bing-Introduces-Support-for-the-data-nosnippet-HTML-Attribute/): HTML directive support.
27. [AI Performance, February 2026](https://blogs.bing.com/webmaster/2026/2/Introducing-AI-Performance-in-Bing-Webmaster-Tools-Public-Preview/): citation reporting public preview.
28. [AI visibility expansion, June 2026](https://blogs.bing.com/search/2026/6/New-AI-Visibility-Insights-in-Bing-Webmaster-Tools-Intents-Topics-Citation-Share-Compare/): Intents, Topics, Citation Share, and Compare.
29. [Site Scan help](https://www.bing.com/webmasters/help/site-scan-623520c9): portal audit workflow.
30. [Microsoft WordPress plugin](https://github.com/microsoft/bing-wordpress-url-submission-plugin): official submission implementation.
31. [Microsoft MCP catalog](https://github.com/microsoft/mcp): official MCP inventory checked for Webmaster support.
32. [Community Go Bing CLI](https://github.com/klixpert-io/bing-cli): broad existing CLI and implementation choices.
33. [Community Go/C# MCP](https://github.com/ncosentino/bing-webmaster-mcp): native MCP implementation and IndexNow support.
34. [Community Python library](https://pypi.org/project/bing-webmaster-tools/): published client version and features.
35. [Community Python MCP](https://pypi.org/project/mcp-server-bwt/): published MCP version and tool pagination behavior.
36. [Community Python CLI](https://pypi.org/project/bing-webmaster-cli/): `bwm`, local date filtering, and limited CLI surface.
37. [Community CLI/MCP compatibility notes](https://github.com/stufently/bing-webmaster-ai-cli-mcp/blob/main/docs/api-surface.md): observed legacy endpoint inconsistencies.
38. [PrintingPress compatibility notes](https://printingpress.dev/library/developer-tools/bing-webmaster): obsolete-method exclusions and site-move failure.
39. [Pickuma quota observation](https://pickuma.com/for-dev/bing-webmaster-api-100-url-daily-quota-in-practice/): 2026 single-site quota and fault behavior.
40. [Community position example](https://gist.github.com/MartijnSch/4f1696f2c539740222818cb8d5076e24): unconfirmed position scaling convention.
41. [Community clicks/impressions report](https://note.com/honeymarron_dev/n/n8dfd69eda0fd?hl=en): reported inconsistent performance counters.
42. [Homebrew core](https://formulae.brew.sh/formula/) and [casks](https://formulae.brew.sh/cask/): exact-name catalog checks.
43. [Debian `bing`](https://packages.debian.org/bing) and [Ubuntu `bing`](https://packages.ubuntu.com/noble/arm64/net/bing): Linux binary collision.
44. [Bitcoin Wallet Tracker](https://github.com/bwt-dev/bwt): `bwt` executable collision.
45. Local Clarity reference: suite behavior, supplemented by the seven source/configuration files linked in Section 9.
