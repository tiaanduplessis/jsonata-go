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

func TestStaticDescendantSumPlanMatchesFullEvaluation(t *testing.T) {
	input := map[string]any{
		"payload": map[string]any{
			"right": map[string]any{
				"children": []any{
					map[string]any{"value": 5},
					map[string]any{"value": 6},
					map[string]any{"nested": map[string]any{"value": 7}},
				},
				"value": 4,
			},
			"left": map[string]any{
				"value": 1,
				"child": map[string]any{
					"value": 2,
					"child": map[string]any{"value": 3},
				},
			},
			"tail": []any{
				map[string]any{"value": 8},
				map[string]any{"value": 9},
				map[string]any{"value": 10},
			},
		},
	}
	assertStaticDescendantSum(t, `$sum(payload.**.value)`, input, 55.0)
}

func TestStaticDescendantSumPlanHandlesPathArraysAndMissingDescendants(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  float64
	}{
		{
			name: "array along head",
			input: map[string]any{"payload": []any{
				map[string]any{"value": 1, "nested": map[string]any{"value": 2}},
				map[string]any{"group": map[string]any{"value": 3}},
			}},
			want: 6,
		},
		{
			name: "missing fields ignored",
			input: map[string]any{"payload": map[string]any{
				"metadata": map[string]any{"label": "not a match"},
				"nested":   map[string]any{"value": 4, "leaf": true},
			}},
			want: 4,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertStaticDescendantSum(t, `$sum(payload.**.value)`, test.input, test.want)
		})
	}
}

func TestStaticDescendantSumPlanIsConservative(t *testing.T) {
	for _, expression := range []string{
		`$sum(payload.**.value)`,
		`$sum(account.records.**.amount)`,
	} {
		node, err := syntax.Parse(expression)
		if err != nil {
			t.Fatal(err)
		}
		if evaluator.BuildStaticDescendantSumPlan(node) == nil {
			t.Fatalf("plan was not built for %q", expression)
		}
	}
	for _, expression := range []string{
		`$sum(payload.*.value)`,
		`$sum(payload.**.$value)`,
		`$sum($payload.**.value)`,
	} {
		node, err := syntax.Parse(expression)
		if err != nil {
			continue
		}
		if evaluator.BuildStaticDescendantSumPlan(node) != nil {
			t.Fatalf("ambiguous expression %q unexpectedly planned", expression)
		}
	}
}

func TestStaticDescendantSumPlanFallsBackOnAmbiguousResults(t *testing.T) {
	node, err := syntax.Parse(`$sum(payload.**.value)`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticDescendantSumPlan(node)
	for _, input := range []any{
		map[string]any{"payload": map[string]any{"value": "12"}},
		map[string]any{"payload": map[string]any{"value": []any{1, 2}}},
		map[string]any{"payload": map[string]any{"value": map[string]any{"n": 1}}},
		map[string]any{"payload": map[string]any{"value": math.NaN()}},
		map[string]any{"payload": map[string]any{"value": math.MaxFloat64, "nested": map[string]any{"value": math.MaxFloat64}}},
		map[string]any{"payload": map[string]any{"metadata": map[string]any{"name": "none"}}},
	} {
		if result, ok := evaluator.EvalStaticDescendantSum(plan, input); ok {
			t.Fatalf("ambiguous input %#v unexpectedly planned result %#v", input, result)
		}
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	cycle["value"] = 1
	if _, ok := evaluator.EvalStaticDescendantSum(plan, map[string]any{"payload": cycle}); ok {
		t.Fatal("cyclic input unexpectedly used descendant sum plan")
	}
	oversized := make(map[string]any, 30_000)
	for i := 0; i < 30_000; i++ {
		oversized["field"+strconv.Itoa(i)] = i
	}
	if _, ok := evaluator.EvalStaticDescendantSum(plan, map[string]any{"payload": oversized}); ok {
		t.Fatal("oversized input unexpectedly used descendant sum plan")
	}
}

func TestStaticDescendantSumPlanRegistryAndConcurrency(t *testing.T) {
	node, err := syntax.Parse(`$sum(payload.**.value)`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticDescendantSumPlan(node)
	if plan.RegistryConflict(map[string]any{"unrelated": func() {}}) {
		t.Fatal("unrelated registry entry disabled plan")
	}
	if !plan.RegistryConflict(map[string]any{"value": func() {}}) {
		t.Fatal("descendant field override did not disable plan")
	}
	expr := jsonata.MustCompile(`$sum(payload.**.value)`)
	if err := expr.RegisterExts(map[string]jsonata.Extension{
		"sum": {Func: func(any) int { return 99 }},
	}); err != nil {
		t.Fatal(err)
	}
	registered, evalErr := expr.Eval(map[string]any{"payload": map[string]any{"value": 1}})
	if evalErr != nil || !reflect.DeepEqual(registered, 99) {
		t.Fatalf("registered sum override returned %#v, %v; want 99", registered, evalErr)
	}
	input := map[string]any{"payload": map[string]any{
		"a": map[string]any{"value": 3},
		"b": map[string]any{"nested": map[string]any{"value": 4}},
	}}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, ok := evaluator.EvalStaticDescendantSum(plan, input)
				if !ok || got != float64(7) {
					t.Errorf("sum = %#v, ok=%v", got, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func assertStaticDescendantSum(t *testing.T, expression string, input any, want float64) {
	t.Helper()
	node, err := syntax.Parse(expression)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticDescendantSumPlan(node)
	fast, ok := evaluator.EvalStaticDescendantSum(plan, input)
	if !ok || !reflect.DeepEqual(fast, want) {
		t.Fatalf("fast = %#v, ok=%v, want %#v", fast, ok, want)
	}
	full, fullErr := jsonata.MustCompile(expression).EvalWithOptions(input, jsonata.EvalOptions{MaxOperations: 100_000})
	if fullErr != nil || !reflect.DeepEqual(full, fast) {
		t.Fatalf("full = %#v, %v; fast = %#v", full, fullErr, fast)
	}
}

func BenchmarkStaticDescendantSumPlan(b *testing.B) {
	node, err := syntax.Parse(`$sum(payload.**.value)`)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticDescendantSumPlan(node)
	input := map[string]any{"payload": map[string]any{
		"left": map[string]any{"value": 1, "child": map[string]any{"value": 2}},
		"right": map[string]any{"value": 3, "children": []any{
			map[string]any{"value": 4},
			map[string]any{"value": 5},
		}},
	}}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := evaluator.EvalStaticDescendantSum(plan, input); !ok {
			b.Fatal("descendant sum plan declined benchmark input")
		}
	}
}
