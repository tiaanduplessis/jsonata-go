# Contributing

Thank you for contributing to jsonata-go. Discuss substantial API,
compatibility, security, or language-semantics changes before opening a pull
request. Keep changes focused and preserve unrelated work in a shared
checkout.

## Prerequisites

- Use the latest patch release of Go 1.26 or 1.27. CI tests both lines.
- Unicode-sensitive behavior follows the tables selected by the supported Go
  toolchain and `golang.org/x/text` v0.39.0. Go 1.26 uses Unicode 15.0.0;
  Go 1.27 and the Node.js 24 oracle use Unicode 17.0.0.
- Use Git and a POSIX shell. The repository uses `make` for its canonical
  checks.
- Node.js 24 and npm are needed only for benchmark-oracle checks or for
  regenerating JavaScript differential oracles. `testdata/benchmark/package-lock.json`
  pins the benchmark dependency.
- Most checks use committed fixtures and need no network once Go modules and
  the benchmark npm dependency are cached. `make vulncheck`, `make bench`,
  `make benchmark-oracle-check`, and commands that download uncached Go or npm
  dependencies may need network access. Runtime evaluation never needs Node,
  npm, JavaScript, cgo, or an external process.

Start with:

```text
make fmt-check lint test test-race docs-check
```

## Repository map

- The root package (`github.com/tiaanduplessis/jsonata-go`) owns the public
  API: `Compile`, `MustCompile`, `Expr`, `Engine`, `EvalOptions`, extensions,
  variables, and structured errors.
- `internal/syntax` contains the lexer, parser, immutable AST, and source
  positions. `internal/evaluator` contains the per-call evaluator, standard
  functions, guardrails, and correctness-gated fast paths.
- `internal/value` contains evaluator-only values such as undefined,
  sequences, ordered objects, and safe input normalization. These values must
  not leak through the public API.
- `jparse`, `jlib`, and `jtypes` provide compatibility packages and extension
  types. `internal/conformance`, `internal/differential`, `internal/security`,
  and `internal/benchmark` contain the evidence harnesses.
- `testdata/reference/jsonata-js-v2.2.2` is the pinned language-neutral suite.
  `testdata/differential` contains generated programs and JavaScript oracles.
  `testdata/benchmark` contains the frozen matrix, corpus, generator, and npm
  lockfile. `reports` contains machine-readable evidence.
- `.github/workflows/ci.yml` defines the normal CI gates. The tagged release
  workflow is in `.github/workflows/release.yml`.

## API and compatibility boundaries

The current development version is `0.0.0-dev`. The recommended first public
release is `v0.1.0`, but this is still a pre-1.0 API: exported APIs and
compatibility behavior may change before a separate v1.0 stability commitment.

Preserve the established compatibility surface: `Compile`, `MustCompile`,
`Expr.Eval`, `Expr.EvalBytes`, `RegisterExts`, `RegisterVars`, `Extension`,
undefined handling, and structured error fields. New embedding behavior should
use `Engine` and `EvalWithOptions` without changing existing callers.

The semantic target is JSONata 2.2 with fixes through `jsonata-js v2.2.2`,
whose pinned revision is recorded in `testdata/reference/.../REFERENCE_COMMIT`
and generated reports. `blues/jsonata-go` is a migration API reference, not the
language oracle. `gnata` is a comparison implementation for differential and
benchmark evidence. Do not add application-specific behavior to this library.

Compiled expressions and engines are safe for concurrent evaluation. Extension
code remains responsible for synchronizing its own shared state. Public inputs,
bindings, and extension results are normalized and copied; cyclic, unsupported,
non-finite, or over-depth values must retain their documented errors.

Every optimization must prove equivalence with the full evaluator. Add a
fallback when the input or expression is outside the proven subset. Compare
both values and structured errors, including JSONata code, token, value, and
position where applicable.

## Test and evidence commands

| Change | Required checks | Notes |
| --- | --- | --- |
| Formatting, parser, evaluator, API, or standard functions | `make fmt-check lint test test-race` | `make test` runs all Go tests. |
| JSONata language behavior | `make conformance` | Uses the pinned suite and writes `reports/conformance/report.json`; no Node is required. A complete run must have no failures, skips, or remaining cases/groups. |
| Generated differential behavior | `make differential` | Replays committed Go and security corpora and verifies generated files are current; it is offline after dependencies are cached. |
| New differential corpus or JavaScript oracle | `make differential-oracle JSONATA_JS_CHECKOUT=/path/to/jsonata-js` | Requires Node and a checkout at the exact pinned commit. Review `testdata/differential/README.md` and preserve the oracle contract. |
| Security or resource-boundary behavior | Add a focused test under `internal/security` or the relevant package, then run `make differential fuzz-smoke vulncheck` | Keep adversarial inputs bounded. Use `SECURITY.md` for vulnerability reports. |
| Fuzz target or parser/evaluator boundary | `make fuzz-smoke`; for deeper work, run the relevant `go test -fuzz` target with a bounded `-fuzztime` | Fuzz targets are discovered under `internal/...`. Turn every fixed finding into a deterministic regression. |
| Performance-sensitive evaluator or API code | `make benchmark-oracle-check`, `make bench`, then `make benchmark-claim` | The oracle check uses Node/npm. `make bench` writes authenticated raw and summary evidence under `reports/benchmark`; the claim command is strict and must pass before publishing a performance claim. |
| Profile investigation | `make benchmark-profile`; validate with `make benchmark-profile-check` | Profiles are host-specific evidence. Do not treat them as a general performance claim. |
| Release preparation | `make release-verify VERSION=vX.Y.Z` | Runs the complete release gate, including benchmark claim, module verification, and evidence checks. |

The complete local quality set is:

```text
make fmt-check lint test test-race conformance differential fuzz-smoke vulncheck docs-check
```

Run focused package tests while iterating, but report the full applicable
commands in the pull request. `go test ./...` and the checks using committed
fixtures do not require Node. `make bench` and `make release-verify` are
resource-intensive; run them on a stable, adequately powered host and record
any environment limitation.

## Adding regressions and updating fixtures

Choose the smallest durable test location:

1. Put ordinary API and evaluator behavior beside the affected package.
2. Put parser or source-position behavior in `internal/syntax` tests.
3. Put compatibility cases that exercise the public surface in root package
   tests, comparing fast and forced-full evaluation when an optimization is
   involved.
4. Put security, cancellation, cycle, depth, operation, and memory-boundary
   cases in `internal/security` or the closest evaluator package.
5. Put local regressions in the affected package tests or the differential
   corpus/tests. Never hand-edit the pinned upstream suite under
   `testdata/reference/`; changes to that suite happen only through an audited
   synchronization at a reviewed upstream commit or an explicit
   compatibility-target update. Missing support must remain visible; it must
   not be hidden as a pass.

For synchronized upstream material, use
`.github/scripts/sync-jsonata-suite.sh`, record the reviewed upstream commit,
and preserve the upstream `LICENSE` and notices. Do not hand-edit generated
oracles. The differential generator rejects a checkout that is not at
`6c7e95fdbf4405a1e741852a7cd8cd985b4305bb`.

For benchmark fixtures, change `testdata/benchmark/matrix.json` first, then
regenerate `corpus.json` with `make benchmark-oracle` and review all source and
hash changes. Prefer `make benchmark-oracle-check` for routine validation,
because it verifies drift without rewriting the frozen fixture. Do not commit
`testdata/benchmark/node_modules` or private input data.

## Generated evidence and ownership

Generated evidence is reproducible output, not a substitute for tests. Do not
edit it by hand. The owning commands are:

- `make conformance` owns `reports/conformance/report.json`.
- `go run ./cmd/differential` owns the differential corpus and
  `reports/feature-matrix.json`.
- Security regression metadata belongs with its tests in
  `reports/security-regressions.json`.
- `make bench` owns `reports/benchmark/verification.json`, raw runs, the JSON
  and Markdown reports, and `benchstat.txt`; `make benchmark-profile` owns
  `reports/benchmark/profiles`.

Commit generated evidence only when the source, fixtures, or benchmark run
that explains it is part of the same reviewed change. Benchmark manifests bind
evidence to the source revision, machine, corpus, commands, and file hashes;
the repository must be clean apart from the allowed evidence output during
collection. Unsupported competitor cells are reported but are not timed or
counted as wins. A fastest-library statement is allowed only when
`reports/benchmark/report.json` has `claim.met: true` and
`make benchmark-claim` passes.

## Pull requests and commits

Use a focused Conventional Commit subject, for example `fix: preserve error
position` or `test: add regex regression`. Keep unrelated work out of the
commit. The pull request description must include:

- the behavior and compatibility boundary changed;
- the exact tests and gates run, including any unavailable Node, npm, network,
  host, or toolchain dependency;
- generated evidence and fixture provenance, when changed;
- security or resource implications; and
- known limitations that remain outside the change.

Do not include credentials, private input data, generated files that are not
needed to reproduce the change, or copied upstream material without its
license notice. New dependencies must be documented in
[DEPENDENCIES.md](DEPENDENCIES.md), use an immutable or tagged version, pass
`go mod verify`, and pass `make vulncheck`.

## Release boundary

A pull request does not create a tag or publish a release. The recommended
first public release is `v0.1.0`; no tag is implied by this document. Before
publishing it, maintainers should complete the API and compatibility review,
run the clean-checkout release gates, create the reviewed semantic-version tag,
verify the public module and release artifacts, and announce the release. The
tagged workflow owns the release mechanics and its gates; see
[RELEASING.md](RELEASING.md) for the exact handoff and
[SECURITY.md](SECURITY.md) for release security controls.

## Reporting issues

Include the Go version, operating system, jsonata-go version or commit, the
smallest expression and input that reproduce the issue, and the complete
structured error. For security issues, use [SECURITY.md](SECURITY.md) instead
of a public issue.
