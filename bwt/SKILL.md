---
name: bwt
description: Query and manage Bing Webmaster Tools with the bwt CLI: search performance, crawl health, URL and sitemap submission, content submission, IndexNow, keyword research, and site settings.
---

# Bing Webmaster Tools (bwt)

Use the installed `bwt` binary. Every subcommand has `--help`; the full reference is
[commands](../docs/commands.md), and [API coverage](../docs/api-coverage.md) maps every Bing method to its command.

## Before anything else

- Discover capabilities with `bwt api methods -o json`: every Bing method with its HTTP verb, `read`/`write`
  effect, status, and the command that calls it.
- Check credentials with `bwt auth status -o json` (local, no network). If it fails, ask the user to run
  `bwt auth login` (it needs their API key from Bing Webmaster Tools > Settings > API Access) or to set
  `BWT_API_KEY`. Never invent, print, or log a key.
- Pass `--site` with the exact URL from `bwt sites list`, or rely on the profile's default site shown by
  `auth status`. Bare hosts are matched; ambiguity is an error.

## Output and errors

- Always use `-o json --no-input`. Data is on stdout; notes and errors are on stderr.
- JSON keeps Bing's field names (`Url`, `Clicks`, `DailyQuota`) inside data and uses snake_case for the CLI's
  own envelope fields (`site`, `row_count`, `filtered_locally`). Dates are RFC 3339; Bing's `__type` metadata
  is removed (`--raw` shows the wire format).
- On failure the exit code is 1 and stderr holds JSON with `method`, `http_status`, `api_error_code`, and
  `api_error_name`. Common names: `InvalidApiKey` (ask for a new login), `NotAuthorized` (wrong site URL or no
  access), `ThrottleUser`/`ThrottleHost` (stop; do not retry in a loop), `UnknownError` on URL methods
  (Bing gave no reason; seen on sites without crawl data; never report it as "not indexed").

## Safety

- Use `--read-only` unless the user asked for a change. Run writes with `--dry-run` first and show the plan.
- Destructive commands (remove, block) need `--yes` with `--no-input`; pass it only when the user asked for
  that exact change.
- Check `bwt quota` before large `submit urls` runs. Repeated submissions spend quota even for the same URL.

## What the data means

- `bwt stats ...` returns Bing's top rows only; the API has no date range or paging, so filters are local
  (`filtered_locally: true`). Never sum query or page rows into site totals; use `bwt stats summary`, which
  states its window. Positions are raw values with an undocumented scale. A new or low-traffic site returns
  no rows at all.
- Never call a submission "indexed": `submit urls`, `submit content`, and `indexnow submit` report what Bing
  accepted. `url info` has no indexed verdict.
- Empty `links` or `crawl issues` results are not proof of health; say so.

## Verified live (2026-10-04)

- Working: sites, stats, crawl, sitemaps, quota, keywords, users, fetch, params, block, preview-blocks, geo,
  deeplink-blocks, `submit urls`, `submit content` (works with an API key), and IndexNow.
- `geo` and `deeplink-blocks` need lowercase country and market codes; `bwt` lowercases them for you.
- `url info`, `url traffic`, and `url children` returned `UnknownError` for a newly added site. Microsoft documents
  no cause, so treat it as "no answer", not "not indexed".
- `bwt experimental site-move` returned HTTP 404; avoid it unless the user explicitly asks.

## Recipes

```bash
bwt stats summary --compare previous -o json             # traffic trend with its window stated
bwt stats queries --sort clicks --limit 20 -o json       # top queries (Bing's top rows)
bwt crawl issues -o json && bwt crawl stats -o json      # crawl health
bwt indexnow submit --sitemap https://www.example.com/sitemap.xml --changed-since 2026-10-01 --dry-run
bwt submit urls https://www.example.com/new-page --dry-run
bwt keywords related "running shoes" --country us --language en-US -o json
```

Prefer `bwt indexnow submit` for new or changed URLs: no Bing quota, and other engines receive it. It needs a
key hosted on the same host as the URLs (`bwt indexnow key generate --host HOST`, then `bwt indexnow key check`).
