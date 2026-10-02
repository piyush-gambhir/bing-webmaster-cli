# Contributing

Use Go 1.26+ (toolchain Go 1.27.1). From the repository root:

```bash
make test
make vet
make build
make docs
```

Run `gofmt -w .` within `cli-go/` after Go changes. Tests must use fake transports or local test servers and
`keyring.MockInit()`; they must not contact Bing, IndexNow, or a real keychain. Configuration tests use
temporary directories. Never commit API keys, OAuth tokens, client secrets, or `cli-go/.env.local`.

Where things go:

- `internal/registry`: one entry per Bing method (verb, parameters, effect, status). It is checked against the
  vendored documentation snapshot; see [docs/compatibility.md](docs/compatibility.md).
- `internal/client`: transport, `d` unwrapping, fault detection, date codec, redaction.
- `internal/auth`, `internal/oauthflow`, `internal/secrets`: login, refresh, and secret storage.
- `cmd`: command composition. Use `opCmd` or `annotate` so read/write effects come from the registry.
- `internal/output`: table, JSON, YAML, and CSV rendering.

Keep local input validation ahead of credential resolution and network requests. Adding or reclassifying a
command requires updating the safety manifest digest after reviewing its annotations. Run `make docs` after
changing commands or flags; CI fails when `docs/commands.md` or `docs/api-coverage.md` drift.

Submit changes through a pull request. Commits must be signed. The default branch requires linear history,
resolved review threads, and passing Go CI and CodeQL checks.

## Dependency maintenance

Prefer current stable releases, pinned to exact module versions and immutable GitHub Action commit SHAs.
Dependabot checks daily and groups minor/patch updates. Run `go get -u -t ./...` and `go mod tidy` inside
`cli-go`, then run platform CI. Update the dependency table in docs/compatibility.md.

## Releases

Repository secrets `BWT_OAUTH_CLIENT_ID` and `BWT_OAUTH_CLIENT_SECRET` provide the built-in OAuth client to
release builds. Run `goreleaser check --config cli-go/.goreleaser.yaml` from the root, run CI, set
`cli-go/VERSION`, and push a signed version tag. A local packaging check is
`goreleaser release --snapshot --clean --skip=publish --config cli-go/.goreleaser.yaml`.
