package evaluator

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestNormalizeExtensionBindingSafePreservesSpecialValues(t *testing.T) {
	result, err := NormalizeExtensionBindingSafe(map[string]any{
		"undefined": value.Undefined,
		"callable":  normalizationTestCallable{},
		"items":     []any{nil, 1.0},
	}, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	object := result.(map[string]any)
	if !value.IsUndefined(object["undefined"]) {
		t.Fatalf("undefined = %#v", object["undefined"])
	}
	if _, ok := object["callable"].(*legacyCallable); !ok {
		t.Fatalf("callable = %T, want *legacyCallable", object["callable"])
	}
	if _, ok := object["items"].(value.Array); !ok {
		t.Fatalf("items = %T, want value.Array", object["items"])
	}
}

func TestNormalizeExtensionBindingSafeRejectsInaccessibleReflection(t *testing.T) {
	type privateValue struct {
		hidden int
	}
	inaccessible := reflect.ValueOf(privateValue{hidden: 1}).Field(0)
	if inaccessible.CanInterface() {
		t.Fatal("test value unexpectedly permits Interface")
	}
	if _, err := NormalizeExtensionBindingSafe(inaccessible, 16, nil); !hasJSONataCode(err, "T0412") {
		t.Fatalf("inaccessible reflection error = %v, want T0412", err)
	}
}

func TestNormalizeEvaluatorValueReportsStablePublicValueErrors(t *testing.T) {
	for name, test := range map[string]struct {
		input any
		code  string
	}{
		"channel":        {input: make(chan int), code: "T0412"},
		"function":       {input: func() {}, code: "T0412"},
		"struct":         {input: struct{ Value int }{Value: 1}, code: "T0412"},
		"non-string map": {input: map[int]any{1: true}, code: "T0412"},
		"nan":            {input: math.NaN(), code: "D1001"},
		"infinity":       {input: math.Inf(1), code: "D1001"},
		"invalid number": {input: json.Number("invalid"), code: "D1001"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeInputValue(newEvalRuntime(Options{}), test.input); !hasJSONataCode(err, test.code) {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestNormalizeExtensionBindingSafePreservesInternalCallableOnRepeatedNormalization(t *testing.T) {
	first, err := NormalizeExtensionBindingSafe(normalizationTestCallable{}, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizeExtensionBindingSafe(first, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("repeated normalization replaced internal callable: %T -> %T", first, second)
	}
}

func TestNormalizeExtensionValueUsesActiveOperationBudget(t *testing.T) {
	runtime := newEvalRuntime(Options{MaxOperations: 2})
	_, err := normalizeExtensionValue(runtime, map[string]any{
		"left":  []any{1.0, 2.0},
		"right": []any{3.0, 4.0},
	})
	if err == nil {
		t.Fatal("normalization unexpectedly exceeded the active operation budget")
	}
}

func TestNormalizeExtensionBindingSafeRejectsCyclesAndDepth(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if _, err := NormalizeExtensionBindingSafe(cyclic, 16, nil); !hasJSONataCode(err, "T0412") {
		t.Fatalf("cycle error = %v, want T0412", err)
	}
	deep := map[string]any{"leaf": true}
	for range 8 {
		deep = map[string]any{"next": deep}
	}
	if _, err := NormalizeExtensionBindingSafe(deep, 4, nil); !hasJSONataCode(err, "U1001") {
		t.Fatalf("depth error = %v, want U1001", err)
	}
}

func TestExtensionResultUsesRuntimeAwareNormalization(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	binding, err := NewReflectedExtension("cyclic", ReflectedExtension{Func: func() any { return cyclic }})
	if err != nil {
		t.Fatal(err)
	}
	_, err = binding.(callableValue).invoke(state{runtime: newEvalRuntime(Options{MaxCallDepth: 16, MaxOperations: 128})}, nil)
	if !hasJSONataCode(err, "T0412") {
		t.Fatalf("cyclic extension result error = %v, want T0412", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = normalizeExtensionValue(newEvalRuntime(Options{Context: canceled}), map[string]any{"a": 1.0})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled extension result error = %v, want context.Canceled", err)
	}
}

type normalizationTestCallable struct{}

func (normalizationTestCallable) Name() string    { return "normalizationCallable" }
func (normalizationTestCallable) ParamCount() int { return 0 }
func (normalizationTestCallable) Call([]reflect.Value) (reflect.Value, error) {
	return reflect.ValueOf(true), nil
}
