package evaluator_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

const staticExtensionArithmeticExpression = `$double(value) + $double(offset)`

func TestStaticExtensionArithmeticPlanIsASTDerived(t *testing.T) {
	n, err := syntax.Parse(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	if evaluator.BuildStaticExtensionArithmeticPlan(n) == nil {
		t.Fatal("extension arithmetic plan was not built")
	}
	for _, expression := range []string{
		`$double(value) - $double(offset)`,
		`$double(value) + $double(1)`,
		`$double(value) + other(offset)`,
		`$double(value, offset) + $double(offset)`,
		`double(value) + $double(offset)`,
	} {
		n, parseErr := syntax.Parse(expression)
		if parseErr == nil && evaluator.BuildStaticExtensionArithmeticPlan(n) != nil {
			t.Fatalf("ambiguous expression was planned: %s", expression)
		}
	}
}

func TestStaticExtensionArithmeticMatchesForcedFullEvaluation(t *testing.T) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{
		"double": {Func: func(number float64) float64 { return number * 2 }},
	}); err != nil {
		t.Fatal(err)
	}
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"value": 21, "offset": 1.5}
	fast, fastErr := expr.Eval(input)
	full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
	if fastErr != nil || fullErr != nil || !reflect.DeepEqual(fast, full) || fast != float64(45) {
		t.Fatalf("fast=(%#v,%v) full=(%#v,%v)", fast, fastErr, full, fullErr)
	}
}

func TestStaticExtensionArithmeticPrimitiveDispatchMatchesFullEvaluation(t *testing.T) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{
		"double": {Func: func(number float64) float64 { return number * 2 }},
	}); err != nil {
		t.Fatal(err)
	}
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []any{
		int(21), int8(21), int16(21), int32(21), int64(21),
		uint(21), uint8(21), uint16(21), uint32(21), uint64(21),
		float32(21), float64(21),
	} {
		t.Run(fmt.Sprintf("%T", input), func(t *testing.T) {
			data := map[string]any{"value": input, "offset": 1.5}
			fast, fastErr := expr.Eval(data)
			full, fullErr := expr.EvalWithOptions(data, jsonata.EvalOptions{Context: context.Background()})
			if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) || fastErrString(fastErr) != fastErrString(fullErr) {
				t.Fatalf("input=%#v: fast=(%#v,%v), full=(%#v,%v)", input, fast, fastErr, full, fullErr)
			}
		})
	}

	for _, nonFinite := range []struct {
		name  string
		value float64
	}{
		{name: "nan", value: math.NaN()},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "negative infinity", value: math.Inf(-1)},
	} {
		t.Run(nonFinite.name, func(t *testing.T) {
			finite := jsonata.MustCompile(staticExtensionArithmeticExpression)
			if err := finite.RegisterExts(map[string]jsonata.Extension{
				"double": {Func: func(number float64) float64 { return nonFinite.value }},
			}); err != nil {
				t.Fatal(err)
			}
			input := map[string]any{"value": 1, "offset": 2}
			fast, fastErr := finite.Eval(input)
			full, fullErr := finite.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
			if fast != nil || full != nil || fastErrString(fastErr) != fastErrString(fullErr) ||
				(errors.Unwrap(fastErr) == nil) != (errors.Unwrap(fullErr) == nil) {
				t.Fatalf("non-finite result: fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
			}
		})
	}
}

func TestStaticExtensionArithmeticFastErrorsAndMixedSignaturesMatchFull(t *testing.T) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{
		"identity": {Func: func(number float64) float64 { return number }},
	}); err != nil {
		t.Fatal(err)
	}
	expr, err := engine.Compile(`$identity(value) + $identity(offset)`)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"value": math.MaxFloat64, "offset": math.MaxFloat64}
	fast, fastErr := expr.Eval(input)
	full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
	if fast != nil || full != nil || fastErrString(fastErr) != fastErrString(fullErr) {
		t.Fatalf("overflow: fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
	}

	leftCalls := 0
	rightCalls := 0
	mixed := jsonata.NewEngine()
	if err := mixed.RegisterExts(map[string]jsonata.Extension{
		"identity":    {Func: func(number float64) float64 { leftCalls++; return number }},
		"passthrough": {Func: func(number any) any { rightCalls++; return number }},
	}); err != nil {
		t.Fatal(err)
	}
	mixedExpr, err := mixed.Compile(`$identity(value) + $passthrough(offset)`)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"value": 2, "offset": 3}
	fast, fastErr = mixedExpr.Eval(data)
	if fastErr != nil || fast != float64(5) || leftCalls != 1 || rightCalls != 1 {
		t.Fatalf("mixed fast=(%#v,%v), calls=(%d,%d); want 5 and one call each", fast, fastErr, leftCalls, rightCalls)
	}
	leftCalls, rightCalls = 0, 0
	full, fullErr = mixedExpr.EvalWithOptions(data, jsonata.EvalOptions{Context: context.Background()})
	if fullErr != nil || full != float64(5) || leftCalls != 1 || rightCalls != 1 {
		t.Fatalf("mixed full=(%#v,%v), calls=(%d,%d); want 5 and one call each", full, fullErr, leftCalls, rightCalls)
	}
}

func fastErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestStaticExtensionArithmeticPreservesErrorsAndFallbacks(t *testing.T) {
	tests := []struct {
		name  string
		fn    any
		input any
		want  string
	}{
		{name: "typed argument", fn: func(number float64) float64 { return number * 2 }, input: map[string]any{"value": "not-a-number", "offset": 1}, want: "argument 1 of function \"double\" does not match function signature"},
		{name: "returned error", fn: func(float64) (float64, error) { return 0, errors.New("extension failed") }, input: map[string]any{"value": 1, "offset": 2}, want: "extension failed"},
		{name: "panic", fn: func(float64) float64 { panic("extension panic") }, input: map[string]any{"value": 1, "offset": 2}, want: "extension \"double\" panicked: extension panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := jsonata.NewEngine()
			if err := engine.RegisterExts(map[string]jsonata.Extension{"double": {Func: test.fn}}); err != nil {
				t.Fatal(err)
			}
			expr, err := engine.Compile(staticExtensionArithmeticExpression)
			if err != nil {
				t.Fatal(err)
			}
			fast, fastErr := expr.Eval(test.input)
			full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: context.Background()})
			if fast != nil || full != nil || fastErr == nil || fullErr == nil || fastErr.Error() != fullErr.Error() || !strings.Contains(fastErr.Error(), test.want) {
				t.Fatalf("fast=(%#v,%v) full=(%#v,%v), want error containing %q", fast, fastErr, full, fullErr, test.want)
			}
		})
	}

	engine := jsonata.NewEngine()
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(map[string]any{"value": 1, "offset": 2}); err == nil {
		t.Fatal("missing extension unexpectedly succeeded")
	}
	if _, err := expr.Eval(map[string]any{"value": 1}); err == nil {
		t.Fatal("missing argument unexpectedly succeeded")
	}
}

func TestStaticExtensionArithmeticUsesCurrentRegistryAndHonorsOptions(t *testing.T) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{"double": {Func: func(float64) float64 { return 2 }}}); err != nil {
		t.Fatal(err)
	}
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	if err := expr.RegisterExts(map[string]jsonata.Extension{"double": {Func: func(number float64) float64 { return number * 3 }}}); err != nil {
		t.Fatal(err)
	}
	if got, err := expr.Eval(map[string]any{"value": 2, "offset": 1}); err != nil || got != float64(9) {
		t.Fatalf("re-registered extension = %#v, %v; want 9", got, err)
	}
	if _, err := expr.EvalWithOptions(map[string]any{"value": 2, "offset": 1}, jsonata.EvalOptions{MaxOperations: 1}); err == nil {
		t.Fatal("operation budget was bypassed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expr.EvalWithOptions(map[string]any{"value": 2, "offset": 1}, jsonata.EvalOptions{Context: ctx}); err == nil {
		t.Fatal("canceled context was bypassed")
	}
}

func TestStaticExtensionArithmeticConcurrentRegistryAndEvaluation(t *testing.T) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{"double": {Func: func(number float64) float64 { return number * 2 }}}); err != nil {
		t.Fatal(err)
	}
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"value": 21, "offset": 1.5}
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, err := expr.Eval(input)
				if err != nil || got != float64(45) {
					t.Errorf("result = %#v, %v", got, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkStaticExtensionArithmeticDecoded(b *testing.B) {
	engine := jsonata.NewEngine()
	if err := engine.RegisterExts(map[string]jsonata.Extension{
		"double": {Func: func(number float64) float64 { return number * 2 }},
	}); err != nil {
		b.Fatal(err)
	}
	expr, err := engine.Compile(staticExtensionArithmeticExpression)
	if err != nil {
		b.Fatal(err)
	}
	input := map[string]any{"value": 21, "offset": 1.5}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if result, evalErr := expr.Eval(input); evalErr != nil || result != float64(45) {
			b.Fatalf("result = %#v, %v", result, evalErr)
		}
	}
}
