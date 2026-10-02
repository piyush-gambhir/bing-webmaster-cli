# Bing Webmaster CLI (`bwt`)

A Go command-line interface for Bing Webmaster Tools: search performance, crawl health, URL and sitemap
submission, IndexNow, keyword research, and site settings.

Designed for people and coding agents: one-command browser login, named profiles, table/JSON/YAML/CSV
output, an effect-based `--read-only` mode, and a single cross-platform binary. Covers all 59 non-obsolete
methods of the Bing Webmaster JSON API plus IndexNow. Independent project; not affiliated with Microsoft.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/piyush-gambhir/bing-webmaster-cli/main/install.sh | sh
```

Installs `bwt` to `~/.local/bin` (override with `INSTALL_DIR`, pin with `VERSION=v0.1.0`) after verifying
SHA-256 checksums. Prebuilt macOS, Linux, and Windows binaries are on the
[releases page](https://github.com/piyush-gambhir/bing-webmaster-cli/releases). Another tool, Bitcoin Wallet
Tracker, also installs a `bwt` binary; the installer and `bwt doctor` warn when one shadows the other.

From source (Go 1.26+, toolchain 1.27.1):

```bash
git clone https://github.com/piyush-gambhir/bing-webmaster-cli.git
cd bing-webmaster-cli/cli-go
make build            # bin/bwt
make install          # $(go env GOPATH)/bin/bwt, or INSTALL_DIR=...
```

Source builds have no built-in OAuth client unless `cli-go/.env.local` provides one (see
[docs/auth.md](docs/auth.md)); API-key login always works.

## Quick start

```bash
bwt auth login                      # browser login; picks a default site
bwt auth login --with-api-key       # or paste an API key (Settings > API Access > API Key)

bwt sites list
bwt stats traffic --since 2026-09-01
bwt stats queries --sort clicks --limit 20 -o csv > top-queries.csv
bwt crawl issues
bwt indexnow key generate           # then upload KEY.txt to your site root
bwt indexnow submit --sitemap https://www.example.com/sitemap.xml --changed-since 2026-10-01
bwt submit urls https://www.example.com/new-page
bwt quota
```

## Commands

| Group | Commands | Bing methods |
| --- | --- | --- |
| `auth`, `login`, `status`, `config` | login (browser or API key), status, list, use, logout | local, plus one `GetUserSites` check |
| `sites` | list, use, add, verify, remove | `GetUserSites`, `AddSite`, `VerifySite`, `RemoveSite` |
| `stats` | traffic, queries, pages, query-pages, page-queries, detail, query-traffic, summary | the seven performance methods |
| `crawl` | stats, issues, settings, set-settings | `GetCrawlStats`, `GetCrawlIssues`, `GetCrawlSettings`, `SaveCrawlSettings` |
| `url` | info, traffic, children, children-traffic | URL information methods |
| `links`, `connected-pages` | counts, list; list, add | link and connected-page methods |
| `sitemaps` | list, get, submit, remove | feed methods |
| `submit`, `quota` | urls; URL and content quota | `SubmitUrl`, `SubmitUrlBatch`, quota methods |
| `indexnow` | key generate, key check, submit | IndexNow protocol (no Bing login needed) |
| `keywords` | get, stats, related | keyword research |
| `fetch` | request, list, get | Fetch as Bingbot |
| `users` | list, add, remove | site roles |
| `params`, `block`, `preview-blocks` | list, add, remove (and enable/disable) | URL normalization, blocked URLs, preview blocks |
| `experimental` | geo, site-move, deeplink-blocks, submit-content | documented methods with unverified current behavior |
| `api`, `doctor`, `update`, `completion`, `version` | raw method call, checks, self-update | |

The [command reference](docs/commands.md) is generated from the command tree. The
[API coverage map](docs/api-coverage.md) lists every Bing method with its command, HTTP verb, read/write
effect, and status, plus what is out of scope and why.

## Authentication

See [docs/auth.md](docs/auth.md) for every method. In short: `bwt auth login` uses your browser and the
built-in OAuth client; `bwt auth login --with-api-key` saves an API key; `BWT_API_KEY` or
`BWT_ACCESS_TOKEN` work without saving anything. Secrets live in the OS keychain; a plaintext 0600 file is
used only with an explicit `--insecure-storage`.

| Variable | Meaning |
| --- | --- |
| `BWT_API_KEY` | API key; overrides saved profiles |
| `BWT_ACCESS_TOKEN` | Bearer token; overrides everything else |
| `BWT_PROFILE`, `BWT_SITE` | Default profile and site |
| `BWT_CONFIG` | Config file path (default `~/.config/bing-webmaster-cli/config.yaml`, XDG-aware) |
| `BWT_CLIENT_ID`, `BWT_CLIENT_SECRET` | Override the built-in OAuth client |
| `BWT_INDEXNOW_KEY` | IndexNow key for submissions |
| `BWT_NO_INPUT`, `BWT_QUIET`, `BWT_VERBOSE`, `BWT_READ_ONLY` | Behavior switches (`1` or `true`) |

## Sites

`--site`/`-s`, then `BWT_SITE`, then the profile's default site. Use URLs exactly as `bwt sites list` prints
them (scheme and trailing slash). A bare host such as `example.com` is matched against your sites, and an
ambiguous match is an error.

## What the data means

- **Performance data is Bing's top rows.** The API has no server-side date range, limit, or paging.
  `--since`, `--until`, `--limit`, and `--sort` work locally, and JSON output says `filtered_locally`.
  Several methods update weekly, traffic daily. Positions are shown raw because their scale is undocumented.
- **Accepted is not indexed.** `submit urls` and `indexnow submit` report what Bing received.
- **Empty is not healthy.** Bing's legacy link and crawl-issue methods can return empty lists for sites
  that have links or issues; `bwt` says so on stderr.
- **No indexing verdict exists.** `url info` shows crawl details; the API has no "indexed" field.
- Dates arrive as `/Date(ms-0700)/`; output converts them to RFC 3339 keeping Bing's offset. `--raw` prints
  the wire JSON.

## Output and safety

`-o table` (default), `json`, `yaml`, or `csv`. Data goes to stdout, diagnostics to stderr, and errors in
JSON/YAML modes are structured (`method`, `http_status`, `api_error_code`, `api_error_name`).

- `--read-only` blocks every command that changes Bing state, local credentials, or the binary. Effects come
  from a registry of all 62 methods, so reads over POST (`url children`) still work.
- `--dry-run` prints write requests (including every batch) without sending them.
- Destructive commands confirm, or need `--yes` with `--no-input`.
- No automatic retries. Throttling (`ThrottleUser`, `ThrottleHost`, HTTP 429) is reported, not retried.
- Redirects never forward credentials, and API keys never appear in logs or errors.

## Development

```bash
make test     # race-enabled; fake transports only, no Bing account needed
make vet
make build
make docs     # regenerates docs/commands.md and docs/api-coverage.md
```

The API snapshot the registry is tested against, and how to refresh it, is in
[docs/compatibility.md](docs/compatibility.md). Design notes: [PLAN.md](PLAN.md). Upstream facts and
sources: [RESEARCH.md](RESEARCH.md). See also [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md),
[CLAUDE.md](CLAUDE.md), and the [agent skill](bwt/SKILL.md). MIT licensed.
