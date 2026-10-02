---
name: bwt
description: Query and manage Bing Webmaster Tools with the bwt CLI: search performance, crawl health, URL and sitemap submission, IndexNow, keyword research, and site settings.
---

# Bing Webmaster Tools (bwt)

Use the installed `bwt` binary; check `--help` on any subcommand. The full reference is
[commands](../docs/commands.md); every Bing method and its command is in [API coverage](../docs/api-coverage.md).

- Use `-o json --no-input` for machine consumption. Diagnostics are on stderr; errors in JSON mode include
  `api_error_name` (for example `InvalidApiKey`, `ThrottleUser`, `NotAuthorized`).
- Start with `bwt auth status` (local, no network) to see which credential and site are active. An
  environment credential (`BWT_API_KEY`, `BWT_ACCESS_TOKEN`) overrides saved profiles.
- Pass `--site` with the exact URL from `bwt sites list`. Bare hosts are matched, but ambiguity is an error.
- Use `--read-only` unless the user asked for a change. Use `--dry-run` before any submission or setting change.
- Performance data (`bwt stats ...`) is Bing's top rows only; the API has no date range or paging. Filters
  are local (`filtered_locally: true`). Do not sum query or page rows into site totals; use
  `bwt stats summary`, which states its window. Positions are raw values with an undocumented scale.
- Never describe a submission as indexed. `submit urls` and `indexnow submit` report what Bing accepted.
- Prefer `bwt indexnow submit` for new or changed URLs (no Bing quota, reaches other engines); it needs a
  key hosted on the site (`bwt indexnow key generate`, then `bwt indexnow key check`).
- Check `bwt quota` before large `submit urls` runs. Do not retry throttled or failed batches in a loop;
  repeated submissions spend quota.
- Empty results from `links` or `crawl issues` are not proof of health; say so and suggest checking the dashboard.
- `url info` has no indexed verdict. Report crawl dates and HTTP status, not "indexed".
- `bwt experimental ...` methods have unverified behavior; avoid them unless the user explicitly asks.
- Setup requires the user's own login (`bwt auth login`) or API key. Never invent credentials or print them.
- Destructive commands need `--yes` with `--no-input`; only pass it when the user asked for that change.
