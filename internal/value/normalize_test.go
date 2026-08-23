package value

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

type normalizationNamedMap map[string]any
type normalizationNamedSlice []any
type normalizationNamedArray [2]any
type normalizationNamedString string
type normalizationNamedNumber float64

func TestFromJSONSafeCopiesContainersAndPreservesSemantics(t *testing.T) {
	shared := map[string]any{"value": 1.0}
	source := OrderedObject{
		Fields: map[string]any{
			"left":  shared,
			"right": shared,
			"array": Array{Items: []any{nil, Undefined}, Keep: true},
		},
		Order: []string{"left", "right", "array"},
	}
	result, err := FromJSONSafe(source, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	normalized := result.(OrderedObject)
	if !reflect.DeepEqual(normalized.Order, source.Order) {
		t.Fatalf("order = %#v, want %#v", normalized.Order, source.Order)
	}
	array := normalized.Fields["array"].(Array)
	if !array.Keep || array.Items[0] != nil || !IsUndefined(array.Items[1]) {
		t.Fatalf("array semantics = %#v", array)
	}
	normalized.Fields["left"].(map[string]any)["value"] = 2.0
	if shared["value"] != 1.0 {
		t.Fatal("normalization retained a caller-owned map")
	}
	if normalized.Fields["right"].(map[string]any)["value"] != 1.0 {
		t.Fatal("normalization retained aliasing between JSON object members")
	}
	normalized.Order[0] = "changed"
	if source.Order[0] != "left" {
		t.Fatal("normalization retained the caller-owned order slice")
	}
}

func TestFromJSONSafeRejectsCycles(t *testing.T) {
	cyclicMap := map[string]any{}
	cyclicMap["self"] = cyclicMap
	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice
	for name, input := range map[string]any{
		"map":   cyclicMap,
		"slice": cyclicSlice,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := FromJSONSafe(input, 32, nil); !errors.Is(err, ErrCyclicValue) {
				t.Fatalf("error = %v, want ErrCyclicValue", err)
			}
		})
	}
}

func TestFromJSONSafeHonorsDepthBudgetAndCancellation(t *testing.T) {
	deep := map[string]any{"leaf": true}
	for range 8 {
		deep = map[string]any{"next": deep}
	}
	if _, err := FromJSONSafe(deep, 4, nil); !errors.Is(err, ErrNormalizationDepth) {
		t.Fatalf("depth error = %v, want ErrNormalizationDepth", err)
	}

	checks := 0
	budgetError := errors.New("budget exhausted")
	_, err := FromJSONSafe([]any{1.0, 2.0, 3.0}, 16, func() error {
		checks++
		if checks > 2 {
			return budgetError
		}
		return nil
	})
	if !errors.Is(err, budgetError) {
		t.Fatalf("budget error = %v, want supplied error", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = FromJSONSafe(map[string]any{"a": 1.0}, 16, canceled.Err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v, want context.Canceled", err)
	}
}

func TestFromJSONSafeWithMapperMapsNestedLeaves(t *testing.T) {
	result, err := FromJSONSafeWithMapper(map[string]any{
		"items": []any{"keep", "replace"},
	}, 8, nil, func(input any) (any, bool, error) {
		if input == "replace" {
			return Undefined, true, nil
		}
		return input, false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	items := result.(map[string]any)["items"].(Array).Items
	if items[0] != "keep" || !IsUndefined(items[1]) {
		t.Fatalf("mapped items = %#v", items)
	}
}

func TestFromJSONSafeCopiesNamedJSONContainers(t *testing.T) {
	nested := normalizationNamedMap{"value": normalizationNamedNumber(1.5)}
	items := normalizationNamedSlice{nested, normalizationNamedString("ok")}
	array := normalizationNamedArray{items, true}
	input := normalizationNamedMap{
		"array":   array,
		"pointer": &nested,
	}

	result, err := FromJSONSafe(input, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	object := result.(map[string]any)
	normalizedArray := object["array"].(Array)
	normalizedItems := normalizedArray.Items[0].(Array)
	if got := normalizedItems.Items[0].(map[string]any)["value"]; got != 1.5 {
		t.Fatalf("named number = %#v, want 1.5", got)
	}
	if got := normalizedItems.Items[1]; got != "ok" {
		t.Fatalf("named string = %#v, want ok", got)
	}
	if got := object["pointer"].(map[string]any)["value"]; got != 1.5 {
		t.Fatalf("pointer value = %#v, want 1.5", got)
	}

	nested["value"] = 9.0
	items[1] = "changed"
	if got := normalizedItems.Items[0].(map[string]any)["value"]; got != 1.5 {
		t.Fatalf("normalization retained named map ownership: %#v", got)
	}
	if got := normalizedItems.Items[1]; got != "ok" {
		t.Fatalf("normalization retained named slice ownership: %#v", got)
	}
}

func TestFromJSONSafeTreatsTypedNilPublicValuesAsNull(t *testing.T) {
	var ordinaryMap map[string]any
	var ordinarySlice []any
	var namedMap normalizationNamedMap
	var namedSlice normalizationNamedSlice
	var pointer *normalizationNamedMap

	for name, input := range map[string]any{
		"map":         ordinaryMap,
		"slice":       ordinarySlice,
		"named map":   namedMap,
		"named slice": namedSlice,
		"pointer":     pointer,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := FromJSONSafe(input, 8, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
		})
	}
}

func TestFromJSONSafeRejectsNamedCycles(t *testing.T) {
	cyclicMap := normalizationNamedMap{}
	cyclicMap["self"] = cyclicMap
	cyclicSlice := make(normalizationNamedSlice, 1)
	cyclicSlice[0] = cyclicSlice
	var pointerCycle any
	pointerCycle = &pointerCycle

	for name, input := range map[string]any{
		"map":     cyclicMap,
		"slice":   cyclicSlice,
		"pointer": pointerCycle,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := FromJSONSafe(input, 32, nil); !errors.Is(err, ErrCyclicValue) {
				t.Fatalf("error = %v, want ErrCyclicValue", err)
			}
		})
	}
}

func TestFromJSONSafeRejectsUnsupportedAndNonFiniteValues(t *testing.T) {
	unsupported := map[string]any{
		"channel":        make(chan int),
		"function":       func() {},
		"struct":         struct{ Value string }{Value: "no"},
		"non-string map": map[int]any{1: true},
	}
	for name, input := range unsupported {
		t.Run(name, func(t *testing.T) {
			if _, err := FromJSONSafe(input, 8, nil); !errors.Is(err, ErrUnsupportedValue) {
				t.Fatalf("error = %v, want ErrUnsupportedValue", err)
			}
		})
	}

	nonFinite := map[string]any{
		"nan":            math.NaN(),
		"positive inf":   math.Inf(1),
		"negative inf":   math.Inf(-1),
		"invalid number": json.Number("not-a-number"),
	}
	for name, input := range nonFinite {
		t.Run(name, func(t *testing.T) {
			if _, err := FromJSONSafe(input, 8, nil); !errors.Is(err, ErrNonFiniteValue) {
				t.Fatalf("error = %v, want ErrNonFiniteValue", err)
			}
		})
	}
}
