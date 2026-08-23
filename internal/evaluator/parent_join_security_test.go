package evaluator

import (
	"context"
	"errors"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestOrderedInputKeysCannotExposeEvaluatorState(t *testing.T) {
	input, err := DecodeJSON([]byte(`{"__proto__":{"constructor":"data"},"constructor":"data","prototype":{"invoke":"data"},"keep":true,"$internal":"data"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []string{`*`, `**`, `$lookup($, "__proto__")`, `$lookup($, "constructor")`, `$lookup($, "$internal")`} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatalf("parse %q: %v", expression, parseErr)
		}
		result, evalErr := EvalWithOptions(node, input, Options{MaxCallDepth: 32, MaxOperations: 2048})
		if evalErr != nil {
			t.Fatalf("evaluate %q: %v", expression, evalErr)
		}
		if containsCallable(result) {
			t.Fatalf("%s exposed executable evaluator state: %#v", expression, result)
		}
	}
}

func TestExternalBindingsCannotEnableAncestorLookup(t *testing.T) {
	node, err := syntax.Parse(`foo.child.secret`)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"foo": map[string]any{"secret": "ancestor", "child": map[string]any{}}}
	result, evalErr := EvalBindingsWithOptions(node, input, map[string]any{"lineage": true}, Options{MaxCallDepth: 16, MaxOperations: 256})
	if result != nil || !errors.Is(evalErr, errUndefined) {
		t.Fatalf("external binding enabled ancestor lookup: result=%#v error=%v", result, evalErr)
	}
}

func TestLineageLookupHonorsRuntimeLimits(t *testing.T) {
	cycle := &contextual{v: map[string]any{"safe": true}, vars: map[string]any{}}
	cycle.parent = cycle
	runtime := newEvalRuntime(Options{MaxOperations: 2})
	if _, err := lineageField(cycle, "missing", runtime); err == nil {
		t.Fatal("cyclic lineage lookup ignored operation budget")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	runtime = newEvalRuntime(Options{Context: canceled})
	if _, err := lineageField(cycle, "missing", runtime); !errors.Is(err, context.Canceled) {
		t.Fatalf("lineageField() error = %v, want context.Canceled", err)
	}
}
