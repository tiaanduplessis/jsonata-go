package evaluator

import (
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func evalExpression(t *testing.T, expression string, input any) any {
	t.Helper()
	n, parseErr := syntax.Parse(expression)
	if parseErr != nil {
		t.Fatalf("parse %q: %v", expression, parseErr)
	}
	v, evalErr := Eval(n, input)
	if evalErr != nil {
		t.Fatalf("eval %q: %v", expression, evalErr)
	}
	return v
}

func TestSequenceAndArrayBoundaries(t *testing.T) {
	input := []any{
		map[string]any{"values": []any{1., 2.}},
		map[string]any{"values": []any{3.}},
	}
	if got := evalExpression(t, "values", input); !reflect.DeepEqual(got, []any{1., 2., 3.}) {
		t.Fatalf("sequence projection: %#v", got)
	}
	if got := evalExpression(t, "[1, 2]", nil); !reflect.DeepEqual(got, []any{1., 2.}) {
		t.Fatalf("constructed array: %#v", got)
	}
	if got := evalExpression(t, "$.[value,epoch][]", []any{map[string]any{"value": 1., "epoch": 2.}}); !reflect.DeepEqual(got, []any{[]any{1., 2.}}) {
		t.Fatalf("keep array: %#v", got)
	}
}

func TestSequenceEmptyOperators(t *testing.T) {
	if got := evalExpression(t, "missing ?? 42", nil); got != 42. {
		t.Fatalf("coalescing: %#v", got)
	}
	if got := evalExpression(t, "[0] ?: 42", nil); got != 42. {
		t.Fatalf("default: %#v", got)
	}
}

func TestExplicitArraysRemainGroupedAcrossFieldPaths(t *testing.T) {
	tests := []struct {
		expression string
		input      any
		want       any
	}{
		{expression: `{"a": [1] }.a`, input: nil, want: []any{1.}},
		{expression: `{"a": [[1]] }.a`, input: nil, want: []any{[]any{1.}}},
		{expression: `a[0].b`, input: []any{
			map[string]any{"a": []any{map[string]any{"b": []any{1.}}, map[string]any{"b": []any{2.}}}},
			map[string]any{"a": []any{map[string]any{"b": []any{3.}}, map[string]any{"b": []any{4.}}}},
		}, want: []any{1.}},
	}
	for _, test := range tests {
		if got := evalExpression(t, test.expression, test.input); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s: got %#v, want %#v", test.expression, got, test.want)
		}
	}
}

func TestUndefinedValuesAreNeverEqual(t *testing.T) {
	if got := evalExpression(t, "nothing = nothing", nil); got != false {
		t.Fatalf("nothing equality: %#v", got)
	}
}

func TestPredicateDoesNotEvaluateAgainstOuterContext(t *testing.T) {
	if got := evalExpression(t, "[0,1,2,3][$ % 2 = 0]", nil); !reflect.DeepEqual(got, []any{0., 2.}) {
		t.Fatalf("predicate projection: %#v", got)
	}
}
