package evaluator

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestSelectValueTransfersUniquePredicateBindings(t *testing.T) {
	contextVars := map[string]any{"$focus": "context"}
	boundVars := map[string]any{"$join": "bound"}
	base := value.Array{Items: []any{
		contextual{v: map[string]any{"keep": true, "id": "context"}, vars: contextVars},
		bound{v: map[string]any{"keep": true, "id": "bound"}, vars: boundVars},
		map[string]any{"keep": true, "id": "plain"},
	}}
	rootVars := map[string]any{"$rootBinding": "root"}
	result, err := selectValue(base, syntax.Name{Value: "keep"}, state{
		current: map[string]any{"container": true},
		vars:    rootVars,
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, ok := result.(sequence)
	if !ok || len(selected) != 3 {
		t.Fatalf("selected = %#v, want three-value sequence", result)
	}

	frames := make([]contextual, len(selected))
	for index, item := range selected {
		frame, ok := item.(contextual)
		if !ok {
			t.Fatalf("selected[%d] = %T, want contextual", index, item)
		}
		frames[index] = frame
		if frame.vars["#"] != index {
			t.Fatalf("selected[%d] index = %#v", index, frame.vars["#"])
		}
		if frame.vars["$rootBinding"] != "root" {
			t.Fatalf("selected[%d] lost root binding: %#v", index, frame.vars)
		}
	}
	if frames[0].vars["$focus"] != "context" || frames[1].vars["$join"] != "bound" {
		t.Fatalf("selected bindings = %#v, %#v", frames[0].vars, frames[1].vars)
	}

	frames[0].vars["$rootBinding"] = "changed"
	frames[0].vars["$mutation"] = true
	if rootVars["$rootBinding"] != "root" || contextVars["$rootBinding"] != nil || boundVars["$rootBinding"] != nil {
		t.Fatal("selected predicate bindings alias an input binding map")
	}
	for index := 1; index < len(frames); index++ {
		if frames[index].vars["$rootBinding"] != "root" || frames[index].vars["$mutation"] != nil {
			t.Fatalf("selected predicate bindings alias sibling %d: %#v", index, frames[index].vars)
		}
	}
	parent, ok := frames[0].parent.(contextual)
	if !ok || parent.vars["$rootBinding"] != "root" {
		t.Fatalf("selected predicate bindings alias parent: %#v", frames[0].parent)
	}
}

func TestSelectValueBindingAndParentSemantics(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		input      any
		want       any
	}{
		{
			name:       "nested focus bindings",
			expression: `groups@$group.items@$item[$item.active and $item.value > $group.limit].{'group':$group.id,'value':$item.value}`,
			input: map[string]any{"groups": []any{
				map[string]any{"id": "a", "limit": 1.0, "items": []any{
					map[string]any{"value": 1.0, "active": true},
					map[string]any{"value": 2.0, "active": true},
				}},
				map[string]any{"id": "b", "limit": 2.0, "items": []any{
					map[string]any{"value": 3.0, "active": true},
					map[string]any{"value": 4.0, "active": false},
				}},
			}},
			want: []any{
				map[string]any{"group": "a", "value": 2.0},
				map[string]any{"group": "b", "value": 3.0},
			},
		},
		{
			name:       "source and selected index bindings",
			expression: `items#$source[value > 1]#$selected.{'source':$source,'selected':$selected,'value':value}`,
			input: map[string]any{"items": []any{
				map[string]any{"value": 1.0},
				map[string]any{"value": 2.0},
				map[string]any{"value": 3.0},
			}},
			want: []any{
				map[string]any{"source": 1, "selected": 0, "value": 2.0},
				map[string]any{"source": 2, "selected": 1, "value": 3.0},
			},
		},
		{
			name:       "parent lineage after predicate",
			expression: `Account.Order.Product[Price > 10].{'sku':SKU,'order':%.OrderID}`,
			input: map[string]any{"Account": map[string]any{"Order": []any{
				map[string]any{"OrderID": "one", "Product": []any{
					map[string]any{"SKU": "low", "Price": 5.0},
					map[string]any{"SKU": "high", "Price": 12.0},
				}},
			}}},
			want: map[string]any{"sku": "high", "order": "one"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evalPhase2(t, test.expression, test.input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("result = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSelectValueBindingsRemainIsolatedThroughTransform(t *testing.T) {
	input := map[string]any{"items": []any{
		map[string]any{"value": 1.0},
		map[string]any{"value": 2.0},
		map[string]any{"value": 3.0},
	}}
	wantInput := map[string]any{"items": []any{
		map[string]any{"value": 1.0},
		map[string]any{"value": 2.0},
		map[string]any{"value": 3.0},
	}}
	got, err := evalPhase2(t, `$ ~> |items[value > 1]|{"selected":true}|`, input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"items": []any{
		map[string]any{"value": 1.0},
		map[string]any{"value": 2.0, "selected": true},
		map[string]any{"value": 3.0, "selected": true},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("transform result = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(input, wantInput) {
		t.Fatalf("transform mutated input: %#v", input)
	}
}

func TestSelectValueBindingsAreConcurrentEvaluationLocal(t *testing.T) {
	node, parseErr := syntax.Parse(`items#$source[value > 1]#$selected.{'source':$source,'selected':$selected,'value':value}`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	const workers = 32
	var wait sync.WaitGroup
	errorsSeen := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wait.Add(1)
		go func() {
			defer wait.Done()
			input := map[string]any{"items": []any{
				map[string]any{"value": float64(worker)},
				map[string]any{"value": float64(worker + 2)},
			}}
			got, err := Eval(node, input)
			if err != nil {
				errorsSeen <- err
				return
			}
			want := map[string]any{"source": 1, "selected": 0, "value": float64(worker + 2)}
			if worker > 1 {
				wantValues := []any{
					map[string]any{"source": 0, "selected": 0, "value": float64(worker)},
					map[string]any{"source": 1, "selected": 1, "value": float64(worker + 2)},
				}
				if !reflect.DeepEqual(got, wantValues) {
					errorsSeen <- fmt.Errorf("worker %d result = %#v, want %#v", worker, got, wantValues)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				errorsSeen <- fmt.Errorf("worker %d result = %#v, want %#v", worker, got, want)
			}
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Error(err)
	}
}
