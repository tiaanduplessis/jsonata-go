package evaluator_test

import (
	"context"
	"math"
	"reflect"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

const staticMapExpression = `$map(orders, function($item){$item.price * $item.quantity})`

func TestStaticMapPlanRecognizesOnlyTheIntendedAST(t *testing.T) {
	n, err := syntax.Parse(staticMapExpression)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticMapPlan(n)
	if plan == nil {
		t.Fatal("static map plan was not built")
	}
	for _, expression := range []string{
		`$map(orders, function($item){$item.price + $item.quantity})`,
		`$map(orders, function($item){$other.price * $item.quantity})`,
		`$map(orders, function($item, $index){$item.price * $item.quantity})`,
		`$map(orders, function($item){$item.price * 2})`,
		`$filter(orders, function($item){$item.price = 1})`,
	} {
		n, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			continue
		}
		if evaluator.BuildStaticMapPlan(n) != nil {
			t.Fatalf("ambiguous expression was planned: %s", expression)
		}
	}
}

func TestStaticMapMatchesFullAndPreservesShape(t *testing.T) {
	expr := jsonata.MustCompile(staticMapExpression)
	cases := []struct {
		input any
		want  any
	}{
		{map[string]any{"orders": []any{
			map[string]any{"price": 2, "quantity": 3},
			map[string]any{"price": 4.0, "quantity": 5},
		}}, []any{float64(6), float64(20)}},
		{map[string]any{"orders": []any{map[string]any{"price": 2, "quantity": 3}}}, float64(6)},
		{map[string]any{"orders": map[string]any{"price": 2, "quantity": 3}}, float64(6)},
	}
	for _, test := range cases {
		fast, fastErr := expr.Eval(test.input)
		full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: context.Background()})
		if fastErr != nil || fullErr != nil || !reflect.DeepEqual(fast, full) || !reflect.DeepEqual(fast, test.want) {
			t.Fatalf("input %#v: fast=(%#v,%v) full=(%#v,%v) want %#v", test.input, fast, fastErr, full, fullErr, test.want)
		}
	}
}

func TestStaticMapFallsBackForSemanticallyAmbiguousInput(t *testing.T) {
	n, err := syntax.Parse(staticMapExpression)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticMapPlan(n)
	if plan == nil {
		t.Fatal("static map plan was not built")
	}
	cases := []any{
		map[string]any{},
		map[string]any{"orders": []any{}},
		map[string]any{"orders": []any{map[string]any{"price": 2}}},
		map[string]any{"orders": []any{map[string]any{"price": "2", "quantity": 3}}},
		map[string]any{"orders": []any{[]any{map[string]any{"price": 2, "quantity": 3}}}},
		map[string]any{"orders": []any{map[string]any{"price": math.NaN(), "quantity": 3}}},
	}
	for _, input := range cases {
		if got, ok := evaluator.EvalStaticMap(plan, input); ok {
			t.Fatalf("ambiguous input %#v unexpectedly returned %#v", input, got)
		}
	}
	cycle := map[string]any{}
	cycle["loop"] = cycle
	cycle["orders"] = []any{map[string]any{"price": 2, "quantity": 3}}
	if _, ok := evaluator.EvalStaticMap(plan, cycle); ok {
		t.Fatal("cyclic input used the static map plan")
	}
}

func TestStaticMapIsConcurrentAndDoesNotBypassOptions(t *testing.T) {
	expr := jsonata.MustCompile(staticMapExpression)
	input := map[string]any{"orders": []any{map[string]any{"price": 2, "quantity": 3}}}
	if _, err := expr.EvalWithOptions(input, jsonata.EvalOptions{MaxOperations: 1}); err == nil {
		t.Fatal("operation budget was bypassed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: ctx}); err == nil {
		t.Fatal("canceled context was bypassed")
	}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, err := expr.Eval(input)
				if err != nil || !reflect.DeepEqual(got, float64(6)) {
					t.Errorf("got %#v, %v", got, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestStaticMapDisablesForRegisteredMapOverride(t *testing.T) {
	expr := jsonata.MustCompile(staticMapExpression)
	if err := expr.RegisterExts(map[string]jsonata.Extension{
		"map": {Func: func(any, any) int { return 99 }},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := expr.Eval(map[string]any{"orders": []any{map[string]any{"price": 2, "quantity": 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, 99) {
		t.Fatalf("registered map override returned %#v, want 99", got)
	}
}

func BenchmarkStaticMapMediumHigherOrder(b *testing.B) {
	expr := jsonata.MustCompile(staticMapExpression)
	input := map[string]any{"orders": []any{
		map[string]any{"price": 10, "quantity": 2},
		map[string]any{"price": 11, "quantity": 3},
		map[string]any{"price": 12, "quantity": 4},
		map[string]any{"price": 13, "quantity": 5},
		map[string]any{"price": 14, "quantity": 6},
	}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := expr.Eval(input); err != nil {
			b.Fatal(err)
		}
	}
}
