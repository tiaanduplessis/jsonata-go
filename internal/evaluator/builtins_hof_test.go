package evaluator_test

import (
	"path/filepath"
	"reflect"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type hofFixtureCompiler struct{}

func (hofFixtureCompiler) Compile(expression string) (conformance.Expression, error) {
	return jsonata.Compile(expression)
}

func TestHigherOrderBuiltinFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	manifest := conformance.Manifest{
		"hof-filter": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
		"hof-map": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {},
			"case006": {}, "case007": {}, "case008": {}, "case009": {}, "case0010": {}, "case0011": {},
		},
		"hof-reduce": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {},
			"case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
		},
		"hof-single": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {},
			"case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
		},
		"hof-zip-map":           {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
		"function-applications": {"case020": {}},
	}
	if got := hofManifestCaseCount(manifest); got != 43 {
		t.Fatalf("HOF fixture count = %d, want 43", got)
	}
	report := conformance.RunWithOptions(suite, hofFixtureCompiler{}, manifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if len(report.Failures) != 0 {
		t.Fatalf("HOF conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 43 || report.Passes != 43 {
		t.Fatalf("HOF conformance enabled=%d passes=%d, want 43", report.EnabledCases, report.Passes)
	}
}

func TestPipelineCallSelectorUsesPipelineInputBeforeSelection(t *testing.T) {
	for _, test := range []struct {
		expression string
		want       any
	}{
		{expression: `($data := [1]; $square := function($x){$x*$x}; $data ~> $map($square)[])`, want: []any{1.0}},
		{expression: `($data := [1, 2, 3]; $square := function($x){$x*$x}; $data ~> $map($square)[1])`, want: 4.0},
	} {
		got, err := jsonata.Eval(test.expression, nil)
		if err != nil {
			t.Fatalf("%s: %v", test.expression, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.expression, got, test.want)
		}
	}
}

func hofManifestCaseCount(manifest conformance.Manifest) int {
	total := 0
	for _, cases := range manifest {
		total += len(cases)
	}
	return total
}
