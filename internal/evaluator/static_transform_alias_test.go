package evaluator

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"
)

func TestStaticTransformCloneRejectsRepeatedMapIdentity(t *testing.T) {
	shared := map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0}
	input := map[string]any{
		"account": map[string]any{"orders": []any{shared}},
		"alias":   shared,
	}
	if staticTransformCloneSafe(input) {
		t.Fatal("repeated map identity was accepted by static transform plan")
	}
}

func TestStaticTransformCloneRejectsRepeatedSliceIdentity(t *testing.T) {
	shared := []any{map[string]any{"status": "pending", "price": 2.0, "quantity": 3.0}}
	input := map[string]any{
		"account": map[string]any{"orders": shared},
		"alias":   shared,
	}
	if staticTransformCloneSafe(input) {
		t.Fatal("repeated slice identity was accepted by static transform plan")
	}
}

func TestStaticTransformCloneRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{name: "nil map", input: map[string]any{"value": map[string]any(nil)}},
		{name: "nil slice", input: map[string]any{"value": []any(nil)}},
		{name: "nan", input: map[string]any{"value": math.NaN()}},
		{name: "positive infinity", input: map[string]any{"value": math.Inf(1)}},
		{name: "invalid number", input: map[string]any{"value": json.Number("not-a-number")}},
		{name: "named value", input: map[string]any{"value": int16(1), "other": struct{}{}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if staticTransformCloneSafe(test.input) {
				t.Fatal("invalid input was accepted by static transform plan")
			}
		})
	}
}

func TestStaticTransformCloneHonorsNodeAndDepthBounds(t *testing.T) {
	withinNodeLimit := make(map[string]any, staticTransformMaxNodes-1)
	for index := 0; index < staticTransformMaxNodes-1; index++ {
		withinNodeLimit[strconv.Itoa(index)] = index
	}
	if !staticTransformCloneSafe(withinNodeLimit) {
		t.Fatal("input at the node limit was rejected")
	}

	overNodeLimit := make(map[string]any, staticTransformMaxNodes)
	for index := 0; index < staticTransformMaxNodes; index++ {
		overNodeLimit[strconv.Itoa(index)] = index
	}
	if staticTransformCloneSafe(overNodeLimit) {
		t.Fatal("input over the node limit was accepted")
	}

	if !staticTransformCloneSafe(nestedStaticPathInput(defaultMaxCallDepth)) {
		t.Fatal("input at the depth limit was rejected")
	}
	if staticTransformCloneSafe(nestedStaticPathInput(defaultMaxCallDepth + 1)) {
		t.Fatal("input over the depth limit was accepted")
	}
}

func staticTransformCloneSafe(input any) bool {
	_, ok := staticTransformClone(input)
	return ok
}
