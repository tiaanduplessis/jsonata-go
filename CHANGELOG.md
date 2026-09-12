# Changelog

All notable changes are recorded here. Release entries are created from the
tagged source after the release verification workflow passes.

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
