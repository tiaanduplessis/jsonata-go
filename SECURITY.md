# Security policy

## Supported versions

The project has no public release tag yet. After the recommended first
`v0.1.0` release, only the latest released minor version will receive security
fixes. Development branches may receive fixes before release but are not
security-support commitments. Security fixes are tested with the latest patch
release in both supported Go lines.

The supported Go major releases are 1.26 and 1.27. Keep Go patched to the
latest available patch release for the selected major version.

Unicode behavior can vary with the supported toolchain and pinned text tables:
Go 1.26 uses Unicode 15.0.0 while Go 1.27 and the Node.js 24 comparison oracle
use Unicode 17.0.0. Treat this as a documented compatibility boundary, not a
security guarantee.

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability. Report it
privately through the repository host's security advisory mechanism:
https://github.com/tiaanduplessis/jsonata-go/security/advisories/new

If that mechanism is unavailable, contact the maintainers through a private
channel configured in the repository metadata and include "jsonata-go
security" in the subject.

Include the affected version or commit, a minimal reproduction, impact, and
any suggested mitigation. Do not include secrets or personal data in the
report.

We will acknowledge a report within seven calendar days, provide an impact
assessment when practical, and coordinate disclosure after a fix or mitigation
is available. Timelines may change when upstream coordination is required.

## Release security checks

Tagged releases run `govulncheck`, `go mod verify`, module-proxy resolution,
and API documentation checks on a clean checkout. Release archives have a
keyless Sigstore signature for their checksum file and GitHub build-provenance
attestations when the repository permissions allow them. These checks do not
replace application-level threat modeling or safe handling of untrusted input.
