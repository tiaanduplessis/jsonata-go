#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

ref=${1:-${RELEASE_REF:-HEAD}}
resolved_ref=$(git rev-parse --verify "$ref^{commit}")
head=$(git rev-parse --verify HEAD)
if [ "$resolved_ref" != "$head" ]; then
	echo "checked out commit $head does not match requested release ref $ref ($resolved_ref)" >&2
	exit 1
fi

go run ./cmd/benchmarkreport -check

evidence_revision=$(jq -r '.repository.revision' reports/benchmark/report.json)
case "$evidence_revision" in
	''|unknown|*[!0-9a-f]*)
		echo 'benchmark evidence does not identify a full Git revision' >&2
		exit 1
		;;
esac
if [ "${#evidence_revision}" -ne 40 ]; then
	echo "benchmark evidence revision is not a full Git SHA: $evidence_revision" >&2
	exit 1
fi
git cat-file -e "$evidence_revision^{commit}"
if ! git merge-base --is-ancestor "$evidence_revision" "$resolved_ref"; then
	echo "benchmark evidence revision $evidence_revision is not an ancestor of release ref $ref" >&2
	exit 1
fi
changed_source=$(git diff --name-only "$evidence_revision" "$resolved_ref" -- . ':(exclude)reports/benchmark/**')
if [ -n "$changed_source" ]; then
	echo 'source changed after the benchmark evidence revision; regenerate and review benchmark evidence before release:' >&2
	printf '%s\n' "$changed_source" >&2
	exit 1
fi

echo "release benchmark evidence is valid for $resolved_ref"
