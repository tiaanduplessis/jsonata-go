package evaluator_test

import (
	"math"
	"reflect"
	"strconv"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func BenchmarkStaticSumPlan(b *testing.B) {
	node, err := syntax.Parse(`$sum(orders[status="paid"].(price * quantity))`)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticSumPlan(node)
	input := map[string]any{"orders": []any{
		map[string]any{"status": "paid", "price": 12, "quantity": 2},
		map[string]any{"status": "pending", "price": 99, "quantity": 9},
		map[string]any{"status": "paid", "price": 4.5, "quantity": 2},
	}}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := evaluator.EvalStaticSum(plan, input); !ok {
			b.Fatal("sum plan declined benchmark input")
		}
	}
}

func TestStaticSumPlanMatchesForcedFullEvaluation(t *testing.T) {
	cases := []struct {
		expression string
		input      any
		want       float64
	}{
		{
			expression: `$sum(orders[status="paid"].amount)`,
			input: map[string]any{"orders": []any{
				map[string]any{"status": "paid", "amount": 12},
				map[string]any{"status": "pending", "amount": 9},
				map[string]any{"status": "paid", "amount": 4.5},
			}},
			want: 16.5,
		},
		{
			expression: `$sum(orders[status="paid"].(price * quantity))`,
			input: map[string]any{"orders": []any{
				map[string]any{"status": "paid", "price": 12, "quantity": 2},
				map[string]any{"status": "pending", "price": 99, "quantity": 9},
				map[string]any{"status": "paid", "price": 4.5, "quantity": 2},
			}},
			want: 33,
		},
		{
			expression: `$sum(events[type="sale" and active].amount)`,
			input: map[string]any{"events": []any{
				map[string]any{"type": "sale", "active": true, "amount": 2},
				map[string]any{"type": "sale", "active": false, "amount": 100},
				map[string]any{"type": "refund", "active": true, "amount": 80},
			}},
			want: 2,
		},
		{
			expression: `$sum(Account.Order.Product.(Price * Quantity))`,
			input: map[string]any{"Account": map[string]any{"Order": []any{
				map[string]any{"Product": []any{
					map[string]any{"Price": 2, "Quantity": 3},
					map[string]any{"Price": 4, "Quantity": 5},
				}},
			}}},
			want: 26,
		},
	}
	for _, test := range cases {
		t.Run(test.expression, func(t *testing.T) {
			node, err := syntax.Parse(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			plan := evaluator.BuildStaticSumPlan(node)
			if plan == nil {
				t.Fatal("sum plan was not built")
			}
			fast, ok := evaluator.EvalStaticSum(plan, test.input)
			if !ok || !reflect.DeepEqual(fast, test.want) {
				t.Fatalf("fast = %#v, ok=%v, want %#v", fast, ok, test.want)
			}
			full, fullErr := jsonata.MustCompile(test.expression).EvalWithOptions(test.input, jsonata.EvalOptions{MaxOperations: 100_000})
			if fullErr != nil || !reflect.DeepEqual(full, fast) {
				t.Fatalf("full = %#v, %v; fast = %#v", full, fullErr, fast)
			}
		})
	}
}

func TestStaticSumPlanIsConservative(t *testing.T) {
	for _, expression := range []string{
		`$sum(orders[status="paid"].amount)`,
		`$sum(orders[status="paid" and active].amount)`,
		`$sum(Account.Order.Product.(Price * Quantity))`,
	} {
		node, err := syntax.Parse(expression)
		if err != nil {
			t.Fatal(err)
		}
		if evaluator.BuildStaticSumPlan(node) == nil {
			t.Fatalf("plan was not built for %q", expression)
		}
	}
	for _, expression := range []string{
		`$sum(orders[status="paid"].(price + quantity))`,
		`$sum(orders[status=$wanted].amount)`,
		`$sum(orders[$contains(status, "paid")].amount)`,
		`$sum(orders[status="paid"].$amount)`,
	} {
		node, err := syntax.Parse(expression)
		if err != nil {
			continue
		}
		if plan := evaluator.BuildStaticSumPlan(node); plan != nil {
			t.Fatalf("ambiguous expression %q unexpectedly planned", expression)
		}
	}
}

func TestStaticSumPlanFallsBackBeforeErrors(t *testing.T) {
	node, err := syntax.Parse(`$sum(orders[status="paid"].amount)`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticSumPlan(node)
	inputs := []any{
		map[string]any{},
		map[string]any{"orders": []any{}},
		map[string]any{"orders": []any{map[string]any{"status": "paid"}}},
		map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": "12"}}},
		map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": math.NaN()}}},
	}
	for _, input := range inputs {
		if result, ok := evaluator.EvalStaticSum(plan, input); ok {
			t.Fatalf("ambiguous input %#v unexpectedly planned result %#v", input, result)
		}
	}
	cycle := map[string]any{}
	cycle["loop"] = cycle
	cycle["orders"] = []any{map[string]any{"status": "paid", "amount": 1}}
	if _, ok := evaluator.EvalStaticSum(plan, cycle); ok {
		t.Fatal("cyclic input unexpectedly used sum plan")
	}
	oversized := make(map[string]any, 100_001)
	oversized["orders"] = []any{map[string]any{"status": "paid", "amount": 1}}
	for i := 0; i < 100_000; i++ {
		oversized["unrelated"+strconv.Itoa(i)] = i
	}
	if _, ok := evaluator.EvalStaticSum(plan, oversized); ok {
		t.Fatal("oversized input unexpectedly used sum plan")
	}
}

func TestStaticSumPlanPreservesNestedArrayBoundaries(t *testing.T) {
	cases := []struct {
		expression string
		input      any
		planned    bool
		want       any
	}{
		{
			expression: `$sum(arr)`,
			input:      map[string]any{"arr": []any{[]any{1, 2}, []any{3}}},
			planned:    false,
		},
		{
			expression: `$sum(arr.n)`,
			input: map[string]any{"arr": []any{
				map[string]any{"n": []any{1, 2}},
				map[string]any{"n": []any{3}},
			}},
			planned: true,
			want:    float64(6),
		},
		{
			expression: `$sum(Account.Order.Product.(Price * Quantity))`,
			input: map[string]any{"Account": map[string]any{"Order": []any{
				map[string]any{"Product": []any{[]any{map[string]any{"Price": 2, "Quantity": 3}}}},
			}}},
			planned: false,
		},
		{
			expression: `$sum(orders[status="paid"].amount)`,
			input: map[string]any{"orders": []any{
				[]any{map[string]any{"status": "paid", "amount": 1}},
			}},
			planned: false,
		},
	}
	for _, test := range cases {
		t.Run(test.expression, func(t *testing.T) {
			node, err := syntax.Parse(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			plan := evaluator.BuildStaticSumPlan(node)
			if plan == nil {
				t.Fatal("sum plan was not built")
			}
			got, ok := evaluator.EvalStaticSum(plan, test.input)
			if ok != test.planned {
				t.Fatalf("planned=%v, got=%#v", ok, got)
			}
			if test.planned && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("fast=%#v, want %#v", got, test.want)
			}
			full, fullErr := jsonata.MustCompile(test.expression).EvalWithOptions(test.input, jsonata.EvalOptions{MaxOperations: 100_000})
			if test.planned && (fullErr != nil || !reflect.DeepEqual(full, got)) {
				t.Fatalf("full=%#v, %v; fast=%#v", full, fullErr, got)
			}
		})
	}
}

func TestStaticSumPlanRegistryAndConcurrency(t *testing.T) {
	node, err := syntax.Parse(`$sum(orders[status="paid"].(price * quantity))`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticSumPlan(node)
	if plan.RegistryConflict(map[string]any{"unrelated": func() {}}) || !plan.RegistryConflict(map[string]any{"price": func() {}}) {
		t.Fatal("registry conflict detection is incorrect")
	}
	input := map[string]any{"orders": []any{map[string]any{"status": "paid", "price": 3, "quantity": 4}}}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, ok := evaluator.EvalStaticSum(plan, input)
				if !ok || got != float64(12) {
					t.Errorf("sum = %#v, ok=%v", got, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}
