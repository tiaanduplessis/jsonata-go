package evaluator

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

type transformIdentityClone struct{}

func (transformIdentityClone) callableName() string { return "test clone" }

func (transformIdentityClone) invoke(_ state, args []any) (any, error) {
	if len(args) != 1 {
		return nil, functionArityError("test clone", 1, len(args))
	}
	return args[0], nil
}

type transformCountingPattern struct{ calls *int }

func (transformCountingPattern) callableName() string { return "test pattern" }

func (f transformCountingPattern) invoke(st state, args []any) (any, error) {
	if len(args) != 0 {
		return nil, functionArityError("test pattern", 0, len(args))
	}
	*f.calls++
	return st.current, nil
}

func TestTransformIsFirstClassAndComposable(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       any
	}{
		{
			name:       "assigned and invoked",
			expression: `($increment := 2; $transform := |item|{"total": value + $increment}|; $transform({"item":{"value":3}}))`,
			want:       map[string]any{"item": map[string]any{"value": 3.0, "total": 5.0}},
		},
		{
			name:       "composed",
			expression: `($first := |$|{"one":1}|; $second := |$|{"two":2}|; ($first ~> $second)({}))`,
			want:       map[string]any{"one": 1.0, "two": 2.0},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evalPhase2(t, test.expression, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestTransformNoMatchAndUndefinedClausesAreNoOps(t *testing.T) {
	input := map[string]any{"item": map[string]any{"value": 1.0}}
	for _, expression := range []string{
		`$ ~> |missing|{"added":true}|`,
		`$ ~> |item|missing|`,
		`$ ~> |item|{},missing|`,
	} {
		got, err := evalPhase2(t, expression, input)
		if err != nil {
			t.Fatalf("%s: %v", expression, err)
		}
		if !reflect.DeepEqual(got, input) {
			t.Fatalf("%s: got %#v, want %#v", expression, got, input)
		}
	}
}

func TestTransformEvaluatesPatternOnce(t *testing.T) {
	node, parseErr := syntax.Parse(`|$pattern()|{}|`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	calls := 0
	env := newLexicalEnv(nil).bindFrame(map[string]any{"$pattern": transformCountingPattern{calls: &calls}})
	call := newTransformValue(node.(syntax.Transform), state{env: env, runtime: newEvalRuntime(Options{})})
	_, err := call.invoke(state{runtime: newEvalRuntime(Options{})}, []any{map[string]any{"item": []any{map[string]any{}, map[string]any{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("pattern calls = %d, want 1", calls)
	}
}

func TestTransformArity(t *testing.T) {
	for _, test := range []struct {
		expression string
		code       string
	}{
		{expression: `(|$|{}|)()`, code: "T0411"},
		{expression: `(|$|{}|)({}, {})`, code: "T0410"},
	} {
		_, err := evalPhase2(t, test.expression, nil)
		if err == nil {
			t.Fatalf("%s: expected %s", test.expression, test.code)
		}
		var coded interface{ JSONataCode() string }
		if !errors.As(err, &coded) || coded.JSONataCode() != test.code {
			t.Fatalf("%s: got %v, want %s", test.expression, err, test.code)
		}
	}
}

func TestTransformUsesLexicalCloneBindingAndProtectsInput(t *testing.T) {
	node, parseErr := syntax.Parse(`|item|{"added":true}|`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	env := newLexicalEnv(nil).bindFrame(map[string]any{"$clone": transformIdentityClone{}})
	call := newTransformValue(node.(syntax.Transform), state{env: env, runtime: newEvalRuntime(Options{})})
	input := map[string]any{"item": map[string]any{"value": 1.0}}
	got, err := call.invoke(state{runtime: newEvalRuntime(Options{})}, []any{input})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"item": map[string]any{"value": 1.0, "added": true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(input, map[string]any{"item": map[string]any{"value": 1.0}}) {
		t.Fatalf("custom clone allowed caller input mutation: %#v", input)
	}
}

func TestTransformCloneOverrideMustBeCallable(t *testing.T) {
	_, err := evalPhase2(t, `($clone := 1; {} ~> |$|{}|)`, nil)
	if err == nil {
		t.Fatal("expected T2013")
	}
	assertTransformError(t, err, "T2013", 21)
}

func TestTransformErrorsIncludeExpressionPositions(t *testing.T) {
	tests := []struct {
		expression string
		code       string
		position   int
	}{
		{expression: `{ } ~> |$|5|`, code: "T2011", position: 11},
		{expression: `{ } ~> |$|{},5|`, code: "T2012", position: 14},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			_, err := evalPhase2(t, test.expression, nil)
			if err == nil {
				t.Fatalf("expected %s", test.code)
			}
			assertTransformError(t, err, test.code, test.position)
		})
	}
}

func TestTransformUpdatePrecedesDeleteAndPrototypeKeysAreData(t *testing.T) {
	got, err := evalPhase2(t, `$ ~> |$|{"remove":"constructor","__proto__":{"safe":true},"constructor":"data"}, remove|`, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"remove":    "constructor",
		"__proto__": map[string]any{"safe": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestTransformUpdateShallowlyReplacesNestedObjects(t *testing.T) {
	got, err := evalPhase2(t, `$ ~> |item|{"nested":{"new":2}}|`, map[string]any{
		"item": map[string]any{"nested": map[string]any{"keep": 1.0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"item": map[string]any{"nested": map[string]any{"new": 2.0}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestTransformHandlesOrderedObjectsAndExplicitArrays(t *testing.T) {
	input := value.OrderedObject{
		Fields: map[string]any{
			"items": value.Array{Items: []any{
				value.OrderedObject{Fields: map[string]any{"value": 1.0}, Order: []string{"value"}},
			}},
		},
		Order: []string{"items"},
	}
	got, err := evalPhase2(t, `$ ~> |items|{"added":true}|`, input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"items": []any{map[string]any{"value": 1.0, "added": true}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestTransformPreservesOrderedObjectFieldsForStringification(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       string
	}{
		{
			name:       "single field",
			expression: `$string({"a":1} ~> |$|{"b":2}|)`,
			want:       `{"a":1,"b":2}`,
		},
		{
			name:       "multiple fields retain update order",
			expression: `$string({"base":0} ~> |$|{"z":1,"a":2}|)`,
			want:       `{"base":0,"z":1,"a":2}`,
		},
		{
			name:       "existing fields retain position",
			expression: `$string({"a":1,"b":2} ~> |$|{"a":3,"z":4,"c":5}|)`,
			want:       `{"a":3,"b":2,"z":4,"c":5}`,
		},
		{
			name:       "numeric keys use ECMAScript order",
			expression: `$string({"base":0} ~> |$|{"z":1,"10":10,"a":2,"2":2}|)`,
			want:       `{"2":2,"10":10,"base":0,"z":1,"a":2}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evalPhase2(t, test.expression, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTransformTraversalRejectsCycles(t *testing.T) {
	node, parseErr := syntax.Parse(`|$|{"added":true}|`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	call := newTransformValue(node.(syntax.Transform), state{runtime: newEvalRuntime(Options{})})
	input := map[string]any{}
	input["self"] = input
	_, err := call.invoke(state{runtime: newEvalRuntime(Options{})}, []any{input})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var coded interface{ JSONataCode() string }
	if !errors.As(err, &coded) || coded.JSONataCode() != "U1001" {
		t.Fatalf("got %v, want U1001", err)
	}
}

func assertTransformError(t *testing.T, err error, code string, position int) {
	t.Helper()
	var coded interface{ JSONataCode() string }
	if !errors.As(err, &coded) || coded.JSONataCode() != code {
		t.Fatalf("got %v, want %s", err, code)
	}
	positioned, ok := err.(interface{ JSONataPosition() (int, bool) })
	if !ok {
		t.Fatalf("%s did not expose a position", code)
	}
	got, present := positioned.JSONataPosition()
	if !present || got != position {
		t.Fatalf("%s position = (%d, %v), want (%d, true)", code, got, present, position)
	}
}
