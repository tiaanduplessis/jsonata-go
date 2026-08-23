package benchmark_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	harness "github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func benchmarkCorpus(t testing.TB) harness.Corpus {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate benchmark test file")
	}
	corpus, err := harness.LoadCorpus(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "benchmark", "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestCompetitorDependenciesArePinned(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate benchmark test file")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, requirement := range []string{
		"github.com/blues/jsonata-go v1.5.4",
		"github.com/recolabs/gnata v0.2.3",
	} {
		if !strings.Contains(string(data), requirement) {
			t.Fatalf("go.mod does not pin %s", requirement)
		}
	}
}

func TestOracleGeneratorPinsJsonataVersion(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate benchmark test file")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "benchmark", "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if got := lock.Packages["node_modules/jsonata"].Version; got != "2.2.2" {
		t.Fatalf("oracle jsonata version = %q, want 2.2.2", got)
	}
}

func TestPinnedMatrixVerification(t *testing.T) {
	corpus := benchmarkCorpus(t)
	runtimes := harness.Implementations()
	matrix := harness.VerifyMatrix(corpus, runtimes)
	if err := matrix.ValidateWorkspace(corpus); err != nil {
		t.Fatal(err)
	}
	wantRecords := len(runtimes) * totalModes(corpus)
	if len(matrix.Records) != wantRecords {
		t.Fatalf("verification records = %d, want %d", len(matrix.Records), wantRecords)
	}
	for _, unsupported := range matrix.Unsupported() {
		if unsupported.Implementation == harness.WorkspaceImplementation {
			t.Fatalf("workspace operation classified unsupported: %+v", unsupported)
		}
		if unsupported.Class == "" || unsupported.Reason == "" {
			t.Fatalf("unsupported operation lacks classification: %+v", unsupported)
		}
	}
}

func TestComparableCoverageIsExplicit(t *testing.T) {
	corpus := benchmarkCorpus(t)
	runtimes := harness.Implementations()
	matrix := harness.VerifyMatrix(corpus, runtimes)
	for _, mode := range []harness.Mode{harness.ModeDecoded, harness.ModeBytes} {
		comparable := matrix.ComparableCases(corpus, runtimes, mode)
		if len(comparable) == 0 {
			t.Fatalf("%s has no fully comparable cases", mode)
		}
		if missing := harness.MissingDimensions(corpus, comparable, mode); len(missing) != 0 {
			t.Logf("%s claim gate remains blocked by missing comparable dimensions: %v", mode, missing)
		}
	}
}

func totalModes(corpus harness.Corpus) int {
	total := 0
	for _, sample := range corpus.Cases {
		total += len(sample.Modes)
	}
	return total
}

func BenchmarkMatrix(b *testing.B) {
	corpus := benchmarkCorpus(b)
	runtimes := harness.Implementations()
	matrix := harness.VerifyMatrix(corpus, runtimes)
	selected := os.Getenv("BENCH_IMPLEMENTATION")
	for _, implementation := range runtimes {
		if selected != "" && selected != implementation.ID {
			continue
		}
		for _, sample := range corpus.Cases {
			for _, mode := range sample.Modes {
				if !matrix.Eligible(implementation.ID, sample.ID, mode) {
					continue
				}
				name := string(mode) + "/" + sample.ID
				if selected == "" {
					name = implementation.ID + "/" + name
				}
				sample := sample
				b.Run(name, func(b *testing.B) {
					switch mode {
					case harness.ModeCompile:
						harness.BenchmarkCompile(b, implementation, sample, mode)
					case harness.ModeDecoded:
						harness.BenchmarkEval(b, implementation, sample)
					case harness.ModeBytes:
						harness.BenchmarkEvalBytes(b, implementation, sample)
					case harness.ModeParallel:
						harness.BenchmarkParallel(b, implementation, sample)
					}
				})
			}
		}
	}
	if selected != "" && !knownImplementation(runtimes, selected) {
		b.Fatalf("unknown BENCH_IMPLEMENTATION %q", selected)
	}
}

func knownImplementation(runtimes []harness.Runtime, wanted string) bool {
	for _, implementation := range runtimes {
		if implementation.ID == wanted {
			return true
		}
	}
	return false
}
