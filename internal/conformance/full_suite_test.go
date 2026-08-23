package conformance_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func fullSuiteOptions(t *testing.T) conformance.Options {
	t.Helper()
	return conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
		EvaluateAll:    true,
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
	}
}

func loadPinnedSuite(t *testing.T) conformance.Suite {
	t.Helper()
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func assertExactGate(t *testing.T, report conformance.Report, want int) {
	t.Helper()
	if report.EnabledCases != want || report.Passes != want || len(report.Failures) != 0 || len(report.Skips) != 0 {
		t.Fatalf("enabled=%d passes=%d failures=%v skips=%v, want %d passing", report.EnabledCases, report.Passes, report.Failures, report.Skips, want)
	}
}

func TestCompleteTransformManifestAgainstPinnedSuite(t *testing.T) {
	report := conformance.RunWithOptions(loadPinnedSuite(t), realSuiteCompiler{}, conformance.TransformManifest, fullSuiteOptions(t))
	assertExactGate(t, report, 104)
}

func TestPerformanceManifestAgainstPinnedSuite(t *testing.T) {
	report := conformance.RunWithOptions(loadPinnedSuite(t), realSuiteCompiler{}, conformance.PerformanceManifest, fullSuiteOptions(t))
	assertExactGate(t, report, 2)
}

func TestFullManifestAgainstPinnedSuite(t *testing.T) {
	report := conformance.RunWithOptions(loadPinnedSuite(t), realSuiteCompiler{}, conformance.FullManifest, fullSuiteOptions(t))
	assertExactGate(t, report, 1686)
	if len(report.RemainingCases) != 0 || len(report.RemainingGroups) != 0 {
		t.Fatalf("full manifest left remaining groups=%v cases=%v", report.RemainingGroups, report.RemainingCases)
	}
}
