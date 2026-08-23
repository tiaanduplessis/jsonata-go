package security_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

// FuzzPublicExtensionBoundary covers reflected custom functions, public JSON
// normalization, cancellation, and panic containment without global registry
// mutation. Inputs and evaluator work are strictly bounded.
func FuzzPublicExtensionBoundary(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`null`),
		[]byte(`{"__proto__":{"safe":true},"constructor":"data"}`),
		[]byte(`[1,"two",null,{"nested":true}]`),
		[]byte(`"panic"`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2*1024 {
			data = data[:2*1024]
		}
		var input any
		if err := json.Unmarshal(data, &input); err != nil {
			return
		}
		engine := jsonata.NewEngine()
		err := engine.RegisterExts(map[string]jsonata.Extension{
			"fuzz": {Func: func(ctx context.Context, value any) (any, error) {
				if text, ok := value.(string); ok && text == "panic" {
					panic("bounded fuzz panic")
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					return value, nil
				}
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		expression, err := engine.Compile(`$fuzz($value)`)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, evalErr := expression.EvalWithOptions(nil, jsonata.EvalOptions{
			Context:       ctx,
			Bindings:      map[string]any{"value": input},
			MaxCallDepth:  16,
			MaxOperations: 1_000,
		})
		if errors.Is(evalErr, context.DeadlineExceeded) {
			return
		}
	})
}
