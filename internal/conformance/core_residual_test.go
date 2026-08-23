package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestCoreResidualManifestAgainstPinnedSuite(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	report := conformance.RunWithOptions(suite, realSuiteCompiler{}, conformance.CoreResidualManifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
		Evaluate: func(expression conformance.Expression, data any, hasInput bool, bindings map[string]any, timeLimit, depth int) (any, error) {
			compiled, ok := expression.(*jsonata.Expr)
			if !ok {
				t.Fatalf("unexpected expression type %T", expression)
			}
			options := jsonata.EvalOptions{Bindings: bindings, MaxOperations: 5_000_000}
			// Suite time and depth are host-harness controls, not public JSONata
			// timeout or stack options.
			if !hasInput {
				return compiled.EvalNoInputWithOptions(options)
			}
			return compiled.EvalWithOptions(data, options)
		},
	})
	if len(report.Failures) != 0 {
		t.Fatalf("core residual conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 35 || report.Passes != 35 {
		t.Fatalf("enabled=%d passes=%d, want 35 passing cases", report.EnabledCases, report.Passes)
	}
	if len(report.Skips) != 0 {
		t.Fatalf("unexpected skips: %+v", report.Skips)
	}
}
