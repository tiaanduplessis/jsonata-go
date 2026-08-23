# JSONata Go benchmark report

The scoped performance claim gate did not pass. No fastest-library claim is supported by this run.

Evidence recorded: `2026-08-23T11:55:41Z`. Report generated: `2026-08-23T12:05:31Z`.

## Environment

| Field | Value |
|---|---|
| Go | `go1.26.6` |
| OS/architecture | `darwin/arm64` |
| CPU | Apple M5 |
| Logical CPUs / GOMAXPROCS | 10 / 10 |
| Power | AC power |
| Source revision / dirty (benchmark artifacts excluded) | `ca6b2906c3025c113e428e4c2f045b76266fb095` / false |
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
| jsonata-go | 1992.89 | 5023.99 | 47.13 |
| blues | 893.18 | 1665.33 | 30.53 |
| gnata | 1009.00 | 4462.67 | 28.53 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 2.231 | 1.975-2.521 | false |
| gnata | 1.975 | 1.745-2.236 | false |

### decoded

Comparable cases: 15. Complete required coverage: true.

| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|
| jsonata-go | 711.57 | 344.53 | 5.13 |
| blues | 3028.04 | 4115.63 | 113.47 |
| gnata | 754.79 | 1705.29 | 27.60 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 0.235 | 0.211-0.262 | true |
| gnata | 0.943 | 0.852-1.043 | false |

### bytes

Comparable cases: 14. Complete required coverage: true.

| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |
|---|---:|---:|---:|
| jsonata-go | 2819.83 | 6525.27 | 116.64 |
| blues | 6635.27 | 8443.86 | 207.79 |
| gnata | 3203.00 | 11396.72 | 347.29 |

| Competitor | Workspace ratio | 95% interval | Statistically faster |
|---|---:|---:|---|
| blues | 0.425 | 0.380-0.475 | true |
| gnata | 0.880 | 0.801-0.968 | true |

## Parallel throughput

| Implementation | Cases | Serial geometric mean ns/op | Parallel geometric mean ns/op | Throughput scale |
|---|---:|---:|---:|---:|
| jsonata-go | 15 | 711.57 | 235.64 | 3.02x |
| gnata | 15 | 754.79 | 447.11 | 1.69x |

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

- jsonata-go is not statistically faster than gnata for decoded (upper 95% ratio 1.043)

## Method

Each implementation/case/mode passed the pinned oracle before it was registered as a benchmark. Collection rotates implementation order each round. Latencies use geometric means over log-transformed repeated estimates. Ratio intervals use a two-sided 95% normal interval over independent log measurements; a claim requires the upper workspace/competitor bound to be below 1 for every competitor in both decoded and raw-input suites. Allocation columns are arithmetic means. Raw-input measurements call each library's public raw-input API directly; return representation is library-specific. Size tiers use serialized input size: small is at most 128 bytes, medium is 129-512 bytes, and large is at least 513 bytes. Unsupported cells are listed above and excluded from timing and claims.

Raw output is in [`raw/`](raw/). Pairwise `benchstat` output is in [`benchstat.txt`](benchstat.txt).
CPU, allocation, mutex, and blocking evidence is in [`profiles/`](profiles/). Hardware cache counters were unavailable on this host, so no cache claim is made.
