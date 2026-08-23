package evaluator

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestCollectionCallbackArityAndParent(t *testing.T) {
	fn := &collectionStateProbe{}
	object := map[string]any{"a": 1.0}
	if _, err := builtinEach(state{current: object}, []any{fn}); err != nil {
		t.Fatal(err)
	}
	if !fn.currentSeen || fn.current != 1.0 {
		t.Fatalf("callback current = %#v, want 1", fn.current)
	}
	if got, ok := fn.parent.(map[string]any); !ok || got["a"] != 1.0 {
		t.Fatalf("callback parent = %#v, want source object", fn.parent)
	}

	node, err := syntax.Parse(`$each({"a": 1}, function($a, $b, $c, $d) {$d})`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Eval(node, nil); !hasCollectionCode(err, "T0411") {
		t.Fatalf("oversized callback error = %v, want T0411", err)
	}
}

func TestCollectionSortComparatorIsBooleanAndStable(t *testing.T) {
	node, err := syntax.Parse(`$sort([{"key": 1, "id": "a"}, {"key": 1, "id": "b"}], function($a, $b) {$a.key > $b.key}).id`)
	if err != nil {
		t.Fatal(err)
	}
	got, evalErr := Eval(node, nil)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	if !reflect.DeepEqual(got, []any{"a", "b"}) {
		t.Fatalf("stable sort = %#v, want [a b]", got)
	}

	node, err = syntax.Parse(`$sort([2, 1], function($a, $b) {1})`)
	if err != nil {
		t.Fatal(err)
	}
	if _, evalErr = Eval(node, nil); !hasCollectionCode(evalErr, "T0412") {
		t.Fatalf("non-boolean comparator error = %v, want T0412", evalErr)
	}
}

func TestCollectionAggregationAndRuntimeChecks(t *testing.T) {
	got, err := builtinSum(state{}, []any{value.Array{}})
	if err != nil || got != 0.0 {
		t.Fatalf("sum([]) = %#v, %v; want 0", got, err)
	}
	got, err = builtinSum(state{}, []any{value.Undefined})
	if err != nil || !value.IsUndefined(got) {
		t.Fatalf("sum(undefined) = %#v, %v; want undefined", got, err)
	}
	if _, err := builtinSum(state{}, []any{value.Array{Items: []any{math.Inf(1)}}}); !hasCollectionCode(err, "D1001") {
		t.Fatalf("non-finite sum error = %v, want D1001", err)
	}

	limited := state{runtime: newEvalRuntime(Options{MaxOperations: 1})}
	if _, err := builtinDistinct(limited, []any{value.Array{Items: []any{1.0, 1.0, 2.0}}}); !hasCollectionCode(err, "U1001") {
		t.Fatalf("distinct budget error = %v, want U1001", err)
	}
}

func TestCollectionBuiltinSignatures(t *testing.T) {
	want := map[string]string{
		"average": "<a<n>:n>", "count": "<a:n>", "length": "<s-:n>",
		"max": "<a<n>:n>", "min": "<a<n>:n>", "sum": "<a<n>:n>",
		"type": "<x:s>", "distinct": "<x:x>", "each": "<o-f:a>",
		"join": "<a<s>s?:s>", "keys": "<x-:a<s>>", "lookup": "<x-s:x>",
		"merge": "<a<o>:o>", "reverse": "<a:a>", "shuffle": "<a:a>",
		"sift": "<o-f?:o>", "sort": "<af?:a>", "spread": "<x-:a<o>>", "zip": "<a+>",
	}
	for name, signature := range want {
		spec, ok := builtinSpecFor(name)
		if !ok {
			t.Fatalf("missing collection builtin %q", name)
		}
		if spec.signature != signature {
			t.Errorf("%s signature = %q, want %q", name, spec.signature, signature)
		}
	}
}

type collectionStateProbe struct {
	currentSeen bool
	current     any
	parent      any
}

func (p *collectionStateProbe) callableName() string { return "collection probe" }

func (p *collectionStateProbe) invoke(st state, args []any) (any, error) {
	p.currentSeen = true
	p.current = st.current
	p.parent = st.parent
	return args[0], nil
}

func hasCollectionCode(err error, want string) bool {
	if err == nil {
		return false
	}
	var coded interface{ JSONataCode() string }
	return errors.As(err, &coded) && coded.JSONataCode() == want
}
