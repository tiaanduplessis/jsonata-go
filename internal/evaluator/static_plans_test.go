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

func TestStaticComparisonPlanMatchesFullEvaluation(t *testing.T) {
	cases := []struct {
		expression string
		input      any
		want       any
	}{
		{"customer.profile.tier = \"gold\"", map[string]any{"customer": map[string]any{"profile": map[string]any{"tier": "gold"}}}, true},
		{"customer.profile.tier != \"gold\"", map[string]any{"customer": map[string]any{"profile": map[string]any{"tier": "silver"}}}, true},
		{"amount = 7", map[string]any{"amount": int64(7)}, true},
		{"active = true", map[string]any{"active": true}, true},
	}
	for _, test := range cases {
		n, err := syntax.Parse(test.expression)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expression, err)
		}
		plan := evaluator.BuildStaticComparisonPlan(n)
		if plan == nil {
			t.Fatalf("BuildStaticComparisonPlan(%q) returned nil", test.expression)
		}
		got, ok := evaluator.EvalStaticComparison(plan, test.input)
		if !ok || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("fast %q = %#v, ok=%v; want %#v", test.expression, got, ok, test.want)
		}
		expr := jsonata.MustCompile(test.expression)
		full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: nil, MaxOperations: 100_000})
		if fullErr != nil || !reflect.DeepEqual(full, got) {
			t.Fatalf("full %q = %#v, %v; fast = %#v", test.expression, full, fullErr, got)
		}
	}
}

func TestStaticComparisonPlanFallsBackForAmbiguousValues(t *testing.T) {
	n, err := syntax.Parse(`a = 1`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticComparisonPlan(n)
	if plan == nil {
		t.Fatal("comparison plan was not built")
	}
	inputs := []any{
		map[string]any{},
		map[string]any{"a": nil},
		map[string]any{"a": []any{1}},
		map[string]any{"a": map[string]any{"n": 1}},
		map[string]any{"a": true},
		map[string]any{"a": math.NaN()},
	}
	for _, input := range inputs {
		if got, ok := evaluator.EvalStaticComparison(plan, input); ok {
			t.Fatalf("ambiguous input %#v unexpectedly planned result %#v", input, got)
		}
	}
	cycle := map[string]any{}
	cycle["loop"] = cycle
	cycle["a"] = 1
	if _, ok := evaluator.EvalStaticComparison(plan, cycle); ok {
		t.Fatal("cyclic input unexpectedly used the comparison plan")
	}
	oversized := make(map[string]any, 100_001)
	oversized["a"] = 1
	for i := 0; i < 100_000; i++ {
		oversized["unrelated"+strconv.Itoa(i)] = i
	}
	if _, ok := evaluator.EvalStaticComparison(plan, oversized); ok {
		t.Fatal("oversized input unexpectedly used the comparison plan")
	}
}

func TestStaticFilterProjectPlanMatchesFullEvaluation(t *testing.T) {
	n, err := syntax.Parse(`orders[status="paid"].amount`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticFilterProjectPlan(n)
	if plan == nil {
		t.Fatal("filter/project plan was not built")
	}
	inputs := []any{
		map[string]any{"orders": []any{
			map[string]any{"status": "paid", "amount": 12},
		}},
		map[string]any{"orders": []any{
			map[string]any{"status": "paid", "amount": 12},
			map[string]any{"status": "pending", "amount": 9},
			map[string]any{"status": "paid", "amount": 4},
		}},
	}
	for _, input := range inputs {
		fast, ok := evaluator.EvalStaticFilterProject(plan, input)
		if !ok {
			t.Fatalf("filter/project did not plan input %#v", input)
		}
		expr := jsonata.MustCompile(`orders[status="paid"].amount`)
		full, err := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: nil, MaxOperations: 100_000})
		if err != nil || !reflect.DeepEqual(fast, full) {
			t.Fatalf("input %#v: fast=%#v full=%#v err=%v", input, fast, full, err)
		}
	}
}

func TestStaticFilterProjectPlanFallsBackForAmbiguity(t *testing.T) {
	n, err := syntax.Parse(`orders[status="paid"].amount`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticFilterProjectPlan(n)
	if plan == nil {
		t.Fatal("filter/project plan was not built")
	}
	inputs := []any{
		map[string]any{},
		map[string]any{"orders": nil},
		map[string]any{"orders": map[string]any{"status": "paid", "amount": 1}},
		map[string]any{"orders": []any{map[string]any{"status": "paid"}}},
		map[string]any{"orders": []any{map[string]any{"status": nil, "amount": 1}}},
		map[string]any{"orders": []any{map[string]any{"status": 1, "amount": 1}}},
		map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": []any{1}}}},
		map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": 1}, []any{1}}},
		map[string]any{"orders": []any{map[string]any{"status": math.NaN(), "amount": 1}}},
	}
	for _, input := range inputs {
		if got, ok := evaluator.EvalStaticFilterProject(plan, input); ok {
			t.Fatalf("ambiguous input %#v unexpectedly planned result %#v", input, got)
		}
	}
	cycle := map[string]any{}
	cycle["loop"] = cycle
	cycle["orders"] = []any{map[string]any{"status": "paid", "amount": 1}}
	if _, ok := evaluator.EvalStaticFilterProject(plan, cycle); ok {
		t.Fatal("cyclic input unexpectedly used the filter/project plan")
	}
	oversized := make(map[string]any, 100_001)
	oversized["orders"] = []any{map[string]any{"status": "paid", "amount": 1}}
	for i := 0; i < 100_000; i++ {
		oversized["unrelated"+strconv.Itoa(i)] = i
	}
	if _, ok := evaluator.EvalStaticFilterProject(plan, oversized); ok {
		t.Fatal("oversized input unexpectedly used the filter/project plan")
	}
}

func TestStaticPlansRegistryAndConcurrentUse(t *testing.T) {
	comparisonNode, err := syntax.Parse(`a = 1`)
	if err != nil {
		t.Fatal(err)
	}
	comparison := evaluator.BuildStaticComparisonPlan(comparisonNode)
	filterNode, err := syntax.Parse(`orders[status="paid"].amount`)
	if err != nil {
		t.Fatal(err)
	}
	filter := evaluator.BuildStaticFilterProjectPlan(filterNode)
	for _, test := range []struct {
		name string
		plan interface{ RegistryConflict(map[string]any) bool }
	}{
		{"comparison", comparison},
		{"filter", filter},
	} {
		if test.plan.RegistryConflict(map[string]any{"unrelated": func() {}}) {
			t.Fatalf("%s was disabled by an unrelated registry entry", test.name)
		}
	}
	if !comparison.RegistryConflict(map[string]any{"a": func() {}}) || !comparison.RegistryConflict(map[string]any{"$a": func() {}}) {
		t.Fatal("comparison shadowing was not detected")
	}
	if !filter.RegistryConflict(map[string]any{"status": func() {}}) || !filter.RegistryConflict(map[string]any{"amount": func() {}}) {
		t.Fatal("filter/project shadowing was not detected")
	}

	input := map[string]any{
		"a": 1,
		"orders": []any{
			map[string]any{"status": "paid", "amount": 3},
			map[string]any{"status": "pending", "amount": 4},
		},
	}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if got, ok := evaluator.EvalStaticComparison(comparison, input); !ok || got != true {
					t.Errorf("comparison = %#v, ok=%v", got, ok)
					return
				}
				if got, ok := evaluator.EvalStaticFilterProject(filter, input); !ok || !reflect.DeepEqual(got, 3) {
					t.Errorf("filter/project = %#v, ok=%v", got, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}
