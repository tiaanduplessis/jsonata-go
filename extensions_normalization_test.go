package jsonata

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

type publicNormalizationNamedMap map[string]any
type publicNormalizationNamedSlice []any

func TestRegisterVarsRejectsCyclicAndOverDepthValues(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if err := NewEngine().RegisterVars(map[string]interface{}{"cyclic": cyclic}); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("cyclic RegisterVars error = %v", err)
	}

	deep := map[string]any{"leaf": true}
	for range 101 {
		deep = map[string]any{"next": deep}
	}
	if err := MustCompile(`1`).RegisterVars(map[string]interface{}{"deep": deep}); err == nil || !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("over-depth RegisterVars error = %v", err)
	}
}

func TestEvaluationRejectsCyclicInputAndPerCallBindings(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic

	if _, err := MustCompile(`$clone($)`).Eval(cyclic); normalizationTestErrorCode(err) != "T0412" {
		t.Fatalf("cyclic input error = %v, want T0412", err)
	}
	if _, err := MustCompile(`$clone($cyclic)`).EvalWithOptions(nil, EvalOptions{
		Bindings: map[string]any{"cyclic": cyclic},
	}); normalizationTestErrorCode(err) != "T0412" {
		t.Fatalf("cyclic binding error = %v, want T0412", err)
	}
	if _, err := MustCompile(`$clone($)`).Eval(map[string]any{"channel": make(chan int)}); normalizationTestErrorCode(err) != "T0412" {
		t.Fatalf("hostile input error = %v, want T0412", err)
	}

	deep := map[string]any{"leaf": true}
	for range 8 {
		deep = map[string]any{"next": deep}
	}
	if _, err := MustCompile(`$clone($)`).EvalWithOptions(deep, EvalOptions{MaxCallDepth: 4}); normalizationTestErrorCode(err) != "U1001" {
		t.Fatalf("over-depth input error = %v, want U1001", err)
	}
}

func TestExtensionResultsUseEvaluationNormalizationBounds(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	expr := MustCompile(`$cyclicResult()`)
	if err := expr.RegisterExts(map[string]Extension{
		"cyclicResult": {Func: func() any { return cyclic }},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); normalizationTestErrorCode(err) != "T0412" {
		t.Fatalf("cyclic extension result error = %v, want T0412", err)
	}

	deep := map[string]any{"leaf": true}
	for range 8 {
		deep = map[string]any{"next": deep}
	}
	expr = MustCompile(`$deepResult()`)
	if err := expr.RegisterExts(map[string]Extension{
		"deepResult": {Func: func() any { return deep }},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.EvalWithOptions(nil, EvalOptions{MaxCallDepth: 4}); normalizationTestErrorCode(err) != "U1001" {
		t.Fatalf("over-depth extension result error = %v, want U1001", err)
	}
}

func TestPublicNormalizationCopiesNamedContainers(t *testing.T) {
	nested := publicNormalizationNamedMap{"value": 1.0}
	items := publicNormalizationNamedSlice{nested}
	expr := MustCompile(`$config.items[0].value`)
	if err := expr.RegisterVars(map[string]interface{}{
		"config": publicNormalizationNamedMap{"items": items},
	}); err != nil {
		t.Fatal(err)
	}

	nested["value"] = 9.0
	items[0] = publicNormalizationNamedMap{"value": 10.0}
	result, err := expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != 1.0 {
		t.Fatalf("registered named container result = %#v, want 1", result)
	}
	expr = MustCompile(`$config`)
	if err := expr.RegisterVars(map[string]interface{}{
		"config": publicNormalizationNamedMap{"items": publicNormalizationNamedSlice{nested}},
	}); err != nil {
		t.Fatal(err)
	}
	result, err = expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	object, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("public result leaked internal value %T", result)
	}
	if _, ok := object["items"].([]any); !ok {
		t.Fatalf("public nested result leaked internal value %T", object["items"])
	}
}

func TestPublicNormalizationTreatsTypedNilAsNull(t *testing.T) {
	var nilMap publicNormalizationNamedMap
	var nilSlice publicNormalizationNamedSlice
	var nilPointer *publicNormalizationNamedMap

	for name, input := range map[string]any{
		"map input":     nilMap,
		"slice input":   nilSlice,
		"pointer input": nilPointer,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := MustCompile(`$`).Eval(input)
			if err != nil {
				t.Fatal(err)
			}
			if result != nil {
				t.Fatalf("result = %#v, want JSON null", result)
			}
		})
	}

	expr := MustCompile(`$registered`)
	if err := expr.RegisterVars(map[string]interface{}{"registered": nilMap}); err != nil {
		t.Fatal(err)
	}
	result, err := expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("typed nil registered variable = %#v, want JSON null", result)
	}

	expr = MustCompile(`$typedNil()`)
	if err := expr.RegisterExts(map[string]Extension{
		"typedNil": {Func: func() publicNormalizationNamedSlice { return nil }},
	}); err != nil {
		t.Fatal(err)
	}
	result, err = expr.Eval(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("typed nil extension result = %#v, want JSON null", result)
	}
}

func TestPublicNormalizationRejectsUnsupportedAndNonFiniteValues(t *testing.T) {
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
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := MustCompile(`$`).Eval(test.input); normalizationTestErrorCode(err) != test.code {
				t.Fatalf("input error = %v, want %s", err, test.code)
			}
			if _, err := MustCompile(`$bad`).EvalWithOptions(nil, EvalOptions{
				Bindings: map[string]any{"bad": test.input},
			}); normalizationTestErrorCode(err) != test.code {
				t.Fatalf("binding error = %v, want %s", err, test.code)
			}
			if err := MustCompile(`1`).RegisterVars(map[string]interface{}{"bad": test.input}); normalizationRegistrationErrorCode(err) != test.code {
				t.Fatalf("registration error = %v, want %s", err, test.code)
			}
		})
	}

	expr := MustCompile(`$badResult()`)
	if err := expr.RegisterExts(map[string]Extension{
		"badResult": {Func: func() any { return make(chan int) }},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); normalizationTestErrorCode(err) != "T0412" {
		t.Fatalf("unsupported extension result error = %v, want T0412", err)
	}

	expr = MustCompile(`$badResult()`)
	if err := expr.RegisterExts(map[string]Extension{
		"badResult": {Func: func() any { return math.Inf(1) }},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := expr.Eval(nil); normalizationTestErrorCode(err) != "D1001" {
		t.Fatalf("non-finite extension result error = %v, want D1001", err)
	}
}

func TestRegisterVarsNormalizationHasFiniteOperationBudget(t *testing.T) {
	var shared any = map[string]any{"leaf": true}
	for range 17 {
		shared = map[string]any{"left": shared, "right": shared}
	}
	err := MustCompile(`1`).RegisterVars(map[string]interface{}{"wide": shared})
	if !errors.Is(err, errRegistrationNormalizationBudget) {
		t.Fatalf("RegisterVars error = %v, want operation budget", err)
	}
}

func TestRegisterVarsRejectsInaccessibleReflectionWithoutPanic(t *testing.T) {
	type privateValue struct {
		hidden int
	}
	inaccessible := reflect.ValueOf(privateValue{hidden: 1}).Field(0)
	if err := MustCompile(`1`).RegisterVars(map[string]interface{}{"hidden": inaccessible}); normalizationRegistrationErrorCode(err) != "T0412" {
		t.Fatalf("RegisterVars error = %v, want T0412", err)
	}
}

func TestPerEvaluationBindingsUseActiveCancellationAndBudget(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := MustCompile(`$binding`).EvalNoInputWithOptions(EvalOptions{
		Context:  canceled,
		Bindings: map[string]any{"binding": map[string]any{"value": true}},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled binding normalization error = %v, want context.Canceled", err)
	}

	if _, err := MustCompile(`$binding`).EvalNoInputWithOptions(EvalOptions{
		Bindings:      map[string]any{"binding": []any{1.0, 2.0, 3.0}},
		MaxOperations: 2,
	}); normalizationTestErrorCode(err) != "U1001" {
		t.Fatalf("binding operation budget error = %v, want U1001", err)
	}
}

func normalizationRegistrationErrorCode(err error) string {
	var jsonataError *Error
	if errors.As(err, &jsonataError) {
		return jsonataError.Code
	}
	return ""
}

func normalizationTestErrorCode(err error) string {
	var jsonataError *Error
	if errors.As(err, &jsonataError) {
		return jsonataError.Code
	}
	return ""
}
