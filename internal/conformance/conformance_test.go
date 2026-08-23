package conformance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testCompiler struct{ evalErr error }

func TestUnmarshalExpressionPreservesUTF16SurrogateEscapes(t *testing.T) {
	tests := map[string]string{
		`"$encodeUrl('\\uD800')"`: `$encodeUrl('\uD800')`,
		`"'\\uD834\\uDD1E'"`:      `'\uD834\uDD1E'`,
		`"'\\\\uD800'"`:           `'\\uD800'`,
		`"'�'"`:                   `'�'`,
	}
	for raw, want := range tests {
		var got string
		if err := unmarshalExpression([]byte(raw), &got); err != nil {
			t.Fatalf("unmarshalExpression(%s): %v", raw, err)
		}
		if got != want {
			t.Fatalf("unmarshalExpression(%s) = %q, want %q", raw, got, want)
		}
	}
}

func (c testCompiler) Compile(string) (Expression, error) { return testExpression{err: c.evalErr}, nil }

type testExpression struct{ err error }

func (e testExpression) Eval(any) (any, error) { return nil, e.err }

type resultCompiler struct{ result any }

func (c resultCompiler) Compile(string) (Expression, error) {
	return resultExpression{result: c.result}, nil
}

type resultExpression struct{ result any }

func (e resultExpression) Eval(any) (any, error) { return e.result, nil }

type inputAwareCompiler struct{}

func (inputAwareCompiler) Compile(string) (Expression, error) { return inputAwareExpression{}, nil }

type inputAwareExpression struct{}

func (inputAwareExpression) Eval(any) (any, error) { return "explicit-input", nil }
func (inputAwareExpression) EvalNoInputBindings(map[string]any) (any, error) {
	return "absent-input", nil
}

type testCodeError string

func (e testCodeError) Error() string       { return string(e) }
func (e testCodeError) JSONataCode() string { return string(e) }

func suitePath() string {
	return filepath.Join("..", "..", "testdata", "reference", ReferenceName)
}

func TestLoadPinnedSuite(t *testing.T) {
	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	if suite.ReferenceCommit != ReferenceCommit {
		t.Fatalf("reference commit = %q", suite.ReferenceCommit)
	}
	if len(suite.Groups) < 80 {
		t.Fatalf("discovered only %d groups", len(suite.Groups))
	}
	if suite.CaseCount() != 1686 {
		t.Fatalf("discovered %d semantic cases, want 1686", suite.CaseCount())
	}
	var foundUnordered bool
	for _, group := range suite.Groups {
		for _, testCase := range group.Cases {
			if !testCase.SupportedInput {
				t.Fatalf("semantic case %s is unsupported: %s", testCase.Reference(), testCase.UnsupportedWhy)
			}
			if testCase.Group == "wildcards" && testCase.ID == "case006" {
				foundUnordered = testCase.Unordered
			}
		}
	}
	if !foundUnordered {
		t.Fatal("did not parse wildcards/case006 unordered metadata")
	}
}

func TestLoadSidecarExpressionCases(t *testing.T) {
	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		group string
		id    string
		want  string
	}{
		{group: "comments", id: "case000", want: "/* header comment */"},
		{group: "hof-reduce", id: "case009", want: "$months := ["},
		{group: "matchers", id: "case000", want: "$generateMatcher := function"},
		{group: "object-constructor", id: "case025", want: "$each(jsonata"},
	}
	for _, test := range tests {
		t.Run(test.group+"/"+test.id, func(t *testing.T) {
			caseValue := findCase(suite, test.group, test.id)
			if caseValue == nil {
				t.Fatalf("case is missing")
			}
			if filepath.Ext(caseValue.Source) != ".json" {
				t.Fatalf("source = %q, want owning JSON case", caseValue.Source)
			}
			if !caseValue.SupportedInput {
				t.Fatalf("sidecar case is unsupported: %s", caseValue.UnsupportedWhy)
			}
			if len(caseValue.Expression) == 0 || !strings.Contains(caseValue.Expression, test.want) {
				t.Fatalf("expression does not contain sidecar content %q", test.want)
			}
		})
	}
}

func findCase(suite Suite, groupName, id string) *Case {
	for groupIndex := range suite.Groups {
		if suite.Groups[groupIndex].Name != groupName {
			continue
		}
		for caseIndex := range suite.Groups[groupIndex].Cases {
			if suite.Groups[groupIndex].Cases[caseIndex].ID == id {
				return &suite.Groups[groupIndex].Cases[caseIndex]
			}
		}
	}
	return nil
}

func TestPhase3ManifestHasExactUnion(t *testing.T) {
	if got := manifestCaseCount(Phase2Manifest); got != 483 {
		t.Fatalf("Phase2Manifest case count = %d, want 483", got)
	}
	if got := manifestCaseCount(Phase3Manifest); got != 552 {
		t.Fatalf("Phase3Manifest case count = %d, want 552", got)
	}

	for group, cases := range Phase2Manifest {
		for id := range cases {
			if _, ok := Phase3Manifest[group][id]; !ok {
				t.Fatalf("Phase3Manifest omitted Phase2 case %s/%s", group, id)
			}
		}
	}
}

func TestPhase3ManifestHasExactAdditionsAndFixtures(t *testing.T) {
	additions := Manifest{
		"closures": {
			"case000": {}, "case001": {},
		},
		"blocks": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {}, "case005": {}, "case006": {},
		},
		"variables": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {}, "case005": {}, "case006": {}, "case007": {},
			"case008": {}, "case009": {}, "case010": {}, "case011": {},
			"case012": {},
		},
		"lambdas": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {}, "case005": {}, "case006": {}, "case007": {},
			"case008": {}, "case009": {}, "case010": {}, "case011": {},
			"case012": {}, "case013": {},
		},
		"function-applications": {"case020": {}},
		"partial-application": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {},
		},
		"higher-order-functions": {
			"case000": {}, "case001": {}, "case002": {},
		},
		"tail-recursion": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {}, "case005": {}, "case006": {}, "case007": {},
			"case008": {}, "case009": {},
		},
		"function-eval": {
			"case000": {}, "case001": {}, "case002": {}, "case003": {},
			"case004": {}, "case005": {},
			"case006": {}, "case007": {},
			"case008#000": {}, "case008#001": {}, "case008#002": {}, "case008#003": {},
		},
		"function-signatures": {"case026": {}, "case034": {}},
	}
	if got := manifestCaseCount(additions); got != 69 {
		t.Fatalf("Phase 3 additions case count = %d, want 69", got)
	}
	for group, cases := range additions {
		for id := range cases {
			if _, alreadyPhase2 := Phase2Manifest[group][id]; alreadyPhase2 {
				t.Fatalf("Phase 3 addition already selected in Phase 2: %s/%s", group, id)
			}
			if _, selected := Phase3Manifest[group][id]; !selected {
				t.Fatalf("Phase3Manifest omitted addition %s/%s", group, id)
			}
		}
	}

	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	for _, regexCase := range []struct{ group, id string }{
		{group: "function-sift", id: "case002"},
		{group: "function-applications", id: "case021"},
	} {
		fixture := findCase(suite, regexCase.group, regexCase.id)
		if fixture == nil || !strings.Contains(fixture.Expression, "/") {
			t.Errorf("expected regex-bearing fixture %s/%s", regexCase.group, regexCase.id)
		}
	}
	for group, cases := range additions {
		loadedGroup := findGroup(suite, group)
		if loadedGroup == nil {
			t.Fatalf("fixture group %q is missing", group)
		}
		for id := range cases {
			found := false
			for _, testCase := range loadedGroup.Cases {
				if testCase.ID == id {
					found = true
					if !testCase.SupportedInput {
						t.Fatalf("Phase 3 fixture %s/%s is not supported input", group, id)
					}
					break
				}
			}
			if !found {
				t.Fatalf("Phase 3 fixture %s/%s is missing", group, id)
			}
		}
	}
}

func TestPhase4ManifestHasExactUnionAndFixtures(t *testing.T) {
	if got := manifestCaseCount(Phase3Manifest); got != 552 {
		t.Fatalf("Phase3Manifest case count = %d, want 552", got)
	}
	if got := manifestCaseCount(Phase4Manifest); got != 1066 {
		t.Fatalf("Phase4Manifest case count = %d, want 1066", got)
	}

	expectedAdditions := map[string]int{
		"context": 4, "encoding": 4,
		"function-abs": 4, "function-append": 6, "function-assert": 8,
		"function-average": 13, "function-boolean": 24, "function-ceil": 4,
		"function-count": 14, "function-decodeUrl": 3, "function-decodeUrlComponent": 3,
		"function-distinct": 8, "function-each": 4, "function-encodeUrl": 3,
		"function-encodeUrlComponent": 3, "function-error": 11, "function-exists": 25,
		"function-floor": 4, "function-join": 12, "function-keys": 7, "function-length": 17,
		"function-lookup": 4, "function-lowercase": 2, "function-max": 27, "function-merge": 5,
		"function-number": 34, "function-pad": 13, "function-power": 7, "function-reverse": 4,
		"function-round": 18, "function-shuffle": 4, "function-sift": 4, "function-sort": 11,
		"function-spread": 4, "function-sqrt": 4, "function-string": 31,
		"function-substring": 19, "function-substringAfter": 5, "function-substringBefore": 5,
		"function-sum": 7, "function-trim": 3, "function-typeOf": 13, "function-uppercase": 2,
		"function-zip": 6, "hof-filter": 4, "hof-map": 12, "hof-reduce": 11,
		"hof-single": 11, "hof-zip-map": 4, "function-signatures": 39,
		"function-applications": 20,
	}

	additions := make(Manifest)
	for group, cases := range Phase4Manifest {
		for id := range cases {
			if _, alreadyPhase3 := Phase3Manifest[group][id]; alreadyPhase3 {
				continue
			}
			if additions[group] == nil {
				additions[group] = make(map[string]struct{})
			}
			additions[group][id] = struct{}{}
		}
	}
	if got := manifestCaseCount(additions); got != 514 {
		t.Fatalf("Phase 4 additions case count = %d, want 514", got)
	}
	for group, want := range expectedAdditions {
		if got := len(additions[group]); got != want {
			t.Errorf("Phase 4 additions %s = %d cases, want %d", group, got, want)
		}
	}
	for group := range additions {
		if _, expected := expectedAdditions[group]; !expected {
			t.Errorf("unexpected Phase 4 addition group %q", group)
		}
	}
	for group, cases := range Phase3Manifest {
		for id := range cases {
			if _, selected := Phase4Manifest[group][id]; !selected {
				t.Errorf("Phase4Manifest omitted Phase3 case %s/%s", group, id)
			}
		}
	}

	for _, excluded := range []struct{ group, id string }{
		{group: "function-sift", id: "case002"},
		{group: "function-applications", id: "case021"},
		{group: "function-signatures", id: "case026"},
		{group: "function-signatures", id: "case034"},
	} {
		if _, selected := additions[excluded.group][excluded.id]; selected {
			t.Errorf("Phase4Manifest unexpectedly selected excluded case %s/%s", excluded.group, excluded.id)
		}
	}

	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	for group, cases := range additions {
		loadedGroup := findGroup(suite, group)
		if loadedGroup == nil {
			t.Fatalf("fixture group %q is missing", group)
		}
		for id := range cases {
			fixture := findCase(suite, group, id)
			if fixture == nil {
				t.Errorf("Phase 4 fixture %s/%s is missing", group, id)
				continue
			}
			if !fixture.SupportedInput {
				t.Errorf("Phase 4 fixture %s/%s is not supported input", group, id)
			}
		}
	}
}

func TestPhase5ManifestHasExactUnionAndFixtures(t *testing.T) {
	if got := manifestCaseCount(Phase4Manifest); got != 1066 {
		t.Fatalf("Phase4Manifest case count = %d, want 1066", got)
	}
	if got := manifestCaseCount(Phase5Manifest); got != 1481 {
		t.Fatalf("Phase5Manifest case count = %d, want 1481", got)
	}

	expectedAdditions := map[string]int{
		"regex": 39, "matchers": 2,
		"function-contains": 8, "function-replace": 12, "function-split": 19,
		"function-formatBase": 9, "function-formatInteger": 65, "function-formatNumber": 45,
		"function-parseInteger": 61, "function-fromMillis": 93, "function-tomillis": 60,
		"function-sift": 1, "function-applications": 1,
	}

	additions := make(Manifest)
	for group, cases := range Phase5Manifest {
		for id := range cases {
			if _, alreadyPhase4 := Phase4Manifest[group][id]; alreadyPhase4 {
				continue
			}
			if additions[group] == nil {
				additions[group] = make(map[string]struct{})
			}
			additions[group][id] = struct{}{}
		}
	}
	if got := manifestCaseCount(additions); got != 415 {
		t.Fatalf("Phase 5 additions case count = %d, want 415", got)
	}
	for group, want := range expectedAdditions {
		if got := len(additions[group]); got != want {
			t.Errorf("Phase 5 additions %s = %d cases, want %d", group, got, want)
		}
	}
	for group := range additions {
		if _, expected := expectedAdditions[group]; !expected {
			t.Errorf("unexpected Phase 5 addition group %q", group)
		}
	}
	for group, cases := range Phase4Manifest {
		for id := range cases {
			if _, selected := Phase5Manifest[group][id]; !selected {
				t.Errorf("Phase5Manifest omitted Phase4 case %s/%s", group, id)
			}
		}
	}

	for _, excluded := range []struct{ group, id string }{
		{group: "function-sift", id: "case000"},
		{group: "function-sift", id: "case001"},
		{group: "function-sift", id: "case003"},
		{group: "function-sift", id: "case004"},
		{group: "function-applications", id: "case020"},
	} {
		if _, selected := additions[excluded.group][excluded.id]; selected {
			t.Errorf("Phase5 additions unexpectedly claimed %s/%s", excluded.group, excluded.id)
		}
	}
	for _, included := range []struct{ group, id string }{
		{group: "function-sift", id: "case002"},
		{group: "function-applications", id: "case021"},
	} {
		if _, selected := additions[included.group][included.id]; !selected {
			t.Errorf("Phase5 additions omitted %s/%s", included.group, included.id)
		}
	}

	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	for _, regexCase := range []struct{ group, id string }{
		{group: "regex", id: "case000"},
		{group: "function-sift", id: "case002"},
		{group: "function-applications", id: "case021"},
	} {
		fixture := findCase(suite, regexCase.group, regexCase.id)
		if fixture == nil || !strings.Contains(fixture.Expression, "/") {
			t.Errorf("expected regex-bearing fixture %s/%s", regexCase.group, regexCase.id)
		}
	}
	for group, cases := range additions {
		if findGroup(suite, group) == nil {
			t.Fatalf("fixture group %q is missing", group)
		}
		for id := range cases {
			fixture := findCase(suite, group, id)
			if fixture == nil {
				t.Errorf("Phase 5 fixture %s/%s is missing", group, id)
				continue
			}
			if !fixture.SupportedInput {
				t.Errorf("Phase 5 fixture %s/%s is not supported input", group, id)
			}
		}
	}

	remaining := 0
	for _, group := range suite.Groups {
		for _, fixture := range group.Cases {
			if _, selected := Phase5Manifest[fixture.Group][fixture.ID]; !selected {
				remaining++
			}
		}
	}
	if remaining != 205 {
		t.Fatalf("Phase5Manifest residual cases = %d, want 205", remaining)
	}
}

func TestFullManifestHasExactPinnedSuiteAccounting(t *testing.T) {
	components := []struct {
		name     string
		manifest Manifest
		count    int
	}{
		{name: "phase5", manifest: Phase5Manifest, count: 1481},
		{name: "parser residual", manifest: ParserResidualManifest, count: 15},
		{name: "core residual", manifest: CoreResidualManifest, count: 35},
		{name: "projection residual", manifest: ProjectionResidualManifest, count: 20},
		{name: "parent/join residual", manifest: ParentJoinResidualManifest, count: 29},
		{name: "transform", manifest: TransformManifest, count: 104},
		{name: "performance", manifest: PerformanceManifest, count: 2},
	}
	owners := make(map[string]string, 1686)
	for _, component := range components {
		if got := manifestCaseCount(component.manifest); got != component.count {
			t.Errorf("%s manifest count = %d, want %d", component.name, got, component.count)
		}
		for group, cases := range component.manifest {
			for id := range cases {
				key := group + "/" + id
				if owner, duplicate := owners[key]; duplicate {
					t.Errorf("%s is selected by both %s and %s", key, owner, component.name)
					continue
				}
				owners[key] = component.name
			}
		}
	}
	if got := len(owners); got != 1686 {
		t.Errorf("component union = %d cases, want 1686", got)
	}
	if got := manifestCaseCount(FullManifest); got != 1686 {
		t.Errorf("FullManifest count = %d, want 1686", got)
	}

	suite, err := LoadSuite(suitePath())
	if err != nil {
		t.Fatal(err)
	}
	if len(FullManifest) != len(suite.Groups) {
		t.Errorf("FullManifest groups = %d, want %d", len(FullManifest), len(suite.Groups))
	}
	for _, group := range suite.Groups {
		selected := FullManifest[group.Name]
		if len(selected) != len(group.Cases) {
			t.Errorf("FullManifest group %s = %d cases, want %d", group.Name, len(selected), len(group.Cases))
		}
		for _, fixture := range group.Cases {
			if _, ok := selected[fixture.ID]; !ok {
				t.Errorf("FullManifest omitted %s", fixture.Reference())
			}
		}
	}
	for group, cases := range FullManifest {
		for id := range cases {
			if findCase(suite, group, id) == nil {
				t.Errorf("FullManifest selected unknown case %s/%s", group, id)
			}
		}
	}
}

func manifestCaseCount(manifest Manifest) int {
	total := 0
	for _, cases := range manifest {
		total += len(cases)
	}
	return total
}

func findGroup(suite Suite, name string) *Group {
	for i := range suite.Groups {
		if suite.Groups[i].Name == name {
			return &suite.Groups[i]
		}
	}
	return nil
}

func TestLoadRejectsProvenanceAndMalformedCases(t *testing.T) {
	tests := []struct {
		name string
		edit func(string) error
	}{
		{name: "missing commit", edit: func(root string) error { return os.Remove(filepath.Join(root, "REFERENCE_COMMIT")) }},
		{name: "wrong commit", edit: func(root string) error {
			return os.WriteFile(filepath.Join(root, "REFERENCE_COMMIT"), []byte("wrong\n"), 0o644)
		}},
		{name: "missing license", edit: func(root string) error { return os.Remove(filepath.Join(root, "LICENSE")) }},
		{name: "malformed case", edit: func(root string) error {
			if err := os.MkdirAll(filepath.Join(root, "test", "test-suite", "groups", "broken"), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, "test", "test-suite", "groups", "broken", "case000.json"), []byte(`{"expr": 42}`), 0o644)
		}},
		{name: "malformed unordered metadata", edit: func(root string) error {
			return os.WriteFile(filepath.Join(root, "test", "test-suite", "groups", "sample", "case000.json"), []byte(`{"expr":"1","data":{},"result":1,"unordered":"yes"}`), 0o644)
		}},
		{name: "duplicate semantic case ID", edit: func(root string) error {
			dir := filepath.Join(root, "test", "test-suite", "groups", "sample")
			if err := os.WriteFile(filepath.Join(dir, "array.json"), []byte(`[{"expr":"1","data":{},"result":1},{"expr":"1","data":{},"result":1}]`), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "array#000.json"), []byte(`{"expr":"1","data":{},"result":1}`), 0o644)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := copyFixture(t)
			if err := test.edit(root); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadSuite(root); !errors.Is(err, ErrInvalidSuite) {
				t.Fatalf("LoadSuite error = %v, want ErrInvalidSuite", err)
			}
		})
	}
}

func TestRunUsesUnorderedTopLevelArrayMatching(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{{
			Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "1",
			HasData: true, ExpectedKind: ExpectedResult, Expected: []any{map[string]any{"id": 1.}, map[string]any{"id": 2.}},
			Unordered: true, SupportedInput: true,
		}}}},
	}
	compiler := resultCompiler{result: []any{map[string]any{"id": 2.}, map[string]any{"id": 1.}}}
	report := Run(suite, compiler, Manifest{"sample": {"case000": {}}})
	if report.Passes != 1 || len(report.Failures) != 0 {
		t.Fatalf("unordered report = %+v", report)
	}

	suite.Groups[0].Cases[0].Unordered = false
	report = Run(suite, compiler, Manifest{"sample": {"case000": {}}})
	if report.Passes != 0 || len(report.Failures) != 1 {
		t.Fatalf("ordered report = %+v", report)
	}
}

func TestRunDistinguishesAbsentInputFromExplicitNull(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{
			{
				Group: "sample", ID: "null", Expression: "$", HasData: true, Data: nil,
				ExpectedKind: ExpectedResult, Expected: "explicit-input", SupportedInput: true,
			},
			{
				Group: "sample", ID: "absent", Expression: "$", HasDataset: true, Dataset: "",
				ExpectedKind: ExpectedResult, Expected: "absent-input", SupportedInput: true,
			},
		}}},
	}
	report := Run(suite, inputAwareCompiler{}, Manifest{"sample": {"null": {}, "absent": {}}})
	if report.Passes != 2 || len(report.Failures) != 0 {
		t.Fatalf("input-presence report = %+v", report)
	}
}

func TestRunReportsEnabledEvaluationErrorsAsFailures(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups:          []Group{{Name: "sample", Cases: []Case{{Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "1", HasData: true, ExpectedKind: ExpectedResult, Expected: float64(1), SupportedInput: true}}}},
	}
	report := Run(suite, testCompiler{evalErr: fmt.Errorf("unsupported syntax")}, Manifest{"sample": {"case000": {}}})
	if report.Passes != 0 || len(report.Failures) != 1 || len(report.RemainingCases) != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestRunRequiresConfiguredUndefinedSentinel(t *testing.T) {
	undefined := errors.New("sentinel text is intentionally not inspected")
	makeSuite := func(kind ExpectedKind, expected any) Suite {
		return Suite{ReferenceCommit: ReferenceCommit, Groups: []Group{{Name: "sample", Cases: []Case{{Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "missing", HasData: true, ExpectedKind: kind, Expected: expected, SupportedInput: true}}}}}
	}
	manifest := Manifest{"sample": {"case000": {}}}

	passed := RunWithOptions(makeSuite(ExpectedUndefined, nil), testCompiler{evalErr: undefined}, manifest, Options{UndefinedError: undefined})
	if passed.Passes != 1 || len(passed.Failures) != 0 {
		t.Fatalf("configured undefined report = %+v", passed)
	}

	unexpectedValue := RunWithOptions(makeSuite(ExpectedUndefined, nil), testCompiler{}, manifest, Options{UndefinedError: undefined})
	if unexpectedValue.Passes != 0 || len(unexpectedValue.Failures) != 1 {
		t.Fatalf("nil-without-error report = %+v", unexpectedValue)
	}

	unexpectedUndefined := RunWithOptions(makeSuite(ExpectedResult, "value"), testCompiler{evalErr: undefined}, manifest, Options{UndefinedError: undefined})
	if unexpectedUndefined.Passes != 0 || len(unexpectedUndefined.Failures) != 1 {
		t.Fatalf("unexpected undefined report = %+v", unexpectedUndefined)
	}
}

func TestRunEvaluatorCallbackPropagatesMetadataAndOutcomes(t *testing.T) {
	undefined := errors.New("undefined")
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{
			{
				Group: "sample", ID: "result", Source: "groups/sample/result.json", Expression: "result",
				Data: map[string]any{"case": "result"}, HasData: true, Bindings: map[string]any{"x": 1.0},
				TimeLimit: 125, Depth: 7, ExpectedKind: ExpectedResult, Expected: "ok", SupportedInput: true,
			},
			{
				Group: "sample", ID: "error", Source: "groups/sample/error.json", Expression: "error",
				Data: map[string]any{"case": "error"}, HasData: true, Bindings: map[string]any{"x": 2.0},
				Depth: 9, ExpectedKind: ExpectedError, ExpectedCode: "T9001", SupportedInput: true,
			},
			{
				Group: "sample", ID: "undefined", Source: "groups/sample/undefined.json", Expression: "undefined",
				Data: map[string]any{"case": "undefined"}, HasData: true, Bindings: map[string]any{"x": 3.0},
				TimeLimit: 50, ExpectedKind: ExpectedUndefined, SupportedInput: true,
			},
		}}},
	}
	manifest := Manifest{"sample": {"result": {}, "error": {}, "undefined": {}}}
	var calls int
	report := RunWithOptions(suite, resultCompiler{result: "unused"}, manifest, Options{
		UndefinedError: undefined,
		Evaluate: func(expression Expression, data any, hasInput bool, bindings map[string]any, timeLimit, depth int) (any, error) {
			calls++
			if !hasInput {
				t.Error("callback unexpectedly received absent input")
			}
			if _, ok := expression.(resultExpression); !ok {
				t.Errorf("callback expression = %T, want compiled expression", expression)
			}
			input, ok := data.(map[string]any)
			if !ok {
				t.Errorf("callback data = %#v, want object", data)
				return nil, nil
			}
			switch input["case"] {
			case "result":
				if timeLimit != 125 || depth != 7 || bindings["x"] != 1.0 {
					t.Errorf("result callback args = time=%d depth=%d bindings=%v", timeLimit, depth, bindings)
				}
				return "ok", nil
			case "error":
				if timeLimit != 0 || depth != 9 || bindings["x"] != 2.0 {
					t.Errorf("error callback args = time=%d depth=%d bindings=%v", timeLimit, depth, bindings)
				}
				return nil, testCodeError("T9001")
			case "undefined":
				if timeLimit != 50 || depth != 0 || bindings["x"] != 3.0 {
					t.Errorf("undefined callback args = time=%d depth=%d bindings=%v", timeLimit, depth, bindings)
				}
				return nil, undefined
			default:
				return nil, fmt.Errorf("unexpected callback case %v", input["case"])
			}
		},
	})
	if calls != 3 {
		t.Fatalf("evaluator callback calls = %d, want 3", calls)
	}
	if report.Passes != 3 || len(report.Failures) != 0 {
		t.Fatalf("callback report = %+v", report)
	}
}

func TestRunReportsMetadataWithoutEvaluatorCallback(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{{
			Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "1",
			HasData: true, ExpectedKind: ExpectedResult, Expected: float64(1), TimeLimit: 1, SupportedInput: true,
		}}}},
	}
	report := Run(suite, resultCompiler{result: float64(1)}, Manifest{"sample": {"case000": {}}})
	if report.Passes != 0 || len(report.Failures) != 1 || !strings.Contains(report.Failures[0].Message, "no evaluator callback") {
		t.Fatalf("metadata-without-callback report = %+v", report)
	}
}

func TestRunDoesNotUseEvaluatorCallbackWithoutMetadata(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{{
			Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "1",
			HasData: true, ExpectedKind: ExpectedResult, Expected: float64(1), SupportedInput: true,
		}}}},
	}
	called := false
	report := RunWithOptions(suite, resultCompiler{result: float64(1)}, Manifest{"sample": {"case000": {}}}, Options{
		Evaluate: func(Expression, any, bool, map[string]any, int, int) (any, error) {
			called = true
			return nil, errors.New("callback must not run")
		},
	})
	if called {
		t.Fatal("evaluator callback was used without metadata")
	}
	if report.Passes != 1 || len(report.Failures) != 0 {
		t.Fatalf("zero-metadata report = %+v", report)
	}
}

func TestRunEvaluateAllUsesCallbackWithoutMetadata(t *testing.T) {
	suite := Suite{
		ReferenceCommit: ReferenceCommit,
		Groups: []Group{{Name: "sample", Cases: []Case{{
			Group: "sample", ID: "case000", Source: "groups/sample/case000.json", Expression: "1",
			HasDataset: true, ExpectedKind: ExpectedResult, Expected: float64(2), SupportedInput: true,
		}}}},
	}
	called := false
	report := RunWithOptions(suite, resultCompiler{result: float64(1)}, Manifest{"sample": {"case000": {}}}, Options{
		EvaluateAll: true,
		Evaluate: func(_ Expression, _ any, hasInput bool, _ map[string]any, _, _ int) (any, error) {
			called = true
			if hasInput {
				t.Error("EvaluateAll callback received explicit input for absent fixture")
			}
			return float64(2), nil
		},
	})
	if !called || report.Passes != 1 || len(report.Failures) != 0 {
		t.Fatalf("EvaluateAll report = %+v, called=%t", report, called)
	}
}

func copyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "REFERENCE_COMMIT"), []byte(ReferenceCommit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "LICENSE"), []byte("MIT license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "test", "test-suite", "groups", "sample")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case000.json"), []byte(`{"expr":"1","data":{},"result":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
