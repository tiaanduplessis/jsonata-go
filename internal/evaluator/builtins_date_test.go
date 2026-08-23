package evaluator

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

type dateFixtureCompiler struct{}

type dateFixtureExpression struct {
	node syntax.Node
}

func (dateFixtureCompiler) Compile(expression string) (conformance.Expression, error) {
	node, err := syntax.Parse(expression)
	if err != nil {
		return nil, err
	}
	return dateFixtureExpression{node: node}, nil
}

func (expression dateFixtureExpression) Eval(data any) (any, error) {
	return EvalBindings(expression.node, data, dateBuiltinBindings())
}

func TestDateBuiltinFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	manifest := make(conformance.Manifest, 2)
	for _, group := range suite.Groups {
		if group.Name != "function-fromMillis" && group.Name != "function-tomillis" {
			continue
		}
		manifest[group.Name] = make(map[string]struct{}, len(group.Cases))
		for _, fixture := range group.Cases {
			manifest[group.Name][fixture.ID] = struct{}{}
		}
	}
	report := conformance.RunWithOptions(suite, dateFixtureCompiler{}, manifest, conformance.Options{
		UndefinedError: errUndefined,
		Evaluate: func(expression conformance.Expression, data any, hasInput bool, bindings map[string]any, _ int, depth int) (any, error) {
			compiled := expression.(dateFixtureExpression)
			allBindings := dateBuiltinBindings()
			for name, binding := range bindings {
				allBindings[name] = binding
			}
			options := Options{MaxCallDepth: max(1, depth/3)}
			if !hasInput {
				return EvalNoInputBindingsWithOptions(compiled.node, allBindings, options)
			}
			return EvalBindingsWithOptions(compiled.node, data, allBindings, options)
		},
	})
	if report.EnabledCases != 153 {
		t.Fatalf("date/time fixture count = %d, want 153", report.EnabledCases)
	}
	if len(report.Failures) != 0 {
		failures := make([]string, len(report.Failures))
		for index, failure := range report.Failures {
			failures[index] = fmt.Sprintf("%s: %s", failure.Reference(), failure.Message)
		}
		t.Fatalf("date/time fixtures: %d passed, failures:\n%s", report.Passes, strings.Join(failures, "\n"))
	}
	if report.Passes != 153 {
		t.Fatalf("date/time fixture passes = %d, want 153", report.Passes)
	}
}

func TestDateBuiltinsShareOneEvaluationTimestamp(t *testing.T) {
	node, err := syntax.Parse(`[$millis(), $millis(), $now(), $now('[Y0001]-[M01]-[D01]T[H01]:[m01]:[s01].[f001][Z01:01t]')]`)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2024, time.February, 29, 12, 34, 56, 789*int(time.Millisecond), time.UTC)
	got, evalErr := EvalBindingsWithOptions(node, nil, dateBuiltinBindings(), Options{Timestamp: fixed})
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	want := []any{float64(fixed.UnixMilli()), float64(fixed.UnixMilli()), "2024-02-29T12:34:56.789Z", "2024-02-29T12:34:56.789Z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("timestamp snapshot = %#v, want %#v", got, want)
	}
}

func TestDateTimeOnlyParsingDefaultsToCapturedDate(t *testing.T) {
	fixed := time.Date(2024, time.February, 29, 7, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		expression string
		want       any
	}{
		{expression: `$toMillis('13:45', '[H]:[m]')`, want: float64(time.Date(2024, time.February, 29, 13, 45, 0, 0, time.UTC).UnixMilli())},
		{expression: `$toMillis('13:45', '[H]:[m]') ~> $fromMillis()`, want: "2024-02-29T13:45:00.000Z"},
		{expression: `$toMillis('13:45', '[H]:[m]') ~> $fromMillis() ~> $substringBefore('T')`, want: "2024-02-29"},
		{expression: `$substringBefore($now(), 'T')`, want: "2024-02-29"},
		{expression: `$toMillis('13:45', '[H]:[m]') ~> $fromMillis() ~> $substringBefore('T') = $substringBefore($now(), 'T')`, want: true},
	} {
		node, err := syntax.Parse(test.expression)
		if err != nil {
			t.Fatalf("%s: %v", test.expression, err)
		}
		got, evalErr := EvalBindingsWithOptions(node, nil, dateBuiltinBindings(), Options{Timestamp: fixed})
		if evalErr != nil {
			t.Fatalf("%s: %v", test.expression, evalErr)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.expression, got, test.want)
		}
	}
}

func TestDateBuiltinsHonorCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	node, err := syntax.Parse(`$toMillis($input)`)
	if err != nil {
		t.Fatal(err)
	}
	bindings := dateBuiltinBindings()
	bindings["$input"] = "2024-01-01"
	if _, err := EvalBindingsWithOptions(node, nil, bindings, Options{Context: canceled}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled date parse error = %v, want context cancellation", err)
	}
}

func TestToMillisGHSA86VWInputIsBounded(t *testing.T) {
	node, err := syntax.Parse(`$toMillis($input)`)
	if err != nil {
		t.Fatal(err)
	}
	bindings := dateBuiltinBindings()
	// The vulnerable reference pattern repeated its optional month group. A
	// long non-match in that group must consume the shared operation budget.
	bindings["$input"] = "2024" + strings.Repeat("-01", 10000) + "!"
	started := time.Now()
	_, evalErr := EvalBindingsWithOptions(node, nil, bindings, Options{MaxOperations: 500})
	if !dateHasCode(evalErr, "U1001") {
		t.Fatalf("adversarial date parse error = %v, want U1001", evalErr)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("adversarial date parse exceeded bounded latency: %s", elapsed)
	}
}

func TestDateBuiltinSignatures(t *testing.T) {
	want := map[string]string{
		"fromMillis": "<n-s?s?:s>",
		"toMillis":   "<s-s?:n>",
		"now":        "<s?s?:s>",
		"millis":     "<:n>",
	}
	for _, spec := range dateBuiltinSpecs {
		if want[spec.name] != spec.signature {
			t.Errorf("%s signature = %q, want %q", spec.name, spec.signature, want[spec.name])
		}
		delete(want, spec.name)
	}
	if len(want) != 0 {
		t.Fatalf("missing date builtin specs: %#v", want)
	}
}

func dateBuiltinBindings() map[string]any {
	bindings := make(map[string]any, len(dateBuiltinSpecs))
	for _, spec := range dateBuiltinSpecs {
		bindings["$"+spec.name] = builtinValue{spec: spec}
	}
	return bindings
}

func dateHasCode(err error, want string) bool {
	var coded interface{ JSONataCode() string }
	return errors.As(err, &coded) && coded.JSONataCode() == want
}
