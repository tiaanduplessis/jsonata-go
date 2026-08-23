package jsonata

import "testing"

func TestLegacyEvalErrorMessages(t *testing.T) {
	tests := []struct {
		err  EvalError
		want string
	}{
		{EvalError{Type: ErrNonIntegerLHS, Value: ".."}, `left side of the ".." operator must evaluate to an integer`},
		{EvalError{Type: ErrNonCallable, Token: "answer"}, `cannot call non-function answer`},
		{EvalError{Type: ErrDuplicateKey, Value: "id"}, `multiple object keys evaluate to the value "id"`},
		{EvalError{Type: ErrType(100)}, `EvalError: unknown error type 100`},
	}

	for _, test := range tests {
		if got := test.err.Error(); got != test.want {
			t.Errorf("%#v.Error() = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestLegacyArgumentErrorMessages(t *testing.T) {
	count := ArgCountError{Func: "sum", Expected: 1, Received: 0}
	if got, want := count.Error(), `function "sum" takes 1 argument(s), got 0`; got != want {
		t.Fatalf("ArgCountError.Error() = %q, want %q", got, want)
	}

	typeError := ArgTypeError{Func: "sum", Which: 1}
	if got, want := typeError.Error(), `argument 1 of function "sum" does not match function signature`; got != want {
		t.Fatalf("ArgTypeError.Error() = %q, want %q", got, want)
	}
}

func TestLegacyErrorConstantsRemainStable(t *testing.T) {
	constants := []ErrType{
		ErrNonIntegerLHS, ErrNonIntegerRHS, ErrNonNumberLHS, ErrNonNumberRHS,
		ErrNonComparableLHS, ErrNonComparableRHS, ErrTypeMismatch, ErrNonCallable,
		ErrNonCallableApply, ErrNonCallablePartial, ErrNumberInf, ErrNumberNaN,
		ErrMaxRangeItems, ErrIllegalKey, ErrDuplicateKey, ErrClone,
		ErrIllegalUpdate, ErrIllegalDelete, ErrNonSortable, ErrSortMismatch,
	}
	for index, value := range constants {
		if value != ErrType(index) {
			t.Fatalf("legacy error constant %d = %d", index, value)
		}
	}
}
