// Command conformance runs the pinned JSONata language-neutral suite and
// writes a deterministic machine-readable report.
package main

import (
	"flag"
	"fmt"
	"os"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type compiler struct{}

func (compiler) Compile(expression string) (conformance.Expression, error) {
	return jsonata.Compile(expression)
}

// conformanceOptions translates compatible reference-runner metadata into
// public evaluator controls. Suite depth is a harness recursion budget, not
// the JSONata 2.2 stack option, so it must not opt evaluations into D1011.
func conformanceOptions(_, _ int) jsonata.EvalOptions {
	return jsonata.EvalOptions{MaxOperations: 5_000_000}
}

func evaluateConformanceCase(expression conformance.Expression, data any, hasInput bool, bindings map[string]any, timeLimit, depth int) (any, error) {
	compiled, ok := expression.(*jsonata.Expr)
	if !ok {
		return nil, fmt.Errorf("conformance evaluator requires *jsonata.Expr, got %T", expression)
	}
	options := conformanceOptions(timeLimit, depth)
	options.Bindings = bindings
	if !hasInput {
		return compiled.EvalNoInputWithOptions(options)
	}
	return compiled.EvalWithOptions(data, options)
}

func conformanceComplete(report conformance.Report) bool {
	return len(report.Failures) == 0 &&
		len(report.Skips) == 0 &&
		len(report.RemainingCases) == 0 &&
		len(report.RemainingGroups) == 0
}

func main() {
	suitePath := flag.String("suite", "testdata/reference/jsonata-js-v2.2.2", "reference suite directory")
	reportPath := flag.String("report", "reports/conformance/report.json", "JSON report path")
	flag.Parse()

	suite, err := conformance.LoadSuite(*suitePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report := conformance.RunWithOptions(suite, compiler{}, conformance.FullManifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
		Evaluate:       evaluateConformanceCase,
		EvaluateAll:    true,
	})
	if err := conformance.WriteJSON(*reportPath, report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(conformance.Summary(report))
	for _, failure := range report.Failures {
		fmt.Fprintf(os.Stderr, "FAIL %s: %s\n", failure.Reference(), failure.Message)
	}
	if !conformanceComplete(report) {
		os.Exit(1)
	}
}
