package evaluator

import (
	"errors"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestSequenceGuardrailRejectsBeforeLargeRangeAllocation(t *testing.T) {
	node, parseErr := syntax.Parse(`[0..9999999]`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	result := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			_, err := EvalNoInputWithOptions(node, Options{MaxSequenceLength: 1})
			if !sequenceGuardrailHasCode(err, "D2015") {
				b.Fatalf("error = %v, want D2015", err)
			}
		}
	})
	if allocated := result.AllocedBytesPerOp(); allocated > 64<<10 {
		t.Fatalf("limited range allocated %d bytes/op before D2015", allocated)
	}
}

func TestSequenceGuardrailRejectsAppendBeforeCopyingInputs(t *testing.T) {
	large := make([]any, 1_000_000)
	result := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			st := state{runtime: newEvalRuntime(Options{MaxOperations: 1_000_000, MaxSequenceLength: 1})}
			_, err := builtinAppend(st, []any{value.Array{Items: large}, value.Array{Items: large}})
			if !sequenceGuardrailHasCode(err, "D2015") {
				b.Fatalf("error = %v, want D2015", err)
			}
		}
	})
	if allocated := result.AllocedBytesPerOp(); allocated > 64<<10 {
		t.Fatalf("limited append allocated %d bytes/op before D2015", allocated)
	}
}

func TestSequenceGuardrailCoversRegexMatchSequence(t *testing.T) {
	node, parseErr := syntax.Parse(`$match("aaaa", /a/)`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	_, err := EvalNoInputWithOptions(node, Options{MaxSequenceLength: 1})
	var runtimeErr runtimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.code != "D2015" {
		t.Fatalf("error = %v, want D2015", err)
	}
	if runtimeErr.value != float64(1) || runtimeErr.msg != "The maximum sequence length of 1 was exceeded." {
		t.Fatalf("D2015 = value %#v message %q, want value 1 and the pinned message", runtimeErr.value, runtimeErr.msg)
	}
}

func TestRangeD2014PreservesPinnedDetails(t *testing.T) {
	node, parseErr := syntax.Parse(`[0..10000000]`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	_, err := EvalNoInputWithOptions(node, Options{MaxSequenceLength: 1})
	var runtimeErr runtimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.code != "D2014" {
		t.Fatalf("error = %v, want D2014", err)
	}
	wantMessage := "The size of the sequence allocated by the range operator (..) must not exceed 1e7.  Attempted to allocate 10000001."
	if runtimeErr.value != float64(10000001) || runtimeErr.msg != wantMessage {
		t.Fatalf("D2014 = value %#v message %q, want value 10000001 and pinned message", runtimeErr.value, runtimeErr.msg)
	}
}

func sequenceGuardrailHasCode(err error, code string) bool {
	var coded interface{ JSONataCode() string }
	return errors.As(err, &coded) && coded.JSONataCode() == code
}
