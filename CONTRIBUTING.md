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
temporary directories. Never commit API keys or other credentials.

Where things go:

- `internal/registry`: one entry per Bing method (verb, parameters, effect, status). It is checked against the
  vendored documentation snapshot; see [docs/compatibility.md](docs/compatibility.md).
- `internal/client`: transport, `d` unwrapping, fault detection, date codec, redaction.
- `internal/auth`, `internal/secrets`: secret naming, the per-profile lock, and keychain storage.
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

To release, change `cli-go/VERSION` in a pull request. When it merges into `main`, the release workflow tags
`vX.Y.Z` on the merge commit, runs the tests and `govulncheck`, builds with GoReleaser (archives, checksums,
SBOMs), attests build provenance, and only then publishes the release. Published releases are immutable, so
a mistake needs a new version. If a release fails, open that failed run in the Actions tab and choose
Re-run jobs: it keeps the original commit and resumes the draft. A version that is already published is
skipped, and releases only run from `main`. Versions with a suffix (`0.2.0-rc.1`) become pre-releases, and
only the newest stable version is marked latest. Do not push tags by hand. Release builds need no repository
secrets. After changing `.goreleaser.yaml`, run `goreleaser check --config cli-go/.goreleaser.yaml`;
a local packaging check is `goreleaser release --snapshot --clean --skip=publish --config cli-go/.goreleaser.yaml`
(SBOMs need Syft installed).

`main` is protected: changes land through pull requests (squash or rebase) with signed commits, linear
history, and passing CI (tests on Linux, macOS, and Windows, staticcheck, govulncheck, the release check) and
CodeQL. Dependabot minor and patch updates merge automatically once those checks pass; major updates wait for
review. Release tags cannot be moved or deleted.
