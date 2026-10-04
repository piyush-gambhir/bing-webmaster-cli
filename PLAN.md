# bing-webmaster-cli: architecture and build plan

A Go command-line interface for Bing Webmaster Tools: search performance, crawl health, URL and sitemap
submission, IndexNow, keyword research, and site settings. Built for people and coding agents, matching the
sibling CLIs in this folder and sharing conventions with [gsc-cli](../gsc-cli/PLAN.md). Facts, limits,
and sources are in [RESEARCH.md](RESEARCH.md); this file is the design.

Status: **released and verified live** (2026-10-04). Phases P0 to P5 are implemented in `cli-go/` and every
command was exercised against a real account. The 2026-10-04 revision below supersedes the OAuth parts of
this plan: login is an API key, because Bing rejects the loopback redirect a CLI needs. Decisions are recorded
in [Decisions](#decisions).

## Revision 2026-10-04: API-key login and live verification

**Why the login changed.** Registering the built-in OAuth client failed: Bing Webmaster Tools rejects
`http://127.0.0.1:47619/callback` and `http://localhost:47619/callback` as "not a valid http or https url".
Browser OAuth would need a hosted relay page, so the OAuth code was removed and `bwt auth login` takes the API
key instead: it opens Bing Webmaster Tools, reads the key at a hidden prompt, checks it before saving, stores
it in the keychain, and picks a default site. Bearer tokens obtained elsewhere still work through
`BWT_ACCESS_TOKEN`.

**Live results.** See [docs/compatibility.md](docs/compatibility.md#live-checks-2026-10-04). Two request bugs
were found and fixed (lowercase country and market codes). Geo-targeting, deep-link blocks, and content
submission (which works with an API key) left `experimental`; site moves stayed (HTTP 404).

**Improvement plan.** Agents are the main users, so output and errors favor predictable machine use while
tables stay readable for people.

| Area | Improvement | Status |
| --- | --- | --- |
| Login | API key flow with steps, browser hand-off, verify-before-save, default site, singular/plural wording | Done |
| Agent output | JSON without Bing's `__type` metadata; RFC 3339 dates; `--raw` for the wire format | Done |
| Agent discovery | `bwt api methods -o json`: every method with effect, status, and command | Done |
| Agent guidance | `bwt/SKILL.md` rewritten agent-first: discovery, credentials, error names, safety, verified behavior, recipes | Done |
| Tables | One-row tables for single objects (`fetch get` no longer dumps the document); named columns for preview and deep-link blocks; dry runs as one row per request; empty paged results print `No results.` | Done |
| Messages | Hints for `UnknownError` (no reason given; seen on sites without crawl data) and for empty stats on new sites | Done |
| IndexNow | Key commands follow the site's www redirect so keys land on the serving host; key-check errors name the right `--host` | Done |
| URL methods | Accept Bing's documented `domain:example.com` form | Done |
| Fetch | `fetch get --save FILE` writes the fetched document | Done |
| Bing quirks | Lowercase country and market codes sent automatically | Done |
| Data semantics | Position scale and weekly-or-daily date buckets need a site with traffic to confirm | Open: needs data, not code |
| Experimental | `site-move` returns 404; recheck when Bing changes it | Open: upstream |


## Goals

1. **Seamless login first.** `bwt auth login` opens a browser, the user clicks Allow, done. Pasting an
   API key is the one-line alternative. Advanced options come later.
2. **Full API coverage.** All 59 non-obsolete methods of the Bing Webmaster JSON API, plus IndexNow.
   The three officially obsolete deep-link methods and the retired SOAP/XML protocols are never
   implemented.
3. **Honest data.** Bing's performance API has no date ranges, filters, or paging, and several legacy
   endpoints return empty or zero values even for healthy sites. The CLI shows what Bing returned, labels
   every local filter or computation, and never turns "empty" into "healthy" or "submitted" into "indexed".
4. **Agent-safe and predictable**: effect-based `--read-only`, a reviewed command-safety manifest, stable
   JSON, data on stdout and diagnostics on stderr, no hidden calls or automatic retries.

Not in scope: dashboard features with no public API (AI Performance and citations, Recommendations, Site
Scan, competitor backlinks, IndexNow Insights). The CLI will add them if Microsoft documents an API.

## Identity

| Item | Value |
| --- | --- |
| Repository | `github.com/piyush-gambhir/bing-webmaster-cli` (local folder `bing-webmaster-cli/`) |
| Go module | `github.com/piyush-gambhir/bing-webmaster-cli/cli-go` |
| Binary | `bwt` (see [Decisions](#decisions)) |
| Env prefix | `BWT_` |
| Config | `~/.config/bing-webmaster-cli/config.yaml` (XDG-aware, 0600, flock + atomic write); never holds secrets unless `--insecure-storage` was chosen |
| Secrets | OS keychain (same package as gsc-cli, 10s call timeout); 0600 file only with explicit `--insecure-storage` |
| Go | 1.26 minimum, toolchain 1.27.1 |
| Layout | `cli-go/`, `docs/`, `bwt/SKILL.md`, `.github/`, `install.sh`, root `Makefile` (same as `clarity-cli`) |

## Authentication

Bing supports two credentials: OAuth 2.0 (scopes `Webmaster.read` and `Webmaster.manage`) and a user-level
API key that covers all of the user's verified sites.

### Phase 1: one-command login

```console
$ bwt auth login
Opening your browser to sign in to Bing Webmaster Tools...
Logged in (profile "default").
Found 2 sites. Default site set to https://www.example.com/ (change with: bwt sites use SITE).
```

1. The CLI listens on a fixed loopback port (proposed `127.0.0.1:47619`, distinct from jira-cli's 8765)
   because Bing requires the redirect URI to match the registered one exactly.
2. It opens `https://www.bing.com/webmasters/oauth/authorize` with the **built-in OAuth client**, scope
   `Webmaster.manage`, and a random `state`. The URL is also printed to stderr.
3. After the user clicks Allow, it exchanges the code (valid 5 minutes) at
   `https://www.bing.com/webmasters/oauth/token` with the client ID and secret.
4. The refresh token (valid until the user revokes access) goes into the OS keychain. Access tokens last
   about an hour and refresh silently. Without a keychain service, login stops and offers
   `--insecure-storage` (a 0600 file) or `BWT_API_KEY`, matching gsc-cli.
5. It calls `GetUserSites` once and sets the default site the same way gsc-cli does: one site is chosen
   automatically, several bring up a picker (skipped under `--no-input`).

The API-key path, for anyone who prefers it or when the browser flow is unavailable:

```console
$ bwt auth login --with-api-key
Bing Webmaster API key (Settings > API Access > API Key): ********
Saved (profile "default"). Found 2 sites.
```

`--with-api-key` reads from a hidden prompt, or from stdin when piped; `BWT_API_KEY` covers CI without
saving anything. (The global `--api-key` flag stays a per-command override, so login uses a different
name.) A newly generated key can take about 30 minutes
to start working; the error for an `InvalidApiKey` response mentions this. The key travels as the
`apikey` query parameter, so the client strips it from every log line and error itself (Go's
`URL.Redacted()` only hides user info, not query parameters).

Also in phase 1: `auth status` (local; `--verify` makes one `GetUserSites` call), `auth list`,
`auth use NAME`, `auth logout`, and `BWT_ACCESS_TOKEN` for an externally obtained bearer token. Bing
documents no token revocation endpoint, so `logout` deletes local secrets and tells the user where to
remove app access or regenerate the key.

Credential precedence: `--access-token` / `BWT_ACCESS_TOKEN` > `--api-key` / `BWT_API_KEY` > selected
profile. Profile selection: `--profile` > `BWT_PROFILE` > saved current.

### Built-in OAuth client: one-time owner setup

1. In Bing Webmaster Tools, open **Settings > API Access**, accept the terms, choose **OAuth Client**, and
   register name "bwt CLI" with redirect URI `http://127.0.0.1:47619/callback`.
2. Store the client ID and secret as GitHub Actions secrets for release builds and in an untracked
   `cli-go/.env.local` for local builds. Builds without them still support `--with-api-key`.
3. Run the P1 smoke test, which answers three questions the docs leave open: whether Bing accepts an
   `http://127.0.0.1` redirect, whether it echoes `state`, and whether `ssl.bing.com` accepts bearer
   tokens (the OAuth docs use `www.bing.com`). Results go into RESEARCH.md.

What the smoke test decides, and the rules that do not depend on it:

- `state` is mandatory. A callback without it, or with the wrong value, fails the login. If Bing turns out
  not to echo `state`, the built-in browser flow is disabled (it cannot be protected against login CSRF)
  and `--with-api-key` becomes the default login until a server-side broker exists.
- The authorize request also carries PKCE parameters (S256). The smoke test checks whether Bing enforces
  them by omitting the verifier once; the result is recorded in RESEARCH.md.
- If Bing rejects an `http://127.0.0.1` redirect, the built-in browser flow is disabled and API-key login
  is the default. (A hosted relay page was considered and dropped: forwarding codes through a public page
  adds browser permission prompts and leakage paths.)
- If bearer tokens only work on `www.bing.com`, OAuth profiles use that host and API-key profiles use
  `ssl.bing.com`.
- The fixed port is bound before the browser opens. If it is busy, login stops with an actionable message
  and suggests `--with-api-key`; it never retries on a different port, because Bing would reject an
  unregistered redirect.

Bing's flow requires a client secret and documents no PKCE. A secret shipped inside a binary can be
extracted (RFC 8252 says such secrets must not be treated as confidential), and a loopback redirect
identifies an address, not an app. The residual risk is a malicious local process intercepting a code.
Binding the port before opening the browser means a squatter makes the login abort instead of receiving
a code; codes expire in 5 minutes; and users who want no shared secret can use `--with-api-key` or their
own OAuth client. The secure long-term design is a small server-side token broker that holds the secret
(later phase). The secret is injected at build time and never committed.

### Later: auth expansion (phase 5)

- Bring your own OAuth client: `bwt auth login --client-id ID --client-secret-stdin --redirect-port N`.
- Server-side token broker (for example a Cloudflare Worker) that keeps the client secret off user
  machines, if the smoke test shows Bing lacks PKCE and the built-in client sees real use.
- Read-only scope: `bwt auth login --scope read` requests `Webmaster.read`.

## Wire format

- JSON only, at `https://ssl.bing.com/webmaster/api.svc/json/{Method}`. GET parameters go in the query
  string, POST parameters in a JSON object with the documented names.
- Successful responses are wrapped in `{"d": ...}`. `{"d":null}` and `{"d":false}` are valid results, not
  errors (`VerifySite` returns a boolean).
- Dates arrive as `/Date(1315349995284-0700)/`. JSON and table output convert them to RFC 3339 keeping
  the offset; `--raw` prints the wire response unchanged. Negative epochs and escaped slashes are handled,
  and the offset is never applied twice.
- Faults are `{"ErrorCode": N, "Message": "..."}`, documented with HTTP 400 and observed under HTTP 200.
  Both are detected and mapped to the enum name (`InvalidApiKey`, `ThrottleUser`, `ThrottleHost`,
  `NotAuthorized`, `Deprecated`, and so on).
- `CrawlSettings.CrawlRate` is a numeric array on the wire, which Go would otherwise encode as base64, so
  it gets a custom type.
- Unknown fields and `__type` metadata are preserved.

## Command surface

`(w)` marks a remote write: blocked by `--read-only`, supports `--dry-run`, and destructive ones confirm
or need `--yes`. `(x)` marks experimental methods whose current behavior is unverified; they print a
notice and ship only after a credentialed check. Method names are Bing's.

```text
auth login | status | list | use NAME | logout                                  (l)  P1
login, status                               top-level aliases (suite convention)     P1
config show | list-profiles | use-profile NAME                                  (l)  P1

sites list                                  GetUserSites                             P2
sites use SITE                              set the profile's default site      (l)  P2
sites add SITE                              AddSite                             (w)  P3
sites verify SITE                           VerifySite                          (w)  P3
sites remove SITE                           RemoveSite                          (w)  P3

stats traffic                               GetRankAndTrafficStats (daily)           P2
stats queries                               GetQueryStats (weekly)                   P2
stats pages                                 GetPageStats (weekly)                    P2
stats query-pages QUERY                     GetQueryPageStats                        P2
stats page-queries URL                      GetPageQueryStats                        P2
stats detail QUERY URL                      GetQueryPageDetailStats                  P2
stats query-traffic QUERY                   GetQueryTrafficStats (daily)             P2
stats summary [--compare previous]          local totals over GetRankAndTrafficStats P4

crawl stats                                 GetCrawlStats                            P2
crawl issues                                GetCrawlIssues                           P2
crawl settings                              GetCrawlSettings                         P2
crawl set-settings                          SaveCrawlSettings                   (w)  P4

url info URL                                GetUrlInfo                               P2
url traffic URL                             GetUrlTrafficInfo                        P2
url children URL [--all] [filters]          GetChildrenUrlInfo (POST, read)          P2
url children-traffic URL [--all]            GetChildrenUrlTrafficInfo                P2

links counts [--all]                        GetLinkCounts                            P2
links list URL [--all]                      GetUrlLinks                              P2
connected-pages list                        GetConnectedPages                        P2
connected-pages add URL                     AddConnectedPage                    (w)  P4

sitemaps list                               GetFeeds                                 P2
sitemaps get URL                            GetFeedDetails                           P2
sitemaps submit URL                         SubmitFeed                          (w)  P3
sitemaps remove URL                         RemoveFeed                          (w)  P3

submit urls URL... [--file F]               SubmitUrl / SubmitUrlBatch (500 max) (w)  P3
experimental submit-content URL --file F   SubmitContent                      (x)(w)  P5
quota                                       GetUrlSubmissionQuota + GetContentSubmissionQuota  P2

indexnow key generate [--no-save]          new hex key, printed and saved locally  (l)  P3
indexnow key check [--site S]               GET https://host/KEY.txt                 P3
indexnow submit URL... [--file F | --sitemap URL]  10,000 per batch            (w)  P3

keywords get QUERY [--country --language --start --end]   GetKeyword                 P2
keywords stats QUERY [--country --language]               GetKeywordStats            P2
keywords related QUERY [...]                              GetRelatedKeywords         P2

fetch request URL                           FetchUrl (schedules a Bingbot fetch) (w)  P3
fetch list                                  GetFetchedUrls                           P2
fetch get URL                               GetFetchedUrlDetails                     P2

users list                                  GetSiteRoles                             P2
users add EMAIL --role ROLE --delegated-url URL --auth-code CODE  AddSiteRoles   (w)  P4
users remove EMAIL [--delegated-url URL]    GetSiteRoles lookup, then RemoveSiteRole (w)  P4

params list                                 GetQueryParameters                       P2
params add | remove NAME                    Add/RemoveQueryParameter            (w)  P4
params enable | disable NAME                EnableDisableQueryParameter         (w)  P4
block list                                  GetBlockedUrls                           P2
block add | remove URL                      Add/RemoveBlockedUrl                (w)  P4
preview-blocks list                         GetActivePagePreviewBlocks               P2
preview-blocks add | remove URL             Add/RemovePagePreviewBlock          (w)  P4

experimental geo list                       GetCountryRegionSettings              (x)  P5
experimental geo add | remove               Add/RemoveCountryRegionSettings    (x)(w)  P5
experimental site-move list                 GetSiteMoves                          (x)  P5
experimental site-move submit               SubmitSiteMove                     (x)(w)  P5
experimental deeplink-blocks list           GetDeepLinkBlocks                     (x)  P5
experimental deeplink-blocks add | remove   Add/RemoveDeepLinkBlock            (x)(w)  P5

api METHOD [--param k=v ...] [--data JSON]   raw call through the operation registry      P4
doctor                                       config, credentials, site access, PATH check P4
completion | version | update [--check]                                                   P1
```

Never implemented: `GetDeepLink`, `GetDeepLinkAlgoUrls`, `UpdateDeepLink` (officially obsolete), and the
SOAP/POX protocols (retired 2026-08-31).

### Operation registry

Every Bing method is one registry entry: name, HTTP verb, parameters with types, response type, and
effect (`read` or `write`). Commands, `--read-only`, `bwt api`, the safety manifest, and tests all use it.
Effect is about behavior, not HTTP method: `GetChildrenUrlInfo` is a POST read; `VerifySite` and
`FetchUrl` change state; an IndexNow submission is a write even as a GET; `indexnow key check` is a read
of the user's own website. `bwt api` refuses methods missing from the registry under `--read-only`.

### Site selection

`--site` / `-s`, then `BWT_SITE`, then the profile default. Values are used exactly as `GetUserSites`
returns them, because samples differ in scheme and trailing slash. A bare host (`example.com`) is matched
against that list, and ambiguity is an error listing the candidates.

### Performance stats

Bing's seven performance methods take only a site (plus a query or page where named) and return Bing's
top rows. There are no date ranges, row limits, or pages on the server side. So:

- `--since`, `--until`, `--limit`, and `--sort` exist but filter and sort **locally**. Output says so, and
  JSON records `"filtered_locally": true`.
- Table columns follow each response type exactly:
  - `QueryStats` (`queries`, `pages`, `query-pages`, `page-queries`): query or page (the `Query` field
    holds page URLs for page methods and is labeled accordingly), date, clicks, impressions, CTR, average
    click position, average impression position.
  - `DetailedQueryStats` (`detail`): date, clicks, impressions, position.
  - `RankAndTrafficStats` (`traffic`, `query-traffic`): date, clicks, impressions, CTR.
  CTR is computed only when impressions are nonzero. Positions stay raw until a live check confirms
  whether Bing scales them by ten, as some community tools assume.
- Clicks greater than impressions have been reported; values are shown as returned, never clamped.
- Rows are never summed into "site totals" except in `stats summary`, which uses the daily site-level
  `GetRankAndTrafficStats` series and states the window it covered.
- Column names match gsc-cli where the meaning matches (`clicks`, `impressions`, `ctr`), so results from
  both CLIs can be compared side by side.

### Submission and IndexNow

- `submit urls` batches up to 500 URLs per `SubmitUrlBatch` call, sequentially. On failure it reports
  which batches succeeded and stops; it never resubmits automatically, because repeated submissions spend
  quota. `--check-quota` adds an explicit quota read first. Output says "accepted", never "indexed".
- `quota` prints the daily and monthly counters exactly as returned. Bing's quota rules are site-specific
  and its public statements conflict, so nothing is hard-coded.
- IndexNow needs no Bing credentials. `indexnow key generate` creates a 32-character hex key and prints
  where to host it (`https://host/KEY.txt`). The key is saved per site in the profile. `key check`
  fetches the file and compares it. `submit` groups URLs by host, sends up to 10,000 per batch to
  `api.indexnow.org` (or `--endpoint bing`), and explains each status: 200 received, 202 key validation
  pending, 403 key not verified, 422 URL outside host, 429 slow down. `--sitemap URL` reads URLs from a
  sitemap (with `--changed-since DATE` using `lastmod`), which is the most common publishing workflow.

### Health commands

`crawl issues` decodes the issue bitmask into labels (301, 302, 4xx, 5xx, robots.txt, malware, DNS,
timeouts). `url info` shows last crawl, discovery date, HTTP status, size, and child count, and never
prints an "indexed" verdict, because the API has no such field. Empty link or issue lists carry a note that
Bing's legacy endpoints can return empty results for sites that do have links or issues.

## Global flags and environment

| Flag | Env | Meaning |
| --- | --- | --- |
| `-o, --output` | | `table` (default), `json`, `yaml`, `csv` (row-shaped results) |
| `--profile` | `BWT_PROFILE` | named profile |
| `-s, --site` | `BWT_SITE` | site for site-scoped commands |
| `--api-key` | `BWT_API_KEY` | API key override (prefer the env var or login) |
| `--access-token` | `BWT_ACCESS_TOKEN` | raw bearer token override |
| `--timeout` | | per-request timeout, default 30s |
| `--no-input` | `BWT_NO_INPUT` | never prompt or open a browser |
| `-q, --quiet` | `BWT_QUIET` | suppress informational stderr |
| `-v, --verbose` | `BWT_VERBOSE` | log method, redacted URL, and status |
| `--read-only` | `BWT_READ_ONLY` | block remote writes, local credential changes, and self-update |
| `--dry-run` | | for writes: show the requests (and batches) and send nothing |
| `--yes` | | confirm destructive commands non-interactively |
| `--raw` | | wire JSON, unconverted |
| | `BWT_CONFIG`, `XDG_CONFIG_HOME` | file locations |
| | `BWT_CLIENT_ID`, `BWT_CLIENT_SECRET` | override the built-in OAuth client |

```yaml
current_profile: default
profiles:
  default:
    auth: oauth                 # oauth | api_key
    client: builtin
    scope: Webmaster.manage
    token_store: keychain
    site: https://www.example.com/
    indexnow:
      www.example.com: { key_location: "https://www.example.com/0123abcd....txt" }   # key itself in keychain
```

## Errors and limits

- Structured errors carry `method`, `http_status`, `api_error_code`, `api_error_name`, `message`, and
  `retry_after` when present. JSON and YAML modes print them on stderr.
- `ThrottleUser` and `ThrottleHost` get specific messages. Bing documents no rate numbers or Retry-After,
  so the CLI reports and stops; there are no automatic retries.
- Paging: `links` advance from page 0 using `TotalPages`; `url children` advance from page 0 until an empty
  page. One page by default, `--all` for more, with a page guard and repeated-page detection.
- Redirects never forward credentials; bodies are size-capped; non-JSON responses are reported with
  status and a short sanitized excerpt.

## Architecture

```text
cli-go/
  main.go
  cmd/                 root, auth, config, sites, stats, crawl, url, links, sitemaps, submit, quota,
                       indexnow, keywords, fetch, users, params, block, preview, experimental, api,
                       doctor, update
  internal/registry/   operation registry: one entry per Bing method, with effect metadata
  internal/auth/       OAuth loopback flow (built-in client via ldflags), refresh, API key handling
  internal/secrets/    keychain store with opt-in 0600 file storage (same package as gsc-cli)
  internal/client/     HTTP core, d-unwrapping, fault detection, MS date codec, key redaction
  internal/indexnow/   key generation, key-file check, host grouping, batching, sitemap reader
  internal/stats/      local filters, CTR, summaries, comparisons
  internal/config/, internal/output/, internal/build/, internal/update/   from clarity-cli
  tools/gendocs/
```

Dependencies: the suite set plus `golang.org/x/oauth2` and `github.com/zalando/go-keyring`. No Bing SDK
exists for Go; existing community CLIs are not used as libraries.

## Safety model

- `--read-only` uses the registry's effect metadata, so read POSTs work and every state change is blocked
  before any request is built.
- Destructive commands (`sites remove`, `sitemaps remove`, `users remove`, `block add` with full removal)
  confirm or need `--yes`.
- The command-safety manifest test from jira-cli hashes every command with its annotations.
- `--no-input` never opens a browser.

## Testing

- Fake transports only in CI; a manual smoke script behind `BWT_LIVE=1` for the open questions above.
- Registry: every entry's verb, path, parameter names, and effect; a test fails if a registry method lacks
  a command or a classification.
- Encoding: query escaping, booleans, numeric enums, nested POST objects, the crawl-rate array.
- Decoding: arrays, objects, booleans, null, `__type`, unknown fields; faults under 400 and 200; HTML and
  oversized responses.
- MS dates: negative epochs, offsets, escaped slashes.
- Credentials: precedence, API key never in logs or errors (including transport errors), OAuth flow against
  a fake server (state, denial, timeout, refresh), keychain timeout and the explicit file opt-in.
- Batching: 500 and 10,000 boundaries, host grouping, partial failure, `--dry-run` sends nothing.
- Suite CI: gofmt, vet, race tests, generated docs check, install smoke test on three OSes, GoReleaser
  snapshot.

## Docs, skill, release

Same set as gsc-cli: `README.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `SECURITY.md`, generated
`docs/commands.md`, `docs/auth.md`, and `bwt/SKILL.md`. The skill tells agents to use `-o json`, treat
performance rows as Bing's top rows, never call a submission "indexed", never read empty legacy results as
healthy, and prefer IndexNow for new or changed URLs.

## Milestones

| Phase | Scope | Done when |
| --- | --- | --- |
| P0 | Scaffold from clarity-cli plus the operation registry, client codecs, output (plus csv) | `make test vet build docs` pass |
| P1 | Seamless login: built-in OAuth, `--with-api-key`, keychain storage (explicit `--insecure-storage` opt-in), refresh, auto default site, profiles; owner smoke test answers the three OAuth questions | Real OAuth and API-key logins work on macOS |
| P2 | All reads: sites, stats, crawl, url, links, sitemaps, quota, keywords, fetch list/get, users list, params/block/preview-block lists | Golden decode tests; real-site smoke check |
| P3 | Core writes and IndexNow: submit urls, sitemaps submit/remove, sites add/verify/remove, fetch request, indexnow key/check/submit, safety manifest | Read-only paths tested; release **v0.1.0** |
| P4 | Remaining writes (crawl settings, users, params, blocks, preview blocks, connected pages), `stats summary`, `api`, `doctor` | **v0.2.0** |
| P5 | Auth expansion (BYO client, read scope); experimental groups including `submit-content`, each printing a notice until a credentialed check confirms it | **v0.3.0** |

## Build status

| Phase | State |
| --- | --- |
| P0 scaffold, registry, client codecs, output with CSV | Done |
| P1 login: built-in OAuth, `--with-api-key`, keychain, refresh, default site, profiles | Done; owner registration and live checks pending |
| P2 all reads | Done |
| P3 core writes, IndexNow, safety manifest | Done |
| P4 remaining writes, `stats summary`, `api`, `doctor` | Done |
| P5 own OAuth client, read scope, experimental groups including `submit-content` | Done |

Owner tasks and live checks (see [docs/auth.md](docs/auth.md) and [docs/compatibility.md](docs/compatibility.md)):
register the built-in OAuth client, add the two repository secrets, then confirm loopback redirect
acceptance, `state` echo, PKCE enforcement, bearer support on `ssl.bing.com`, position scale, and the
experimental methods against a real account.

Deviations from the plan above, with reasons:

- IndexNow keys are kept in the config file, not the keychain, because the protocol publishes them at
  `https://host/KEY.txt`; they are not secrets.
- Each OAuth profile uses two keychain items (refresh credentials, cached access token) so each stays under
  Windows Credential Manager's 2,560-byte limit.
- `auth logout` keeps the profile's default site and IndexNow keys; only credentials are deleted.
- JSON performance rows stay exactly as Bing returns them (dates converted); CTR appears only in table and
  CSV output. `crawl issues` adds a derived `issue_labels` field because the bitmask is unreadable otherwise.
- `submit urls` has `--batch-size` (default 500) for sites whose daily quota is smaller than a batch.
- `users add` fills `--auth-code` from `GetUserSites` when omitted.
- Experimental geo and deep-link removals send the object built from flags (no lookup), since their
  current behavior is unverified anyway; `block remove` and `users remove` look up the exact object.

## Decisions

1. **Binary name:** `bwt`. `install.sh` and `bwt doctor` warn when another `bwt` (Bitcoin Wallet Tracker)
   shadows it on `PATH`.
2. **Built-in OAuth client:** implemented through `-ldflags` (`BWT_OAUTH_CLIENT_ID`,
   `BWT_OAUTH_CLIENT_SECRET`), overridable at run time with `BWT_CLIENT_ID` and `BWT_CLIENT_SECRET`. The
   owner registers it once.
3. **Experimental legacy groups:** shipped under `bwt experimental` (geo, site-move, deeplink-blocks), each
   printing a notice on every run.
4. **Content submission:** shipped as `bwt experimental submit-content`, with a confirmation because content
   may be indexed despite robots.txt.
5. **Date conversion:** `/Date(...)/` becomes RFC 3339 with Bing's offset by default; `--raw` prints the wire JSON.
