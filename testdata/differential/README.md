# Differential oracle

`cases.json` is a deterministic generated corpus. `oracle.json` records the
result from jsonata-js commit `6c7e95fdbf4405a1e741852a7cd8cd985b4305bb`
(the v2.2.2 tag). Normal tests read these committed files and do not require
Node.js or network access.

`fuzz-cases.json` is a second, bounded generated campaign. It uses seed
`0x4f4950467a11`, emits 512 syntax-valid programs with JSON inputs, and varies
both the program literal and document values. `fuzz-oracle.json` is its
structured result from the same pinned checkout. The Go tests replay both
fixtures offline; Node.js is needed only when regenerating the oracle.

`shape-cases.json` is an enumerated grid rather than a random campaign. It
crosses value shapes (missing, null, scalars, empty, single-item, and nested
arrays, and objects) with path step constructs (fields, predicates, blocks,
wildcards, keep-array, sort, focus and position bindings, and the parent
operator) and with the contexts that consume a path result. It also passes the
arguments of every deterministic standard function and operator through each of
those path forms, and as an undefined value. Each input gives the root the
field names that a path selects below the root, so a step that falls back to
the root fails. `shape-oracle.json` is its result from the same pinned
checkout, and the Go test replays every case through both `EvalBytes` and
`Eval`.

Error comparisons use the language-neutral suite contract: JSONata error code
and token. Human-readable messages and implementation-specific stack or source
positions are not compatibility fields.

Regenerate the evidence only from an audited checkout of that exact commit:

```text
git -C /path/to/jsonata-js checkout 6c7e95fdbf4405a1e741852a7cd8cd985b4305bb
go run ./cmd/differential -output testdata/differential/cases.json
node testdata/differential/generate-oracle.mjs \
  --checkout /path/to/jsonata-js \
  --corpus testdata/differential/cases.json \
  --output testdata/differential/oracle.json
go run ./cmd/differential -fuzz-output testdata/differential/fuzz-cases.json
node testdata/differential/generate-oracle.mjs \
  --checkout /path/to/jsonata-js \
  --corpus testdata/differential/fuzz-cases.json \
  --output testdata/differential/fuzz-oracle.json
go run ./cmd/differential -shape-output testdata/differential/shape-cases.json
node testdata/differential/generate-oracle.mjs \
  --checkout /path/to/jsonata-js \
  --corpus testdata/differential/shape-cases.json \
  --output testdata/differential/shape-oracle.json
go test ./internal/differential -count=1
```

The same regeneration is available as an explicit, network-capable step:

```text
make differential-oracle JSONATA_JS_CHECKOUT=/path/to/jsonata-js
```

The checkout must be at the pinned commit. This target is not part of the
offline `make differential` gate.

The generator refuses a checkout whose `git rev-parse HEAD` differs from the
pinned commit.
