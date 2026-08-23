package jsonata

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestSecurityResidualLargeRangeRejectsWithoutAllocation(t *testing.T) {
	start := time.Now()
	_, err := MustCompile(`[-1e308..1e308]`).EvalNoInputWithOptions(EvalOptions{})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("large range took %s; expected an immediate guard", elapsed)
	}
	jsonataErr := assertPublicJSONataCode(t, err, "D2014")
	if value, ok := jsonataErr.Value.(float64); !ok || !math.IsInf(value, 1) {
		t.Fatalf("large range D2014 value = %#v, want +Inf", jsonataErr.Value)
	}
	want := "The size of the sequence allocated by the range operator (..) must not exceed 1e7.  Attempted to allocate null."
	if jsonataErr.Message != want {
		t.Fatalf("large range D2014 message = %q, want %q", jsonataErr.Message, want)
	}
}

func TestSecurityResidualEvalBytesCanceledInvalidJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := EvalBytesWithOptions("$", []byte(`{"unterminated"`), EvalOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled invalid JSON error = %v, want context.Canceled", err)
	}
}

func TestSecurityResidualEvalBytesCancellationDuringDecode(t *testing.T) {
	ctx := newCancelDuringDecodeContext(4)
	data := []byte("[" + strings.Repeat("0,", 30000) + "0]")
	_, err := EvalBytesWithOptions("$", data, EvalOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("decode cancellation error = %v, want context.Canceled", err)
	}
}

func TestSecurityResidualEvalBytesContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := EvalBytesWithOptions("$", []byte(`[1,2,3]`), EvalOptions{Context: ctx})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("decode deadline error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSecurityResidualEvalBytesDecodeControls(t *testing.T) {
	data := []byte("[" + strings.Repeat("0,", 30000) + "0]")
	_, err := EvalBytesWithOptions("$count()", data, EvalOptions{MaxOperations: 2})
	if !strings.Contains(errString(err), "operation budget") {
		t.Fatalf("decode budget error = %v, want operation budget", err)
	}

	_, err = EvalBytesWithOptions("$", data, EvalOptions{Timeout: time.Nanosecond})
	jsonataErr := assertPublicJSONataCode(t, err, "D1012")
	if jsonataErr.Message == "" {
		t.Fatal("timeout error has no message")
	}

	got, err := EvalBytesWithOptions("$", []byte(`{"safe":1,"constructor":2,"safe":3}`), EvalOptions{})
	if err != nil {
		t.Fatalf("normal decode error = %v", err)
	}
	if string(got) != `{"constructor":2,"safe":3}` {
		t.Fatalf("normal decode = %s, want duplicate-key compatibility", got)
	}
}

func TestSecurityResidualEvalBytesSharesDecodeBudgetWithEvaluation(t *testing.T) {
	_, err := EvalBytesWithOptions("1", []byte(`0`), EvalOptions{MaxOperations: 1})
	assertPublicJSONataCode(t, err, "U1001")
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type cancelDuringDecodeContext struct {
	done      chan struct{}
	remaining int
}

func newCancelDuringDecodeContext(checks int) *cancelDuringDecodeContext {
	return &cancelDuringDecodeContext{done: make(chan struct{}), remaining: checks}
}

func (c *cancelDuringDecodeContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *cancelDuringDecodeContext) Done() <-chan struct{} {
	if c.remaining > 0 {
		c.remaining--
		if c.remaining == 0 {
			close(c.done)
		}
	}
	return c.done
}

func (c *cancelDuringDecodeContext) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}

func (c *cancelDuringDecodeContext) Value(any) any { return nil }
