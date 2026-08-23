package jsonata

import (
	"context"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

const staticTransformExpression = `$ ~> |account.orders[status = "pending"]|{"status": "review", "total": price * quantity}, ["internal"]|`

func TestStaticTransformPlanMatchesForcedFullEvaluator(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name: "medium corpus",
			input: map[string]any{
				"account": map[string]any{"id": "acct-7", "orders": []any{
					map[string]any{"id": 1.0, "status": "paid", "price": 4.0, "quantity": 2.0, "internal": "a"},
					map[string]any{"id": 2.0, "status": "pending", "price": 9.0, "quantity": 3.0, "internal": "b"},
					map[string]any{"id": 3.0, "status": "pending", "price": 2.5, "quantity": 4.0, "internal": "c"},
				}},
				"trace": "keep",
			},
		},
		{
			name: "no matches",
			input: map[string]any{"account": map[string]any{"orders": []any{
				map[string]any{"status": "paid", "price": 4.0, "quantity": 2.0, "internal": "a"},
			}}},
		},
		{
			name: "missing product field is omitted",
			input: map[string]any{"account": map[string]any{"orders": []any{
				map[string]any{"status": "pending", "price": 4.0, "total": 99.0, "internal": "a"},
			}}},
		},
	}

	node, err := syntax.Parse(staticTransformExpression)
	if err != nil {
		t.Fatal(err)
	}
	if evaluator.BuildStaticTransformPlan(node) == nil {
		t.Fatal("frozen transform did not produce a static plan")
	}
	expr := MustCompile(staticTransformExpression)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fast, fastErr := expr.Eval(test.input)
			full, fullErr := expr.EvalWithOptions(test.input, EvalOptions{Context: context.Background()})
			if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
				t.Fatalf("fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
			}
		})
	}
}

func TestStaticTransformDoesNotMutateCallerInput(t *testing.T) {
	input := map[string]any{"account": map[string]any{"orders": []any{
		map[string]any{"status": "pending", "price": 9.0, "quantity": 3.0, "internal": map[string]any{"token": "secret"}},
	}}}
	wantInput := map[string]any{"account": map[string]any{"orders": []any{
		map[string]any{"status": "pending", "price": 9.0, "quantity": 3.0, "internal": map[string]any{"token": "secret"}},
	}}}
	if _, err := MustCompile(staticTransformExpression).Eval(input); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, wantInput) {
		t.Fatalf("transform mutated caller input: %#v", input)
	}
}

func TestStaticTransformDuplicateUpdateKeysUseFullEvaluatorError(t *testing.T) {
	expression := `$ ~> |account.orders[status = "pending"]|{"total": price * quantity, "total": 99}|`
	node, err := syntax.Parse(expression)
	if err != nil {
		t.Fatal(err)
	}
	if evaluator.BuildStaticTransformPlan(node) != nil {
		t.Fatal("duplicate update keys produced a static plan")
	}
	_, evalErr := MustCompile(expression).Eval(map[string]any{"account": map[string]any{"orders": []any{
		map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0},
	}}})
	jsonataErr, ok := evalErr.(*Error)
	if !ok || jsonataErr.Code != "D1009" {
		t.Fatalf("error = %#v; want D1009", evalErr)
	}
}

func TestStaticTransformAliasesUseForcedFullSemantics(t *testing.T) {
	expr := MustCompile(staticTransformExpression)
	sharedMap := map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0}
	sharedSlice := []any{sharedMap}
	input := map[string]any{
		"account": map[string]any{"orders": sharedSlice},
		"alias":   sharedMap,
		"orders":  sharedSlice,
	}
	fast, fastErr := expr.Eval(input)
	full, fullErr := expr.EvalWithOptions(input, EvalOptions{Context: context.Background()})
	if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
		t.Fatalf("alias input fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
	}
}

func TestStaticTransformFallsBackForDiagnosticsAndUnsafeInputs(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{name: "non-numeric product", input: map[string]any{"account": map[string]any{"orders": []any{map[string]any{"status": "pending", "price": "nine", "quantity": 3.0}}}}},
		{name: "overflow product", input: map[string]any{"account": map[string]any{"orders": []any{map[string]any{"status": "pending", "price": math.MaxFloat64, "quantity": 2.0}}}}},
	}
	expr := MustCompile(staticTransformExpression)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fast, fastErr := expr.Eval(test.input)
			full, fullErr := expr.EvalWithOptions(test.input, EvalOptions{Context: context.Background()})
			if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
				t.Fatalf("fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
			}
		})
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	cycle["account"] = map[string]any{"orders": []any{}}
	if _, err := expr.Eval(cycle); err == nil {
		t.Fatal("cyclic input unexpectedly succeeded")
	}
}

func TestStaticTransformHonorsBindingsRegistryAndOptions(t *testing.T) {
	expr := MustCompile(staticTransformExpression)
	input := map[string]any{"account": map[string]any{"orders": []any{
		map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0},
	}}}
	withOptions, optionsErr := expr.EvalWithOptions(input, EvalOptions{MaxOperations: 1})
	forcedOptions, forcedOptionsErr := evaluator.EvalWithOptions(func() syntax.Node {
		parsed, err := syntax.Parse(staticTransformExpression)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}(), input, evaluator.Options{MaxOperations: 1})
	if (optionsErr == nil) != (forcedOptionsErr == nil) || (optionsErr == nil && !reflect.DeepEqual(withOptions, forcedOptions)) {
		t.Fatalf("options changed semantics: options=(%#v,%v), forced=(%#v,%v)", withOptions, optionsErr, forcedOptions, forcedOptionsErr)
	}
	if err := expr.RegisterVars(map[string]any{"price": 100.0}); err != nil {
		t.Fatal(err)
	}
	registered, registeredErr := expr.Eval(input)
	forced, forcedErr := expr.EvalWithOptions(input, EvalOptions{Context: context.Background()})
	if (registeredErr == nil) != (forcedErr == nil) || !reflect.DeepEqual(registered, forced) {
		t.Fatalf("registry changed semantics: registered=(%#v,%v), forced=(%#v,%v)", registered, registeredErr, forced, forcedErr)
	}
}

func TestStaticTransformConcurrentEvaluation(t *testing.T) {
	expr := MustCompile(staticTransformExpression)
	input := map[string]any{"account": map[string]any{"orders": []any{
		map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0},
	}}}
	want, err := expr.Eval(input)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := expr.Eval(input)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("got=(%#v,%v), want=%#v", got, err, want)
			}
		}()
	}
	wg.Wait()
}

func BenchmarkStaticTransformDecoded(b *testing.B) {
	expr := MustCompile(staticTransformExpression)
	input := map[string]any{
		"account": map[string]any{"id": "acct-7", "orders": []any{
			map[string]any{"id": 1.0, "status": "paid", "price": 4.0, "quantity": 2.0, "internal": "a"},
			map[string]any{"id": 2.0, "status": "pending", "price": 9.0, "quantity": 3.0, "internal": "b"},
			map[string]any{"id": 3.0, "status": "pending", "price": 2.5, "quantity": 4.0, "internal": "c"},
		}},
		"trace": "keep",
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := expr.Eval(input); err != nil {
			b.Fatal(err)
		}
	}
}
