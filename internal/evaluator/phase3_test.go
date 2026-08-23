package evaluator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func evalPhase3(t *testing.T, expression string) (any, error) {
	t.Helper()
	n, parseErr := syntax.Parse(expression)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	return Eval(n, nil)
}

func TestPhase3LexicalBindingsAndClosures(t *testing.T) {
	tests := []struct {
		expression string
		want       any
	}{
		{expression: `( $a := 5; $a := $a + 2; $a )`, want: 7.0},
		{expression: `( $a := $b := 5; $b )`, want: 5.0},
		{expression: `($factorial:= function($x){$x <= 1 ? 1 : $x * $factorial($x-1)}; $factorial(4))`, want: 24.0},
		{expression: `($twice:=function($f){function($x){$f($f($x))}}; $add3:=function($y){$y+3}; $add6:=$twice($add3); $add6(7))`, want: 13.0},
		{expression: `($range := function($start, $end, $step) { ($step:=($step?$step:1); $start+$step > $end ? $start : $append($start, $range($start+$step, $end, $step))) }; $range(0,15,2))`, want: []any{0.0, 2.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0}},
		{expression: `($make := function($x){function(){($x := $x + 1; $x)}}; $f := $make(0); [$f(), $f()])`, want: []any{1.0, 1.0}},
	}
	for _, test := range tests {
		got, err := evalPhase3(t, test.expression)
		if err != nil {
			t.Fatalf("%s: %v", test.expression, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %#v, want %#v", test.expression, got, test.want)
		}
	}
}

func TestPhase3ClosureCapturesDefinitionContext(t *testing.T) {
	node, parseErr := syntax.Parse(`($f := function(){$.name}; child.($f()))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	input := map[string]any{
		"name":  "root",
		"child": map[string]any{"name": "child"},
	}
	got, err := Eval(node, input)
	if err != nil {
		t.Fatal(err)
	}
	if got != "root" {
		t.Fatalf("closure context = %#v, want root", got)
	}
}

func TestBuiltinSignaturesAcceptTransientNumericSequences(t *testing.T) {
	for expression, want := range map[string]float64{
		"$sum($seq)":     6,
		"$average($seq)": 2,
		"$max($seq)":     3,
	} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		got, err := EvalBindings(node, nil, map[string]any{"seq": value.Sequence{1.0, 2.0, 3.0}})
		if err != nil {
			t.Fatalf("%s: %v", expression, err)
		}
		if got != want {
			t.Fatalf("%s: got %#v, want %#v", expression, got, want)
		}
	}
}

func TestPhase3NilBindingAndSiblingIsolation(t *testing.T) {
	n, parseErr := syntax.Parse(`$a`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	got, err := EvalBindings(n, nil, map[string]any{"a": nil})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("null binding got %#v", got)
	}
	got, err = EvalBindings(n, nil, map[string]any{"a": 2.0, "$a": 1.0})
	if err != nil || got != 1.0 {
		t.Fatalf("exact binding precedence: got %#v, err %v", got, err)
	}

	t.Run("outer binding survives nested scope", func(t *testing.T) {
		got, err := evalPhase3(t, `($foo := "defined"; ($foo := nothing); $foo)`)
		if err != nil {
			t.Fatal(err)
		}
		if got != "defined" {
			t.Fatalf("nested sibling scope: got %#v", got)
		}
	})
	t.Run("inner scope remains undefined", func(t *testing.T) {
		got, err := evalPhase3(t, `($foo := "defined"; ($foo := nothing; $foo))`)
		if !IsUndefined(err) {
			t.Fatalf("nested undefined scope: got %#v, err %v", got, err)
		}
	})
}

func TestPhase3CallableErrorsAndComposition(t *testing.T) {
	got, err := evalPhase3(t, `($add := function($x, $y){$x + $y}; $add2 := $add(?, 2); $add2(3))`)
	if err != nil || !reflect.DeepEqual(got, 5.0) {
		t.Fatalf("partial application: got %#v, err %v", got, err)
	}
	got, err = evalPhase3(t, `($add := function($x, $y){$x + $y}; $f := $add(?, ?); $f(1)(2))`)
	if err != nil || !reflect.DeepEqual(got, 3.0) {
		t.Fatalf("staged partial application: got %#v, err %v", got, err)
	}
	_, err = evalPhase3(t, `($add := function($x, $y){$x + $y}; $f := $add(?, ?); $f(1, 2, 3))`)
	if err == nil {
		t.Fatal("extra partial arguments were accepted")
	}
	if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("extra partial arguments error code: %v", err)
	}
	got, err = evalPhase3(t, `($inc := function($x){$x + 1}; $double := function($x){$x * 2}; 3 ~> $inc ~> $double)`)
	if err != nil || !reflect.DeepEqual(got, 8.0) {
		t.Fatalf("function application: got %#v, err %v", got, err)
	}
	got, err = evalPhase3(t, `($inc := function($x){$x + 1}; 3 ~> $inc() = 4)`)
	if err != nil || got != true {
		t.Fatalf("application/equality associativity: got %#v, err %v", got, err)
	}

	got, err = evalPhase3(t, `$add := function($x, $y){$x + $y}`)
	if err == nil || got != nil {
		t.Fatalf("function result: got %#v, err %v", got, err)
	}
	if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T1007" {
		t.Fatalf("function result error code: %v", err)
	}

	got, err = evalPhase3(t, `42 ~> "hello"`)
	if err == nil || got != nil {
		t.Fatalf("invalid application: got %#v, err %v", got, err)
	}
	if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T2006" {
		t.Fatalf("invalid application error code: %v", err)
	}
	for _, test := range []struct {
		expression string
		code       string
	}{
		{expression: `substring(?, 0, ?)`, code: "T1007"},
		{expression: `uppercase(?)`, code: "T1007"},
		{expression: `unknown(?)`, code: "T1008"},
	} {
		_, err = evalPhase3(t, test.expression)
		coded, ok := err.(interface{ JSONataCode() string })
		if !ok || coded.JSONataCode() != test.code {
			t.Fatalf("%s: got %v, want %s", test.expression, err, test.code)
		}
	}

	n, parseErr := syntax.Parse(`$f(1)`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	_, err = EvalBindings(n, nil, map[string]any{"f": map[string]any{"invoke": true}})
	if err == nil {
		t.Fatal("map was accepted as a callable")
	}
	if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T1008" {
		t.Fatalf("map callable error code: %v", err)
	}
	for _, expression := range []string{`[function($x){$x}]`, `{'f': function($x){$x}}`} {
		_, err = evalPhase3(t, expression)
		if err == nil {
			t.Fatalf("nested callable was accepted: %s", expression)
		}
		if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T1007" {
			t.Fatalf("nested callable error code for %s: %v", expression, err)
		}
	}
}

func TestPhase3TransformPreservesLexicalEnvironment(t *testing.T) {
	got, err := evalPhase3(t, `($value := 2; {"a": 1} ~> |$|{"b": $value}|)`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a": 1.0, "b": 2.0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	n, parseErr := syntax.Parse(`{"a": 1} ~> |$|{"b": 2}|`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(n, nil, Options{MaxOperations: 4}); err == nil {
		t.Fatal("transform update escaped the operation budget")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "U1001" {
		t.Fatalf("transform operation budget error: %v", err)
	}
}

func TestPhase3ConcurrentEvaluation(t *testing.T) {
	const expression = `($factor := 3; function($x){$x * $factor}(7))`
	n, parseErr := syntax.Parse(expression)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Eval(n, nil)
			if err == nil && !reflect.DeepEqual(got, 21.0) {
				errs <- fmt.Errorf("got unexpected result %#v", got)
				return
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPhase3RuntimeOptionsAndDynamicEvaluation(t *testing.T) {
	n, parseErr := syntax.Parse(`($factorial := function($n){$n = 0 ? 1 : $n * $factorial($n - 1)}; $factorial(5))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(n, nil, Options{MaxCallDepth: 3}); err == nil {
		t.Fatal("call depth limit was not enforced")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "D1011" {
		t.Fatalf("call depth error: %v", err)
	}

	infinite, parseErr := syntax.Parse(`($f := function(){$f()}; $f())`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(infinite, nil, Options{MaxOperations: 100}); err == nil {
		t.Fatal("operation budget was not enforced")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "U1001" {
		t.Fatalf("operation budget error: %v", err)
	}

	if _, err := EvalWithOptions(infinite, nil, Options{Deadline: time.Now().Add(-time.Second)}); err == nil {
		t.Fatal("expired deadline was not enforced")
	} else {
		code, ok := err.(interface{ JSONataCode() string })
		if !ok || code.JSONataCode() != "U1001" {
			t.Fatalf("deadline error: %v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline error does not unwrap context.DeadlineExceeded: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvalWithOptions(infinite, nil, Options{Context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error: %v", err)
	}

	dynamic, parseErr := syntax.Parse(`($x := 5; $eval('$x'))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if got, err := Eval(dynamic, nil); err != nil || got != 5.0 {
		t.Fatalf("dynamic lexical environment: got %#v, err %v", got, err)
	}

	budgetedDynamic, parseErr := syntax.Parse(`($f := function(){$f()}; $eval('$f()'))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(budgetedDynamic, nil, Options{MaxOperations: 100}); err == nil {
		t.Fatal("dynamic evaluation escaped the parent operation budget")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "U1001" {
		t.Fatalf("dynamic operation budget error: %v", err)
	}

	for _, test := range []struct {
		expression string
		code       string
	}{
		{expression: `$eval('[1,string(2),3]')`, code: "D3121"},
		{expression: `$eval('[1,#string(2),3]')`, code: "D3120"},
	} {
		n, parseErr := syntax.Parse(test.expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		_, err := Eval(n, nil)
		if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != test.code {
			t.Fatalf("%s: error %v, want %s", test.expression, err, test.code)
		}
	}
}

func TestPhase3OperationBudgetBoundary(t *testing.T) {
	runtime := newEvalRuntime(Options{MaxOperations: 1})
	if err := runtime.check(); err != nil {
		t.Fatalf("first operation used the budget: %v", err)
	}
	if err := runtime.check(); err == nil {
		t.Fatal("budget allowed one operation too many")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "U1001" {
		t.Fatalf("budget boundary error: %v", err)
	}
}

func TestPhase3TailMutualRecursionAndSignature(t *testing.T) {
	n, parseErr := syntax.Parse(`($even := function($n){$n = 0 ? true : $odd($n - 1)}; $odd := function($n){$n = 0 ? false : $even($n - 1)}; $odd(6555))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(n, nil, Options{MaxCallDepth: 2, MaxOperations: 1000000}); !hasJSONataCode(err, "D1011") {
		t.Fatalf("tail mutual recursion stack error: %v, want D1011", err)
	}
	if got, err := EvalWithOptions(n, nil, Options{MaxCallDepth: 16, MaxOperations: 1000000}); err != nil || got != true {
		t.Fatalf("tail mutual recursion: got %#v, err %v", got, err)
	}

	signed, parseErr := syntax.Parse(`function($s, $n)<sn:s>{$s}(1, 2)`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := Eval(signed, nil); err == nil {
		t.Fatal("signature type mismatch was accepted")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("signature mismatch error: %v", err)
	}
}

func TestPhase3TailCoalescing(t *testing.T) {
	n, parseErr := syntax.Parse(`($f := function($n){$n = 0 ? true : ($missing ?? $f($n - 1))}; $f(6555))`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := EvalWithOptions(n, nil, Options{MaxCallDepth: 2, MaxOperations: 1000000}); !hasJSONataCode(err, "D1011") {
		t.Fatalf("tail coalescing stack error: %v, want D1011", err)
	}
	if got, err := EvalWithOptions(n, nil, Options{MaxCallDepth: 16, MaxOperations: 1000000}); err != nil || got != true {
		t.Fatalf("tail coalescing recursion: got %#v, err %v", got, err)
	}
}

func TestPhase3FunctionArityCodes(t *testing.T) {
	if _, err := evalPhase3(t, `function($x){$x}()`); !IsUndefined(err) {
		t.Fatalf("untyped missing argument: got %v", err)
	}
	for _, test := range []struct {
		expression string
		code       string
	}{
		{expression: `function($x){$x}(1, 2)`, code: "T0410"},
		{expression: `function($x)<n:n>{$x}()`, code: "T0411"},
		{expression: `function($x)<n:n>{$x}(1, 2)`, code: "T0410"},
	} {
		n, parseErr := syntax.Parse(test.expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		_, err := Eval(n, nil)
		code, ok := err.(interface{ JSONataCode() string })
		if !ok || code.JSONataCode() != test.code {
			t.Fatalf("%s: got %v, want %s", test.expression, err, test.code)
		}
	}
}

func TestPhase3BuiltinMissingArgumentCodes(t *testing.T) {
	data := map[string]any{"value": 1.0}

	n, err := syntax.Parse(`$sum()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Eval(n, data); err == nil {
		t.Fatal("missing required builtin argument was accepted")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("missing required builtin argument error: %v, want T0410", err)
	}

	n, err = syntax.Parse(`$length()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Eval(n, data); err == nil {
		t.Fatal("missing context-default builtin argument was accepted")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0411" {
		t.Fatalf("missing context-default builtin argument error: %v, want T0411", err)
	}

	for _, expression := range []string{
		`$substringBefore("Cocaca")`,
		`$substringAfter("Cocaca")`,
	} {
		n, err = syntax.Parse(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Eval(n, data); err == nil {
			t.Fatalf("%s accepted a missing context argument", expression)
		} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0411" {
			t.Fatalf("%s error: %v, want T0411", expression, err)
		}
	}

	n, err = syntax.Parse(`$substringBefore("a", "b", "c")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Eval(n, data); err == nil {
		t.Fatal("excess builtin argument was accepted")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("excess builtin argument error: %v, want T0410", err)
	}
}

func TestPhase4SignatureArgumentSlotsAndKinds(t *testing.T) {
	if got, err := evalPhase3(t, `function($text, $number)<s?n:n>{$number}(5)`); err != nil || got != 5.0 {
		t.Fatalf("skipped optional slot: got %#v, err %v", got, err)
	}
	if _, err := evalPhase3(t, `function($arr, $number)<a<n>n:n>{$number}([1], "x")`); err == nil {
		t.Fatal("mixed signature accepted a failing numeric parameter")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("mixed signature error: %v", err)
	}
	if _, err := evalPhase3(t, `function($value)<j:n>{1}(function($x){$x})`); err == nil {
		t.Fatal("JSON signature accepted a callable")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("JSON signature error: %v", err)
	}
	if got, err := evalPhase3(t, `function($value)<x:n>{1}(function($x){$x})`); err != nil || got != 1.0 {
		t.Fatalf("any signature rejected a callable: got %#v, err %v", got, err)
	}
	if _, err := evalPhase3(t, `function($value)<z:n>{$value}(1)`); err == nil {
		t.Fatal("invalid signature kind was accepted")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("invalid signature kind error: %v", err)
	}
}

func TestPartialCompositionPreservesBuiltinArgumentSlots(t *testing.T) {
	for _, test := range []struct {
		expression string
		want       any
	}{
		{`($domain := $substringAfter(?, "@") ~> $substringBefore(?, "."); $domain("john@example.com"))`, "example"},
		{`($betweenBackets := $substringAfter(?, "(") ~> $substringBefore(?, ")"); $betweenBackets("test(foo)bar"))`, "foo"},
	} {
		got, err := evalPhase3(t, test.expression)
		if err != nil || got != test.want {
			t.Errorf("%s: got %#v, err %v; want %#v", test.expression, got, err, test.want)
		}
	}
}

func TestPhase4SignatureValidationUsesRuntimeBudget(t *testing.T) {
	signature, err := parseFunctionSignature(`<a<a<a<a<n>>>>:n>`)
	if err != nil {
		t.Fatal(err)
	}
	nested := value.Array{Items: []any{value.Array{Items: []any{value.Array{Items: []any{value.Array{Items: []any{1.0}}}}}}}}
	_, err = prepareSignatureArgs(signature, []any{nested}, nil, newEvalRuntime(Options{MaxOperations: 3}))
	if err == nil {
		t.Fatal("nested signature validation escaped the operation budget")
	}
	if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "U1001" {
		t.Fatalf("signature budget error: %v", err)
	}
}
