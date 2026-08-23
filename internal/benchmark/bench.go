package benchmark

import (
	"encoding/json"
	"fmt"
	"testing"
)

var benchmarkSink any

// BenchmarkCompile measures compilation after proving that the resulting
// expression evaluates to its pinned oracle. A compile failure during timing
// is reported as a benchmark failure rather than silently discarded.
func BenchmarkCompile(b *testing.B, runtime Runtime, c Case, mode Mode) {
	b.Helper()
	if _, err := VerifyCase(runtime, c, mode); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expr, err := runtime.compile(c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSink = expr
	}
}

// BenchmarkEval measures decoded Go-value evaluation after the correctness
// gate has passed and while reusing one immutable compiled expression.
func BenchmarkEval(b *testing.B, runtime Runtime, c Case) {
	b.Helper()
	expr, err := VerifyCase(runtime, c, ModeDecoded)
	if err != nil {
		b.Fatal(err)
	}
	var input any
	if err := json.Unmarshal([]byte(c.Input), &input); err != nil {
		b.Fatal(fmt.Errorf("decode case %q: %w", c.ID, err))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := expr.Eval(input)
		if !expectedTimedError(c, err) {
			b.Fatal(err)
		}
		benchmarkSink = result
	}
}

// BenchmarkEvalBytes measures evaluation directly from raw JSON bytes.
func BenchmarkEvalBytes(b *testing.B, runtime Runtime, c Case) {
	b.Helper()
	expr, err := VerifyCase(runtime, c, ModeBytes)
	if err != nil {
		b.Fatal(err)
	}
	input := []byte(c.Input)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := expr.EvalBytes(input)
		if !expectedTimedError(c, err) {
			b.Fatal(err)
		}
		benchmarkSink = result
	}
}

// BenchmarkParallel measures concurrent use of one compiled expression. The
// correctness gate runs before RunParallel starts, so a mismatch cannot be
// hidden by a worker or included in timing.
func BenchmarkParallel(b *testing.B, runtime Runtime, c Case) {
	b.Helper()
	expr, err := VerifyCase(runtime, c, ModeDecoded)
	if err != nil {
		b.Fatal(err)
	}
	var input any
	if err := json.Unmarshal([]byte(c.Input), &input); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := expr.Eval(input)
			if !expectedTimedError(c, err) {
				b.Error(err)
			}
		}
	})
}

func expectedTimedError(c Case, err error) bool {
	if c.Oracle.Error != nil {
		return err != nil
	}
	return err == nil
}
