package evaluator

import (
	"math"
	"strconv"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestStaticPathInputSafeDetectsCyclesAndAllowsAliases(t *testing.T) {
	cycleMap := map[string]any{}
	cycleMap["self"] = cycleMap
	if staticPathInputSafe(cycleMap, defaultMaxCallDepth) {
		t.Fatal("map cycle was accepted")
	}

	cycleSlice := make([]any, 1)
	cycleSlice[0] = cycleSlice
	if staticPathInputSafe(cycleSlice, defaultMaxCallDepth) {
		t.Fatal("slice cycle was accepted")
	}

	sharedMap := map[string]any{"value": 1}
	mapAliases := map[string]any{"left": sharedMap, "right": sharedMap}
	if !staticPathInputSafe(mapAliases, defaultMaxCallDepth) {
		t.Fatal("repeated map alias was rejected")
	}
	sharedSlice := []any{1, 2, 3}
	sliceAliases := map[string]any{"left": sharedSlice, "right": sharedSlice}
	if !staticPathInputSafe(sliceAliases, defaultMaxCallDepth) {
		t.Fatal("repeated slice alias was rejected")
	}
}

func TestStaticPathInputSafeHonorsDepthBound(t *testing.T) {
	if !staticPathInputSafe(1, 1) {
		t.Fatal("scalar at the depth limit was rejected")
	}
	if staticPathInputSafe(map[string]any{"value": 1}, 1) {
		t.Fatal("value beyond the depth limit was accepted")
	}
	if !staticPathInputSafe(nestedStaticPathInput(defaultMaxCallDepth), defaultMaxCallDepth) {
		t.Fatal("input at the default depth limit was rejected")
	}
	if staticPathInputSafe(nestedStaticPathInput(defaultMaxCallDepth+1), defaultMaxCallDepth) {
		t.Fatal("input beyond the default depth limit was accepted")
	}
}

func TestStaticPathSelectAndValidate(t *testing.T) {
	input := map[string]any{
		"a":   map[string]any{"b": "selected", "other": 1},
		"bad": math.NaN(),
	}
	_, _, valid := staticPathSelectAndValidate(input, []string{"a", "b"}, defaultMaxCallDepth)
	if valid {
		t.Fatal("invalid unrelated branch was accepted")
	}

	shared := map[string]any{"b": "selected"}
	input = map[string]any{"a": shared, "alias": shared}
	selected, found, valid := staticPathSelectAndValidate(input, []string{"a", "b"}, defaultMaxCallDepth)
	if !valid || !found || selected != "selected" {
		t.Fatalf("valid aliased input was not selected: selected=%#v found=%v valid=%v", selected, found, valid)
	}

	cycle := map[string]any{"a": map[string]any{"b": 1}}
	cycle["loop"] = cycle
	if _, _, valid = staticPathSelectAndValidate(cycle, []string{"a", "b"}, defaultMaxCallDepth); valid {
		t.Fatal("cyclic unrelated branch was accepted")
	}
}

func nestedStaticPathInput(scalarDepth int) any {
	var current any = 1
	for depth := scalarDepth - 1; depth > 0; depth-- {
		current = map[string]any{"next": current}
	}
	return current
}

func BenchmarkStaticPathInputSafeSimple(b *testing.B) {
	input := map[string]any{"customer": map[string]any{"profile": map[string]any{"tier": "gold"}}}
	b.ReportAllocs()
	for b.Loop() {
		if !staticPathInputSafe(input, defaultMaxCallDepth) {
			b.Fatal("simple input was rejected")
		}
	}
}

func BenchmarkStaticPathInputSafeDeep(b *testing.B) {
	input := nestedStaticPathInput(12)
	b.ReportAllocs()
	for b.Loop() {
		if !staticPathInputSafe(input, defaultMaxCallDepth) {
			b.Fatal("deep input was rejected")
		}
	}
}

func BenchmarkStaticPathInputSafeWide(b *testing.B) {
	input := make(map[string]any, 256)
	for index := 0; index < 256; index++ {
		input["field"+strconv.Itoa(index)] = index
	}
	b.ReportAllocs()
	for b.Loop() {
		if !staticPathInputSafe(input, defaultMaxCallDepth) {
			b.Fatal("wide input was rejected")
		}
	}
}

func BenchmarkStaticPathSelectAndValidateSimple(b *testing.B) {
	input := map[string]any{"customer": map[string]any{"profile": map[string]any{"tier": "gold"}}}
	fields := []string{"customer", "profile", "tier"}
	b.ReportAllocs()
	for b.Loop() {
		selected, found, valid := staticPathSelectAndValidate(input, fields, defaultMaxCallDepth)
		if !valid || !found || selected != "gold" {
			b.Fatal("simple input was not selected")
		}
	}
}

func BenchmarkStaticPathSeparateLookupAndValidationSimple(b *testing.B) {
	input := map[string]any{"customer": map[string]any{"profile": map[string]any{"tier": "gold"}}}
	fields := []string{"customer", "profile", "tier"}
	b.ReportAllocs()
	for b.Loop() {
		selected, found := staticValueAtPath(input, fields)
		if !found || !staticPathInputSafe(input, defaultMaxCallDepth) || selected != "gold" {
			b.Fatal("simple input was not selected")
		}
	}
}

func BenchmarkStaticPathSelectAndValidateDeep(b *testing.B) {
	const depth = 12
	input := nestedStaticPathInput(depth + 1)
	fields := make([]string, depth)
	for index := range fields {
		fields[index] = "next"
	}
	b.ReportAllocs()
	for b.Loop() {
		selected, found, valid := staticPathSelectAndValidate(input, fields, defaultMaxCallDepth)
		if !valid || !found || selected != 1 {
			b.Fatal("deep input was not selected")
		}
	}
}

func BenchmarkStaticPathSeparateLookupAndValidationDeep(b *testing.B) {
	const depth = 12
	input := nestedStaticPathInput(depth + 1)
	fields := make([]string, depth)
	for index := range fields {
		fields[index] = "next"
	}
	b.ReportAllocs()
	for b.Loop() {
		selected, found := staticValueAtPath(input, fields)
		if !found || !staticPathInputSafe(input, defaultMaxCallDepth) || selected != 1 {
			b.Fatal("deep input was not selected")
		}
	}
}

func BenchmarkStaticPathSelectAndValidateWide(b *testing.B) {
	input := make(map[string]any, 256)
	for index := 0; index < 256; index++ {
		input["field"+strconv.Itoa(index)] = index
	}
	fields := []string{"field0"}
	b.ReportAllocs()
	for b.Loop() {
		selected, found, valid := staticPathSelectAndValidate(input, fields, defaultMaxCallDepth)
		if !valid || !found || selected != 0 {
			b.Fatal("wide input was not selected")
		}
	}
}

func BenchmarkStaticPathSeparateLookupAndValidationWide(b *testing.B) {
	input := make(map[string]any, 256)
	for index := 0; index < 256; index++ {
		input["field"+strconv.Itoa(index)] = index
	}
	fields := []string{"field0"}
	b.ReportAllocs()
	for b.Loop() {
		selected, found := staticValueAtPath(input, fields)
		if !found || !staticPathInputSafe(input, defaultMaxCallDepth) || selected != 0 {
			b.Fatal("wide input was not selected")
		}
	}
}

func BenchmarkStaticPathAggregatePlanInputValidation(b *testing.B) {
	node, err := syntax.Parse(`$sum(orders[status="paid"].amount)`)
	if err != nil {
		b.Fatal(err)
	}
	plan := BuildStaticSumPlan(node)
	input := map[string]any{"orders": []any{
		map[string]any{"status": "paid", "amount": 12},
		map[string]any{"status": "pending", "amount": 9},
		map[string]any{"status": "paid", "amount": 4},
	}}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := EvalStaticSum(plan, input); !ok {
			b.Fatal("aggregate plan declined benchmark input")
		}
	}
}
