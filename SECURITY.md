# Security policy

Security fixes target the latest release. Update before reporting a potential issue.

Report vulnerabilities privately using [GitHub private vulnerability reporting](https://github.com/piyush-gambhir/bing-webmaster-cli/security/advisories/new).
Do not include real API keys or private site data in a public issue.

## Credential handling

- Saved API keys are stored in the OS keychain: macOS Keychain, Windows Credential Manager, or the Linux
  Secret Service, namespaced per config file. Keychain calls time out after 10 seconds.
- A plaintext `secrets.yaml` (mode 0600, not encrypted) is used only when you pass `--insecure-storage`.
  The config file never holds secrets.
- API keys travel in the `apikey` query parameter, as Bing requires. The CLI removes them from verbose
  logs, error messages, and transport errors. Prefer `BWT_API_KEY` or `auth login` over the `--api-key`
  flag, which can be recorded in shell history.
- HTTP redirects are never followed for authenticated calls, so credentials cannot be forwarded.
- `bwt auth login` checks a key with Bing before saving it, so a mistyped key is never stored. Logging out
  removes the key locally; regenerating it in Bing Webmaster Tools > Settings > API Access invalidates it
  everywhere.
- Release binaries contain no embedded credentials. Bing's OAuth registration rejects loopback redirect URIs,
  so the CLI has no browser OAuth login and ships no client secret (see [docs/auth.md](docs/auth.md)).

## Scope

Commands that change Bing state are marked in the [API coverage map](docs/api-coverage.md) and blocked by
`--read-only`. `update` replaces the local executable after SHA-256 checksum verification, extracting only
the `bwt` (or `bwt.exe`) regular file from the archive; a failed update leaves the old binary in place.

In an interactive terminal, `bwt` asks GitHub for the latest release at most once a day (an anonymous
request with no account or usage data) to print an update notice. It never runs when stderr is not a
terminal or `CI` is set, and `BWT_NO_UPDATE_NOTIFIER=1`, `NO_UPDATE_NOTIFIER=1`, or `--quiet` turns it off.
The result is cached in `update-check.json` in the config directory.

Releases are immutable once published and include SBOMs and signed build-provenance attestations; verify an
archive with `gh attestation verify <archive> --repo piyush-gambhir/bing-webmaster-cli`.

Microsoft controls the upstream APIs. Report Bing service vulnerabilities through Microsoft's security
reporting process rather than this repository.
