# Bing Webmaster CLI (`bwt`)

A Go command-line interface for Bing Webmaster Tools: search performance, crawl health, URL, sitemap, and
content submission, IndexNow, keyword research, and site settings.

Built mainly for coding agents and usable by people: API-key login with keychain storage, named profiles,
stable JSON (plus table, YAML, and CSV), an effect-based `--read-only` mode, dry runs, and a single
cross-platform binary. Covers all 59 non-obsolete methods of the Bing Webmaster JSON API plus IndexNow, checked
end to end against a live account. Independent project; not affiliated with Microsoft.

**Docs:** [projects.piyushgambhir.com/bing-webmaster-cli](https://projects.piyushgambhir.com/bing-webmaster-cli)
([llms.txt](https://projects.piyushgambhir.com/bing-webmaster-cli/llms.txt) for agents)

[![CI](https://github.com/piyush-gambhir/bing-webmaster-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/piyush-gambhir/bing-webmaster-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/piyush-gambhir/bing-webmaster-cli)](https://github.com/piyush-gambhir/bing-webmaster-cli/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/piyush-gambhir/bing-webmaster-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/piyush-gambhir/bing-webmaster-cli)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/piyush-gambhir/bing-webmaster-cli/main/install.sh | sh
```

Installs `bwt` to `~/.local/bin` (override with `INSTALL_DIR`, pin with `VERSION=v0.1.2`) after verifying
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


### Verify a download

Releases are immutable once published and ship SBOMs plus signed build-provenance attestations. To confirm an
archive was built by this repository's release workflow:

```bash
gh attestation verify bing-webmaster-cli_darwin_arm64.tar.gz --repo piyush-gambhir/bing-webmaster-cli
```

## Quick start

```bash
bwt auth login                      # opens Bing Webmaster Tools; paste your API key; picks a default site

bwt sites list
bwt stats summary --compare previous    # traffic trend, with the window stated
bwt stats queries --sort clicks --limit 20 -o csv > top-queries.csv
bwt crawl issues
bwt indexnow key generate           # follows your site's www redirect; then upload KEY.txt to that host
bwt indexnow key check
bwt indexnow submit --sitemap https://www.example.com/sitemap.xml --changed-since 2026-10-01
bwt submit urls https://www.example.com/new-page --dry-run   # see the request, then run without --dry-run
bwt quota
```

The API key is under **Settings (gear icon) > API Access > API Key** in Bing Webmaster Tools; one key covers
all your sites. For scripts and CI, set `BWT_API_KEY` instead of logging in.

## For AI agents

Agents should read [bwt/SKILL.md](bwt/SKILL.md). The essentials:

- Run commands with `-o json --no-input`; data is on stdout, notes and structured errors on stderr.
- `bwt api methods -o json` lists every Bing method with its read/write effect and the command that calls it.
- `bwt auth status -o json` shows the active credential and site without a network call.
- Use `--read-only` by default and `--dry-run` before any write; destructive commands need `--yes`.

## Commands

| Group | Commands | Bing methods |
| --- | --- | --- |
| `auth`, `login`, `status`, `config` | login (API key), status, list, use, logout | local, plus one `GetUserSites` check |
| `sites` | list, use, add, verify, remove | `GetUserSites`, `AddSite`, `VerifySite`, `RemoveSite` |
| `stats` | traffic, queries, pages, query-pages, page-queries, detail, query-traffic, summary | the seven performance methods |
| `crawl` | stats, issues, settings, set-settings | `GetCrawlStats`, `GetCrawlIssues`, `GetCrawlSettings`, `SaveCrawlSettings` |
| `url` | info, traffic, children, children-traffic | URL information methods |
| `links`, `connected-pages` | counts, list; list, add | link and connected-page methods |
| `sitemaps` | list, get, submit, remove | feed methods |
| `submit`, `quota` | urls, content; URL and content quota | `SubmitUrl`, `SubmitUrlBatch`, `SubmitContent`, quota methods |
| `indexnow` | key generate, key check, submit | IndexNow protocol (no Bing login needed) |
| `keywords` | get, stats, related | keyword research |
| `fetch` | request, list, get | Fetch as Bingbot |
| `users` | list, add, remove | site roles |
| `params`, `block`, `preview-blocks` | list, add, remove (and enable/disable) | URL normalization, blocked URLs, preview blocks |
| `geo`, `deeplink-blocks` | list, add, remove | country or region targeting, deep-link blocks |
| `experimental` | site-move | documented, but returned HTTP 404 in a live check |
| `api`, `doctor`, `update`, `completion`, `version` | raw method call, `api methods` (the method registry as data), checks, self-update | |

The [command reference](docs/commands.md) is generated from the command tree. The
[API coverage map](docs/api-coverage.md) lists every Bing method with its command, HTTP verb, read/write
effect, and status, plus what is out of scope and why.

## Authentication

See [docs/auth.md](docs/auth.md). In short: `bwt auth login` opens Bing Webmaster Tools, takes your API key
(Settings > API Access > API Key) at a hidden prompt, checks it, and saves it in the OS keychain; one key
covers all your sites. `BWT_API_KEY` works without saving anything. A plaintext 0600 file is used only with
an explicit `--insecure-storage`. There is no browser OAuth login: Bing's OAuth registration rejects the
loopback redirect a CLI needs.

| Variable | Meaning |
| --- | --- |
| `BWT_API_KEY` | API key; overrides saved profiles |
| `BWT_ACCESS_TOKEN` | Bearer token; overrides everything else |
| `BWT_PROFILE`, `BWT_SITE` | Default profile and site |
| `BWT_CONFIG` | Config file path (default `~/.config/bing-webmaster-cli/config.yaml`, XDG-aware) |
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
- **Accepted is not indexed.** `submit urls`, `submit content`, and `indexnow submit` report what Bing
  received.
- **Empty is not healthy.** Bing's legacy link and crawl-issue methods can return empty lists for sites
  that have links or issues; `bwt` says so on stderr.
- **No indexing verdict exists.** `url info` shows crawl details; the API has no "indexed" field.
- Dates arrive as `/Date(ms-0700)/`; output converts them to RFC 3339 keeping Bing's offset. `--raw` prints
  the wire JSON.

## Output and safety

`-o table` (default), `json`, `yaml`, or `csv`. Data goes to stdout, diagnostics to stderr, and errors in
JSON/YAML modes are structured (`method`, `http_status`, `api_error_code`, `api_error_name`). JSON keeps
Bing's field names, converts dates to RFC 3339, and drops Bing's `__type` metadata; `--raw` prints the wire
response. `bwt api methods -o json` lists every Bing method with its read/write effect and command, so agents
can discover capabilities without reading docs.

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

The docs site lives in `web/` (Next.js and Fumadocs, exported as static files). Its command reference and API
coverage pages are generated from `docs/` at build time. `cd web && pnpm install && pnpm dev` runs it locally;
`scripts/deploy-docs.sh` deploys it.

The API snapshot the registry is tested against, and how to refresh it, is in
[docs/compatibility.md](docs/compatibility.md). Design notes: [PLAN.md](PLAN.md). Upstream facts and
sources: [RESEARCH.md](RESEARCH.md). See also [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md),
[CLAUDE.md](CLAUDE.md), and the [agent skill](bwt/SKILL.md). MIT licensed.
