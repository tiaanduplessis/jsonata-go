# Changelog

All notable changes are recorded here. Release entries are created from the
tagged source after the release verification workflow passes.

## v0.2.0 - 2026-11-01

- Fixed values that a path of two or more steps selects. They no longer carry
  evaluator context into functions, so `$string(a.b)` no longer adds JSON
  quotes, and `$trim`, `$error`, the URL and base64 functions no longer panic.
  Extension functions receive the same plain values.
- Fixed a missing field that resolved to a field of the same name at the
  document root. `a.x` is now undefined when `a` has no `x`.
- Fixed `undefined != value`, which returned true. Any comparison with an
  undefined operand is now false, as in jsonata-js.
- Fixed object constructors that used the text of an unquoted key when the key
  expression was undefined. Such a pair is now left out.
- The last step of a path now keeps an empty or single-item array, and a path
  that binds a focus or position variable, or that uses `%`, flattens arrays as
  a tuple stream does. A step or a predicate after a focus binding uses the
  context of the focus step, and a path that ends with a focus binding gives
  that context, as in jsonata-js. Removed the ancestor lookup that this
  replaces, which could return a field of an ancestor of the same name.
- The parent operator now follows the jsonata-js ancestry rules, so
  `a.b.%.c` and `$f(a.b.%.c)` are accepted and an unresolvable `%` is S0217.
- Aligned `$each`, `$match`, `$distinct`, `$append`, `$substring`,
  `$substringBefore`, `$lookup`, sort, and wildcard results with jsonata-js
  for single-item, empty, and undefined values.
- A binding in a block no longer hides an outer variable or a standard
  function before the binding runs, so `$now := $now()` works.
- Removed the `golang.org/x/vuln` tool requirement from the module, so a
  consumer requires only `golang.org/x/text` and `github.com/dlclark/regexp2`.
- Added the shape differential campaign, which crosses value shapes, path
  constructs, and function arguments against jsonata-js v2.2.2.

## v0.1.1 - 2026-09-12

- Corrected public-release and security-support documentation and moved
  installation instructions into the quickstart.
- Updated all CodeQL actions to 4.37.9 and grouped future updates.
- Added a private email contact for security and conduct reports.

## [v0.1.0](https://github.com/tiaanduplessis/jsonata-go/releases/tag/v0.1.0) - 2026-08-24

- First public release for evaluation. The API remains pre-1.0.
- Documented installation, JSONata 2.2.2 compatibility, context-aware
  evaluation, custom functions, variables, structured errors, resource
  guardrails, input ownership, and concurrent use.
- Documented the scoped benchmark method and the rule that no performance
  claim is made without complete correctness-gated evidence.
- Added dependency, license, and upstream-fixture provenance policy.
- Added release verification, module-proxy validation, checksum signing,
  GitHub provenance attestations, generated API documentation, and a reviewed
  upstream-watch workflow.
- Updated continuous integration and support policy to Go 1.26 and 1.27.

## Release policy

The module follows semantic versioning. A patch release contains compatible
bug or security fixes; a minor release may add compatible API or language
support; a major release may change the public API or compatibility target.
The JSONata compatibility target is changed only by an explicit reviewed
update to the pinned reference, fixtures, and conformance evidence.
