# Bing Webmaster CLI agent guide

Use `bwt --help` and `docs/commands.md` for current flags; `bwt/SKILL.md` has operational guidance.

- Implementation: Go/Cobra in `cli-go/`; module `github.com/piyush-gambhir/bing-webmaster-cli/cli-go`.
- `make test`, `make vet`, `make build`, and `make docs` run from the repo root. `make docs` regenerates
  both `docs/commands.md` and `docs/api-coverage.md`. A unit test fails when `api-coverage.md` drifts;
  CI fails when either drifts.
- Every Bing method lives in `internal/registry` with its verb, parameters, and effect. Commands declare
  the methods they call with `annotate`, which sets `mutates` and `experimental` from the registry. Never
  hand-write those annotations for registry methods, and never classify effect by HTTP verb.
- The registry is tested against the vendored documentation snapshot
  (`internal/registry/testdata/bing-webmaster-api.methods.json`). When upstream changes, refresh it as
  `docs/compatibility.md` describes and update the sha256 there.
- Adding or reclassifying a command changes the safety manifest digest in `cmd/safety_manifest_test.go`;
  review the annotations, then update the digest deliberately.
- Credentials: secrets go to the OS keychain through `internal/secrets`; the plaintext file is only for
  explicit `--insecure-storage`. Never write secrets to the config file, stdout, logs, or errors. API keys
  travel in the URL, so keep using the client's redaction.
- OAuth: `state` is mandatory, the callback port is bound before the browser opens, token requests use
  `AuthStyleInParams` (exactly one request), and refresh runs under the per-profile lock.
- Preserve stdout as command data and stderr as diagnostics. No automatic retries, no background calls.
- Do not present performance rows as complete, submissions as indexed, or empty legacy results as healthy.
- Tests use fake transports, temporary configs, and `keyring.MockInit()`; never the network or a real keychain.
- Document user-visible changes in README.md, docs/, and `bwt/SKILL.md`.
- Releases: bump `cli-go/VERSION` in a pull request; merging to `main` tags and publishes it (see
  CONTRIBUTING.md). Never push release tags by hand. `main` requires pull requests and passing checks.
