package benchmark

import (
	"errors"
	"strings"
	"testing"
)

type fakeCompiled struct {
	value any
	err   error
}

func (f fakeCompiled) Eval(any) (any, error) { return f.value, f.err }
func (f fakeCompiled) EvalBytes([]byte) (any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte(`null`), nil
}

func TestCompareRejectsWrongExpectedValue(t *testing.T) {
	err := Compare(map[string]any{"answer": 7}, nil, Oracle{Value: []byte(`{"answer":8}`)}, nil)
	if err == nil || !strings.Contains(err.Error(), "value mismatch") {
		t.Fatalf("expected value mismatch, got %v", err)
	}
}

func TestCompareRejectsWrongExpectedStructuredError(t *testing.T) {
	actual := errors.New(`{"code":"S0211","position":4}`)
	err := Compare(nil, actual, Oracle{Error: &OracleError{Code: "S0212", Position: 4}}, func(error) OracleError {
		return OracleError{Code: "S0211", Position: 4}
	})
	if err == nil || !strings.Contains(err.Error(), `code expected "S0212", got "S0211"`) {
		t.Fatalf("expected structured error mismatch, got %v", err)
	}
}

func TestCompareAcceptsStructuredError(t *testing.T) {
	err := Compare(nil, errors.New("ignored"), Oracle{Error: &OracleError{Code: "S0211", Position: 4}}, func(error) OracleError {
		return OracleError{Code: "S0211", Position: 4}
	})
	if err != nil {
		t.Fatalf("expected matching error, got %v", err)
	}
}

func TestVerifyCaseRejectsBeforeTiming(t *testing.T) {
	runtime := Runtime{Compile: func(string) (Compiled, error) {
		return fakeCompiled{value: "wrong"}, nil
	}}
	c := Case{ID: "gate", Expression: "42", Input: `{}`, Modes: []Mode{ModeDecoded}, Oracle: Oracle{Value: []byte(`42`)}}
	_, err := VerifyCase(runtime, c, ModeDecoded)
	var verificationErr *VerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("expected verification error, got %v", err)
	}
	if !strings.Contains(verificationErr.Reason, "value mismatch") {
		t.Fatalf("expected value mismatch reason, got %q", verificationErr.Reason)
	}
}

func TestVerifyCaseRejectsIncorrectErrorBeforeTiming(t *testing.T) {
	runtime := Runtime{Compile: func(string) (Compiled, error) {
		return fakeCompiled{err: errors.New("wrong error")}, nil
	}}
	c := Case{ID: "error-gate", Expression: "42", Input: `{}`, Modes: []Mode{ModeDecoded}, Oracle: Oracle{Value: []byte(`42`)}}
	_, err := VerifyCase(runtime, c, ModeDecoded)
	var verificationErr *VerificationError
	if !errors.As(err, &verificationErr) {
		t.Fatalf("expected verification error, got %v", err)
	}
	if !strings.Contains(verificationErr.Reason, "structured error") {
		t.Fatalf("expected structured error mismatch reason, got %q", verificationErr.Reason)
	}
}

func TestVerifyCaseDecodesRawResultBeforeComparison(t *testing.T) {
	runtime := Runtime{Compile: func(string) (Compiled, error) {
		return rawCompiled{output: []byte(`{"answer":7}`)}, nil
	}}
	c := Case{ID: "raw-result", Expression: "42", Input: `{}`, Modes: []Mode{ModeBytes}, Oracle: Oracle{Value: []byte(`{"answer":7}`)}}
	if _, err := VerifyCase(runtime, c, ModeBytes); err != nil {
		t.Fatalf("expected decoded raw result to match, got %v", err)
	}
}

type rawCompiled struct{ output []byte }

func (r rawCompiled) Eval(any) (any, error)         { return nil, nil }
func (r rawCompiled) EvalBytes([]byte) (any, error) { return r.output, nil }
