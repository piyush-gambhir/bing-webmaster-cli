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

## Behaviors the documentation does not settle

No authenticated calls were made while building the CLI. These items need a live check with a real
account; until then the CLI takes the conservative behavior shown.

| Question | Current behavior |
| --- | --- |
| Does Bing accept `http://127.0.0.1:47619/callback` as an OAuth redirect? | Built-in browser login uses it. If Bing rejects it, browser login is disabled and `--with-api-key` is the login path |
| Does Bing echo `state`? | Required. A missing or wrong `state` fails the login (no login CSRF exposure) |
| Does Bing enforce PKCE? | The CLI sends S256 PKCE parameters; harmless if ignored |
| Do bearer tokens work on `ssl.bing.com`? | OAuth calls use `www.bing.com`, as Bing's OAuth guide does. `BWT_OAUTH_API_HOST=ssl.bing.com` switches hosts for testing |
| Refresh `grant_type` | Bing's table says `authorization_code`, its sample says `refresh_token`. The CLI sends the standard `refresh_token` |
| Scale of `AvgClickPosition` and `AvgImpressionPosition` | Shown raw; some community tools divide by 10 |
| Whether performance `Date` values mark days or week buckets | Shown as returned, never interpolated |
| `DateTime` syntax for keyword GET parameters | `YYYY-MM-DD` |
| Submission quota rules | Printed exactly as returned; nothing hard-coded |
| `SubmitContent` authentication (OAuth only, or API key too) | `bwt experimental submit-content` |
| `BlockReason` enum values | `--reason` takes the number |
| Geo-targeting, site moves, deep-link blocks | `bwt experimental` with a notice on every run |
| `CrawlRate` valid range and hour timezone | Accepts 0 to 255 per hour; documented as unknown |

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
| `golang.org/x/oauth2` | 0.37.0 |
| `github.com/zalando/go-keyring` | 0.2.8 |
| `github.com/gofrs/flock` / `github.com/google/renameio/v2` | 0.13.1 / 2.0.2 |
| `go.yaml.in/yaml/v3` | 3.0.5 |
| `golang.org/x/sys` / `golang.org/x/term` | 0.48.0 / 0.46.0 |
