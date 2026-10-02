# Authentication

Bing Webmaster Tools offers two credentials: OAuth 2.0 (scopes `Webmaster.read` and `Webmaster.manage`)
and an API key that covers every verified site in your account. `bwt` supports both, plus externally
issued bearer tokens. IndexNow needs neither: it uses a public key hosted on your site.

## Quick reference

| Method | Command | Best for | Saved |
| --- | --- | --- | --- |
| Browser login, built-in client | `bwt auth login` | Most people | refresh token in the OS keychain |
| API key | `bwt auth login --with-api-key` | Servers, CI, or when the browser flow is unavailable | API key in the OS keychain |
| API key from the environment | `BWT_API_KEY=... bwt ...` | CI with a secret manager | nothing |
| Bearer token from the environment | `BWT_ACCESS_TOKEN=... bwt ...` | Tokens issued by your own tooling | nothing |
| Your own OAuth client | `bwt auth login --client-id ID --client-secret-stdin --redirect-port N` | Teams that want their own app registration | refresh token and client secret in the keychain |
| Read-only OAuth | `bwt auth login --scope read` | Least privilege | as browser login |

Credential precedence: `--access-token`, then `BWT_ACCESS_TOKEN`, then `--api-key`, then `BWT_API_KEY`,
then the selected profile. Profile selection: `--profile`, then `BWT_PROFILE`, then the saved current
profile. An environment credential overrides a saved profile; `bwt auth status` shows which one is active.

## Browser login (default)

```console
$ bwt auth login
Opening your browser to sign in. If it does not open, visit:
  https://www.bing.com/webmasters/oauth/authorize?...
Found 2 sites. Choose a default with: bwt sites use SITE
auth         oauth
profile      default
saved        true
sites        2
token_store  keychain
verified     true
```

What happens:

1. `bwt` binds `127.0.0.1:47619` (the redirect registered with the built-in client) **before** opening
   the browser. If the port is busy, login stops and tells you; it never switches ports, because Bing
   requires an exact redirect match.
2. The consent URL carries a random `state` and PKCE S256 parameters. The URL is printed on stderr in
   case the browser does not open.
3. After you click Allow, `bwt` checks `state` (a missing or wrong value fails the login), exchanges
   the code once, and saves the refresh token in the OS keychain.
4. It calls `GetUserSites` once. With one site, that site becomes the profile's default; with several,
   an interactive terminal shows a picker, otherwise you choose later with `bwt sites use SITE`.

Access tokens last about an hour. `bwt` refreshes them on demand under a per-profile lock, so several
commands running at once refresh only once. Bing's refresh tokens stay valid until access is revoked.

`--no-input` never opens a browser; use `--with-api-key` with the key on stdin instead.

## API key

Generate a key in **Bing Webmaster Tools > Settings > API Access > API Key**. One key covers all your
verified sites. A new key can take about 30 minutes to start working.

```bash
bwt auth login --with-api-key                        # hidden prompt
printf '%s' "$KEY" | bwt auth login --with-api-key --no-input --profile ci
BWT_API_KEY="$KEY" bwt sites list                    # nothing saved
```

The key travels as the `apikey` query parameter, so `bwt` removes it from verbose logs, error messages,
and transport errors itself. Prefer the environment variable or `auth login` over the `--api-key` flag,
which can end up in shell history.

## Where secrets live

Secrets go to the OS keychain: macOS Keychain, Windows Credential Manager, or the Secret Service on Linux.
Entries are namespaced by config file: the default config uses the service `bing-webmaster-cli`, and any
other path (`BWT_CONFIG`, a custom `XDG_CONFIG_HOME`, or one config per environment) gets
`bing-webmaster-cli (<hash>)`, so same-named profiles in different configs never overwrite each other.
The config file (`~/.config/bing-webmaster-cli/config.yaml`, mode 0600) holds profile names, the auth
method, the OAuth client ID, the default site, and IndexNow keys, which are public by design.

Keychain calls time out after 10 seconds, so a locked keychain waiting for an unlock prompt fails cleanly.
On a machine without a keychain service, login stops and offers two choices:

- `--insecure-storage`: keep secrets in `secrets.yaml` next to the config, mode 0600, **unencrypted**.
  The choice is recorded on the profile and reported by `bwt doctor`.
- `BWT_API_KEY` or `BWT_ACCESS_TOKEN`: nothing is saved at all.

Each OAuth profile has two keychain items: the refresh token (with the client secret for your own
client) and a cached access token. They are separate so each item stays under Windows Credential
Manager's 2,560-byte limit.

## Logout and revocation

`bwt auth logout` deletes the profile's saved credentials and keeps its default site and IndexNow keys.
Bing documents no token revocation endpoint, so to revoke access remove the app or regenerate the API key
under **Bing Webmaster Tools > Settings > API Access**.

## Your own OAuth client

1. In Bing Webmaster Tools, open **Settings > API Access**, accept the terms, choose **OAuth Client**, and
   register your client with redirect URI `http://127.0.0.1:PORT/callback`.
2. Log in with your client ID and the matching port. The secret is read from stdin, never from a flag:

```bash
bwt auth login --client-id "$CLIENT_ID" --client-secret-stdin --redirect-port 8400 < client-secret.txt
```

The profile records the client ID, so refreshes always use the client that issued the token.

## Built-in OAuth client: one-time owner setup

Release binaries embed the project's OAuth client through build-time `-ldflags`. The values are never in
the repository; a build without them still supports API keys, and `BWT_CLIENT_ID` / `BWT_CLIENT_SECRET`
override them at run time.

1. In Bing Webmaster Tools, open **Settings > API Access > OAuth Client** and register the name "bwt CLI"
   with redirect URI `http://127.0.0.1:47619/callback`.
2. Add the client ID and secret as repository secrets `BWT_OAUTH_CLIENT_ID` and
   `BWT_OAUTH_CLIENT_SECRET` (the release workflow passes them to GoReleaser), and to an untracked
   `cli-go/.env.local` for local builds:

   ```make
   BWT_OAUTH_CLIENT_ID=...
   BWT_OAUTH_CLIENT_SECRET=...
   ```

3. Run the live checks below once and record the answers in [compatibility.md](compatibility.md).

| Check | How | If it fails |
| --- | --- | --- |
| Bing accepts the loopback redirect | `bwt auth login` completes | Browser login is unusable; document `--with-api-key` as the login path |
| Bing echoes `state` | Login succeeds (it fails closed without `state`) | Same as above |
| PKCE is enforced | Optional: exchange a code without `code_verifier` using a test script | Note that PKCE is not enforced |
| Bearer tokens work on `ssl.bing.com` | `BWT_OAUTH_API_HOST=ssl.bing.com bwt sites list` | Keep the default `www.bing.com` host |

### Why shipping a client secret is acceptable here

Bing's OAuth flow requires a client secret and documents no PKCE. Any secret inside a distributed binary
can be extracted (RFC 8252 says such secrets must not be treated as confidential), and a loopback
redirect identifies an address, not an app. The remaining risk is a malicious local process intercepting
a code. `bwt` binds the callback port before opening the browser, so a process already holding the port
makes the login abort instead of receiving a code; codes expire after 5 minutes; and anyone who wants no
shared secret can use an API key or their own client. A server-side token broker that keeps the secret off
user machines is the stronger design if the built-in client sees wide use.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `InvalidApiKey` right after creating a key | Wait about 30 minutes; new keys take time to activate |
| `cannot listen on 127.0.0.1:47619` | Another login is running or another program holds the port; finish it or use `--with-api-key` |
| `state mismatch` | Start the login again from the terminal; do not reuse an old browser tab |
| `different OAuth client than this build provides` | You logged in with another build or client ID; run `bwt auth login` again |
| `invalid_grant` | Access was revoked or expired; run `bwt auth login` |
| `No OS keychain is available` | Use `--insecure-storage`, or `BWT_API_KEY` without saving |
| `NotAuthorized` | Use the site URL exactly as `bwt sites list` prints it (scheme and trailing slash) |
