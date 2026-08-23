# jsonata-go

[![CI](https://github.com/tiaanduplessis/jsonata-go/actions/workflows/ci.yml/badge.svg)](https://github.com/tiaanduplessis/jsonata-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tiaanduplessis/jsonata-go.svg)](https://pkg.go.dev/github.com/tiaanduplessis/jsonata-go)
[![License](https://img.shields.io/github/license/tiaanduplessis/jsonata-go)](LICENSE)

`jsonata-go` is a pure-Go implementation of the JSONata 2.2 expression
language. It is intended for applications that need to evaluate JSONata
without JavaScript, cgo, or an external process.

The compatibility baseline is `jsonata-js v2.2.2`. Conformance reports record
the exact upstream revision used as the semantic oracle. The public API is
built around immutable compiled expressions, ordinary Go JSON values,
and context-aware evaluation for safe embedding in concurrent services.

## Quickstart

Compile an expression once and evaluate it with ordinary Go JSON values:

```go
package main

import (
	"fmt"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

func main() {
	expression, err := jsonata.Compile(`Account.Name`)
	if err != nil {
		panic(err)
	}
	value, err := expression.Eval(map[string]any{
		"Account": map[string]any{"Name": "Ada"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(value)
}
```

Output:

```text
Ada
```

Compiled expressions support per-call controls without sharing state between
evaluations:

```go
import (
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

var input any
expr := jsonata.MustCompile("$eval('$x')")
value, err := expr.EvalWithOptions(input, jsonata.EvalOptions{
	Bindings: map[string]any{"x": 7},
	Timeout:  time.Second,
})
```

`Context`, `MaxCallDepth`, and `MaxOperations` are also available through
`EvalOptions`. The existing `Eval`, `EvalBindings`, and context-aware wrappers
remain compatible convenience APIs.

## Migrating extensions

Code using the `blues/jsonata-go` v1.5.4 registration API only needs its import
changed. The `Extension`, `RegisterExts`, `RegisterVars`, and matching `Expr`
methods retain the same shapes:

```go
import (
	"strings"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

expr := jsonata.MustCompile(`$uppercase(name)`)
err := expr.RegisterExts(map[string]jsonata.Extension{
	"uppercase": {Func: strings.ToUpper},
})
```

An extension may add `context.Context` as its first Go parameter. The evaluator
supplies the evaluation context without consuming a JSONata argument. Extension
calls are synchronous: context-aware functions can stop cooperatively when the
context is canceled, while legacy functions run to completion and are never
detached into background goroutines.

Use `NewEngine` when registrations must be isolated from package-level state.
An engine snapshots its registry when it compiles an expression; later engine
registrations affect only expressions compiled afterward. Expression-level and
per-call bindings override that snapshot.

### Migrating from gnata

`gnata` does not have an import-compatible registration surface, so migrate at
the expression boundary rather than relying on its implementation-specific
fast-path methods:

| gnata | jsonata-go |
| --- | --- |
| `gnata.Compile(source)` | `jsonata.Compile(source)` |
| `expression.Eval(ctx, input)` | `expression.EvalContext(ctx, input)` or `EvalWithOptions` |
| `expression.EvalBytes(ctx, raw)` | `expression.EvalBytesWithOptions(raw, EvalOptions{Context: ctx})` |
| `expression.EvalWithVars(ctx, input, vars)` | `expression.EvalWithOptions(input, EvalOptions{Context: ctx, Bindings: vars})` |

Adapt each `gnata.CustomFunc` to a typed `Extension` and register it with an
`Engine` or expression. `gnata`'s streaming evaluator, cache controls,
`IsFastPath`, and `OrderedMap` are not part of this API; use ordinary Go JSON
values and the documented guardrails instead. Re-run the conformance and
application tests after migration because the compatibility target is
JSONata 2.2.2, not a competitor's behavior.

## Installation and compatibility

The repository is currently a pre-release development version (`0.0.0-dev`);
no public release tag exists yet. The recommended first public release is
`v0.1.0`. Once that release is published, add it with the Go toolchain:

```text
go get github.com/tiaanduplessis/jsonata-go@v0.1.0
```

The supported Go major releases are 1.26 and 1.27; use the latest patch
release in either line. The language target is JSONata 2.2 with the fixes in
`jsonata-js v2.2.2`. The package is pure Go and does not start a JavaScript
runtime, invoke cgo, or execute an external process. The compatibility API is
kept for existing callers, while new code can use `Engine` and
`EvalWithOptions` to avoid package-global registrations.

The root `jsonata` package is the supported application API and is the
recommended import for new code. The exported `jparse`, `jtypes`, `jlib`, and
`jlib/jxpath` packages remain available as focused compatibility surfaces for
legacy integrations and extension implementations. Use them when you need
their specialized APIs; prefer the root package for compiling and evaluating
expressions. Their compatibility behavior is covered by the repository's
tests, but they are not a replacement for the higher-level root API.

This is a pre-1.0 API. The first `v0.1.0` release is intended for public
evaluation, but exported APIs and compatibility behavior may still change
before `v1.0.0`. Treat the API as subject to change until the project makes a
separate v1.0 stability commitment.

Project references:

- [Contributing](CONTRIBUTING.md), [Code of Conduct](CODE_OF_CONDUCT.md), and
  [security policy](SECURITY.md)
- [Release process](RELEASING.md)
- [Changelog](CHANGELOG.md)
- [Conformance evidence](reports/conformance/report.json)
- [Feature matrix and compatibility boundaries](reports/feature-matrix.json)
- [Benchmark evidence](reports/benchmark/report.json)

## Evaluation APIs

Use `Compile` when an expression will run more than once. `Eval` evaluates a
Go value and `EvalBytes` accepts one JSON document and returns JSON bytes. A
missing result is reported as `ErrUndefined`; an explicit `nil` input remains
JSON null. `EvalNoInput` is available when JSONata's empty input sequence is
required.

The context-aware forms are synchronous and do not detach work into a
background goroutine:

```go
import (
	"context"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

value, err := expr.EvalWithOptions(input, jsonata.EvalOptions{
	Context: ctx,
	Bindings: map[string]any{"limit": 10},
	MaxCallDepth: 64,
	MaxOperations: 500_000,
	MaxSequenceLength: 10_000,
})
```

`Engine` owns registrations and snapshots them when it compiles an expression.
An `Expr` may be evaluated concurrently. Registering an extension or variable
does not alter an evaluation already in progress. Extension functions that
share mutable state must still synchronize that state themselves.

This complete example installs the module, registers a function, applies a
deadline and operation budget, and evaluates one compiled expression from
multiple goroutines:

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

func main() {
	expr := jsonata.MustCompile(`$double(value)`)
	if err := expr.RegisterExts(map[string]jsonata.Extension{
		"double": {Func: func(value float64) float64 { return value * 2 }},
	}); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	options := jsonata.EvalOptions{Context: ctx, MaxOperations: 100_000}
	input := map[string]any{"value": 21.0}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := expr.EvalWithOptions(input, options)
			if err != nil {
				panic(err)
			}
			fmt.Println(value)
		}()
	}
	group.Wait()
}
```

## Custom functions and variables

Register a typed Go function with `Extension`. Its parameters and return values
are checked before invocation; a function may return either one value or a
value followed by `error`.

```go
import (
	"log"
	"strings"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

expr := jsonata.MustCompile(`$title(name)`)
if err := expr.RegisterExts(map[string]jsonata.Extension{
	"title": {Func: func(name string) string {
		return strings.ToUpper(name)
	}},
}); err != nil {
	log.Fatal(err)
}
value, err := expr.Eval(map[string]any{"name": "ada"})
```

A function whose first parameter is `context.Context` receives the active
evaluation context. It must check that context while doing blocking or
expensive work. Legacy functions without a context parameter are invoked
synchronously and cannot be forcibly interrupted by a timeout. The optional
`UndefinedHandler` and `EvalContextHandler` fields retain the compatibility
registration behavior.

Use `RegisterVars` for package, engine, or expression registrations. Prefer
per-call `EvalOptions.Bindings` for request-specific values. Maps, slices, and
extension results are normalized into owned evaluation values; cyclic values,
unsupported Go values, non-finite numbers, or values deeper than the input
normalization limit return an error. Do not mutate a value concurrently with
an evaluation or rely on a caller-owned map being retained after the call.

## Errors and guardrails

Syntax and evaluation failures that have a JSONata code return `*Error` (also
available as the `JSONataError` alias). Its `Code`, `Token`, `Value`, and
`Position` fields are stable inspection fields. Use `errors.As` to inspect it
and `errors.Is` for a wrapped context cancellation or deadline. Extension
argument mismatches return `*ArgCountError` or `*ArgTypeError`; an empty result
returns `ErrUndefined`.

`EvalOptions` applies limits to one call. A zero or negative `MaxCallDepth`
uses the default depth of 100, and a zero or negative `MaxOperations` uses the
default budget of 100,000. Set `Timeout` or `Context` for cancellation;
`MaxSequenceLength` is disabled when it is zero or negative and limits positive
sequence growth when enabled. Resource-limit failures use the documented
JSONata error taxonomy (including `U1001` for evaluation budget/depth and
`D2015` for the configured sequence limit). The JSONata range maximum remains
`D2014` and is independent of `MaxSequenceLength`. Choose limits for the input
size and extension workload, and test cancellation behavior in the extensions
you provide.

Inspect structured errors with `errors.As` and sentinel errors with
`errors.Is`:

```go
import (
	"context"
	"errors"
	"fmt"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

canceled, cancel := context.WithCancel(context.Background())
cancel()
_, err := jsonata.MustCompile(`1 + 1`).EvalContext(canceled, nil)

var structured *jsonata.Error
if errors.As(err, &structured) {
	fmt.Println(structured.Code, errors.Is(err, context.Canceled))
}

_, err = jsonata.MustCompile(`missing`).Eval(nil)
fmt.Println(errors.Is(err, jsonata.ErrUndefined))
// U1001 true
// true
```

## Performance evidence

Benchmark reports are scoped to the frozen corpus, toolchain, host, and
correctness-gated implementation matrix. Unsupported competitor cells are not
timed or counted as wins. The repository makes no general fastest-library
claim: such a claim is valid only when `reports/benchmark/report.json` records
`claim.met: true` for complete decoded and raw-input suites. Reproduce the
evidence with `make bench`; see the generated report for the exact machine,
source revision, raw data, and statistical method.

The pinned language-neutral suite can be synchronized with its upstream MIT
notice preserved:

```text
./.github/scripts/sync-jsonata-suite.sh testdata/reference/jsonata-js-v2.2.2
```

The default reference commit is `6c7e95fdbf4405a1e741852a7cd8cd985b4305bb`,
the commit tagged `v2.2.2`.

### Regex string boundary

Regular expressions use ECMAScript UTF-16 matching and indexes, including for
astral characters. Go strings cannot safely represent an isolated UTF-16
surrogate. Boolean matches remain exact, but a match, capture, replacement,
split part, or callback argument that would contain an isolated surrogate
returns `U1002` instead of emitting invalid UTF-8 or replacement characters.
The [feature matrix](reports/feature-matrix.json) records this documented
boundary. Exact representation of those results would require a future opt-in
UTF-16 result API.

### Other documented compatibility boundaries

Unicode case mapping is toolchain-dependent. With the pinned
`golang.org/x/text` v0.39.0 dependency, Go 1.26 uses Unicode 15.0.0 and leaves
U+019B unchanged, while Go 1.27 uses Unicode 17.0.0 and uppercases it to
U+A7DC, matching the Node.js 24 oracle. The project does not promise
host-independent ECMAScript case mappings; rerun the pinned oracle checks when
a supported toolchain or Unicode table changes. This boundary and its evidence
are recorded in the [feature matrix](reports/feature-matrix.json).

## Public-release status

The repository contains the pre-release implementation and its pinned
conformance, differential, security, and benchmark harnesses. The committed
conformance report currently records zero failures and zero skips, but that is
not an API-stability guarantee. Maintainers recommend `v0.1.0` as the first
public release after the API review and clean-checkout release gates pass; no
tag is claimed here. Performance remains a scoped evidence claim only: if the
strict benchmark claim gate is not met, no fastest-library claim is published.

For the public-release sequence, maintainers review the API and compatibility
boundaries, run the repository's release gates, create the reviewed semantic
version tag, verify the public module and release artifacts, and then announce
the release. The tagged workflow owns the release mechanics; see
[Contributing](CONTRIBUTING.md) for the contribution-side checklist.

## Requirements

Go 1.26 or 1.27 is supported. Use the latest patch release in the selected
major line; CI tests both lines and updates them as security fixes become
available.

## Quality gates

The repository's canonical checks are available through `make`:

```text
make fmt-check
make lint
make test
make test-race
make conformance
make fuzz-smoke
make vulncheck
make bench
```

All commands listed above are present in this repository. A failed command is
a failed check; inspect its output and the generated report before treating a
change as verified.

## Reproducing benchmark evidence

`make bench` first applies the read-only oracle drift gate, then verifies every
requested implementation/case/mode against the pinned `jsonata-js v2.2.2`
oracle. It collects ten rotated benchmark rounds with allocation metrics,
generates the report under `reports/benchmark`, and runs a pinned `benchstat`.
The run manifest binds the corpus pins, source revision and dirty status,
machine, commands, rotation, and raw file hashes. Unsupported competitor cells
are reported but never timed or counted as wins. `make benchmark-claim` is the
strict claim gate and fails unless both complete comparable evaluation suites
support the scoped result. Reports and profile checks reject evidence collected
from a dirty source checkout; generated files under `reports/benchmark` are
excluded from that source-status check.

Regenerate the frozen oracle only when the matrix changes:

```text
make benchmark-oracle
git diff -- testdata/benchmark/corpus.json
```

The read-only drift gate regenerates the oracle in memory and fails without
modifying the fixture when the matrix, vendored datasets, generator, dependency
lock, or frozen output differ:

```text
make benchmark-oracle-check
```

Set `BENCH_POWER_MODE` when the host cannot detect its power source. Raw-input
measurements call each library's public raw-input API directly, so output
representation remains library-specific. Size tiers use serialized input size:
small is at most 128 bytes, medium is 129-512 bytes, and large is at least 513
bytes. The 15-case matrix retains synthetic size and adversarial workloads and
includes two source-identified inputs from the vendored pinned jsonata-js suite:
`dataset1.json` and `dataset5.json`.

`make benchmark-profile` captures CPU, allocation, mutex, and blocking profiles
for representative transform and large-array workloads and writes authenticated
profile metadata. `make benchmark-profile-check` rejects stale or modified
profile evidence. Hardware cache-counter collection is host-specific and must
be recorded separately when available.

## License

The project is licensed under the MIT License. The JSONata language and the
upstream `jsonata-js` test material retain their own copyright and license
notices; synchronized test data must preserve those notices.
