#!/bin/sh
set -eu

repository=${JSONATA_REFERENCE_REPOSITORY:-https://github.com/jsonata-js/jsonata.git}
commit=${JSONATA_REFERENCE_COMMIT:-6c7e95fdbf4405a1e741852a7cd8cd985b4305bb}
destination=${1:-testdata/reference/jsonata-js-v2.2.2}

temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/jsonata-reference.XXXXXX")
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

git clone --filter=blob:none --no-checkout --quiet "$repository" "$temporary_directory/repository"
git -C "$temporary_directory/repository" fetch --quiet --depth=1 origin "$commit"
git -C "$temporary_directory/repository" checkout --quiet --detach "$commit"

mkdir -p "$destination"
git -C "$temporary_directory/repository" archive "$commit" -- test/test-suite LICENSE | tar -x -C "$destination"
printf '%s\n' "$commit" > "$destination/REFERENCE_COMMIT"
