package evaluator

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func evalPhase2(t *testing.T, expr string, input any) (any, error) {
	t.Helper()
	node, parseErr := syntax.Parse(expr)
	if parseErr != nil {
		t.Fatalf("parse %q: %v", expr, parseErr)
	}
	return Eval(node, input)
}

func TestSequenceConsumersDoNotMutateOwnedArrays(t *testing.T) {
	owned := value.Array{Items: []any{3.0, 1.0, 2.0}}
	wantOwned := value.Array{Items: []any{3.0, 1.0, 2.0}}

	sorted, err := builtinSort(state{}, []any{owned})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(owned, wantOwned) {
		t.Fatalf("sort mutated evaluator-owned array: %#v", owned)
	}
	if got, ok := sorted.(value.Array); !ok || !reflect.DeepEqual(got.Items, []any{1.0, 2.0, 3.0}) {
		t.Fatalf("sort result = %#v", sorted)
	}

	if _, err := builtinAppend(state{}, []any{owned, value.Array{Items: []any{4.0}}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(owned, wantOwned) {
		t.Fatalf("append mutated evaluator-owned array: %#v", owned)
	}

	selected, err := selectValue(owned, syntax.Literal{Kind: syntax.Number, Value: "1"}, state{})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := selected.(contextual); !ok || got.v != 1.0 {
		t.Fatalf("select result = %#v", selected)
	}
	if !reflect.DeepEqual(owned, wantOwned) {
		t.Fatalf("select mutated evaluator-owned array: %#v", owned)
	}

	input := map[string]any{"items": []any{
		map[string]any{"value": 1.0},
		map[string]any{"value": 2.0},
	}}
	wantInput := map[string]any{"items": []any{
		map[string]any{"value": 1.0},
		map[string]any{"value": 2.0},
	}}
	if _, err := evalPhase2(t, `$ ~> |items|{"changed":true}|`, input); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, wantInput) {
		t.Fatalf("transform mutated caller input: %#v", input)
	}
}

func TestPredicateEvaluatesAgainstEachCandidate(t *testing.T) {
	got, err := evalPhase2(t, `[0..9][$ % 2 = 0]`, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{float64(0), float64(2), float64(4), float64(6), float64(8)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestArrayConstructorFlattensPathSelectedArraysOnly(t *testing.T) {
	input := map[string]any{"groups": []any{"x", "y"}}
	for _, test := range []struct {
		expr string
		want any
	}{
		{`[groups]`, []any{"x", "y"}},
		{`[[groups]]`, []any{[]any{"x", "y"}}},
	} {
		got, err := evalPhase2(t, test.expr, input)
		if err != nil {
			t.Fatalf("%s: %v", test.expr, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s = %#v, want %#v", test.expr, got, test.want)
		}
	}
}

func TestTransformUpdatesAndDeletesDeepClone(t *testing.T) {
	input := map[string]any{"items": []any{
		map[string]any{"price": float64(2), "remove": true},
		map[string]any{"price": float64(3), "remove": true},
	}}
	got, err := evalPhase2(t, `$ ~> |items|{"total": price * 2}, "remove"|`, input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"items": []any{
		map[string]any{"price": float64(2), "total": float64(4)},
		map[string]any{"price": float64(3), "total": float64(6)},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(input, map[string]any{"items": []any{
		map[string]any{"price": float64(2), "remove": true},
		map[string]any{"price": float64(3), "remove": true},
	}}) {
		t.Fatal("transform mutated input")
	}
}

func TestTransformRejectsNonObjectUpdateAndDelete(t *testing.T) {
	for _, test := range []struct {
		expr string
		code string
	}{
		{`$ ~> |item|5|`, "T2011"},
		{`$ ~> |item|{},5|`, "T2012"},
	} {
		t.Run(test.code, func(t *testing.T) {
			_, err := evalPhase2(t, test.expr, map[string]any{"item": map[string]any{}})
			if err == nil {
				t.Fatal("expected evaluation error")
			}
			var coded interface{ JSONataCode() string }
			if !errors.As(err, &coded) || coded.JSONataCode() != test.code {
				t.Fatalf("got %v, want %s", err, test.code)
			}
		})
	}
}
