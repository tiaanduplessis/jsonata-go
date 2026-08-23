package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestParserCommentsAndErrorsResidualCases(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	report := conformance.RunWithOptions(suite, realSuiteCompiler{}, conformance.ParserResidualManifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if report.EnabledCases != 15 || report.Passes != 15 || len(report.Failures) != 0 || len(report.Skips) != 0 {
		t.Fatalf("enabled=%d passes=%d failures=%v skips=%v", report.EnabledCases, report.Passes, report.Failures, report.Skips)
	}
}
