#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

version=${1:-${GITHUB_REF_NAME:-}}
if [ -z "$version" ]; then
	echo 'usage: make release-verify VERSION=vX.Y.Z' >&2
	exit 2
fi
if ! printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "release version must be a semantic version tag: $version" >&2
	exit 2
fi

if [ "${GITHUB_REF_TYPE:-}" = tag ] && [ "${GITHUB_REF_NAME:-}" != "$version" ]; then
	echo "workflow tag and requested release version differ" >&2
	exit 1
fi

if [ -n "${RELEASE_REF:-}" ]; then
	requested_commit=$(git rev-parse --verify "${RELEASE_REF}^{commit}")
	checked_out_commit=$(git rev-parse --verify HEAD)
	if [ "$requested_commit" != "$checked_out_commit" ]; then
		echo "checked out commit does not match requested release ref ${RELEASE_REF}" >&2
		exit 1
	fi
fi

module=$(go list -m -f '{{.Path}}')
case "$module" in
	github.com/tiaanduplessis/jsonata-go) ;;
	*) echo "unexpected module path: $module" >&2; exit 1 ;;
esac

echo "verifying $module $version"
make fmt-check
make lint
make test
make test-race
make conformance
make differential
make fuzz-smoke
make vulncheck
make docs-check
make benchmark-oracle-check
.github/scripts/release-evidence-verify.sh "${RELEASE_REF:-HEAD}"
go mod verify
go list -m all >/dev/null

echo "release verification passed for $module $version"
