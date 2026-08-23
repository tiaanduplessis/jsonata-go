package evaluator

import (
	"errors"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestCoreResidualNumericTypeErrorsPreserveOperandPosition(t *testing.T) {
	for _, test := range []struct {
		expr string
		code string
	}{
		{expr: `false + 1`, code: "T2001"},
		{expr: `false + $missing`, code: "T2001"},
		{expr: `false > $missing`, code: "T2010"},
	} {
		t.Run(test.expr, func(t *testing.T) {
			n, err := syntax.Parse(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			_, evalErr := Eval(n, nil)
			if evalErr == nil {
				t.Fatalf("expected %s", test.code)
			}
			var coded interface{ JSONataCode() string }
			if !errors.As(evalErr, &coded) || coded.JSONataCode() != test.code {
				t.Fatalf("got %v, want %s", evalErr, test.code)
			}
		})
	}
}

func TestCoreResidualUndefinedComparisonsRemainUndefinedForCompatibleTypes(t *testing.T) {
	for _, expr := range []string{`3 > $missing`, `$missing <= "hello"`} {
		n, err := syntax.Parse(expr)
		if err != nil {
			t.Fatal(err)
		}
		_, evalErr := Eval(n, nil)
		if !errors.Is(evalErr, errUndefined) {
			t.Fatalf("%s: got %v, want undefined", expr, evalErr)
		}
	}
}

func TestCoreResidualDefaultTreatsFunctionAsFalse(t *testing.T) {
	n, err := syntax.Parse(`function(){true} ?: 42`)
	if err != nil {
		t.Fatal(err)
	}
	got, evalErr := Eval(n, nil)
	if evalErr != nil || got != 42.0 {
		t.Fatalf("got %#v, %v; want 42", got, evalErr)
	}
}

func TestCoreResidualRangeValidatesUndefinedEndpointsAndBounds(t *testing.T) {
	for _, test := range []struct {
		start, end any
		code       string
	}{
		{start: value.Undefined, end: true, code: "T2004"},
		{start: false, end: value.Undefined, code: "T2003"},
		{start: 0.0, end: 10000000.0, code: "D2014"},
	} {
		_, err := numberRange(test.start, test.end)
		if err == nil {
			t.Fatalf("range %#v..%#v: expected %s", test.start, test.end, test.code)
		}
		var coded interface{ JSONataCode() string }
		if !errors.As(err, &coded) || coded.JSONataCode() != test.code {
			t.Fatalf("range %#v..%#v: got %v, want %s", test.start, test.end, err, test.code)
		}
	}
}
