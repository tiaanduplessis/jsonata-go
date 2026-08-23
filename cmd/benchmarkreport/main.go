// Command benchmarkreport validates raw benchmark evidence and renders the
// scoped statistical claim report.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

type reportDocument struct {
	GeneratedAt string                       `json:"generated_at"`
	EvidenceAt  string                       `json:"evidence_at"`
	Repository  benchmark.RepositoryMetadata `json:"repository"`
	Analysis    benchmark.Analysis           `json:"analysis"`
}

func main() {
	corpusPath := flag.String("corpus", "testdata/benchmark/corpus.json", "frozen benchmark corpus path")
	matrixPath := flag.String("matrix", "testdata/benchmark/matrix.json", "benchmark source matrix path")
	generatorPath := flag.String("generator", "testdata/benchmark/generate-oracle.mjs", "oracle generator path")
	packageLockPath := flag.String("package-lock", "testdata/benchmark/package-lock.json", "oracle dependency lock path")
	referenceRoot := flag.String("reference-root", "testdata/reference/jsonata-js-v2.2.2", "vendored jsonata-js reference root")
	rawDir := flag.String("raw", "reports/benchmark/raw", "raw benchmark output directory")
	jsonPath := flag.String("json", "reports/benchmark/report.json", "JSON report path")
	markdownPath := flag.String("markdown", "reports/benchmark/README.md", "Markdown report path")
	requireClaim := flag.Bool("require-claim", false, "fail after writing reports unless the performance claim gate passes")
	check := flag.Bool("check", false, "validate committed evidence without writing reports")
	flag.Parse()

	if err := benchmark.ValidateOracleArtifacts(*matrixPath, *corpusPath, *generatorPath, *packageLockPath, *referenceRoot); err != nil {
		fail(err)
	}
	corpus, err := benchmark.LoadCorpus(*corpusPath)
	if err != nil {
		fail(err)
	}
	runtimes := benchmark.Implementations()
	matrix := benchmark.VerifyMatrix(corpus, runtimes)
	if err := matrix.ValidateWorkspace(corpus); err != nil {
		fail(err)
	}
	evidence, err := loadRunEvidence(filepath.Join(*rawDir, "run.json"))
	if err != nil {
		fail(err)
	}
	currentMachine := benchmark.CurrentMachineMetadata()
	currentRepository := benchmark.CurrentRepositoryMetadataExcluding("reports/benchmark", *rawDir, *jsonPath, *markdownPath)
	if *check {
		if err := checkCommittedEvidence(*jsonPath, *markdownPath, *rawDir, corpus, runtimes, matrix, evidence); err != nil {
			fail(err)
		}
		_, _ = fmt.Fprintln(os.Stdout, "committed benchmark evidence is internally consistent")
		return
	}
	if err := benchmark.ValidateRunManifest(evidence, corpus, *rawDir, currentMachine, currentRepository); err != nil {
		fail(err)
	}
	analysis, err := analyzeEvidence(*rawDir, corpus, runtimes, matrix, evidence)
	if err != nil {
		fail(err)
	}
	analysis.Machine = evidence.Machine
	document := reportDocument{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		EvidenceAt:  evidence.RecordedAt,
		Repository:  evidence.Repository,
		Analysis:    analysis,
	}
	if err := writeJSON(*jsonPath, document); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*markdownPath, renderMarkdown(document), 0o644); err != nil {
		fail(err)
	}
	if analysis.Claim.Met {
		_, _ = fmt.Fprintln(os.Stdout, "performance claim gate: met")
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "performance claim gate: not met: %s\n", strings.Join(analysis.Claim.Reasons, "; "))
		if *requireClaim {
			os.Exit(1)
		}
	}
}

func checkCommittedEvidence(jsonPath, markdownPath, rawDir string, corpus benchmark.Corpus, runtimes []benchmark.Runtime, matrix benchmark.VerificationMatrix, evidence benchmark.RunManifest) error {
	if err := benchmark.ValidateRunManifest(evidence, corpus, rawDir, evidence.Machine, evidence.Repository); err != nil {
		return fmt.Errorf("validate committed benchmark manifest: %w", err)
	}
	if evidence.Repository.Revision == "unknown" || evidence.Repository.Revision == "" || evidence.Repository.Dirty {
		return fmt.Errorf("committed benchmark evidence must identify a clean source revision")
	}
	report, err := loadReport(jsonPath)
	if err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, report.GeneratedAt); err != nil {
		return fmt.Errorf("benchmark report has an invalid generated_at timestamp: %w", err)
	}
	if report.EvidenceAt != evidence.RecordedAt {
		return fmt.Errorf("benchmark report evidence_at differs from raw run manifest")
	}
	if report.Repository != evidence.Repository {
		return fmt.Errorf("benchmark report repository metadata differs from raw run manifest")
	}
	if report.Analysis.Machine != evidence.Machine {
		return fmt.Errorf("benchmark report machine metadata differs from raw run manifest")
	}
	analysis, err := analyzeEvidence(rawDir, corpus, runtimes, matrix, evidence)
	if err != nil {
		return err
	}
	reportedAnalysis, err := json.Marshal(report.Analysis)
	if err != nil {
		return fmt.Errorf("encode benchmark report analysis: %w", err)
	}
	computedAnalysis, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("encode computed benchmark analysis: %w", err)
	}
	if string(reportedAnalysis) != string(computedAnalysis) {
		return fmt.Errorf("benchmark report analysis differs from the authenticated raw evidence")
	}
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("read benchmark Markdown report: %w", err)
	}
	if string(markdown) != string(renderMarkdown(report)) {
		return fmt.Errorf("benchmark Markdown report differs from the authenticated JSON report")
	}
	return nil
}

func analyzeEvidence(rawDir string, corpus benchmark.Corpus, runtimes []benchmark.Runtime, matrix benchmark.VerificationMatrix, evidence benchmark.RunManifest) (benchmark.Analysis, error) {
	raw := make(map[string][]benchmark.Measurement, len(runtimes))
	for _, runtime := range runtimes {
		path := filepath.Join(rawDir, runtime.ID+".txt")
		file, err := os.Open(path)
		if err != nil {
			return benchmark.Analysis{}, fmt.Errorf("open %s raw output: %w", runtime.ID, err)
		}
		measurements, parseErr := benchmark.ParseBenchmarkOutput(file)
		closeErr := file.Close()
		if parseErr != nil {
			return benchmark.Analysis{}, parseErr
		}
		if closeErr != nil {
			return benchmark.Analysis{}, closeErr
		}
		raw[runtime.ID] = measurements
	}
	analysis, err := benchmark.AnalyzeMeasurements(corpus, runtimes, matrix, raw)
	if err != nil {
		return benchmark.Analysis{}, err
	}
	analysis.Machine = evidence.Machine
	return analysis, nil
}

func loadReport(path string) (reportDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reportDocument{}, fmt.Errorf("read benchmark report: %w", err)
	}
	var report reportDocument
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return reportDocument{}, fmt.Errorf("decode benchmark report: %w", err)
	}
	return report, nil
}

func loadRunEvidence(path string) (benchmark.RunManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return benchmark.RunManifest{}, fmt.Errorf("read run evidence: %w", err)
	}
	var evidence benchmark.RunManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return benchmark.RunManifest{}, fmt.Errorf("decode run evidence: %w", err)
	}
	return evidence, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func renderMarkdown(document reportDocument) []byte {
	analysis := document.Analysis
	var out bytes.Buffer
	_, _ = fmt.Fprintln(&out, "# JSONata Go benchmark report")
	_, _ = fmt.Fprintln(&out)
	if analysis.Claim.Met {
		_, _ = fmt.Fprintln(&out, "The scoped performance claim gate passed on this host.")
	} else {
		_, _ = fmt.Fprintln(&out, "The scoped performance claim gate did not pass. No fastest-library claim is supported by this run.")
	}
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintf(&out, "Evidence recorded: `%s`. Report generated: `%s`.\n\n", document.EvidenceAt, document.GeneratedAt)
	_, _ = fmt.Fprintln(&out, "## Environment")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "| Field | Value |")
	_, _ = fmt.Fprintln(&out, "|---|---|")
	_, _ = fmt.Fprintf(&out, "| Go | `%s` |\n", analysis.Machine.GoVersion)
	_, _ = fmt.Fprintf(&out, "| OS/architecture | `%s/%s` |\n", analysis.Machine.GOOS, analysis.Machine.GOARCH)
	_, _ = fmt.Fprintf(&out, "| CPU | %s |\n", analysis.Machine.CPUModel)
	_, _ = fmt.Fprintf(&out, "| Logical CPUs / GOMAXPROCS | %d / %d |\n", analysis.Machine.CPUCount, analysis.Machine.GOMAXPROCS)
	_, _ = fmt.Fprintf(&out, "| Power | %s |\n", analysis.Machine.PowerMode)
	_, _ = fmt.Fprintf(&out, "| Source revision / dirty (benchmark artifacts excluded) | `%s` / %t |\n", document.Repository.Revision, document.Repository.Dirty)
	_, _ = fmt.Fprintf(&out, "| Repetitions / benchtime / warm-ups | %d / `%s` / %d |\n", analysis.Run.Count, analysis.Run.Benchtime, analysis.Run.Warmup)
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "## Pinned implementations")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "| ID | Module | Version | Commit |")
	_, _ = fmt.Fprintln(&out, "|---|---|---|---|")
	for _, implementation := range analysis.Implementations {
		_, _ = fmt.Fprintf(&out, "| %s | `%s` | `%s` | `%s` |\n", implementation.ID, implementation.Module, implementation.Version, implementation.Commit)
	}
	_, _ = fmt.Fprintf(&out, "\nOracle: `jsonata-js %s` at `%s`.\n", analysis.Reference.Version, analysis.Reference.Commit)
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "## Comparable results")
	for _, suite := range analysis.Suites {
		_, _ = fmt.Fprintf(&out, "\n### %s\n\n", suite.Mode)
		_, _ = fmt.Fprintf(&out, "Comparable cases: %d. Complete required coverage: %t.\n\n", len(suite.ComparableCases), suite.Complete)
		if len(suite.MissingDimensions) != 0 {
			_, _ = fmt.Fprintf(&out, "Missing dimensions: `%s`.\n\n", strings.Join(suite.MissingDimensions, "`, `"))
		}
		_, _ = fmt.Fprintln(&out, "| Implementation | Geometric mean ns/op | Mean B/op | Mean allocs/op |")
		_, _ = fmt.Fprintln(&out, "|---|---:|---:|---:|")
		for _, metrics := range suite.Metrics {
			_, _ = fmt.Fprintf(&out, "| %s | %.2f | %.2f | %.2f |\n", metrics.Implementation, metrics.GeometricMeanNS, metrics.ArithmeticMeanBytes, metrics.ArithmeticMeanAllocs)
		}
		if len(suite.Comparisons) != 0 {
			_, _ = fmt.Fprintln(&out, "\n| Competitor | Workspace ratio | 95% interval | Statistically faster |")
			_, _ = fmt.Fprintln(&out, "|---|---:|---:|---|")
			for _, comparison := range suite.Comparisons {
				_, _ = fmt.Fprintf(&out, "| %s | %.3f | %.3f-%.3f | %t |\n", comparison.Competitor, comparison.Ratio, comparison.Lower95, comparison.Upper95, comparison.StatisticallyFaster)
			}
		}
	}
	_, _ = fmt.Fprintln(&out, "\n## Parallel throughput")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "| Implementation | Cases | Serial geometric mean ns/op | Parallel geometric mean ns/op | Throughput scale |")
	_, _ = fmt.Fprintln(&out, "|---|---:|---:|---:|---:|")
	for _, parallel := range analysis.Parallel {
		_, _ = fmt.Fprintf(&out, "| %s | %d | %.2f | %.2f | %.2fx |\n", parallel.Implementation, parallel.Cases, parallel.SerialGeoMean, parallel.ParallelGeoMean, parallel.ThroughputScale)
	}
	_, _ = fmt.Fprintln(&out, "\n## Unsupported cells")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "Unsupported cells were not timed and were not counted as wins.")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "| Implementation | Case | Mode | Class | Reason |")
	_, _ = fmt.Fprintln(&out, "|---|---|---|---|---|")
	for _, unsupported := range analysis.Unsupported {
		_, _ = fmt.Fprintf(&out, "| %s | %s | %s | %s | %s |\n", unsupported.Implementation, unsupported.CaseID, unsupported.Mode, unsupported.Class, unsupported.Reason)
	}
	_, _ = fmt.Fprintln(&out, "\n## Claim gate")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintf(&out, "Scope: %s.\n\n", analysis.Claim.Scope)
	_, _ = fmt.Fprintf(&out, "Met: **%t**.\n", analysis.Claim.Met)
	for _, reason := range analysis.Claim.Reasons {
		_, _ = fmt.Fprintf(&out, "\n- %s\n", reason)
	}
	_, _ = fmt.Fprintln(&out, "\n## Method")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "Each implementation/case/mode passed the pinned oracle before it was registered as a benchmark. Collection rotates implementation order each round. Latencies use geometric means over log-transformed repeated estimates. Ratio intervals use a two-sided 95% normal interval over independent log measurements; a claim requires the upper workspace/competitor bound to be below 1 for every competitor in both decoded and raw-input suites. Allocation columns are arithmetic means. Raw-input measurements call each library's public raw-input API directly; return representation is library-specific. Size tiers use serialized input size: small is at most 128 bytes, medium is 129-512 bytes, and large is at least 513 bytes. Unsupported cells are listed above and excluded from timing and claims.")
	_, _ = fmt.Fprintln(&out)
	_, _ = fmt.Fprintln(&out, "Raw output is in [`raw/`](raw/). Pairwise `benchstat` output is in [`benchstat.txt`](benchstat.txt).")
	_, _ = fmt.Fprintln(&out, "CPU, allocation, mutex, and blocking evidence is in [`profiles/`](profiles/). Hardware cache counters were unavailable on this host, so no cache claim is made.")
	return out.Bytes()
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
