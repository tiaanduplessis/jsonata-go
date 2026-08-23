package syntax

import "testing"

func TestParenthesizedBindingKeepsItsOwnBlock(t *testing.T) {
	tests := []struct {
		name       string
		expr       string
		outerCount int
		innerCount int
	}{
		{
			name:       "variables/case010",
			expr:       `($foo := "defined"; ($foo := nothing); $foo)`,
			outerCount: 3,
			innerCount: 1,
		},
		{
			name:       "variables/case011",
			expr:       `($foo := "defined"; ($foo := nothing; $foo))`,
			outerCount: 2,
			innerCount: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node, err := Parse(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			outer, ok := node.(Block)
			if !ok || len(outer.Expressions) != test.outerCount {
				t.Fatalf("got %#v, want outer block", node)
			}
			innerIndex := 1
			inner, ok := outer.Expressions[innerIndex].(Block)
			if !ok || len(inner.Expressions) != test.innerCount {
				t.Fatalf("got %#v, want nested block with %d expressions", outer.Expressions[innerIndex], test.innerCount)
			}
			if _, ok := inner.Expressions[0].(Bind); !ok {
				t.Fatalf("got %#v, want nested binding", inner.Expressions[0])
			}
		})
	}
}
