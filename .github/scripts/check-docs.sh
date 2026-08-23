#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$root"

required_files='README.md CHANGELOG.md CONTRIBUTING.md SECURITY.md DEPENDENCIES.md CODE_OF_CONDUCT.md LICENSE RELEASING.md doc.go .github/CODEOWNERS .github/dependabot.yml .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE/bug_report.md .github/ISSUE_TEMPLATE/feature_request.md .github/ISSUE_TEMPLATE/config.yml'
for file in $required_files; do
	if [ ! -f "$file" ]; then
		echo "documentation file is missing: $file" >&2
		exit 1
	fi
done

grep -F '1.26' README.md >/dev/null
grep -F '1.27' README.md >/dev/null
grep -F 'jsonata-js v2.2.2' README.md >/dev/null
grep -F 'claim.met: true' README.md >/dev/null
grep -F 'v0.1.0' README.md >/dev/null
if grep -E 'go get github\.com/tiaanduplessis/jsonata-go@v1\.0\.0|release-candidate|release candidate' README.md CHANGELOG.md CONTRIBUTING.md SECURITY.md DEPENDENCIES.md; then
	echo 'public documentation contains stale pre-release wording' >&2
	exit 1
fi

if grep -Ein 'TODO|FIXME|placeholder' README.md CHANGELOG.md CONTRIBUTING.md SECURITY.md DEPENDENCIES.md RELEASING.md doc.go; then
	echo 'release documentation contains a placeholder marker' >&2
	exit 1
fi

if grep -F 'strings.Title' README.md CONTRIBUTING.md example_extensions_test.go; then
	echo 'release examples must not use deprecated strings.Title' >&2
	exit 1
fi

if grep -RE '^[[:space:]]*uses:' .github/workflows | grep -Ev '@[0-9a-f]{40}([[:space:]]*#.*)?$'; then
	echo 'workflow actions must be pinned to full commit SHAs' >&2
	exit 1
fi

module=$(go list -m -f '{{.Path}}')
go list ./... >/dev/null
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/jsonata-docs.XXXXXX")
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

check_markdown_links() {
	file=$1
	directory=$(dirname "$file")
	awk '
	{
		line = $0
		while (match(line, /\]\([^)]*\)/)) {
			destination = substr(line, RSTART + 2, RLENGTH - 3)
			print destination
			line = substr(line, RSTART + RLENGTH)
		}
	}' "$file" |
	while IFS= read -r destination; do
		case "$destination" in
			''|'#'*|http://*|https://*|mailto:*|ftp:*)
				continue
				;;
		esac
		case "$destination" in
			\<*\>)
				destination=${destination#<}
				destination=${destination%>}
				;;
		esac
		target=${destination%%\#*}
		target=${target%%\?*}
		if [ -n "$target" ] && [ ! -e "$directory/$target" ]; then
			echo "broken local Markdown link in $file: $destination" >&2
			exit 1
		fi
	done
}

for file in \
	README.md \
	CHANGELOG.md \
	CONTRIBUTING.md \
	RELEASING.md \
	SECURITY.md \
	DEPENDENCIES.md \
	CODE_OF_CONDUCT.md \
	.github/PULL_REQUEST_TEMPLATE.md \
	.github/ISSUE_TEMPLATE/bug_report.md \
	.github/ISSUE_TEMPLATE/feature_request.md \
	reports/benchmark/README.md \
	reports/benchmark/profiles/README.md \
	testdata/differential/README.md; do
	check_markdown_links "$file"
done

if ! go test . -run '^Example' -count=1; then
	echo 'root package examples failed' >&2
	exit 1
fi

go doc "$module" >"$temporary_directory/api.txt"
for symbol in Compile EvalOptions Extension Error Engine; do
	grep -E "(^|[[:space:]])${symbol}([[:space:]]|$)" "$temporary_directory/api.txt" >/dev/null || {
		echo "public API documentation does not contain $symbol" >&2
		exit 1
	}
done

for package_spec in \
	"jsonata:$module" \
	"jlib:$module/jlib" \
	"jxpath:$module/jlib/jxpath" \
	"jparse:$module/jparse" \
	"jtypes:$module/jtypes"; do
	package_name=${package_spec%%:*}
	package_path=${package_spec#*:}
	package_doc="$temporary_directory/$package_name.txt"
	if ! go doc "$package_path" >"$package_doc"; then
		echo "public package documentation failed for $package_path" >&2
		exit 1
	fi
	grep -E "^Package ${package_name}([[:space:]]|$)" "$package_doc" >/dev/null || {
		echo "public package documentation is missing for $package_path" >&2
		exit 1
	}
done

echo "documentation and public API checks passed for $module"
