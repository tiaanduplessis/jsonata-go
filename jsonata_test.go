package jsonata

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestPhase1Examples(t *testing.T) {
	input := map[string]any{"customer": map[string]any{"name": "Ada"}}
	tests := []struct {
		expr string
		want any
	}{
		{"42", float64(42)},
		{`"hello"`, "hello"},
		{"true", true},
		{"null", nil},
		{"customer.name", "Ada"},
		{`[1, "two", false]`, []any{float64(1), "two", false}},
		{`{"name": customer.name, "ok": true}`, map[string]any{"name": "Ada", "ok": true}},
	}
	for _, tt := range tests {
		got, err := Eval(tt.expr, input)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Eval(%q) = %#v, %v; want %#v", tt.expr, got, err, tt.want)
		}
	}
}

func TestUndefinedDistinctFromNull(t *testing.T) {
	got, err := Eval("missing", map[string]any{"present": nil})
	if !errors.Is(err, ErrUndefined) || got != nil {
		t.Fatalf("missing field = %#v, %v; want ErrUndefined", got, err)
	}
	got, err = Eval("present", map[string]any{"present": nil})
	if err != nil || got != nil {
		t.Fatalf("null field = %#v, %v; want nil, nil", got, err)
	}
}

func TestUTF16LiteralPublicBoundary(t *testing.T) {
	for _, expression := range []string{
		`'\uD800'`,
		`['\uD800']`,
		`'prefix' & '\uD800'`,
		`$uppercase('\uD800')`,
		`$decodeUrl('\uD800')`,
		`{'\uD800': 1}`,
		`{'x': 1}.'\uD800'`,
	} {
		compiled, compileErr := Compile(expression)
		if compileErr != nil {
			t.Fatalf("Compile(%q): %v", expression, compileErr)
		}
		_, evalErr := compiled.Eval(nil)
		var jsonataErr *Error
		if !errors.As(evalErr, &jsonataErr) || jsonataErr.Code != "U1002" {
			t.Fatalf("Eval(%q) error = %v, want U1002", expression, evalErr)
		}
	}

	for _, expression := range []string{`$encodeUrl('\uD800')`, `$encodeUrlComponent('\uDFFF')`, `'\uD800' ~> $encodeUrl`} {
		compiled, compileErr := Compile(expression)
		if compileErr != nil {
			t.Fatalf("Compile(%q): %v", expression, compileErr)
		}
		_, evalErr := compiled.Eval(nil)
		var jsonataErr *Error
		if !errors.As(evalErr, &jsonataErr) || jsonataErr.Code != "D3140" {
			t.Fatalf("Eval(%q) error = %v, want D3140", expression, evalErr)
		}
	}

	got, err := Eval(`$encodeUrlComponent('�')`, nil)
	if err != nil || got != "%EF%BF%BD" {
		t.Fatalf("valid U+FFFD encoding = %#v, %v", got, err)
	}
}

func TestAbsentInputIsDistinctFromExplicitNull(t *testing.T) {
	expression := MustCompile("$")
	got, err := expression.Eval(nil)
	if err != nil || got != nil {
		t.Fatalf("explicit null input = %#v, %v; want nil, nil", got, err)
	}
	got, err = expression.EvalNoInput()
	if got != nil || !errors.Is(err, ErrUndefined) {
		t.Fatalf("absent input = %#v, %v; want ErrUndefined", got, err)
	}
	got, err = EvalNoInputWithOptions(`$x`, EvalOptions{Bindings: map[string]any{"x": "bound"}})
	if err != nil || got != "bound" {
		t.Fatalf("absent input with bindings = %#v, %v; want bound", got, err)
	}
	got, err = EvalNoInput("null")
	if err != nil || got != nil {
		t.Fatalf("null literal without input = %#v, %v; want nil, nil", got, err)
	}
	got, err = EvalNoInput(`[$round(4), $formatBase(100)]`)
	want := []any{float64(4), "100"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit arguments without input = %#v, %v; want %#v", got, err, want)
	}
}

func TestEvalBytesUseNumberAndRejectTrailingJSON(t *testing.T) {
	got, err := EvalBytes("$", []byte(`9007199254740993`))
	if err != nil || !bytes.Equal(got, []byte(`9007199254740992`)) {
		t.Fatalf("large number = %s, %v", got, err)
	}
	if _, err := EvalBytes("$", []byte(`1 2`)); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
}

func TestCompileErrorFields(t *testing.T) {
	_, err := Compile("[1,")
	var je *JSONataError
	if !errors.As(err, &je) {
		t.Fatalf("error = %T %v; want *JSONataError", err, err)
	}
	if je.Code == "" || je.Position < 0 {
		t.Fatalf("error fields = %#v", je)
	}
}

func TestEvalDoesNotExposeInputOwnership(t *testing.T) {
	input := map[string]any{"nested": map[string]any{"n": 1.0}}
	got, err := Eval("nested", input)
	if err != nil {
		t.Fatal(err)
	}
	got.(map[string]any)["n"] = 2.0
	if input["nested"].(map[string]any)["n"] != 1.0 {
		t.Fatal("result aliases caller input")
	}
}

func TestConcurrentEvaluation(t *testing.T) {
	e := MustCompile("item.value")
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			got, err := e.Eval(map[string]any{"item": map[string]any{"value": n}})
			if err != nil || got != n {
				t.Errorf("evaluation %d = %#v, %v", n, got, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestPhase2BindingsAndSortProjection(t *testing.T) {
	input := []any{3.0, 1.0, 4.0, 1.0, 5.0}
	got, err := Eval(`$#$pos[$pos<3]^(>$)`, input)
	if err != nil || !reflect.DeepEqual(got, []any{4.0, 3.0, 1.0}) {
		t.Fatalf("index binding and descending sort = %#v, %v", got, err)
	}

	data := map[string]any{
		"Employee": []any{map[string]any{"SSN": "1", "FirstName": "Ada"}},
		"Contact":  []any{map[string]any{"ssn": "1", "Phone": []any{map[string]any{"type": "mobile", "number": "1"}}}},
	}
	got, err = Eval(`Employee@$e.(Contact)[ssn=$e.SSN]{ $e.FirstName: Phone[type='mobile'].number }`, data)
	if err != nil || !reflect.DeepEqual(got, map[string]any{"Ada": "1"}) {
		t.Fatalf("join object projection = %#v, %v", got, err)
	}
}

func TestEvalWithOptionsBindingsAndDynamicContext(t *testing.T) {
	expr := MustCompile(`$eval('$x')`)
	got, err := expr.EvalWithOptions(nil, EvalOptions{Bindings: map[string]any{"x": 7.0}})
	if err != nil || got != 7.0 {
		t.Fatalf("dynamic binding = %#v, %v", got, err)
	}

	got, err = EvalWithOptions(`$eval('$', 9)`, nil, EvalOptions{})
	if err != nil || got != 9.0 {
		t.Fatalf("dynamic context = %#v, %v", got, err)
	}
}

func TestEvalWithOptionsMapsCodesAndLimits(t *testing.T) {
	codeExpr := MustCompile(`42 ~> "hello"`)
	_, err := codeExpr.EvalWithOptions(nil, EvalOptions{})
	var jsonataErr *Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "T2006" {
		t.Fatalf("coded error = %T %v, want public T2006", err, err)
	}

	recursive := MustCompile(`($f := function($n){$n = 0 ? 1 : $n * $f($n - 1)}; $f(4))`)
	_, err = recursive.EvalWithOptions(nil, EvalOptions{MaxCallDepth: 2})
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D1011" {
		t.Fatalf("depth error = %T %v, want public D1011", err, err)
	}

	_, err = MustCompile("1").EvalWithOptions(nil, EvalOptions{Timeout: time.Nanosecond})
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D1012" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %T %v, want D1012 wrapping deadline", err, err)
	}
}

func TestEvalWithOptionsCancellationAndConcurrentBindings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := MustCompile("1").EvalWithOptions(nil, EvalOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}

	expr := MustCompile("$x")
	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(want float64) {
			defer wg.Done()
			got, evalErr := expr.EvalWithOptions(nil, EvalOptions{Bindings: map[string]any{"x": want}})
			if evalErr != nil || got != want {
				t.Errorf("bound evaluation = %#v, %v; want %v", got, evalErr, want)
			}
		}(float64(i))
	}
	wg.Wait()
}
