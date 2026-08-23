package security_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

func TestGHSA2943CraftedExpressionCannotReachHostCallables(t *testing.T) {
	const expression = `(
 $hasOwnProperty := $spread($string);
 $__proto__ := $constructor;
 $constructor("return process.getBuiltinModule('child_process').execSync('true')")()
)`
	_, err := jsonata.EvalWithOptions(expression, map[string]any{}, boundedOptions())
	assertCode(t, err, "T1006")
}

func TestGHSA86ToMillisAdversarialInputConsumesBudget(t *testing.T) {
	attack := "2024" + strings.Repeat("-01", 10_000) + "!"
	started := time.Now()
	_, err := jsonata.MustCompile(`$toMillis($input)`).EvalWithOptions(nil, jsonata.EvalOptions{
		Bindings:      map[string]any{"input": attack},
		MaxCallDepth:  32,
		MaxOperations: 500,
	})
	assertCode(t, err, "U1001")
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("adversarial date parse exceeded bounded latency: %s", elapsed)
	}
}

func TestPrototypeAndConstructorNamesRemainPlainData(t *testing.T) {
	input := map[string]any{
		"__proto__":   map[string]any{"safe": true},
		"constructor": "data",
		"prototype":   "data",
	}
	result, err := jsonata.Eval(`{"p":$lookup($,"__proto__"),"c":$lookup($,"constructor"),"t":$lookup($,"prototype")}`, input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"p": map[string]any{"safe": true}, "c": "data", "t": "data"}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("hostile keys = %#v, want %#v", result, want)
	}

	transformed, err := jsonata.Eval(`$ ~> |$|{"__proto__":{"safe":true},"constructor":"data"}|`, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	wantTransform := map[string]any{"__proto__": map[string]any{"safe": true}, "constructor": "data"}
	if !reflect.DeepEqual(transformed, wantTransform) {
		t.Fatalf("transform hostile keys = %#v, want %#v", transformed, wantTransform)
	}
}

func TestInternalFunctionFlagsCannotBeConstructed(t *testing.T) {
	for _, test := range []struct {
		expression string
		value      string
		position   int
	}{
		{expression: `{"_jsonata_function":true}`, value: "_jsonata_function", position: 1},
		{expression: `{"_jsonata_lambda":true}`, value: "_jsonata_lambda", position: 1},
		{expression: `($k := "_jsonata_function"; {$k:true})`, value: "_jsonata_function", position: 29},
	} {
		_, err := jsonata.Eval(test.expression, nil)
		assertCode(t, err, "D1013")
		var structured *jsonata.Error
		if !errors.As(err, &structured) || structured.Value != test.value || structured.Token != "" || structured.Position != test.position {
			t.Fatalf("reserved-key error = %#v, want value %q, empty token, position %d", structured, test.value, test.position)
		}
	}
	for _, key := range []string{"_jsonata_", "_jsonata_other", "_jsonata_functionx"} {
		expression := fmt.Sprintf(`$lookup({%q:true}, %q)`, key, key)
		result, err := jsonata.Eval(expression, nil)
		if err != nil || result != true {
			t.Fatalf("near-miss internal key %q = %#v, %v; want true", key, result, err)
		}
	}
}

func TestWildcardDoesNotTraverseFunctionInternals(t *testing.T) {
	result, err := jsonata.Eval(`$exists({"f":$sum}.*.*)`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Fatalf("wildcard traversed a function wrapper: %#v", result)
	}
}

func TestConcurrentRootIsEvaluationLocal(t *testing.T) {
	expression := jsonata.MustCompile(`$$.id & ":" & value`)
	var wait sync.WaitGroup
	for index := 0; index < 64; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			input := map[string]any{"id": fmt.Sprintf("root-%d", index), "value": index}
			want := fmt.Sprintf("root-%d:%d", index, index)
			for iteration := 0; iteration < 20; iteration++ {
				result, err := expression.Eval(input)
				if err != nil || result != want {
					t.Errorf("evaluation %d = %#v, %v; want %q", index, result, err, want)
					return
				}
			}
		}()
	}
	wait.Wait()
}

func TestResourceGuardrailsBoundSequenceRecursionAndCancellation(t *testing.T) {
	tests := []string{
		`[1..1000000]`,
		`($f := function($n){$f($n + 1)}; $f(0))`,
	}
	for _, expression := range tests {
		_, err := jsonata.EvalWithOptions(expression, nil, jsonata.EvalOptions{MaxCallDepth: 16, MaxOperations: 100})
		assertCode(t, err, "U1001")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := jsonata.EvalWithOptions(`$map([1..100], function($v){$v * 2})`, nil, jsonata.EvalOptions{Context: canceled})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled evaluation error = %v, want context.Canceled", err)
	}
}

func TestHostileCyclicInputReturnsBoundedError(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	_, err := jsonata.Eval(`$`, cycle)
	assertCode(t, err, "T0412")
}

func boundedOptions() jsonata.EvalOptions {
	return jsonata.EvalOptions{MaxCallDepth: 32, MaxOperations: 2_000, Timeout: time.Second}
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var structured *jsonata.Error
	if !errors.As(err, &structured) || structured.Code != code {
		t.Fatalf("error = %T %v, want %s", err, err, code)
	}
}
