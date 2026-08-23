# Changelog

All notable changes are recorded here. Release entries are created from the
tagged source after the release verification workflow passes.

## Unreleased

- The recommended first public release is `v0.1.0`; no release tag is claimed
  until the API review and release gates are complete. This remains a pre-1.0
  API, so compatibility commitments are not yet the same as a v1.0 release.
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

## First public release

The maintainers recommend `v0.1.0` as the first public release. Before
publishing it, complete the API and compatibility review, run the clean
checkout release gates, create the reviewed tag, and verify the public module
proxy and release artifacts. The release workflow performs the repository's
release mechanics; this document records the public status and sequence only.
