package evaluator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
)

// TestStaticPlansAgainstForcedFullEvaluation keeps the public fast-path
// wrappers honest. A non-nil context disables all immutable plans, so each
// case compares the normal public call with the same expression evaluated by
// the complete evaluator. The cases are deliberately table-driven: aliases,
// unsupported values, and unusual JSON representations are easy to add
// without creating plan-specific test assumptions.
func TestStaticPlansAgainstForcedFullEvaluation(t *testing.T) {
	valid := func(fields ...any) map[string]any {
		out := make(map[string]any, len(fields)/2)
		for i := 0; i < len(fields); i += 2 {
			out[fields[i].(string)] = fields[i+1]
		}
		return out
	}
	shared := valid("value", 3)
	cycle := valid("loop", nil)
	cycle["loop"] = cycle
	sharedOrders := []any{valid("status", "paid", "amount", 2, "price", 2, "quantity", 3)}
	cases := []struct {
		name  string
		expr  string
		input any
	}{
		{"path scalar", "customer.profile.tier", valid("customer", valid("profile", valid("tier", "gold")))},
		{"path missing", "customer.profile.tier", valid("customer", valid("profile", valid()))},
		{"path null", "customer.profile.tier", valid("customer", valid("profile", valid("tier", nil)))},
		{"path aliases", "customer.profile.tier", valid("customer", valid("profile", valid("tier", "gold")), "alias", shared)},
		{"comparison string", `customer.profile.tier = "gold"`, valid("customer", valid("profile", valid("tier", "gold")))},
		{"comparison numeric", "amount = 7", valid("amount", int64(7))},
		{"comparison numeric types", "amount != 7", valid("amount", json.Number("7"))},
		{"comparison missing", "amount = 7", valid()},
		{"filter empty", `orders[status="paid"].amount`, valid("orders", []any{})},
		{"filter one", `orders[status="paid"].amount`, valid("orders", []any{valid("status", "paid", "amount", 2)})},
		{"filter many", `orders[status="paid"].amount`, valid("orders", []any{valid("status", "paid", "amount", 2), valid("status", "pending", "amount", 4), valid("status", "paid", "amount", 3)})},
		{"sum path", "$sum(values)", valid("values", []any{1, 2.5, int64(3)})},
		{"sum filter", `$sum(orders[status="paid"].amount)`, valid("orders", []any{valid("status", "paid", "amount", 2), valid("status", "pending", "amount", 4), valid("status", "paid", "amount", 3)})},
		{"sum product", `$sum(orders[status="paid"].(price * quantity))`, valid("orders", []any{valid("status", "paid", "price", 2, "quantity", 3), valid("status", "pending", "price", 4, "quantity", 9)})},
		{"contains match", `$contains(label, /quick|fox/)`, valid("label", "the quick brown fox")},
		{"contains no match", `$contains(label, /quick|fox/)`, valid("label", "the slow brown dog")},
		{"contains escaped", `$contains(label, /a/)`, valid("label", "\\u0061")},
		{"map one", `$map(orders, function($item){$item.price * $item.quantity})`, valid("orders", sharedOrders)},
		{"map many", `$map(orders, function($item){$item.price * $item.quantity})`, valid("orders", []any{valid("price", 2, "quantity", 3), valid("price", 4, "quantity", 5)})},
		{"transform match", `$ ~> |orders[status="pending"]|{"total": price * quantity}|`, valid("orders", []any{valid("status", "pending", "price", 2, "quantity", 3)})},
		{"transform no match", `$ ~> |orders[status="paid"]|{"total": price * quantity}|`, valid("orders", []any{valid("status", "pending", "price", 2, "quantity", 3)})},
		{"transform duplicate update", `$ ~> |orders[status="pending"]|{"total": price * quantity, "total": 99}|`, valid("orders", []any{valid("status", "pending", "price", 2, "quantity", 3)})},
		{"descendant sum", "$sum(payload.**.value)", valid("payload", valid("a", valid("value", 2), "b", []any{valid("value", 3)}))},
		{"descendant missing", "$sum(payload.**.value)", valid("payload", valid("a", valid("label", "none")))},
		{"cycle fallback", "customer.profile.tier", cycle},
		{"nonfinite fallback", "customer.profile.tier", valid("customer", valid("profile", valid("tier", "gold")), "bad", math.NaN())},
		{"nested arrays fallback", `$sum(orders[status="paid"].amount)`, valid("orders", []any{[]any{valid("status", "paid", "amount", 2)}})},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expr := jsonata.MustCompile(test.expr)
			fast, fastErr := expr.Eval(test.input)
			full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: context.Background()})
			assertSameEvaluation(t, test.expr, fast, fastErr, full, fullErr)
		})
	}
}

func TestStaticPlansGeneratedScalarMatrix(t *testing.T) {
	values := []any{int(7), int8(7), int16(7), int32(7), int64(7), uint(7), uint8(7), uint16(7), uint32(7), uint64(7), float32(7), float64(7), json.Number("7")}
	for index, scalar := range values {
		cases := []struct {
			expr  string
			input any
		}{
			{"a.b", map[string]any{"a": map[string]any{"b": scalar}}},
			{"a = 7", map[string]any{"a": scalar}},
			{"$sum(values)", map[string]any{"values": []any{scalar}}},
			{`orders[status="paid"].amount`, map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": scalar}}}},
			{`$sum(orders[status="paid"].amount)`, map[string]any{"orders": []any{map[string]any{"status": "paid", "amount": scalar}}}},
			{`$map(orders, function($item){$item.price * $item.quantity})`, map[string]any{"orders": []any{map[string]any{"price": scalar, "quantity": 2}}}},
			{`$ ~> |orders[status="paid"]|{"total": price * quantity}|`, map[string]any{"orders": []any{map[string]any{"status": "paid", "price": scalar, "quantity": 2}}}},
			{"$sum(payload.**.value)", map[string]any{"payload": map[string]any{"nested": map[string]any{"value": scalar}}}},
		}
		for caseIndex, test := range cases {
			t.Run(fmt.Sprintf("scalar-%02d-case-%02d", index, caseIndex), func(t *testing.T) {
				expr := jsonata.MustCompile(test.expr)
				fast, fastErr := expr.Eval(test.input)
				full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: context.Background()})
				assertSameEvaluation(t, test.expr, fast, fastErr, full, fullErr)
			})
		}
	}
}

func TestStaticPlansAgainstForcedFullRawScalarEvaluation(t *testing.T) {
	cases := []struct {
		name string
		expr string
		json string
	}{
		{"path number", "customer.profile.amount", `{"customer":{"profile":{"amount":7}}}`},
		{"path escaped key", `customer["profile"]`, `{"customer":{"profile":7}}`},
		{"path duplicate key", "a.b", `{"a":{"b":1,"b":2}}`},
		{"path duplicate container", "a.b", `{"a":{"b":1,"b":{"x":2}}}`},
		{"path null", "a.b", `{"a":{"b":null}}`},
		{"path invalid", "a.b", `{"a":{"b":1}`},
		{"comparison true", "active = true", `{"active":true}`},
		{"comparison escaped string", `label = "a"`, `{"label":"\\u0061"}`},
		{"comparison duplicate", `a = 2`, `{"a":1,"a":2}`},
		{"contains escaped", `$contains(label, /a/)`, `{"label":"\\u0061"}`},
		{"contains duplicate", `$contains(label, /a/)`, `{"label":"b","label":"a"}`},
		{"contains null", `$contains(label, /a/)`, `{"label":null}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expr := jsonata.MustCompile(test.expr)
			fast, fastErr := expr.EvalBytes([]byte(test.json))
			full, fullErr := expr.EvalBytesWithOptions([]byte(test.json), jsonata.EvalOptions{Context: context.Background()})
			if (fastErr == nil) != (fullErr == nil) {
				t.Fatalf("%s: fast=(%s,%v), full=(%s,%v)", test.expr, fast, fastErr, full, fullErr)
			}
			if fastErr == nil {
				var fastValue, fullValue any
				if err := json.Unmarshal(fast, &fastValue); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(full, &fullValue); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(fastValue, fullValue) {
					t.Fatalf("%s: fast=%s, full=%s", test.expr, fast, full)
				}
			}
		})
	}
}

func TestStaticPlansRespectRegistryOverrides(t *testing.T) {
	tests := []struct {
		caseName string
		expr     string
		data     any
		function string
	}{
		{"sum", "$sum(values)", map[string]any{"values": []any{1, 2}}, "sum"},
		{"map", `$map(orders, function($item){$item.price * $item.quantity})`, map[string]any{"orders": []any{map[string]any{"price": 2, "quantity": 3}}}, "map"},
		{"contains", `$contains(label, /a/)`, map[string]any{"label": "a"}, "contains"},
	}
	for _, test := range tests {
		t.Run(test.caseName, func(t *testing.T) {
			expr := jsonata.MustCompile(test.expr)
			var function any
			switch test.function {
			case "sum":
				function = func(any) int { return 99 }
			case "map":
				function = func(any, any) int { return 99 }
			case "contains":
				function = func(any, any) int { return 99 }
			}
			if err := expr.RegisterExts(map[string]jsonata.Extension{test.function: {Func: function}}); err != nil {
				t.Fatal(err)
			}
			got, err := expr.Eval(test.data)
			if err != nil || !reflect.DeepEqual(got, 99) {
				t.Fatalf("override result = %#v, %v; want 99", got, err)
			}
		})
	}
}

func assertSameEvaluation(t *testing.T, expression string, fast any, fastErr error, full any, fullErr error) {
	t.Helper()
	if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
		t.Fatalf("%s: fast=(%#v,%v), full=(%#v,%v)", expression, fast, fastErr, full, fullErr)
	}
	if fastErr != nil && fmt.Sprint(fastErr) != fmt.Sprint(fullErr) {
		t.Fatalf("%s: fast error=%v, full error=%v", expression, fastErr, fullErr)
	}
}

func TestStaticPlanRawOutputIsCanonical(t *testing.T) {
	expr := jsonata.MustCompile("a.b")
	got, err := expr.EvalBytes([]byte(`{"a":{"b":1e2}}`))
	if err != nil || !bytes.Equal(got, []byte("100")) {
		t.Fatalf("got %q, %v; want canonical 100", got, err)
	}
	if strings.Contains(string(got), "e") {
		t.Fatal("raw fast path retained noncanonical numeric spelling")
	}
}
