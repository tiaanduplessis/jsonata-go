package benchmark_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	harness "github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func TestFrozenOracleArtifactsAreCurrent(t *testing.T) {
	root := repositoryRoot(t)
	if err := validateOracleArtifacts(root, filepath.Join(root, "testdata", "benchmark", "matrix.json")); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenOracleArtifactsRejectMatrixDrift(t *testing.T) {
	root := repositoryRoot(t)
	matrixPath := filepath.Join(root, "testdata", "benchmark", "matrix.json")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(t.TempDir(), "matrix.json")
	if err := os.WriteFile(stalePath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateOracleArtifacts(root, stalePath); err == nil || !strings.Contains(err.Error(), "differs from the frozen generation pin") {
		t.Fatalf("expected matrix drift rejection, got %v", err)
	}
}

func TestCorpusIncludesDistinctPinnedOfficialDatasets(t *testing.T) {
	corpus := benchmarkCorpus(t)
	sources := make(map[string]struct{})
	for _, sample := range corpus.Cases {
		if sample.Source == nil {
			continue
		}
		if sample.Source.Commit != harness.ReferenceCommit || sample.Source.Kind != "jsonata-js-dataset" {
			t.Fatalf("case %q has invalid source: %+v", sample.ID, sample.Source)
		}
		sources[sample.Source.Path] = struct{}{}
	}
	if len(sources) < 2 {
		t.Fatalf("official source datasets = %d, want at least 2", len(sources))
	}
}

func validateOracleArtifacts(root, matrixPath string) error {
	benchmarkRoot := filepath.Join(root, "testdata", "benchmark")
	return harness.ValidateOracleArtifacts(
		matrixPath,
		filepath.Join(benchmarkRoot, "corpus.json"),
		filepath.Join(benchmarkRoot, "generate-oracle.mjs"),
		filepath.Join(benchmarkRoot, "package-lock.json"),
		filepath.Join(root, "testdata", "reference", "jsonata-js-v2.2.2"),
	)
}

func repositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate benchmark test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
