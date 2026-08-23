package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type realSuiteCompiler struct{}

func (realSuiteCompiler) Compile(expression string) (conformance.Expression, error) {
	return jsonata.Compile(expression)
}

func TestPhase1ManifestAgainstPinnedSuite(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	report := conformance.RunWithOptions(suite, realSuiteCompiler{}, conformance.Phase1Manifest, conformance.Options{UndefinedError: jsonata.ErrUndefined})
	if len(report.Failures) != 0 {
		t.Fatalf("conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 30 || report.Passes != report.EnabledCases {
		t.Fatalf("enabled=%d passes=%d, want 30 passing cases", report.EnabledCases, report.Passes)
	}
	if len(report.RemainingCases) == 0 || len(report.RemainingGroups) == 0 {
		t.Fatalf("remaining suite material was not reported: %+v", report)
	}
	if len(report.Skips) != 0 {
		t.Fatalf("unexpected skips: %+v", report.Skips)
	}
}
