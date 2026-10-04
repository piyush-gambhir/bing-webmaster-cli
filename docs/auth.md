# Authentication

Bing Webmaster Tools signs API calls with an API key. One key belongs to your Microsoft account and covers
every site you can access in Bing Webmaster Tools.

## Log in

```console
$ bwt auth login
Log in with your Bing Webmaster Tools API key (one key covers all your sites):
  1. Opening https://www.bing.com/webmasters
  2. Go to Settings (gear icon) > API Access > API Key, then Generate or copy your key.
  3. Paste it below. Input is hidden.
Bing Webmaster API key:
Logged in (profile "default"). Found 1 site; default site is https://www.example.com/ (change with: bwt sites use SITE).
```

What happens:

1. In a terminal, `bwt` opens Bing Webmaster Tools and reads the key from a hidden prompt. Piped input is
   read from stdin instead, so scripts work too.
2. The key is checked with one `GetUserSites` call **before** anything is saved. If Bing rejects it, nothing
   is saved. A newly generated key can take about 30 minutes to start working; `--no-verify` saves it
   without the check.
3. The key goes into the OS keychain (macOS Keychain, Windows Credential Manager, or the Secret Service on
   Linux). Without a keychain service, login stops unless you pass `--insecure-storage` (a 0600 plaintext
   file).
4. A default site is chosen: your only site, a numbered pick when you have several, or none under
   `--no-input` (set one later with `bwt sites use SITE`).

## Without saving anything (CI and agents)

```bash
BWT_API_KEY=... bwt sites list -o json                    # one command, nothing stored
printf '%s' "$KEY" | bwt auth login --no-input --profile ci  # or save it to a named profile
```

Prefer the environment variable or stdin over `--api-key`, which can end up in shell history.

## Several accounts or clients

Each profile is a separate login. `bwt auth login --profile client-b` adds one, `bwt auth list` shows them,
`--profile NAME` or `BWT_PROFILE` picks one per command, and `bwt auth use NAME` changes the default. Parallel
commands are safe: each profile has its own keychain entry, and login and logout take a per-profile lock.

Keychain entries are namespaced by config file: the default config uses the service `bing-webmaster-cli`,
and any other path (`BWT_CONFIG`, a custom `XDG_CONFIG_HOME`, or one config per environment) gets
`bing-webmaster-cli (<hash>)`, so same-named profiles in different configs never overwrite each other.

## Bearer tokens

`--access-token` or `BWT_ACCESS_TOKEN` sends a bearer token obtained elsewhere (for example from your own
OAuth web application). Bearer calls go to `www.bing.com`, as Bing's OAuth guide shows;
`BWT_OAUTH_API_HOST=ssl.bing.com` switches hosts.

## Why there is no browser login

Bing's OAuth client registration rejects loopback redirect URIs: `http://127.0.0.1:47619/callback` and
`http://localhost:47619/callback` are both refused as "not a valid http or https url" (checked 2026-10-04).
A command-line tool can only receive an OAuth code on the local machine, so browser OAuth would need a public
web page that relays codes to the CLI. That adds a server and a place where codes can leak, while an API key
gives the same access with one copy and paste.

## Credential precedence

`--access-token` > `BWT_ACCESS_TOKEN` > `--api-key` > `BWT_API_KEY` > the selected profile. Profile selection
is `--profile` > `BWT_PROFILE` > the saved current profile. `bwt auth status` shows which source is active
without a network call; add `--verify` for one `GetUserSites` check.

## Logging out and rotating the key

`bwt auth logout` deletes the saved key from this machine and keeps the profile's site and IndexNow keys.
It does not invalidate the key: Bing allows one API key per account, and regenerating it in **Settings >
API Access** invalidates it everywhere. After regenerating, run `bwt auth login` again on each machine.

The key travels in the `apikey` query parameter, as Bing requires. `bwt` removes it from verbose logs, error
messages, and transport errors.

## Troubleshooting

| Message | Fix |
| --- | --- |
| `Bing did not accept this key, so nothing was saved` | Check the key; a new key can take about 30 minutes to activate. Retry later, or pass `--no-verify` to save it now |
| `InvalidApiKey` | The key was regenerated or mistyped; run `bwt auth login` |
| `No OS keychain is available` | Start a keychain service, pass `--insecure-storage`, or use `BWT_API_KEY` |
| `OS keychain did not respond within 10s` | Unlock the keychain and retry |
| `NotAuthorized` | Use the site URL exactly as `bwt sites list` prints it, and check that your account can access the site |
