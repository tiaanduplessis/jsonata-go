package evaluator

import (
	"context"
	"errors"
	"testing"
)

func TestAppendHonorsOperationBudgetAndCancellation(t *testing.T) {
	input := make([]any, bulkOperationCheckStride+1)
	_, err := builtinAppend(state{runtime: newEvalRuntime(Options{MaxOperations: 1})}, []any{input, input})
	if !hasJSONataCode(err, "U1001") {
		t.Fatalf("budgeted append error = %v, want U1001", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = builtinAppend(state{runtime: newEvalRuntime(Options{Context: canceled})}, []any{input, input})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled append error = %v, want context.Canceled", err)
	}
}

func TestBulkPreallocationSaturatesLargeBudgets(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	runtime := newEvalRuntime(Options{MaxOperations: maxInt64})
	if capacity := bulkOperationCapacity(1024, runtime); capacity != 1024 {
		t.Fatalf("large-budget capacity = %d, want 1024", capacity)
	}
	if _, err := numberRangeWithState(state{runtime: runtime}, 1.0, 2.0); err != nil {
		t.Fatalf("large-budget range: %v", err)
	}
	if _, err := builtinAppend(state{runtime: runtime}, []any{[]any{1.0}, []any{2.0}}); err != nil {
		t.Fatalf("large-budget append: %v", err)
	}
}
