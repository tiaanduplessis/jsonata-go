# Dependency and license provenance

The module and checksum files are the source of truth for exact dependency
versions. This document records why the project uses each direct dependency and
where its license and provenance can be checked. Transitive dependencies are
covered by the same policy and are enumerated by `go list -m all` in a release
workflow.

The repository supports Go 1.26 and 1.27. With the pinned
`golang.org/x/text` v0.39.0 dependency, Go 1.26 uses Unicode 15.0.0 casing
tables while Go 1.27 uses Unicode 17.0.0. The Node.js 24 differential oracle
also uses Unicode 17.0.0. A toolchain or Unicode-table update requires fresh
compatibility evidence.

| Module | Use | License and provenance |
| --- | --- | --- |
| `github.com/dlclark/regexp2` | ECMAScript-compatible regular-expression fallback | MIT; [upstream repository](https://github.com/dlclark/regexp2) and the module's `LICENSE` file |
| `golang.org/x/text` | Unicode and text processing | BSD-3-Clause; [upstream repository](https://go.googlesource.com/text) and the module's `LICENSE` file |
| `golang.org/x/vuln` | Reproducible `govulncheck` release gate | BSD-3-Clause; [upstream repository](https://go.googlesource.com/vuln) and the module's `LICENSE` file |
| `github.com/blues/jsonata-go` | Compatibility and differential-test reference | MIT; [upstream repository](https://github.com/blues/jsonata-go) and the module's `LICENSE` file |
| `github.com/recolabs/gnata` | Differential-test and benchmark reference | MIT with an upstream notice; [upstream repository](https://github.com/RecoLabs/gnata) and the module's `LICENSE` and `NOTICE` files |

The two reference implementations are test and benchmark inputs; they are not
required to embed the evaluator at runtime. Keep them pinned while evidence is
being reproduced. A compatibility target changes only through a reviewed
change to the pinned reference, its fixtures, and the corresponding reports.

## Upstream JSONata material

The synchronized language-neutral suite under
`testdata/reference/jsonata-js-v2.2.2` comes from the pinned `jsonata-js`
commit recorded in `REFERENCE_COMMIT`. The sync script copies the upstream MIT
notice with the suite. The benchmark oracle under `testdata/benchmark` has its
own npm lockfile and records the same upstream reference; do not remove or
rewrite those notices when updating fixtures.

## Policy

Before adding a dependency, contributors must record its purpose, source
repository, license, and whether it is needed at runtime or only in tests or
tools. Use a tagged or immutable module version, run `go mod verify`, and run
`make vulncheck`. Changes to a pinned reference or generated fixture must show
the new source revision and preserve its license notices. Release automation
publishes the module list and checksum metadata so consumers can audit the
exact graph used for a tag.

This project does not copy third-party source into the main package without
retaining the original copyright and license terms. A dependency with an
unclear, incompatible, or undisclosed license is not accepted until its legal
status is resolved.
