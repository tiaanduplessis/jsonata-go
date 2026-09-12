# JSONata Go benchmark report

The scoped performance claim gate did not pass. No fastest-library claim is supported by this run.

Evidence recorded: `2026-09-12T12:25:59Z`. Report generated: `2026-09-12T12:34:49Z`.

## Environment

| Field | Value |
|---|---|
| Go | `go1.27.1` |
| OS/architecture | `darwin/arm64` |
| CPU | Apple M5 |
| Logical CPUs / GOMAXPROCS | 10 / 10 |
| Power | AC power |
| Source revision / dirty (benchmark artifacts excluded) | `7a27d198a8b0d84cb94bf9bd996fde959c6a7c6d` / false |
| Repetitions / benchtime / warm-ups | 10 / `200ms` / 1 |

## Pinned implementations

| ID | Module | Version | Commit |
|---|---|---|---|
| jsonata-go | `github.com/tiaanduplessis/jsonata-go` | `workspace` | `` |
| blues | `github.com/blues/jsonata-go` | `v1.5.4` | `e0d39c06990dd541e7d6dbac338853bef894b8f4` |
| gnata | `github.com/recolabs/gnata` | `v0.2.3` | `8abdb304a3c096c88a43760fd1160ada1851b29d` |

Oracle: `jsonata-js 2.2.2` at `6c7e95fdbf4405a1e741852a7cd8cd985b4305bb`.

## Comparable results

### compile

Comparable cases: 15. Complete required coverage: true.

| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|
| jsonata-go | 1465.83 | 5023.99 | 47.40 |
| blues | 625.36 | 1665.33 | 30.53 |
| gnata | 840.36 | 4462.67 | 28.53 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 2.344 | 2.287-2.403 | false |
| gnata | 1.744 | 1.692-1.798 | false |

### decoded

Comparable cases: 15. Complete required coverage: true.

| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|
| jsonata-go | 565.72 | 344.53 | 5.13 |
| blues | 2166.55 | 4016.97 | 112.00 |
| gnata | 614.07 | 1707.63 | 27.67 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 0.261 | 0.257-0.266 | true |
| gnata | 0.921 | 0.902-0.941 | true |

### bytes

Comparable cases: 14. Complete required coverage: true.

| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|
| jsonata-go | 2718.83 | 6344.98 | 117.21 |
| blues | 6180.37 | 8409.99 | 217.07 |
| gnata | 1631.25 | 7259.34 | 162.00 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 0.440 | 0.429-0.452 | true |
| gnata | 1.667 | 1.626-1.708 | false |

## Parallel throughput

| Implementation | Cases | Serial geometric mean ns/op | Parallel geometric mean ns/op | Throughput scale |
|---|---:|---:|---:|---:|
| jsonata-go | 15 | 565.72 | 208.40 | 2.71x |
| gnata | 15 | 614.07 | 375.70 | 1.63x |

## Unsupported cells

Unsupported cells were not timed and were not counted as wins.

| Implementation | Case | Mode | Class | Reason |
|---|---|---|---|---|
| blues | adversarial-deep-path | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | adversarial-descendant | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | adversarial-wide-object | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | large-filter-reduce | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | large-regex-scan | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | medium-aggregate | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | medium-higher-order | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | medium-transform | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | official-dataset1-contacts | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | official-dataset5-orders | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | small-comparison | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | small-custom-function | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | small-projection | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | small-regex | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| blues | small-simple-path | parallel | api-or-safety-limit | v1.5.4 does not document compiled expressions as safe for concurrent evaluation |
| gnata | small-custom-function | bytes | api-or-safety-limit | v0.2.3 has no raw-input evaluation API that accepts a custom-function environment |

## Claim gate

Scope: lowest statistically supported geometric-mean latency on complete decoded and raw-input comparable suites.

Met: **false**.

- jsonata-go is not statistically faster than gnata for bytes (upper 95% ratio 1.708)

## Method

Each implementation/case/mode passed the pinned oracle before it was registered as a benchmark. Collection rotates implementation order each round. Latencies use geometric means over log-transformed repeated estimates. Ratio intervals use a two-sided 95% normal interval over independent log measurements; a claim requires the upper workspace/competitor bound to be below 1 for every competitor in both decoded and raw-input suites. Allocation columns are arithmetic means. Raw-input measurements call each library's public raw-input API directly; return representation is library-specific. Size tiers use serialized input size: small is at most 128 bytes, medium is 129-512 bytes, and large is at least 513 bytes. Unsupported cells are listed above and excluded from timing and claims.

Raw output is in [`raw/`](raw/). Pairwise `benchstat` output is in [`benchstat.txt`](benchstat.txt).
CPU, allocation, mutex, and blocking evidence is in [`profiles/`](profiles/). Hardware cache counters were unavailable on this host, so no cache claim is made.
