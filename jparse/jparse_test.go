package jparse

import (
	"errors"
	"regexp"
	"testing"
)

func TestParseLiteralsAndRootShapes(t *testing.T) {
	tests := []struct {
		expr string
		kind any
		text string
	}{
		{`"hello"`, &StringNode{}, `"hello"`},
		{"42", &NumberNode{}, "42"},
		{"true", &BooleanNode{}, "true"},
		{"null", &NullNode{}, "null"},
		{"foo", &PathNode{}, "foo"},
		{"foo.bar", &PathNode{}, "foo.bar"},
		{"[1, 2]", &ArrayNode{}, "[1, 2]"},
	}
	for _, test := range tests {
		node, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		switch test.kind.(type) {
		case *StringNode:
			if _, ok := node.(*StringNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		case *NumberNode:
			if _, ok := node.(*NumberNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		case *BooleanNode:
			if _, ok := node.(*BooleanNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		case *NullNode:
			if _, ok := node.(*NullNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		case *PathNode:
			if _, ok := node.(*PathNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		case *ArrayNode:
			if _, ok := node.(*ArrayNode); !ok {
				t.Fatalf("Parse(%q) returned %T", test.expr, node)
			}
		}
		if got := node.String(); got != test.text {
			t.Errorf("Parse(%q).String() = %q, want %q", test.expr, got, test.text)
		}
	}
}

func TestParseStructuredNodes(t *testing.T) {
	tests := []struct {
		expr string
		want any
	}{
		{`/ab+/`, &RegexNode{}},
		{"foo[0]", &PathNode{}},
		{"foo[]", &PathNode{}},
		{"*", &WildcardNode{}},
		{"**", &DescendentNode{}},
		{"1 + 2", &NumericOperatorNode{}},
		{"1 = 1", &ComparisonOperatorNode{}},
		{"true and false", &BooleanOperatorNode{}},
		{`"a" & "b"`, &StringConcatenationNode{}},
		{"Product^(Price, >Name)", &SortNode{}},
		{"$x := 1", &AssignmentNode{}},
		{"$x(1)", &FunctionCallNode{}},
		{"$x(?)", &PartialNode{}},
		{"$x ~> $f", &FunctionApplicationNode{}},
		{"1 ? 2 : 3", &ConditionalNode{}},
		{"function($x){$x}", &LambdaNode{}},
	}
	for _, test := range tests {
		node, err := Parse(test.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expr, err)
		}
		switch test.want.(type) {
		case *RegexNode:
			if _, ok := node.(*RegexNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *PredicateNode:
			if _, ok := node.(*PredicateNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *PathNode:
			if _, ok := node.(*PathNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *WildcardNode:
			if _, ok := node.(*WildcardNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *DescendentNode:
			if _, ok := node.(*DescendentNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *RangeNode:
			if _, ok := node.(*RangeNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *NumericOperatorNode:
			if _, ok := node.(*NumericOperatorNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *ComparisonOperatorNode:
			if _, ok := node.(*ComparisonOperatorNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *BooleanOperatorNode:
			if _, ok := node.(*BooleanOperatorNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *StringConcatenationNode:
			if _, ok := node.(*StringConcatenationNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *AssignmentNode:
			if _, ok := node.(*AssignmentNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *FunctionCallNode:
			if _, ok := node.(*FunctionCallNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *PartialNode:
			if _, ok := node.(*PartialNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *FunctionApplicationNode:
			if _, ok := node.(*FunctionApplicationNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *ConditionalNode:
			if _, ok := node.(*ConditionalNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		case *LambdaNode:
			if _, ok := node.(*LambdaNode); !ok {
				t.Errorf("%q returned %T", test.expr, node)
			}
		}
	}
}

func TestSortString(t *testing.T) {
	node, err := Parse("Product^(Price, >Name)")
	if err != nil {
		t.Fatal(err)
	}
	if got := node.String(); got != "Product^(Price, >Name)" {
		t.Fatalf("sort String() = %q", got)
	}
}

func TestCompatibilityPathAndObjectShapes(t *testing.T) {
	node, err := Parse(`path[type="home"][0][]`)
	if err != nil {
		t.Fatal(err)
	}
	path, ok := node.(*PathNode)
	if !ok || !path.KeepArrays || len(path.Steps) != 1 {
		t.Fatalf("unexpected path shape: %#v", node)
	}
	predicate, ok := path.Steps[0].(*PredicateNode)
	if !ok || len(predicate.Filters) != 2 {
		t.Fatalf("unexpected predicate shape: %#v", path.Steps[0])
	}
	comparison, ok := predicate.Filters[0].(*ComparisonOperatorNode)
	if !ok {
		t.Fatalf("unexpected comparison shape: %#v", predicate.Filters[0])
	}
	if _, ok := comparison.LHS.(*PathNode); !ok {
		t.Fatalf("comparison name was not pathified: %#v", comparison.LHS)
	}

	node, err = Parse(`{"one": 1}`)
	if err != nil {
		t.Fatal(err)
	}
	object := node.(*ObjectNode)
	if _, ok := object.Pairs[0][0].(*StringNode); !ok {
		t.Fatalf("quoted object key = %T", object.Pairs[0][0])
	}
}

func TestParseErrorShape(t *testing.T) {
	_, err := Parse("[")
	if err == nil {
		t.Fatal("Parse did not reject malformed expression")
	}
	var parseErr *Error
	if !errors.As(err, &parseErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if parseErr.Type != ErrUnexpectedEOF {
		t.Errorf("error type = %d, want %d", parseErr.Type, ErrUnexpectedEOF)
	}
	if parseErr.Position < 0 {
		t.Errorf("invalid error position %d", parseErr.Position)
	}
}

func TestEnumsAndPublicMethods(t *testing.T) {
	if ParamTypeNumber.String() != "n" || (ParamTypeNumber|ParamTypeString).String() != "(ns)" {
		t.Fatal("parameter type formatting changed")
	}
	if ParamOptional.String() != "?" || ParamVariadic.String() != "+" || ParamContextable.String() != "-" {
		t.Fatal("parameter option formatting changed")
	}
	if NumericDivide.String() != "/" || ComparisonGreaterEqual.String() != ">=" || BooleanOr.String() != "or" {
		t.Fatal("operator formatting changed")
	}
	name := NameNode{Value: "a b", escaped: true}
	if !name.Escaped() || name.String() != "`a b`" {
		t.Fatal("escaped name behavior changed")
	}
	r := RegexNode{Value: regexp.MustCompile("x")}
	if r.String() != "/x/" {
		t.Fatalf("regex formatting = %q", r.String())
	}
	node, err := Parse("`a b`")
	if err != nil {
		t.Fatal(err)
	}
	path := node.(*PathNode)
	if !path.Steps[0].(*NameNode).Escaped() {
		t.Fatal("quoted name lost escaped marker")
	}
}

var (
	_ Node = (*StringNode)(nil)
	_ Node = (*NumberNode)(nil)
	_ Node = (*BooleanNode)(nil)
	_ Node = (*NullNode)(nil)
	_ Node = (*RegexNode)(nil)
	_ Node = (*VariableNode)(nil)
	_ Node = (*PathNode)(nil)
	_ Node = (*ArrayNode)(nil)
	_ Node = (*ObjectNode)(nil)
	_ Node = (*LambdaNode)(nil)
	_ Node = (*TypedLambdaNode)(nil)
)
