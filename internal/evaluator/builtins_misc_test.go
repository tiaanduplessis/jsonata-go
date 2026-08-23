package evaluator

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestMiscBuiltinSignatures(t *testing.T) {
	want := map[string]string{
		"clone":  "<(oa)-:o>",
		"random": "<:n>",
	}
	for name, signature := range want {
		spec, ok := builtinSpecFor(name)
		if !ok {
			t.Fatalf("missing builtin %q", name)
		}
		if spec.signature != signature {
			t.Errorf("%s signature = %q, want %q", name, spec.signature, signature)
		}
	}
}

func TestRandomRangeAndDistributionSanity(t *testing.T) {
	const (
		samples = 8192
		buckets = 16
	)
	counts := [buckets]int{}
	total := 0.0
	for range samples {
		result, err := builtinRandom(state{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		number, ok := result.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number >= 1 {
			t.Fatalf("$random() = %#v, want finite number in [0,1)", result)
		}
		total += number
		counts[int(number*buckets)]++
	}
	mean := total / samples
	if mean < 0.45 || mean > 0.55 {
		t.Fatalf("$random() sample mean = %v, want broad sanity range [0.45,0.55]", mean)
	}
	for index, count := range counts {
		if count < samples/(buckets*2) {
			t.Fatalf("$random() bucket %d count = %d, want at least %d", index, count, samples/(buckets*2))
		}
	}
}

func TestRandomConcurrentEvaluation(t *testing.T) {
	node, err := syntax.Parse(`$random()`)
	if err != nil {
		t.Fatal(err)
	}
	const (
		workers     = 16
		evaluations = 256
	)
	var group sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range evaluations {
				result, evalErr := Eval(node, nil)
				if evalErr != nil {
					errorsSeen <- evalErr
					return
				}
				number, ok := result.(float64)
				if !ok || number < 0 || number >= 1 || math.IsNaN(number) || math.IsInf(number, 0) {
					errorsSeen <- errors.New("$random returned an invalid number")
					return
				}
			}
		}()
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
}

func TestRandomValidationAndCancellation(t *testing.T) {
	spec, _ := builtinSpecFor("random")
	if _, err := (builtinValue{spec: spec}).invoke(state{}, []any{1.0}); !hasJSONataCode(err, "T0410") {
		t.Fatalf("$random(1) error = %v, want T0410", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := builtinRandom(state{runtime: newEvalRuntime(Options{Context: canceled})}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled $random error = %v, want context.Canceled", err)
	}
}

func TestCloneParityAndIndependentCopy(t *testing.T) {
	node, err := syntax.Parse(`$clone($)`)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{
		"name":  "source",
		"items": []any{1.0, map[string]any{"nested": true}},
	}
	result, evalErr := Eval(node, input)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	want := map[string]any{
		"name":  "source",
		"items": []any{1.0, map[string]any{"nested": true}},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("$clone($) = %#v, want %#v", result, want)
	}
	input["name"] = "mutated"
	input["items"].([]any)[1].(map[string]any)["nested"] = false
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("clone changed after source mutation: %#v", result)
	}

	callableNode, err := syntax.Parse(`$clone({"f": function(){1}, "a": 2})`)
	if err != nil {
		t.Fatal(err)
	}
	callableResult, evalErr := Eval(callableNode, nil)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	if !reflect.DeepEqual(callableResult, map[string]any{"f": "", "a": float64(2)}) {
		t.Fatalf("callable clone = %#v, want callable converted to empty string", callableResult)
	}

	for expression, want := range map[string]any{
		`$clone([1, undefined, null])`:        []any{1.0, nil},
		`$clone({"a": undefined, "b": null})`: map[string]any{"b": nil},
	} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		got, evalErr := Eval(node, nil)
		if evalErr != nil {
			t.Fatalf("%s: %v", expression, evalErr)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, want %#v", expression, got, want)
		}
	}

	contextNode, err := syntax.Parse(`$clone()`)
	if err != nil {
		t.Fatal(err)
	}
	contextResult, evalErr := Eval(contextNode, map[string]any{"a": 1.0})
	if evalErr != nil || !reflect.DeepEqual(contextResult, map[string]any{"a": 1.0}) {
		t.Fatalf("context clone = %#v, %v", contextResult, evalErr)
	}
}

func TestClonePreservesPrivateSemanticsAndOrder(t *testing.T) {
	nested := map[string]any{"value": "original"}
	source := value.OrderedObject{
		Fields: map[string]any{
			"10":        10.0,
			"2":         2.0,
			"undefined": value.Undefined,
			"nested":    nested,
			"array": value.Array{
				Items: []any{nil, value.Undefined, contextual{v: "unwrapped"}},
				Keep:  true,
			},
		},
		Order: []string{"2", "10", "undefined", "nested", "array"},
	}
	result, err := builtinClone(state{}, []any{source})
	if err != nil {
		t.Fatal(err)
	}
	cloned, ok := result.(value.OrderedObject)
	if !ok {
		t.Fatalf("clone type = %T, want value.OrderedObject", result)
	}
	if !reflect.DeepEqual(cloned.Order, source.Order) {
		t.Fatalf("clone order = %#v, want %#v", cloned.Order, source.Order)
	}
	if !value.IsUndefined(cloned.Fields["undefined"]) {
		t.Fatalf("clone undefined field = %#v, want Undefined", cloned.Fields["undefined"])
	}
	array, ok := cloned.Fields["array"].(value.Array)
	if !ok || !array.Keep || array.Items[0] != nil || !value.IsUndefined(array.Items[1]) || array.Items[2] != "unwrapped" {
		t.Fatalf("clone array semantics = %#v", cloned.Fields["array"])
	}
	nested["value"] = "source mutation"
	source.Order[0] = "source mutation"
	if cloned.Fields["nested"].(map[string]any)["value"] != "original" || cloned.Order[0] != "2" {
		t.Fatalf("clone retained source references: %#v", cloned)
	}
	cloned.Fields["nested"].(map[string]any)["value"] = "clone mutation"
	if nested["value"] != "source mutation" {
		t.Fatal("source retained clone reference")
	}
}

func TestCloneUndefinedAndSignatureErrors(t *testing.T) {
	spec, _ := builtinSpecFor("clone")
	clone := builtinValue{spec: spec}
	result, err := clone.invoke(state{current: value.Undefined}, []any{value.Undefined})
	if err != nil || !value.IsUndefined(result) {
		t.Fatalf("$clone(undefined) = %#v, %v; want Undefined", result, err)
	}
	for _, input := range []any{nil, true, 1.0, "object"} {
		if _, err := clone.invoke(state{}, []any{input}); !hasJSONataCode(err, "T0410") {
			t.Errorf("$clone(%#v) error = %v, want T0410", input, err)
		}
		if _, err := clone.invoke(state{current: input}, nil); !hasJSONataCode(err, "T0411") {
			t.Errorf("$clone() with context %#v error = %v, want T0411", input, err)
		}
	}
}

func TestGroupedArrayChoiceClassifiesWithoutChangingStandaloneCoercion(t *testing.T) {
	grouped, err := parseFunctionSignature("<(oa)-:o>")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareBuiltinSignatureArgs(grouped, []any{1.0}, nil, nil); !hasJSONataCode(err, "T0410") {
		t.Fatalf("grouped array choice scalar error = %v, want T0410", err)
	}
	if _, err := prepareBuiltinSignatureArgs(grouped, nil, 1.0, nil); !hasJSONataCode(err, "T0411") {
		t.Fatalf("grouped array choice context error = %v, want T0411", err)
	}
	for _, input := range []any{value.Array{Items: []any{1.0}}, []any{1.0}, sequence{1.0}, map[string]any{"a": 1.0}} {
		if _, err := prepareBuiltinSignatureArgs(grouped, []any{input}, nil, nil); err != nil {
			t.Errorf("grouped signature rejected %T: %v", input, err)
		}
	}

	standalone, err := parseFunctionSignature("<a:a>")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBuiltinSignatureArgs(standalone, []any{1.0}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	array, ok := prepared[0].(value.Array)
	if !ok || !reflect.DeepEqual(array.Items, []any{1.0}) {
		t.Fatalf("standalone array coercion = %#v, want [1]", prepared[0])
	}
}

func TestCloneRejectsCyclesAndHostileValues(t *testing.T) {
	cyclicMap := map[string]any{}
	cyclicMap["self"] = cyclicMap
	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice
	tests := []struct {
		name  string
		input any
	}{
		{name: "map cycle", input: cyclicMap},
		{name: "slice cycle", input: cyclicSlice},
		{name: "channel", input: map[string]any{"bad": make(chan int)}},
		{name: "function", input: map[string]any{"bad": func() {}}},
		{name: "invalid number", input: map[string]any{"bad": json.Number("invalid")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := builtinClone(state{runtime: newEvalRuntime(Options{MaxCallDepth: 32, MaxOperations: 256})}, []any{test.input})
			want := "T0412"
			if test.name == "invalid number" {
				want = "D1001"
			}
			if !hasJSONataCode(err, want) {
				t.Fatalf("clone error = %v, want %s", err, want)
			}
		})
	}
}

func TestCloneRuntimeBounds(t *testing.T) {
	deep := map[string]any{"leaf": true}
	for range 32 {
		deep = map[string]any{"next": deep}
	}
	if _, err := builtinClone(state{runtime: newEvalRuntime(Options{MaxCallDepth: 8, MaxOperations: 1000})}, []any{deep}); !hasJSONataCode(err, "U1001") {
		t.Fatalf("depth-limited clone error = %v, want U1001", err)
	}
	wide := value.Array{Items: make([]any, 64)}
	if _, err := builtinClone(state{runtime: newEvalRuntime(Options{MaxCallDepth: 100, MaxOperations: 8})}, []any{wide}); !hasJSONataCode(err, "U1001") {
		t.Fatalf("operation-limited clone error = %v, want U1001", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := builtinClone(state{runtime: newEvalRuntime(Options{Context: canceled})}, []any{map[string]any{"a": 1.0}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled clone error = %v, want context.Canceled", err)
	}
}

func TestCloneSharedValuesAndConcurrentCalls(t *testing.T) {
	shared := map[string]any{"value": 1.0}
	source := map[string]any{"left": shared, "right": shared}
	result, err := builtinClone(state{}, []any{source})
	if err != nil {
		t.Fatal(err)
	}
	clone := result.(map[string]any)
	clone["left"].(map[string]any)["value"] = 2.0
	if clone["right"].(map[string]any)["value"] != 1.0 {
		t.Fatal("clone retained alias between separate JSON object members")
	}

	const workers = 16
	var group sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 128 {
				result, cloneErr := builtinClone(state{}, []any{source})
				if cloneErr != nil {
					errorsSeen <- cloneErr
					return
				}
				if !reflect.DeepEqual(result, source) {
					errorsSeen <- errors.New("concurrent clone differs from source")
					return
				}
			}
		}()
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
}
