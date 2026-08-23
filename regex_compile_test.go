package jsonata

import (
	"errors"
	"testing"
)

func TestCompileValidatesRegexLiterals(t *testing.T) {
	for _, source := range []string{`/[/]/`, `/[a/b]/`, `/(a/b)/`, `/({a/b}[c/d])/`, `/\//`, `/[\/]/`} {
		if _, err := Compile(source); err != nil {
			t.Fatalf("Compile(%q): %v", source, err)
		}
	}
	for _, test := range []struct {
		source string
		code   string
	}{
		{source: `//`, code: "S0301"},
		{source: `/[/`, code: "S0302"},
		{source: `/(abc/`, code: "S0302"},
		{source: `/(a/b/`, code: "S0302"},
		{source: `/)/`, code: "S0302"},
		{source: `/a/g`, code: "S0201"},
	} {
		_, err := Compile(test.source)
		var coded *Error
		if !errors.As(err, &coded) || coded.Code != test.code {
			t.Errorf("Compile(%q) error = %v, want %s", test.source, err, test.code)
		}
	}
}

func TestEvaluationErrorsExposeRuntimeTokens(t *testing.T) {
	for _, test := range []struct {
		expression, code, token string
	}{
		{expression: `( $A := function(){$min(2, 3)}; $A() )`, code: "T0410", token: "min"},
		{expression: `( $B := function(){''}; $A := function(){2 + $B()}; $A() )`, code: "T2002", token: "+"},
	} {
		_, err := Eval(test.expression, nil)
		var jsonataErr *Error
		if !errors.As(err, &jsonataErr) || jsonataErr.Code != test.code || jsonataErr.Token != test.token {
			t.Errorf("Eval(%q) error = %#v, want code=%s token=%q", test.expression, jsonataErr, test.code, test.token)
		}
	}
}
