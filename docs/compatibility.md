# API compatibility and pinned snapshot

`bwt` targets the Bing Webmaster **JSON** API as documented on learn.microsoft.com, plus the IndexNow
protocol. This page records the exact documentation snapshot that the operation registry and its tests
were built against, so upstream changes can be detected and reviewed instead of discovered in production.

## Pinned snapshot

Captured **2026-10-03**.

| Source | Page updated | Docs commit |
| --- | --- | --- |
| [IWebmasterApi interface reference](https://learn.microsoft.com/en-us/dotnet/api/microsoft.bing.webmaster.api.interfaces.iwebmasterapi?view=bing-webmaster-dotnet) | 2023-11-14T06:46:00Z | `c23de406dd3bbb02346c29d6b11403443b6a7d64` |
| 62 per-method reference pages (one per method) | 2023-11-14T06:46:00Z (all 62) | `c23de406dd3bbb02346c29d6b11403443b6a7d64` (all 62) |
| [OAuth 2.0 guide](https://learn.microsoft.com/en-us/bingwebmaster/oauth2) | 2026-08-07T07:48:00Z | `d317a8d8afceaaa7fb797ad9476bfdf2887bf0d8` |
| [Supported API protocols](https://learn.microsoft.com/en-us/bingwebmaster/api-protocols) (JSON URL format, SOAP/POX retirement) | 2026-08-10T11:39:00Z | `5c6d2ee97fbe3216591e07c5b6ab5c2f069be873` |
| [Getting access](https://learn.microsoft.com/en-us/bingwebmaster/getting-access) (API keys) | 2022-10-13T18:10:00Z | `dff8078e497b5f1f794b7a703a4e4b17b5214a49` |
| [Getting started](https://learn.microsoft.com/en-us/bingwebmaster/getting-started) (examples, fault format) | 2022-10-13T18:10:00Z | `b36e8959d808fffab3ae7cbbbcfc2d42b536aa2d` |
| [IndexNow protocol](https://www.indexnow.org/documentation) | no version metadata | not applicable |

Vendored file: `cli-go/internal/registry/testdata/bing-webmaster-api.methods.json`

sha256: `9e75efc5c1748189a27b0c5b761dcfb1b7b7e6947b7b9bd5157ae7ed0bb644ac`

It holds, for all 62 methods: the method name, the HTTP verb read from the `WebGet` or `WebInvoke`
attribute, the `Obsolete` marker, the full C# signature, the page URL, and the page's `updated_at` and
`git_commit_id` metadata. Totals: 35 GET, 27 POST; obsolete: `GetDeepLink`, `GetDeepLinkAlgoUrls`,
`UpdateDeepLink`.

The documentation's source repository (`MicrosoftDocs/bing-webmaster-dotnet`) is private, so the snapshot
is parsed from the published pages; the commit IDs come from each page's metadata.

The JSON endpoint is `https://ssl.bing.com/webmaster/api.svc/json/METHOD`. The protocols page states:
"Legacy SOAP and POX APIs will be retired on August 31, 2026." The CLI never used them.

## What the tests enforce

| Test | Guarantee |
| --- | --- |
| `internal/registry` `TestRegistryMatchesSnapshot` | The registry lists exactly the snapshot's 62 methods with the same HTTP verbs, parameter names in order, and obsolete markers |
| `internal/registry` `TestEffects` | GET methods are reads; POST methods are writes except `GetChildrenUrlInfo`, a documented read over POST |
| `cmd` `TestEveryMethodHasACommand` | Every non-obsolete method has a runnable command; experimental methods live under `bwt experimental`; obsolete methods are unreachable |
| `cmd` `TestMutatesMatchesRegistry` | A command blocks under `--read-only` exactly when one of its methods writes |
| `cmd` `TestCoverageDocIsCurrent` | [api-coverage.md](api-coverage.md) matches the registry and the command tree |
| `cmd` `TestCompatibilityRecordsSnapshotHash` | This page records the sha256 of the vendored snapshot |
| `cmd` `TestAgentSafetyCommandManifest` | Adding or reclassifying any command needs a deliberate digest update |

## Live checks (2026-10-04)

Checked end to end against a real account (one verified site with little traffic) using an API key. Reads ran
under `--read-only`; writes were limited to reversible ones on a nonexistent test page or a throwaway test site,
and every change was undone. Commands with outside effects (`users add`, `users remove`, `connected-pages add`,
`sitemaps remove`, `experimental site-move submit`) were checked with `--dry-run` only.

| Area | Result |
| --- | --- |
| Login | API key verified with `GetUserSites`, saved in the macOS Keychain, default site chosen |
| Browser OAuth | Not possible: Bing's OAuth client registration rejects `http://127.0.0.1:47619/callback` and `http://localhost:47619/callback` ("not a valid http or https url"), so the CLI has no OAuth login |
| Sites | `sites list`, and `sites add`, `verify` (returns false for an unverified site), `remove` on a test site |
| Submission | `SubmitUrl` and `SubmitUrlBatch` accepted and counted against the daily and monthly quota; `SubmitContent` works with an API key (the docs conflict on this) and is counted against the content quota |
| Sitemaps, fetch, quota, crawl settings | Working; `SaveCrawlSettings` accepts a no-op save; `CrawlRate` is a 24-value numeric array |
| Keywords | `GetKeyword`, `GetKeywordStats` (weekly rows), `GetRelatedKeywords` return data with `YYYY-MM-DD` dates |
| Users, params, blocks, preview blocks | `GetSiteRoles` returns roles; add, enable/disable, and remove lifecycles work |
| Geo-targeting | Works only with a lowercase country code (`us`); `US` returns `InvalidParameter`. `bwt` lowercases it |
| Deep-link blocks | Work only with a lowercase market (`en-us`); `en-US` returns `InvalidParameter`. `bwt` lowercases it |
| IndexNow | `api.indexnow.org` answered 202 (key validation pending) for an unhosted key |
| Performance and crawl stats | Empty arrays for a site with little traffic; not an error |
| `GetUrlInfo`, `GetUrlTrafficInfo`, `GetChildrenUrlInfo`, `GetChildrenUrlTrafficInfo` | `{"ErrorCode":2,"Message":"ERROR!!! UnknownError"}` for every URL form, also when called directly with curl; consistent with no crawl data for the site |
| `GetSiteMoves` | HTTP 404; kept under `bwt experimental` |

Still unsettled, because the test site had no data: the scale of `AvgClickPosition` and
`AvgImpressionPosition`, whether performance `Date` values mark days or week buckets, and the URL information
methods on a site Bing has crawled. `BlockReason` values beyond 1 (`ContentNeedsToBeRemoved`) are unknown.

## Refreshing the snapshot

1. Fetch the interface page and the 62 method pages, one request every few seconds (learn.microsoft.com
   rate-limits bursts with HTTP 429). From each method page take the `lang-csharp` signature block: the
   verb is the `WebGet` or `WebInvoke(Method=...)` attribute, `Obsolete` marks obsolete methods, and the
   parameters come from the signature. Record the page's `updated_at` and `git_commit_id` meta tags.
2. Replace the vendored JSON and run `make test`. `TestRegistryMatchesSnapshot` lists added, removed, or
   changed methods.
3. Update `internal/registry`, add or adjust commands, run `make docs`, and update the dates, commits,
   and sha256 on this page.
4. Recheck the guide pages above for changes to OAuth, endpoints, quotas, or retirements, and update
   [RESEARCH.md](../RESEARCH.md) with what changed.

## Dependency baseline

| Component | Version |
| --- | --- |
| Go | 1.26 minimum, toolchain 1.27.1 |
| `github.com/spf13/cobra` | 1.10.2 |
| `github.com/zalando/go-keyring` | 0.2.8 |
| `github.com/gofrs/flock` / `github.com/google/renameio/v2` | 0.13.1 / 2.0.2 |
| `go.yaml.in/yaml/v3` | 3.0.5 |
| `golang.org/x/sys` / `golang.org/x/term` | 0.48.0 / 0.46.0 |
