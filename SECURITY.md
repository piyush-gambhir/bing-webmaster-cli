# Security policy

Security fixes target the latest release. Update before reporting a potential issue.

Report vulnerabilities privately using [GitHub private vulnerability reporting](https://github.com/piyush-gambhir/bing-webmaster-cli/security/advisories/new).
Do not include real API keys, OAuth tokens, or private site data in a public issue.

## Credential handling

- Saved secrets (OAuth refresh and access tokens, API keys, your own client secrets) are stored in the OS
  keychain: macOS Keychain, Windows Credential Manager, or the Linux Secret Service. Keychain calls time out
  after 10 seconds.
- A plaintext `secrets.yaml` (mode 0600, not encrypted) is used only when you pass `--insecure-storage`.
  The config file never holds secrets.
- API keys travel in the `apikey` query parameter, as Bing requires. The CLI removes them from verbose
  logs, error messages, and transport errors. Prefer `BWT_API_KEY` or `auth login` over the `--api-key`
  flag, which can be recorded in shell history.
- HTTP redirects are never followed for authenticated calls, so credentials cannot be forwarded.
- OAuth login binds the callback port before opening the browser, requires a matching `state`, sends PKCE
  parameters, and exchanges the code exactly once. Bing documents no revocation endpoint; revoke access in
  Bing Webmaster Tools > Settings > API Access.

## Built-in OAuth client

Release binaries embed an OAuth client ID and secret injected at build time; they are not in the
repository. Secrets inside distributed binaries can be extracted and are not treated as confidential. See
[docs/auth.md](docs/auth.md) for the threat model and alternatives (API key, your own client).

## Scope

Commands that change Bing state are marked in the [API coverage map](docs/api-coverage.md) and blocked by
`--read-only`. `update` replaces the local executable after SHA-256 checksum verification.

Releases are immutable once published and include SBOMs and signed build-provenance attestations; verify an
archive with `gh attestation verify <archive> --repo piyush-gambhir/bing-webmaster-cli`.

Microsoft controls the upstream APIs. Report Bing service vulnerabilities through Microsoft's security
reporting process rather than this repository.
