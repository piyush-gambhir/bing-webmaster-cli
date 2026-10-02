# bwt command reference

Generated from the command tree. Run `make docs` to refresh.

Global flags apply to every command.

```text
      --access-token string   Bearer token override (prefer BWT_ACCESS_TOKEN)
      --api-key string        API key override (prefer BWT_API_KEY or auth login)
      --dry-run               For writes: print the requests and send nothing
      --no-input              Never prompt or open a browser
  -o, --output string         Output format: table, json, yaml, csv (default "table")
      --profile string        Named profile (or BWT_PROFILE)
  -q, --quiet                 Suppress informational stderr output
      --raw                   Print Bing's wire JSON without date conversion
      --read-only             Block remote writes, local credential changes, and self-update
  -s, --site string           Site URL exactly as in sites list, or a bare host (or BWT_SITE)
      --timeout duration      HTTP request timeout (default 30s)
  -v, --verbose               Log method, redacted URL, and status to stderr
      --yes                   Confirm destructive commands without a prompt
```

## bwt

Bing Webmaster Tools from your terminal

Read and manage Bing Webmaster Tools data: search performance, crawl health, URL and sitemap
submission, IndexNow, keyword research, and site settings.
Data goes to stdout and diagnostics to stderr. --read-only blocks every state change.

```text
bwt
```

## bwt api

Call any Bing Webmaster API method directly

Sends an authenticated request to https://ssl.bing.com/webmaster/api.svc/json/METHOD with the same
redaction, fault handling, and date conversion as other commands. Known methods use their documented
HTTP verb and effect; unknown methods need --http and count as writes, so --read-only refuses them.
Writes ask for confirmation (or --yes): a raw call can remove sites, sitemaps, or users.
--param values are sent as strings; use --data for typed JSON values (POST only).

```text
bwt api METHOD [flags]
```

```bash
  bwt api GetUserSites
  bwt api GetQueryStats --param siteUrl=https://example.com/
  bwt api SubmitUrlBatch --data '{"siteUrl":"https://example.com/","urlList":["https://example.com/a"]}'
```

```text
      --data string         JSON object of parameters (POST)
      --http string         HTTP method for methods not in the registry: GET or POST
      --param stringArray   Parameter as name=value (repeatable)
```

## bwt auth

Log in, inspect credentials, and manage profiles

```text
bwt auth
```

## bwt auth list

List saved profiles without secrets

```text
bwt auth list
```

## bwt auth login

Log in with your browser (or an API key) and pick a default site

Opens Bing's consent page in your browser using the built-in OAuth client, saves the login in the OS
keychain, and sets a default site. --with-api-key saves an API key instead (Bing Webmaster Tools >
Settings > API Access > API Key), read from a hidden prompt or from stdin when piped.
Without an OS keychain, login stops unless you pass --insecure-storage (a 0600 plaintext file).

changes local configuration.

```text
bwt auth login [flags]
```

```bash
  bwt auth login
  bwt auth login --with-api-key
  printf '%s' "$KEY" | bwt auth login --with-api-key --no-input --profile ci
  bwt auth login --client-id ID --client-secret-stdin --redirect-port 8400 < secret.txt
```

```text
      --client-id string      Use your own OAuth client instead of the built-in one
      --client-secret-stdin   Read your OAuth client secret from stdin (with --client-id)
      --insecure-storage      Store secrets in a 0600 plaintext file instead of the OS keychain
      --no-verify             Skip the GetUserSites check after saving
      --redirect-port int     Loopback port registered with your own client (with --client-id)
      --scope string          OAuth scope: manage (read and write) or read (default "manage")
      --with-api-key          Save an API key instead of using the browser
```

## bwt auth logout

Delete the profile's saved credentials (keeps its site and IndexNow keys)

Deletes the saved OAuth login or API key from the keychain or secrets file. Bing has no token revocation endpoint. To revoke access, remove the app or regenerate the API key under Bing Webmaster Tools > Settings > API Access.

changes local configuration.

```text
bwt auth logout
```

## bwt auth status

Show the active credential source without revealing secrets

Local only unless --verify, which makes one GetUserSites call.

```text
bwt auth status [flags]
```

```text
      --verify   Check access with one GetUserSites call
```

## bwt auth use

Set the default profile

changes local configuration.

```text
bwt auth use NAME
```

## bwt block

Block URLs from Bing results (Block URLs tool)

```text
bwt block
```

## bwt block add

Block a page or directory from Bing results

--request cache-only removes the cached copy; full-removal removes the URL from Bing results until it expires.
There is no default for --request, and full removal asks for confirmation.

changes Bing state (blocked by --read-only); Bing methods: AddBlockedUrl.

```text
bwt block add URL [flags]
```

```text
      --request string   cache-only or full-removal (required)
      --type string      page or directory (default "page")
```

## bwt block list

List blocked URLs

Bing methods: GetBlockedUrls.

```text
bwt block list
```

## bwt block remove

Unblock a URL

Looks up the block with GetBlockedUrls and removes exactly that entry.

changes Bing state (blocked by --read-only); Bing methods: GetBlockedUrls, RemoveBlockedUrl.

```text
bwt block remove URL
```

## bwt completion

Generate shell completion script

```text
bwt completion [bash|zsh|fish|powershell]
```

## bwt config

Inspect configuration and select profiles

```text
bwt config
```

## bwt config list-profiles

List saved profiles without secrets

```text
bwt config list-profiles
```

## bwt config show

Show the active credential source without revealing secrets

Local only unless --verify, which makes one GetUserSites call.

```text
bwt config show [flags]
```

```text
      --verify   Check access with one GetUserSites call
```

## bwt config use-profile

Set the default profile

changes local configuration.

```text
bwt config use-profile NAME
```

## bwt connected-pages

Pages (such as social profiles) connected to your site

```text
bwt connected-pages
```

## bwt connected-pages add

Connect a page to your site

changes Bing state (blocked by --read-only); Bing methods: AddConnectedPage.

```text
bwt connected-pages add URL
```

## bwt connected-pages list

List connected pages

Bing methods: GetConnectedPages.

```text
bwt connected-pages list
```

## bwt crawl

Crawl statistics, crawl issues, and crawl settings

```text
bwt crawl
```

## bwt crawl issues

URLs with crawl issues, with the issue bitmask decoded

Lists URLs Bing had trouble crawling. issue_labels is decoded locally from the Issues bitmask.
Fixed issues can stay listed for several days, and an empty list does not prove the site has none.

Bing methods: GetCrawlIssues.

```text
bwt crawl issues
```

## bwt crawl set-settings

Change hourly crawl rate or crawl boost

Reads the current crawl settings, applies your changes, and saves the full object.
--rate takes 24 comma-separated hourly values (Bing does not document the valid range or timezone).

changes Bing state (blocked by --read-only); Bing methods: GetCrawlSettings, SaveCrawlSettings.

```text
bwt crawl set-settings [flags]
```

```bash
  bwt crawl set-settings --rate 5,5,5,5,5,5,5,5,5,5,5,5,5,5,5,5,6,6,6,5,4,3,3,4
  bwt crawl set-settings --boost on
```

```text
      --boost string   Crawl boost: on or off
      --rate string    24 comma-separated hourly crawl-rate values
```

## bwt crawl settings

Show crawl rate and crawl boost settings

CrawlRate has 24 hourly values. Bing does not document their valid range or the timezone of the hours.

Bing methods: GetCrawlSettings.

```text
bwt crawl settings
```

## bwt crawl stats

Daily crawl statistics

Daily counts of crawled pages, pages in the index, crawl errors, and HTTP status classes. Bing updates this daily.

Bing methods: GetCrawlStats.

```text
bwt crawl stats
```

## bwt doctor

Check configuration, credentials, site selection, and PATH

Runs local checks without network access; --verify adds one GetUserSites call. Exits nonzero when a check fails.

```text
bwt doctor [flags]
```

```text
      --verify   Also check access with one GetUserSites call
```

## bwt experimental

Documented Bing methods whose current behavior is unverified

These methods are still in Bing's documentation, but there is no current evidence that they work.
Each run prints a notice. Confirm results in the Bing Webmaster Tools dashboard.

```text
bwt experimental
```

## bwt experimental deeplink-blocks

Block deep links (sitelinks) shown under a result

```text
bwt experimental deeplink-blocks
```

## bwt experimental deeplink-blocks add

Block a deep link

changes Bing state (blocked by --read-only); experimental; Bing methods: AddDeepLinkBlock.

```text
bwt experimental deeplink-blocks add [flags]
```

```text
      --deep-link-url string   Deep link URL to block (required)
      --market string          Market, such as en-US (required)
      --search-url string      Result URL the deep link appears under (required)
```

## bwt experimental deeplink-blocks list

List deep-link blocks

experimental; Bing methods: GetDeepLinkBlocks.

```text
bwt experimental deeplink-blocks list
```

## bwt experimental deeplink-blocks remove

Remove a deep-link block

changes Bing state (blocked by --read-only); experimental; Bing methods: RemoveDeepLinkBlock.

```text
bwt experimental deeplink-blocks remove [flags]
```

```text
      --deep-link-url string   Deep link URL to block (required)
      --market string          Market, such as en-US (required)
      --search-url string      Result URL the deep link appears under (required)
```

## bwt experimental geo

Country or region targeting for pages, directories, or hosts

```text
bwt experimental geo
```

## bwt experimental geo add

Target a URL to a country or region

changes Bing state (blocked by --read-only); experimental; Bing methods: AddCountryRegionSettings.

```text
bwt experimental geo add [flags]
```

```text
      --country string   Two-letter ISO country code (required)
      --type string      page, directory, domain, or subdomain (default "page")
      --url string       Page, directory, or host URL (required)
```

## bwt experimental geo list

List targeting settings

experimental; Bing methods: GetCountryRegionSettings.

```text
bwt experimental geo list
```

## bwt experimental geo remove

Remove a targeting setting

changes Bing state (blocked by --read-only); experimental; Bing methods: RemoveCountryRegionSettings.

```text
bwt experimental geo remove [flags]
```

```text
      --country string   Two-letter ISO country code (required)
      --type string      page, directory, domain, or subdomain (default "page")
      --url string       Page, directory, or host URL (required)
```

## bwt experimental site-move

Site moves (Bing's site move tool)

```text
bwt experimental site-move
```

## bwt experimental site-move list

List submitted site moves

experimental; Bing methods: GetSiteMoves.

```text
bwt experimental site-move list
```

## bwt experimental site-move submit

Tell Bing that content moved to a new location

changes Bing state (blocked by --read-only); experimental; Bing methods: SubmitSiteMove.

```text
bwt experimental site-move submit [flags]
```

```text
      --from string    Source URL (required)
      --scope string   domain, host, or directory (required)
      --to string      Target URL (required)
      --type string    local or global (required)
```

## bwt experimental submit-content

Push a page's full HTTP response to Bing (content submission)

Sends a complete HTTP response (status line, headers, blank line, body) for URL, base64-encoded, up to 10 MB.
Bing's docs conflict on whether this needs OAuth, and submitted content may be indexed even if robots.txt
disallows it (NOINDEX is honored).

changes Bing state (blocked by --read-only); experimental; Bing methods: SubmitContent.

```text
bwt experimental submit-content URL [flags]
```

```text
      --dynamic-serving string   none, pc, mobile, amp, tablet, or nonvisual (default "none")
      --file string              File with the full HTTP response (required)
      --structured-data string   File with structured data, normally JSON-LD
```

## bwt fetch

Fetch as Bingbot: request a fetch and read the results

```text
bwt fetch
```

## bwt fetch get

Show a fetched URL's status, headers, and document

The table shows the status; use -o json for the response headers and document.

Bing methods: GetFetchedUrlDetails.

```text
bwt fetch get URL
```

## bwt fetch list

List fetch requests and whether they completed

Bing methods: GetFetchedUrls.

```text
bwt fetch list
```

## bwt fetch request

Ask Bingbot to fetch a URL

Schedules a Bingbot fetch. Check the result later with bwt fetch list and bwt fetch get.

changes Bing state (blocked by --read-only); Bing methods: FetchUrl.

```text
bwt fetch request URL
```

## bwt indexnow

Notify Bing and other engines of new or changed URLs with IndexNow

IndexNow needs no Bing login: you host a key file on your site and submit URLs with the key.
A submission to one participating endpoint is shared with the other IndexNow engines.
Keys are public by design (served at https://host/KEY.txt), so the CLI keeps them in the config file.

```text
bwt indexnow
```

## bwt indexnow key

Create and check the IndexNow key for a host

```text
bwt indexnow key
```

## bwt indexnow key check

Fetch the key file and confirm it matches

```text
bwt indexnow key check [flags]
```

```text
      --host string           Host to check (default: the selected site's host)
      --key string            Key to check (default: the saved key)
      --key-location string   Key file URL (default: https://host/KEY.txt)
```

## bwt indexnow key generate

Create a new key and save it for a host

Creates a 32-character hexadecimal key and shows where to host it. The key is saved in the selected
profile unless --no-save. Upload a text file named KEY.txt containing only the key to the site root,
then run bwt indexnow key check.

changes local configuration.

```text
bwt indexnow key generate [flags]
```

```text
      --host string   Host the key is for (default: the selected site's host)
      --no-save       Print the key without saving it
```

## bwt indexnow submit

Submit new, changed, or deleted URLs with IndexNow

Groups URLs by host and sends up to 10,000 per request, one request at a time. Stops at the first
rejected batch. 200 and 202 mean received (202: key validation pending); neither means indexed.

changes Bing state (blocked by --read-only).

```text
bwt indexnow submit [URL...] [flags]
```

```bash
  bwt indexnow submit https://example.com/new-post
  bwt indexnow submit --sitemap https://example.com/sitemap.xml --changed-since 2026-10-01
  bwt indexnow submit --file changed.txt --endpoint bing
```

```text
      --changed-since string   With --sitemap: only URLs whose lastmod is on or after YYYY-MM-DD
      --endpoint string        indexnow (api.indexnow.org), bing, or an https URL (default "indexnow")
      --file string            Read URLs from a file, one per line (- for stdin)
      --key string             IndexNow key for every host (default: saved key per host, or BWT_INDEXNOW_KEY)
      --key-location string    Key file URL (single host only)
      --sitemap string         Read URLs from a sitemap or sitemap index
```

## bwt keywords

Bing keyword research (market data, not your site's performance)

Keyword research reports Bing search demand for a query in a market. It is not filtered to your site.
Bing does not document the accepted date syntax for these methods; the CLI sends YYYY-MM-DD.

```text
bwt keywords
```

## bwt keywords get

Impressions for one query over a date range

Bing methods: GetKeyword.

```text
bwt keywords get QUERY [flags]
```

```text
      --country string    Market country code, such as us or gb (default "us")
      --end string        End date YYYY-MM-DD (default: today, UTC)
      --language string   Market language, such as en-US (default "en-US")
      --start string      Start date YYYY-MM-DD (default: 30 days before --end)
```

## bwt keywords related

Related queries with impressions over a date range

Bing methods: GetRelatedKeywords.

```text
bwt keywords related QUERY [flags]
```

```text
      --country string    Market country code, such as us or gb (default "us")
      --end string        End date YYYY-MM-DD (default: today, UTC)
      --language string   Market language, such as en-US (default "en-US")
      --start string      Start date YYYY-MM-DD (default: 30 days before --end)
```

## bwt keywords stats

Weekly impression history for one query

Bing methods: GetKeywordStats.

```text
bwt keywords stats QUERY [flags]
```

```text
      --country string    Market country code, such as us or gb (default "us")
      --language string   Market language, such as en-US (default "en-US")
```

## bwt links

Inbound links Bing has found

Bing's legacy link methods can return empty results for sites that have links; empty is not proof of none.

```text
bwt links
```

## bwt links counts

Pages on your site with inbound link counts

Pages start at 0. --all follows Bing's TotalPages (bounded by --max-pages).

Bing methods: GetLinkCounts.

```text
bwt links counts [flags]
```

```text
      --all             Fetch pages until the end (bounded by --max-pages)
      --max-pages int   Upper bound on pages fetched with --all (default 100)
      --page int        Page to fetch, starting at 0
```

## bwt links list

Inbound links to one page, with anchor text

Pages start at 0. --all follows Bing's TotalPages (bounded by --max-pages).

Bing methods: GetUrlLinks.

```text
bwt links list URL [flags]
```

```text
      --all             Fetch pages until the end (bounded by --max-pages)
      --max-pages int   Upper bound on pages fetched with --all (default 100)
      --page int        Page to fetch, starting at 0
```

## bwt login

Alias for auth login

Opens Bing's consent page in your browser using the built-in OAuth client, saves the login in the OS
keychain, and sets a default site. --with-api-key saves an API key instead (Bing Webmaster Tools >
Settings > API Access > API Key), read from a hidden prompt or from stdin when piped.
Without an OS keychain, login stops unless you pass --insecure-storage (a 0600 plaintext file).

changes local configuration.

```text
bwt login [flags]
```

```bash
  bwt auth login
  bwt auth login --with-api-key
  printf '%s' "$KEY" | bwt auth login --with-api-key --no-input --profile ci
  bwt auth login --client-id ID --client-secret-stdin --redirect-port 8400 < secret.txt
```

```text
      --client-id string      Use your own OAuth client instead of the built-in one
      --client-secret-stdin   Read your OAuth client secret from stdin (with --client-id)
      --insecure-storage      Store secrets in a 0600 plaintext file instead of the OS keychain
      --no-verify             Skip the GetUserSites check after saving
      --redirect-port int     Loopback port registered with your own client (with --client-id)
      --scope string          OAuth scope: manage (read and write) or read (default "manage")
      --with-api-key          Save an API key instead of using the browser
```

## bwt params

URL query parameters Bing should ignore when normalizing URLs

```text
bwt params
```

## bwt params add

Add a parameter to ignore

changes Bing state (blocked by --read-only); Bing methods: AddQueryParameter.

```text
bwt params add NAME
```

## bwt params disable

Disable a parameter

changes Bing state (blocked by --read-only); Bing methods: EnableDisableQueryParameter.

```text
bwt params disable NAME
```

## bwt params enable

Enable a parameter

changes Bing state (blocked by --read-only); Bing methods: EnableDisableQueryParameter.

```text
bwt params enable NAME
```

## bwt params list

List normalization parameters

Bing methods: GetQueryParameters.

```text
bwt params list
```

## bwt params remove

Remove a parameter

changes Bing state (blocked by --read-only); Bing methods: RemoveQueryParameter.

```text
bwt params remove NAME
```

## bwt preview-blocks

Block or allow page previews (snippets) in Bing results

```text
bwt preview-blocks
```

## bwt preview-blocks add

Block the preview for a URL

--reason is Bing's BlockReason number. Microsoft does not publish the enum values; existing blocks in
preview-blocks list show the values in use.

changes Bing state (blocked by --read-only); Bing methods: AddPagePreviewBlock.

```text
bwt preview-blocks add URL [flags]
```

```text
      --reason string   BlockReason number (required)
```

## bwt preview-blocks list

List active page preview blocks

Bing methods: GetActivePagePreviewBlocks.

```text
bwt preview-blocks list
```

## bwt preview-blocks remove

Remove a preview block

changes Bing state (blocked by --read-only); Bing methods: RemovePagePreviewBlock.

```text
bwt preview-blocks remove URL
```

## bwt quota

Remaining URL and content submission quota

Shows Bing's daily and monthly counters exactly as returned. Quotas are site-specific; nothing is assumed.

Bing methods: GetUrlSubmissionQuota, GetContentSubmissionQuota.

```text
bwt quota
```

## bwt sitemaps

Sitemaps and feeds submitted to Bing

```text
bwt sitemaps
```

## bwt sitemaps get

Show the sitemaps inside a sitemap index

Bing methods: GetFeedDetails.

```text
bwt sitemaps get URL
```

## bwt sitemaps list

List submitted sitemaps and feeds

Bing methods: GetFeeds.

```text
bwt sitemaps list
```

## bwt sitemaps remove

Remove a sitemap or feed

changes Bing state (blocked by --read-only); Bing methods: RemoveFeed.

```text
bwt sitemaps remove URL
```

## bwt sitemaps submit

Submit a sitemap or feed URL

Supports XML sitemaps, sitemap indexes, RSS 2.0, Atom 0.3/1.0, and text feeds.

changes Bing state (blocked by --read-only); Bing methods: SubmitFeed.

```text
bwt sitemaps submit URL
```

## bwt sites

List, add, verify, remove, and select sites

```text
bwt sites
```

## bwt sites add

Add a site to your account (it still needs verification)

Adds the site to your account. Adding is harmless if the site is already there.
Verify ownership afterwards with an XML file, meta tag, or DNS record, then run bwt sites verify.

changes Bing state (blocked by --read-only); Bing methods: AddSite.

```text
bwt sites add SITE
```

## bwt sites list

List the sites in your Bing Webmaster account

Lists your sites. Use these URLs exactly (scheme and trailing slash) in other commands.
JSON also includes AuthenticationCode and DnsVerificationCode for verification.

Bing methods: GetUserSites.

```text
bwt sites list
```

## bwt sites remove

Remove a site from your account

changes Bing state (blocked by --read-only); Bing methods: RemoveSite.

```text
bwt sites remove SITE
```

## bwt sites use

Set the profile's default site (local)

Saves SITE as the default for the selected profile. A bare host is matched against your sites.

changes local configuration.

```text
bwt sites use SITE
```

## bwt sites verify

Ask Bing to verify ownership of a site

Asks Bing to check the site's verification file, meta tag, or DNS record. Prints verified: false when Bing could not confirm ownership.

changes Bing state (blocked by --read-only); Bing methods: VerifySite.

```text
bwt sites verify SITE
```

## bwt stats

Search performance: Bing's top queries, pages, and traffic

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).

```text
bwt stats
```

## bwt stats detail

Position detail for one query and page

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data weekly.

Bing methods: GetQueryPageDetailStats.

```text
bwt stats detail QUERY URL [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats page-queries

Queries that lead to a page

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data weekly.

Bing methods: GetPageQueryStats.

```text
bwt stats page-queries URL [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats pages

Top pages

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data weekly.

Bing methods: GetPageStats.

```text
bwt stats pages [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats queries

Top queries

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data weekly.

Bing methods: GetQueryStats.

```text
bwt stats queries [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats query-pages

Pages that rank for a query

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data weekly.

Bing methods: GetQueryPageStats.

```text
bwt stats query-pages QUERY [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats query-traffic

Traffic history for one query (top queries only)

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data daily.

Bing methods: GetQueryTrafficStats.

```text
bwt stats query-traffic QUERY [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt stats summary

Totals over the latest N days of site traffic, computed locally

Sums GetRankAndTrafficStats over the N calendar days ending at the latest day Bing returned.
CTR is recomputed from the sums. --compare previous adds the preceding N days; percentage changes
are null when the earlier value is zero, and the CTR change is in percentage points.

Bing methods: GetRankAndTrafficStats.

```text
bwt stats summary [flags]
```

```text
      --compare string   Compare with: previous
      --days int         Window length in days (default 28)
```

## bwt stats traffic

Site clicks and impressions by day

Bing returns its own top rows with no date range, row limit, or paging on the server.
--since, --until, --limit, and --sort filter and sort locally; JSON records filtered_locally.
Positions are shown as Bing returns them (their scale is undocumented).
Bing updates this data daily.

Bing methods: GetRankAndTrafficStats.

```text
bwt stats traffic [flags]
```

```text
      --asc            Sort ascending
      --limit int      Keep at most N rows after sorting (local)
      --since string   Keep rows on or after this day (YYYY-MM-DD, local filter)
      --sort string    Sort locally by date, clicks, impressions, ctr, query, position, avg-click-position, or avg-impression-position (descending)
      --until string   Keep rows on or before this day (YYYY-MM-DD, local filter)
```

## bwt status

Alias for auth status

Local only unless --verify, which makes one GetUserSites call.

```text
bwt status [flags]
```

```text
      --verify   Check access with one GetUserSites call
```

## bwt submit

Submit URLs to Bing (prefer indexnow submit for new or changed URLs)

```text
bwt submit
```

## bwt submit urls

Submit URLs with Bing's URL Submission API

Submits URLs for crawling. One URL uses SubmitUrl; more are sent in batches of up to 500 with
SubmitUrlBatch, one after another. On a failure the command reports the batches Bing accepted and
stops; it never resubmits automatically because repeated submissions spend quota.
Accepted means Bing received the URLs; it does not mean they are indexed.

changes Bing state (blocked by --read-only); Bing methods: SubmitUrl, SubmitUrlBatch, GetUrlSubmissionQuota.

```text
bwt submit urls [URL...] [flags]
```

```bash
  bwt submit urls https://example.com/new-page
  bwt submit urls --file urls.txt --check-quota
```

```text
      --batch-size int   URLs per batch (1-500) (default 500)
      --check-quota      Read the URL submission quota first
      --file string      Read URLs from a file, one per line (- for stdin)
```

## bwt update

Install the latest GitHub release after SHA-256 verification

```text
bwt update [flags]
```

```text
      --check   Only check the latest published release
```

## bwt url

What Bing knows about a URL and the URLs under it

Bing's URL data has no explicit indexed verdict. IsPage and HttpStatus describe crawling, not indexing.

```text
bwt url
```

## bwt url children

URLs under a URL, with crawl details and filters

Lists URLs below URL. Pages start at 0; --all continues until Bing returns an empty page.
This is a read even though Bing implements it with POST.

Bing methods: GetChildrenUrlInfo.

```text
bwt url children URL [flags]
```

```text
      --all                 Fetch pages until the end (bounded by --max-pages)
      --crawl-date string   Crawled within: any, last-week, last-two-weeks, last-three-weeks (default "any")
      --discovered string   Discovered within: any, last-week, last-month (default "any")
      --doc-flags string    Comma list: blocked-by-robots, malware
      --http string         Comma list: 2xx, 3xx, 301, 302, 4xx, 5xx, other
      --max-pages int       Upper bound on pages fetched with --all (default 100)
      --page int            Page to fetch, starting at 0
```

## bwt url children-traffic

Clicks and impressions for URLs under a URL

Bing methods: GetChildrenUrlTrafficInfo.

```text
bwt url children-traffic URL [flags]
```

```text
      --all             Fetch pages until the end (bounded by --max-pages)
      --max-pages int   Upper bound on pages fetched with --all (default 100)
      --page int        Page to fetch, starting at 0
```

## bwt url info

Crawl details for one URL

Shows last crawl, discovery date, HTTP status, size, anchors, and child count. A zero status or IsPage=false is not an indexing verdict.

Bing methods: GetUrlInfo.

```text
bwt url info URL
```

## bwt url traffic

Clicks and impressions for one URL (window undocumented)

Bing methods: GetUrlTrafficInfo.

```text
bwt url traffic URL
```

## bwt users

Users and roles delegated on a site

```text
bwt users
```

## bwt users add

Delegate access to a user

Grants EMAIL a role on the site (or on --delegated-url, a host under it).
--auth-code defaults to the delegated site's AuthenticationCode from your site list.

changes Bing state (blocked by --read-only); Bing methods: AddSiteRoles, GetUserSites.

```text
bwt users add EMAIL [flags]
```

```text
      --auth-code string       Authentication code of the delegated site (default: looked up)
      --delegated-url string   Site or host to delegate (default: the selected site)
      --role string            Role: admin, read-only, or read-write (required)
```

## bwt users list

List users with access to the site

Bing methods: GetSiteRoles.

```text
bwt users list [flags]
```

```text
      --all-subdomains   Include users of all subdomains
```

## bwt users remove

Remove a user's delegated access

Looks up the user's role with GetSiteRoles and removes exactly that role. Several matches are an error;
narrow them with --delegated-url.

changes Bing state (blocked by --read-only); Bing methods: GetSiteRoles, RemoveSiteRole.

```text
bwt users remove EMAIL [flags]
```

```text
      --delegated-url string   Only the role on this site or host
```

## bwt version

Print build information

```text
bwt version
```
