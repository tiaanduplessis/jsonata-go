package evaluator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

type scalarFixtureCompiler struct{}

type scalarFixtureExpression struct {
	node syntax.Node
}

func (scalarFixtureCompiler) Compile(expression string) (conformance.Expression, error) {
	node, err := syntax.Parse(expression)
	if err != nil {
		return nil, err
	}
	return scalarFixtureExpression{node: node}, nil
}

func (expression scalarFixtureExpression) Eval(data any) (any, error) {
	return Eval(expression.node, data)
}

func (expression scalarFixtureExpression) EvalNoInputBindings(bindings map[string]any) (any, error) {
	return EvalNoInputBindingsWithOptions(expression.node, bindings, Options{})
}

func TestScalarBuiltinFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	groups := []string{
		"function-abs", "function-assert", "function-boolean", "function-ceil",
		"function-error", "function-exists", "function-floor", "function-number",
		"function-power", "function-round", "function-sqrt",
	}
	manifest := make(conformance.Manifest, len(groups))
	for _, groupName := range groups {
		for _, group := range suite.Groups {
			if group.Name != groupName {
				continue
			}
			manifest[groupName] = make(map[string]struct{}, len(group.Cases))
			for _, fixture := range group.Cases {
				manifest[groupName][fixture.ID] = struct{}{}
			}
		}
	}
	report := conformance.RunWithOptions(suite, scalarFixtureCompiler{}, manifest, conformance.Options{UndefinedError: errUndefined})
	if report.EnabledCases != 143 {
		t.Fatalf("scalar fixture count = %d, want 143", report.EnabledCases)
	}
	if len(report.Failures) != 0 {
		failures := make([]string, len(report.Failures))
		for index, failure := range report.Failures {
			failures[index] = fmt.Sprintf("%s: %s", failure.Reference(), failure.Message)
		}
		t.Fatalf("scalar fixtures: %d passed, failures:\n%s", report.Passes, strings.Join(failures, "\n"))
	}
	if report.Passes != 143 {
		t.Fatalf("scalar fixture passes = %d, want 143", report.Passes)
	}
}

func TestNotBuiltinRegressions(t *testing.T) {
	for expression, want := range map[string]any{
		"$not(true)":     false,
		"$not(false)":    true,
		"$not(nothing)":  value.Undefined,
		"$not($boolean)": true,
	} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		got, err := Eval(node, nil)
		if value.IsUndefined(want) {
			if !IsUndefined(err) {
				t.Fatalf("%s: got result=%#v err=%v, want undefined", expression, got, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", expression, err)
		}
		if got != want {
			t.Fatalf("%s: got %#v, want %#v", expression, got, want)
		}
	}
}

func TestNumberUsesContextDefault(t *testing.T) {
	node, parseErr := syntax.Parse(`$number()`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	got, err := Eval(node, "5")
	if err != nil {
		t.Fatal(err)
	}
	if got != 5.0 {
		t.Fatalf("got %#v, want 5", got)
	}
}

func TestScalarBuiltinSignatures(t *testing.T) {
	want := map[string]string{
		"abs":     "<n-:n>",
		"assert":  "<bs?:x>",
		"boolean": "<x-:b>",
		"ceil":    "<n-:n>",
		"error":   "<s?:x>",
		"exists":  "<x:b>",
		"floor":   "<n-:n>",
		"not":     "<x-:b>",
		"number":  "<(nsb)-:n>",
		"power":   "<n-n:n>",
		"round":   "<n-n?:n>",
		"sqrt":    "<n-:n>",
	}
	for name, signature := range want {
		spec, ok := builtinSpecFor(name)
		if !ok {
			t.Fatalf("missing scalar builtin %q", name)
		}
		if spec.signature != signature {
			t.Errorf("%s signature = %q, want %q", name, spec.signature, signature)
		}
	}
}

func TestScalarNumberGrammar(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{input: "0", want: 0},
		{input: "-0.05", want: -0.05},
		{input: "1e-2", want: 0.01},
		{input: "0xFFFFFFFFFFFFFFFF", want: 18446744073709551616},
		{input: "0B101", want: 5},
		{input: "0O12", want: 10},
	}
	for _, test := range tests {
		got, err := parseScalarNumber(test.input)
		if err != nil || got != test.want {
			t.Errorf("parseScalarNumber(%q) = %v, %v; want %v", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"+1", ".5", "1.", "1_000", "-0x1", "0x-1", "0x", "10e500"} {
		if _, err := parseScalarNumber(input); err == nil {
			t.Errorf("parseScalarNumber(%q) accepted invalid input", input)
		}
	}
}

func TestScalarSpecialCases(t *testing.T) {
	if _, err := builtinPower(state{}, []any{2.0, value.Undefined}); scalarErrorCode(err) != "D3061" {
		t.Fatalf("power with undefined exponent: %v", err)
	}
	if _, err := builtinError(state{}, []any{""}); scalarErrorMessage(err) != "$error() function evaluated" {
		t.Fatalf("empty error message: %v", err)
	}
	if _, err := builtinAssert(state{}, []any{false, ""}); scalarErrorMessage(err) != "$assert() statement failed" {
		t.Fatalf("empty assert message: %v", err)
	}
	if _, err := builtinRound(state{}, []any{1.0, 0.5}); scalarErrorCode(err) != "D1001" {
		t.Fatalf("non-integer round precision: %v", err)
	}
}

func scalarErrorCode(err error) string {
	var coded interface{ JSONataCode() string }
	if errors.As(err, &coded) {
		return coded.JSONataCode()
	}
	return ""
}

func scalarErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
