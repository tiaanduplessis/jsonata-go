SHELL := /bin/sh

GO ?= go
GOVULNCHECK_VERSION ?= v1.7.0
GITLEAKS_VERSION ?= v8.30.1
BENCHSTAT_VERSION ?= v0.0.0-20251208221838-04cf7a2dca90

.PHONY: fmt-check lint test test-race conformance differential differential-oracle fuzz-smoke vulncheck secret-scan bench benchmark-verify benchmark-oracle benchmark-oracle-check benchmark-claim benchmark-profile benchmark-profile-check docs-check release-verify

fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "gofmt is required for the listed files" >&2; exit 1; }

lint:
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

conformance:
	@test -d internal/conformance || { echo "conformance implementation is not present: internal/conformance" >&2; exit 1; }
	$(GO) test ./internal/conformance/... -count=1
	$(GO) run ./cmd/conformance -report reports/conformance/report.json

differential:
	$(GO) run ./cmd/differential -check
	$(GO) test ./internal/differential ./internal/security -count=1

differential-oracle:
	@test -n "$(JSONATA_JS_CHECKOUT)" || { echo "JSONATA_JS_CHECKOUT must point to the pinned jsonata-js checkout" >&2; exit 1; }
	$(GO) run ./cmd/differential
	node testdata/differential/generate-oracle.mjs --checkout "$(JSONATA_JS_CHECKOUT)" --corpus testdata/differential/cases.json --output testdata/differential/oracle.json
	node testdata/differential/generate-oracle.mjs --checkout "$(JSONATA_JS_CHECKOUT)" --corpus testdata/differential/fuzz-cases.json --output testdata/differential/fuzz-oracle.json

fuzz-smoke:
	@test -d internal || { echo "fuzz targets are not present: internal" >&2; exit 1; }
	@set -eu; \
		packages=$$($(GO) list ./internal/...); \
	found=0; \
	for package in $$packages; do \
		listing=$$($(GO) test "$$package" -list '^Fuzz'); \
		targets=$$(printf '%s\n' "$$listing" | awk '/^Fuzz[A-Za-z0-9_]*$$/ { print }'); \
		for target in $$targets; do \
			found=1; \
			echo "fuzz smoke: $$package/$$target"; \
			$(GO) test "$$package" -run '^$$' -fuzz="^$$target$$" -fuzztime=10s; \
		done; \
	done; \
	test "$$found" -eq 1 || { echo "no fuzz targets found under internal" >&2; exit 1; }

vulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

secret-scan:
	$(GO) run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION) git --redact --log-opts=--all

bench: benchmark-oracle-check
	@test -d internal/benchmark || { echo "benchmark implementation is not present: internal/benchmark" >&2; exit 1; }
	@mkdir -p reports/benchmark/raw
	$(GO) run ./cmd/benchverify -report reports/benchmark/verification.json >/dev/null
	$(GO) run ./cmd/benchmarkrun
	$(GO) run ./cmd/benchmarkreport
	$(GO) run golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION) \
		reports/benchmark/raw/jsonata-go.txt \
		reports/benchmark/raw/blues.txt \
		reports/benchmark/raw/gnata.txt > reports/benchmark/benchstat.txt

benchmark-verify: benchmark-oracle-check
	@mkdir -p reports/benchmark
	$(GO) run ./cmd/benchverify -report reports/benchmark/verification.json

benchmark-oracle:
	cd testdata/benchmark && npm ci --ignore-scripts && npm run generate

benchmark-oracle-check:
	cd testdata/benchmark && npm ci --ignore-scripts && npm run check

benchmark-claim:
	$(GO) run ./cmd/benchmarkreport -require-claim

benchmark-profile: benchmark-oracle-check
	$(GO) run ./cmd/benchmarkprofile

benchmark-profile-check:
	$(GO) run ./cmd/benchmarkprofile -check

docs-check:
	.github/scripts/check-docs.sh

release-verify:
	.github/scripts/release-verify.sh "$(VERSION)"
