package jsonata

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestReferenceDiagnosticMetadataAndTypedValues(t *testing.T) {
	tests := []struct {
		expression string
		code       string
		token      string
		value      any
		position   int
		message    string
	}{
		{`1 + null`, "T2002", "+", nil, 3, `The right side of the "+" operator must evaluate to a number`},
		{`"😀" + "x"`, "T2001", "+", "😀", 6, `The left side of the "+" operator must evaluate to a number`},
	}
	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			_, err := Eval(test.expression, nil)
			var structured *Error
			if !errors.As(err, &structured) {
				t.Fatalf("error = %T %v, want *Error", err, err)
			}
			if structured.Code != test.code || structured.Token != test.token || structured.Position != test.position || structured.Message != test.message {
				t.Fatalf("error = %#v, want code=%s token=%q position=%d message=%q", structured, test.code, test.token, test.position, test.message)
			}
			if structured.Value != test.value {
				t.Fatalf("value = %#v (%T), want %#v (%T)", structured.Value, structured.Value, test.value, test.value)
			}
		})
	}
}

func TestReferenceStackAndTimeoutDiagnostics(t *testing.T) {
	shallow := MustCompile(`1 + 1`)
	_, err := shallow.EvalWithOptions(nil, EvalOptions{MaxCallDepth: 1})
	var structured *Error
	if !errors.As(err, &structured) || structured.Code != "D1011" || structured.Value != float64(1) {
		t.Fatalf("shallow stack error = %#v, want pinned D1011", err)
	}
	if got, err := shallow.EvalWithOptions(nil, EvalOptions{MaxCallDepth: 2}); err != nil || got != 2.0 {
		t.Fatalf("shallow stack boundary = %#v, %v; want 2", got, err)
	}

	stackExpr := MustCompile(`($f := function(){$f() + 1}; $f())`)
	_, err = stackExpr.EvalWithOptions(nil, EvalOptions{MaxCallDepth: 3})
	if !errors.As(err, &structured) || structured.Code != "D1011" || structured.Value != float64(3) || structured.Message != "Stack overflow. Check for non-terminating recursive function.  Consider rewriting as tail-recursive" {
		t.Fatalf("stack error = %#v, want pinned D1011", err)
	}

	timeoutExpr := MustCompile(`($f := function(){$f()}; $f())`)
	_, err = timeoutExpr.EvalWithOptions(nil, EvalOptions{Timeout: 5 * time.Millisecond, MaxCallDepth: 1_000_000, MaxOperations: 1_000_000_000})
	if !errors.As(err, &structured) || structured.Code != "D1012" || structured.Value != float64(5) || structured.Message != "Evaluation timeout after 5 milliseconds. Check for infinite loop" {
		t.Fatalf("timeout error = %#v, want pinned D1012", err)
	}
}

func TestContextErrorsRemainWrapped(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := EvalWithOptions(`1 + 1`, nil, EvalOptions{Context: canceled})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	var structured *Error
	if !errors.As(err, &structured) || structured.Code != "U1001" {
		t.Fatalf("error = %#v, want wrapped U1001", err)
	}
}

func TestModernErrorAsLegacyEvalError(t *testing.T) {
	_, err := Eval(`1 + null`, nil)
	var legacy EvalError
	if !errors.As(err, &legacy) || legacy.Type != ErrNonNumberRHS || legacy.Value != "null" {
		t.Fatalf("legacy error = %#v, want T2002 mapping", legacy)
	}
	var legacyPointer *EvalError
	if !errors.As(err, &legacyPointer) || legacyPointer == nil || legacyPointer.Type != ErrNonNumberRHS {
		t.Fatalf("legacy pointer = %#v, want T2002 mapping", legacyPointer)
	}
}

func TestReferenceCallMetadataDoesNotReplaceArgumentMetadata(t *testing.T) {
	tests := []struct {
		expression string
		token      string
		position   int
		value      any
	}{
		{`$number("x")`, "number", 8, "x"},
		{`$foo()`, "foo", 5, nil},
		{`($foo)()`, "", 7, nil},
		{`($f:=function(){1+null};$f())`, "+", 27, nil},
		{`($f:=function(){1/"x"};$f())`, "/", 26, "x"},
		{`$count(1+null)`, "+", 9, nil},
	}
	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			_, err := Eval(test.expression, nil)
			var structured *Error
			if !errors.As(err, &structured) {
				t.Fatalf("error = %T %v, want *Error", err, err)
			}
			if structured.Token != test.token || structured.Position != test.position || !reflect.DeepEqual(structured.Value, test.value) {
				t.Fatalf("error = %#v, want token=%q position=%d value=%#v", structured, test.token, test.position, test.value)
			}
		})
	}
}

func TestReferenceRangeAndObjectDiagnosticMetadata(t *testing.T) {
	for _, test := range []struct {
		expression string
		token      string
		value      any
		position   int
	}{
		{`[0..10000000]`, "..", float64(10000001), 4},
		{`[-1e308..1e308]`, "..", math.Inf(1), 9},
		{`{[1]:true}`, "", []any{float64(1)}, 1},
	} {
		t.Run(test.expression, func(t *testing.T) {
			_, err := Eval(test.expression, nil)
			var structured *Error
			if !errors.As(err, &structured) {
				t.Fatalf("error = %T %v, want *Error", err, err)
			}
			if structured.Token != test.token || structured.Position != test.position || !reflect.DeepEqual(structured.Value, test.value) {
				t.Fatalf("error = %#v, want token=%q position=%d value=%#v", structured, test.token, test.position, test.value)
			}
		})
	}
}
