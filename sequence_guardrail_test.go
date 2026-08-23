package jsonata

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestSequenceGuardrailPinnedImplementationCases(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{name: "range", expression: `[0..1001]`},
		{name: "intermediate path", expression: `[0..100].([0..100]) ~> count()`},
		{name: "append", expression: `$append([0..600], [0..600]) ~> $count()`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expr := MustCompile(test.expression)
			_, err := expr.EvalNoInputWithOptions(EvalOptions{
				MaxOperations:     5_000_000,
				MaxSequenceLength: 1000,
			})
			jsonataErr := assertPublicJSONataCode(t, err, "D2015")
			wantValue := float64(1000)
			if test.name == "range" {
				wantValue = 1002
			}
			if test.name == "append" {
				wantValue = 1202
			}
			if jsonataErr.Value != wantValue {
				t.Fatalf("D2015 value = %#v, want %#v", jsonataErr.Value, wantValue)
			}
			wantMessage := fmt.Sprintf("The maximum sequence length of %.0f was exceeded.", wantValue)
			if jsonataErr.Message != wantMessage {
				t.Fatalf("D2015 message = %q, want %q", jsonataErr.Message, wantMessage)
			}
		})
	}
}

func TestSequenceGuardrailBoundaryAndDisabledSemantics(t *testing.T) {
	exact := MustCompile(`[0..999]`)
	got, err := exact.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: 1000})
	if err != nil {
		t.Fatal(err)
	}
	values, ok := got.([]any)
	if !ok || len(values) != 1000 {
		t.Fatalf("exact-boundary result = %T %#v, want []any len 1000", got, got)
	}

	appendExact := MustCompile(`$append([1,2], [3])`)
	if got, err := appendExact.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: 3}); err != nil || !reflect.DeepEqual(got, []any{1.0, 2.0, 3.0}) {
		t.Fatalf("exact-boundary append = %#v, %v", got, err)
	}
	_, err = appendExact.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: 2})
	assertPublicJSONataCode(t, err, "D2015")
	for _, expression := range []string{`$append(missing, $)`, `$append($, missing)`} {
		got, err := MustCompile(expression).EvalWithOptions([]any{1, 2, 3, 4}, EvalOptions{MaxSequenceLength: 3})
		if err != nil || !reflect.DeepEqual(got, []any{1, 2, 3, 4}) {
			t.Fatalf("undefined append argument for %s = %#v, %v", expression, got, err)
		}
	}

	disabled := MustCompile(`[0..1001] ~> $count()`)
	for _, limit := range []int{0, -1} {
		got, err := disabled.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: limit})
		if err != nil || got != float64(1002) {
			t.Fatalf("disabled limit %d = %#v, %v; want 1002", limit, got, err)
		}
	}
	got, err = disabled.EvalNoInput()
	if err != nil || got != float64(1002) {
		t.Fatalf("compatibility wrapper default = %#v, %v; want 1002", got, err)
	}
}

func TestSequenceGuardrailCoversSequenceProducers(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		input      any
	}{
		{name: "map", expression: `$map($, function($v){$v})`, input: []any{1, 2, 3, 4}},
		{name: "filter", expression: `$filter($, function($v){true})`, input: []any{1, 2, 3, 4}},
		{name: "each", expression: `$each($, function($v){$v})`, input: map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}},
		{name: "keys", expression: `$keys($)`, input: map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}},
		{name: "lookup", expression: `$lookup($, "v")`, input: []any{map[string]any{"v": 1}, map[string]any{"v": 2}, map[string]any{"v": 3}, map[string]any{"v": 4}}},
		{name: "lookup nested sequence", expression: `$lookup($, "v")`, input: []any{map[string]any{"v": []any{1, 2, 3, 4}}}},
		{name: "spread", expression: `$spread($)`, input: map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}},
		{name: "array constructor", expression: `[1,2,3,4]`},
		{name: "projection", expression: `v`, input: []any{map[string]any{"v": 1}, map[string]any{"v": 2}, map[string]any{"v": 3}, map[string]any{"v": 4}}},
		{name: "group", expression: `$.{"all": $}`, input: []any{1, 2, 3, 4}},
		{name: "wildcard", expression: `$.*`, input: map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}},
		{name: "transform matches", expression: `$ ~> |$.*|{"seen":true}|`, input: map[string]any{"a": map[string]any{}, "b": map[string]any{}, "c": map[string]any{}, "d": map[string]any{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expr := MustCompile(test.expression)
			_, err := expr.EvalWithOptions(test.input, EvalOptions{MaxOperations: 100_000, MaxSequenceLength: 3})
			assertPublicJSONataCode(t, err, "D2015")
		})
	}
}

func TestSequenceGuardrailCancellationAndConcurrentOptions(t *testing.T) {
	expr := MustCompile(`[0..31]`)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := expr.EvalNoInputWithOptions(EvalOptions{Context: canceled, MaxSequenceLength: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled evaluation error = %v, want context.Canceled", err)
	}

	const workers = 24
	var group sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for worker := range workers {
		group.Add(1)
		go func(limit int) {
			defer group.Done()
			got, evalErr := expr.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: limit})
			if limit < 32 {
				var jsonataErr *Error
				if !errors.As(evalErr, &jsonataErr) || jsonataErr.Code != "D2015" {
					errorsSeen <- errors.New("limited concurrent evaluation did not return D2015")
				}
				return
			}
			if evalErr != nil {
				errorsSeen <- evalErr
				return
			}
			if values, ok := got.([]any); !ok || len(values) != 32 {
				errorsSeen <- errors.New("unlimited concurrent evaluation returned the wrong sequence")
			}
		}(31 + worker%2)
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
}

func TestSequenceGuardrailPreservesD2014RangeMaximum(t *testing.T) {
	expr := MustCompile(`[0..10000000]`)
	_, err := expr.EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: 1})
	jsonataErr := assertPublicJSONataCode(t, err, "D2014")
	if jsonataErr.Value != float64(10000001) {
		t.Fatalf("D2014 value = %#v, want 10000001", jsonataErr.Value)
	}
	wantMessage := "The size of the sequence allocated by the range operator (..) must not exceed 1e7.  Attempted to allocate 10000001."
	if jsonataErr.Message != wantMessage {
		t.Fatalf("D2014 message = %q, want %q", jsonataErr.Message, wantMessage)
	}
}

func TestSequenceGuardrailCoversRegexMatch(t *testing.T) {
	_, err := MustCompile(`$match("aaaa", /a/)`).EvalNoInputWithOptions(EvalOptions{MaxSequenceLength: 1})
	jsonataErr := assertPublicJSONataCode(t, err, "D2015")
	if jsonataErr.Value != float64(1) || jsonataErr.Message != "The maximum sequence length of 1 was exceeded." {
		t.Fatalf("D2015 = value %#v message %q, want value 1 and the pinned message", jsonataErr.Value, jsonataErr.Message)
	}
}

func TestSequenceGuardrailRemainsOrthogonalToOperationBudget(t *testing.T) {
	expr := MustCompile(`[0..1001]`)
	_, err := expr.EvalNoInputWithOptions(EvalOptions{MaxOperations: 1, MaxSequenceLength: 1})
	assertPublicJSONataCode(t, err, "U1001")
}

func assertPublicJSONataCode(t *testing.T, err error, code string) *Error {
	t.Helper()
	var jsonataErr *Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != code {
		t.Fatalf("error = %#v, want %s", err, code)
	}
	return jsonataErr
}
