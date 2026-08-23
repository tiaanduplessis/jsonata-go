#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

version=${1:-${GITHUB_REF_NAME:-}}
out=${2:-dist}
if ! printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "release version must be a semantic version tag: $version" >&2
	exit 2
fi

module=$(go list -m -f '{{.Path}}')
name=${module##*/}
mkdir -p "$out"
rm -f "$out/$name-$version.tar.gz" "$out/api-docs.txt" "$out/checksums.txt" \
	"$out/conformance-report.json" "$out/benchmark-report.json" \
	"$out/benchmark-report.md" "$out/benchmark-benchstat.txt"

git archive --format=tar --prefix="$name-$version/" "$version" | gzip -n >"$out/$name-$version.tar.gz"

go list ./... | while IFS= read -r package; do
	case "$package" in
		*/internal/*) continue ;;
	esac
	printf '\n### %s\n\n' "$package" >>"$out/api-docs.txt"
	go doc "$package" >>"$out/api-docs.txt"
done

for evidence in \
	"reports/conformance/report.json:$out/conformance-report.json" \
	"reports/benchmark/report.json:$out/benchmark-report.json" \
	"reports/benchmark/README.md:$out/benchmark-report.md" \
	"reports/benchmark/benchstat.txt:$out/benchmark-benchstat.txt"; do
	source=${evidence%%:*}
	destination=${evidence#*:}
	test -f "$source" || { echo "release evidence is missing: $source" >&2; exit 1; }
	cp "$source" "$destination"
done

if command -v sha256sum >/dev/null 2>&1; then
	(cd "$out" && sha256sum ./*.tar.gz api-docs.txt conformance-report.json benchmark-report.json benchmark-report.md benchmark-benchstat.txt) >"$out/checksums.txt"
else
	(cd "$out" && shasum -a 256 ./*.tar.gz api-docs.txt conformance-report.json benchmark-report.json benchmark-report.md benchmark-benchstat.txt) >"$out/checksums.txt"
fi

echo "created release artifacts in $out"
