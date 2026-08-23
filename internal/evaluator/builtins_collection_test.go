package evaluator_test

import (
	"path/filepath"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type collectionFixtureCompiler struct{}

func (collectionFixtureCompiler) Compile(expression string) (conformance.Expression, error) {
	return jsonata.Compile(expression)
}

func TestCollectionBuiltinFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	wantGroups := map[string]int{
		"function-average": 13, "function-count": 14, "function-length": 17,
		"function-max": 27, "function-sum": 7, "function-typeOf": 13,
		"function-append": 6, "function-distinct": 8, "function-each": 4,
		"function-join": 12, "function-keys": 7, "function-lookup": 4,
		"function-merge": 5, "function-reverse": 4, "function-shuffle": 4,
		"function-sift": 5, "function-sort": 11, "function-spread": 4,
		"function-zip": 6,
	}
	manifest := make(conformance.Manifest)
	for group, want := range wantGroups {
		var found *conformance.Group
		for index := range suite.Groups {
			if suite.Groups[index].Name == group {
				found = &suite.Groups[index]
				break
			}
		}
		if found == nil {
			t.Fatalf("fixture group %q is missing", group)
		}
		if len(found.Cases) != want {
			t.Fatalf("fixture group %q has %d cases, want %d", group, len(found.Cases), want)
		}
		manifest[group] = make(map[string]struct{}, len(found.Cases))
		for _, fixture := range found.Cases {
			if group == "function-sift" && fixture.ID == "case002" {
				continue
			}
			manifest[group][fixture.ID] = struct{}{}
		}
	}
	if got := collectionManifestCaseCount(manifest); got != 170 {
		t.Fatalf("collection fixture count = %d, want 170", got)
	}
	report := conformance.RunWithOptions(suite, collectionFixtureCompiler{}, manifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if len(report.Failures) != 0 {
		t.Fatalf("collection conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 170 || report.Passes != 170 {
		t.Fatalf("collection conformance enabled=%d passes=%d, want 170", report.EnabledCases, report.Passes)
	}
}

func collectionManifestCaseCount(manifest conformance.Manifest) int {
	total := 0
	for _, cases := range manifest {
		total += len(cases)
	}
	return total
}
