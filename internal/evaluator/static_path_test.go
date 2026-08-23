package evaluator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

const (
	staticPathTestMaxFields  = 64
	staticPathTestOperations = 100_000
)

func TestBuildStaticPathPlanIsConservative(t *testing.T) {
	tests := []struct {
		expr   string
		fields []string
	}{
		{expr: "a", fields: []string{"a"}},
		{expr: "a.b.c", fields: []string{"a", "b", "c"}},
		{expr: "$.a.b", fields: []string{"a", "b"}},
		{expr: "$$.a.b", fields: []string{"a", "b"}},
	}
	for _, test := range tests {
		n, err := syntax.Parse(test.expr)
		if err != nil {
			t.Fatal(err)
		}
		plan := evaluator.BuildStaticPathPlan(n)
		if plan == nil {
			t.Fatalf("%q did not produce a static path plan", test.expr)
		}
		if plan.RegistryConflict(map[string]any{"double": func() {}}) {
			t.Fatalf("unrelated registry binding disabled %q", test.expr)
		}
	}
	n, err := syntax.Parse("a.b")
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	if !plan.RegistryConflict(map[string]any{"a": func() {}}) || !plan.RegistryConflict(map[string]any{"$b": func() {}}) {
		t.Fatal("shadowing registry bindings did not disable the static path")
	}
	for _, expr := range []string{"a[0]", "a[]", "a.$x", "a.b()", "a.b[c]", "a.b.c?d:e"} {
		n, err := syntax.Parse(expr)
		if err != nil {
			continue
		}
		if plan := evaluator.BuildStaticPathPlan(n); plan != nil {
			t.Fatalf("dynamic expression %q unexpectedly planned: %#v", expr, plan)
		}
	}
	longPath, err := syntax.Parse("a." + strings.Repeat("b.", staticPathTestMaxFields) + "value")
	if err != nil {
		t.Fatal(err)
	}
	if plan := evaluator.BuildStaticPathPlan(longPath); plan != nil {
		t.Fatal("overly long static path unexpectedly planned")
	}
}

func TestStaticPathDecodedMatchesFullEvaluator(t *testing.T) {
	inputs := []any{
		map[string]any{"a": map[string]any{"b": float64(7)}},
		map[string]any{"a": map[string]any{"b": "line\nvalue"}},
		map[string]any{"a": map[string]any{"b": true}},
		map[string]any{"a": map[string]any{"b": nil}},
		map[string]any{"a": map[string]any{"b": map[string]any{"c": 1}}},
		map[string]any{"a": []any{map[string]any{"b": 1}}},
		[]any{map[string]any{"a": map[string]any{"b": 1}}},
		map[string]any{"a": map[string]any{}},
	}
	for _, exprText := range []string{"a.b", "$.a.b", "$$.a.b"} {
		expr := jsonata.MustCompile(exprText)
		for _, input := range inputs {
			fast, fastErr := expr.Eval(input)
			full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
			if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
				t.Fatalf("%s input %#v: fast=(%#v,%v), full=(%#v,%v)", exprText, input, fast, fastErr, full, fullErr)
			}
		}
	}
}

func TestStaticPathBytesRetainsFullEvaluatorSemantics(t *testing.T) {
	expr := jsonata.MustCompile("a.b")
	inputs := []string{
		`{"a":{"b":7}}`,
		`{"a":{"b":9007199254740993}}`,
		`{"a":{"b":"line\nvalue"}}`,
		`{"a":{"b":"\\u0061"}}`,
		`{"a":{"b":true}}`,
		`{"a":{"b":null}}`,
		`{"a":{"b":{"c":1}}}`,
		`{"a":{"b":[1]}}`,
		`{"a":[{"b":1}]}`,
		`[{"a":{"b":1}}]`,
		`{"a":{"b":1,"b":2}}`,
		`{"a.b":{"c":1}}`,
		`{"a":{"b":1`,
	}
	for _, input := range inputs {
		fast, fastErr := expr.EvalBytes([]byte(input))
		full, fullErr := expr.EvalBytesWithOptions([]byte(input), jsonata.EvalOptions{Context: context.Background()})
		if (fastErr == nil) != (fullErr == nil) {
			t.Fatalf("input %s: fast error=%v, full error=%v", input, fastErr, fullErr)
		}
		if fastErr == nil {
			var fastJSON, fullJSON any
			if err := json.Unmarshal(fast, &fastJSON); err != nil {
				t.Fatalf("fast output %s: %v", fast, err)
			}
			if err := json.Unmarshal(full, &fullJSON); err != nil {
				t.Fatalf("full output %s: %v", full, err)
			}
			if !reflect.DeepEqual(fastJSON, fullJSON) {
				t.Fatalf("input %s: fast=%s full=%s", input, fast, full)
			}
		}
	}
}

func TestStaticPathDoesNotBypassOptionsOrConcurrentSafety(t *testing.T) {
	expr := jsonata.MustCompile("a.b")
	for _, input := range []any{
		map[string]any{"a": map[string]any{"b": 1}, "bad": math.NaN()},
		func() any {
			cycle := map[string]any{}
			cycle["loop"] = cycle
			cycle["a"] = map[string]any{"b": 1}
			return cycle
		}(),
	} {
		fast, fastErr := expr.Eval(input)
		full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
		if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
			t.Fatalf("unsafe input fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expr.EvalWithOptions(map[string]any{"a": map[string]any{"b": 1}}, jsonata.EvalOptions{Context: ctx}); err == nil {
		t.Fatal("canceled context was bypassed")
	}
	if _, err := expr.EvalBytesWithOptions([]byte(`{"a":{"b":1}}`), jsonata.EvalOptions{MaxOperations: 1}); err == nil {
		t.Fatal("operation budget was bypassed")
	}

	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				decoded, err := expr.Eval(map[string]any{"a": map[string]any{"b": i}})
				if err != nil || decoded != i {
					t.Errorf("decoded result = %#v, %v", decoded, err)
					return
				}
				encoded, err := expr.EvalBytes([]byte(`{"a":{"b":1}}`))
				if err != nil || !bytes.Equal(encoded, []byte("1")) {
					t.Errorf("encoded result = %s, %v", encoded, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestStaticPathFallsBackBeforeOperationBudget(t *testing.T) {
	expr := jsonata.MustCompile("a.b")
	input := make(map[string]any, staticPathTestOperations+1)
	input["a"] = map[string]any{"b": 1}
	for i := 0; i <= staticPathTestOperations; i++ {
		input["unrelated"+strconv.Itoa(i)] = i
	}

	fast, fastErr := expr.Eval(input)
	full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
	if fast != nil || full != nil || fastErr == nil || fullErr == nil {
		t.Fatalf("oversized input fast=(%#v,%v), full=(%#v,%v); both should hit the operation budget", fast, fastErr, full, fullErr)
	}
	var fastJSON, fullJSON *jsonata.Error
	if !errors.As(fastErr, &fastJSON) || !errors.As(fullErr, &fullJSON) || fastJSON.Code != fullJSON.Code {
		t.Fatalf("oversized input errors fast=%v full=%v; expected matching structured diagnostics", fastErr, fullErr)
	}
}
