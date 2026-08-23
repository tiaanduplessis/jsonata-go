package evaluator

import (
	"errors"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestProjectionSecurityDoesNotTreatCraftedFieldsAsEvaluatorState(t *testing.T) {
	input := map[string]any{
		"items": []any{
			map[string]any{"key": "__proto__", "value": "blocked"},
			map[string]any{"key": "constructor", "value": "blocked"},
			map[string]any{"key": "prototype", "value": "blocked"},
		},
		"__proto__":   map[string]any{"invoke": "blocked"},
		"constructor": "blocked",
		"prototype":   map[string]any{"invoke": "blocked"},
	}
	for _, expression := range []string{
		`items{$string(key): value}`,
		`$lookup($, "__proto__")`,
		`$lookup($, "constructor")`,
		`*`,
		`**`,
	} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatalf("parse %q: %v", expression, parseErr)
		}
		result, evalErr := EvalWithOptions(node, input, Options{MaxCallDepth: 32, MaxOperations: 2048})
		if evalErr != nil {
			t.Fatalf("evaluate %q: %v", expression, evalErr)
		}
		if containsCallable(result) {
			t.Fatalf("%s exposed an executable evaluator value: %#v", expression, result)
		}
	}
}

func TestGroupedProjectionPreservesDynamicObjectOrderForString(t *testing.T) {
	node, err := syntax.Parse(`$string([{"first":"b","second":"x","value":1},{"first":"a","second":"y","value":2}]{first: value, second: value})`)
	if err != nil {
		t.Fatal(err)
	}
	result, evalErr := EvalWithOptions(node, nil, Options{MaxCallDepth: 32, MaxOperations: 2048})
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	if result != `{"b":1,"x":1,"a":2,"y":2}` {
		t.Fatalf("ordered grouped projection = %q, want %q", result, `{"b":1,"x":1,"a":2,"y":2}`)
	}
}

func TestGroupedProjectionRejectsDuplicateKeysAcrossPairs(t *testing.T) {
	node, err := syntax.Parse(`[{"key":"a","value":1}]{$string(key): value, $string(key): value}`)
	if err != nil {
		t.Fatal(err)
	}
	_, evalErr := EvalWithOptions(node, nil, Options{MaxCallDepth: 32, MaxOperations: 2048})
	var coded interface{ JSONataCode() string }
	if !errors.As(evalErr, &coded) || coded.JSONataCode() != "D1009" {
		t.Fatalf("duplicate grouped key error = %v, want D1009", evalErr)
	}
}
