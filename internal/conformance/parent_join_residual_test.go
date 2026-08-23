package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestParentJoinResidualManifestAgainstPinnedSuite(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	report := conformance.RunWithOptions(suite, realSuiteCompiler{}, conformance.ParentJoinResidualManifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if len(report.Failures) != 0 {
		t.Fatalf("parent/join residual conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 29 || report.Passes != report.EnabledCases {
		t.Fatalf("enabled=%d passes=%d, want 29 passing cases", report.EnabledCases, report.Passes)
	}
	if len(report.Skips) != 0 {
		t.Fatalf("unexpected skips: %+v", report.Skips)
	}
}
