package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestTransformScalarResidualCasesAgainstPinnedSuite(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	manifest := conformance.Manifest{
		"transform": {
			"case031": {},
			"case054": {},
			"case055": {},
			"case057": {},
			"case076": {},
			"case084": {},
		},
	}
	report := conformance.RunWithOptions(suite, realSuiteCompiler{}, manifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if report.EnabledCases != 6 || report.Passes != 6 || len(report.Failures) != 0 || len(report.Skips) != 0 {
		t.Fatalf("enabled=%d passes=%d failures=%v skips=%v", report.EnabledCases, report.Passes, report.Failures, report.Skips)
	}
}
