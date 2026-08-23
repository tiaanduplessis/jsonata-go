package evaluator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestPhase3TailRecursionConformance(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range suite.Groups {
		if group.Name != "tail-recursion" {
			continue
		}
		for _, fixture := range group.Cases {
			fixture := fixture
			t.Run(fixture.ID, func(t *testing.T) {
				data := fixture.Data
				if fixture.HasDataset && fixture.Dataset != "" {
					path := filepath.Join(suite.Root, "test", "test-suite", "datasets", fixture.Dataset+".json")
					contents, readErr := os.ReadFile(path)
					if readErr != nil {
						t.Fatal(readErr)
					}
					if readErr = json.Unmarshal(contents, &data); readErr != nil {
						t.Fatal(readErr)
					}
				}
				n, parseErr := syntax.Parse(fixture.Expression)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				result, evalErr := evaluatorEvalWithOptions(n, data, fixture)
				if fixture.ExpectedKind == conformance.ExpectedError {
					if evalErr == nil {
						t.Fatalf("got %#v, want %s", result, fixture.ExpectedCode)
					}
					if code, ok := evalErr.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != fixture.ExpectedCode {
						t.Fatalf("error %v, want %s", evalErr, fixture.ExpectedCode)
					}
					return
				}
				if fixture.ExpectedKind == conformance.ExpectedUndefined {
					if evalErr == nil || evalErr.Error() != "undefined" {
						t.Fatalf("error %v, want undefined", evalErr)
					}
					return
				}
				if evalErr != nil {
					t.Fatal(evalErr)
				}
				if !reflect.DeepEqual(result, fixture.Expected) {
					t.Fatalf("got %#v, want %#v", result, fixture.Expected)
				}
			})
		}
	}
}

func evaluatorEvalWithOptions(n syntax.Node, data any, fixture conformance.Case) (any, error) {
	options := evaluator.Options{MaxOperations: 5_000_000}
	// Suite time and depth are host-harness controls, not public JSONata
	// timeout or stack options.
	return evaluator.EvalWithOptions(n, data, options)
}
