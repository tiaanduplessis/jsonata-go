package syntax

import (
	"reflect"
	"testing"
)

func TestPhase2DiagnosticCodes(t *testing.T) {
	tests := []struct {
		expr string
		code string
		pos  int
	}{
		{`"no closing quote`, "S0101", 17},
		{"`no closing backtick", "S0105", 20},
		{`[1!2]`, "S0204", 3},
		{`@ bar`, "S0211", 1},
		{`1=`, "S0207", 2},
		{`[1,2,3]{"num": $}[true]`, "S0209", 18},
		{`[1,2,3]{"num": $}{"num": $}`, "S0210", 18},
		{`$.7`, "S0213", 3},
		{`$lowercase("Missing close brackets"`, "S0203", 36},
	}
	for _, test := range tests {
		t.Run(test.code+"/"+test.expr, func(t *testing.T) {
			_, err := Parse(test.expr)
			if err == nil {
				t.Fatal("expected parse error")
			}
			if err.Code != test.code || err.Position != test.pos {
				t.Fatalf("got %s at %d, want %s at %d", err.Code, err.Position, test.code, test.pos)
			}
		})
	}
}

func TestLiteralParseCachesNumericAndRegexProducts(t *testing.T) {
	for _, test := range []struct {
		expr  string
		kind  TokenKind
		check func(t *testing.T, literal Literal)
	}{
		{expr: "0x2a", kind: Number, check: func(t *testing.T, literal Literal) {
			if !literal.NumberParsed || literal.NumberValue != 42 {
				t.Fatalf("numeric cache = parsed:%v value:%v, want parsed 42", literal.NumberParsed, literal.NumberValue)
			}
		}},
		{expr: `/a+/im`, kind: Regex, check: func(t *testing.T, literal Literal) {
			if !literal.RegexParsed {
				t.Fatal("regex cache is not populated")
			}
		}},
	} {
		t.Run(test.expr, func(t *testing.T) {
			node, err := Parse(test.expr)
			if err != nil {
				t.Fatalf("Parse(%q): %v", test.expr, err)
			}
			literal, ok := node.(Literal)
			if !ok || literal.Kind != test.kind {
				t.Fatalf("Parse(%q) = %#v, want %v literal", test.expr, node, test.kind)
			}
			test.check(t, literal)
		})
	}
}

func TestLiteralParsePreservesDeferredNonDecimalOverflow(t *testing.T) {
	node, err := Parse("0xfffffffffffffffff")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	literal := node.(Literal)
	if literal.NumberParsed {
		t.Fatal("overflowing non-decimal literal should retain deferred evaluation")
	}
}

func BenchmarkParseLiteralCompileMatrix(b *testing.B) {
	for _, expression := range []string{"12345", `/a+/`, `/a/i`, `/(?<=foo)bar/`} {
		b.Run(expression, func(b *testing.B) {
			for b.Loop() {
				if _, err := Parse(expression); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestExpectedDelimiterBeforeNonEOFRemainsS0202(t *testing.T) {
	_, err := Parse(`$lowercase("x"]`)
	if err == nil || err.Code != "S0202" {
		t.Fatalf("got %#v, want S0202", err)
	}
}

func TestSingleQuotedEscapes(t *testing.T) {
	node, err := Parse(`'don\'t\n'`)
	if err != nil {
		t.Fatal(err)
	}
	literal, ok := node.(Literal)
	if !ok || literal.Value != "don't\n" {
		t.Fatalf("got %#v, want decoded single-quoted string", node)
	}
}

func TestUnicodeSurrogateEscapes(t *testing.T) {
	for _, expression := range []string{`"\uD834\uDD1E"`, `'\uD834\uDD1E'`} {
		node, err := Parse(expression)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expression, err)
		}
		literal, ok := node.(Literal)
		if !ok || literal.Value != "𝄞" {
			t.Fatalf("Parse(%q) = %#v, want U+1D11E", expression, node)
		}
	}
	for _, test := range []struct {
		expression string
		units      []uint16
	}{
		{`"\uD834"`, []uint16{0xd834}},
		{`"\uDD1E"`, []uint16{0xdd1e}},
		{`'\uD834'`, []uint16{0xd834}},
		{`'\uDD1E'`, []uint16{0xdd1e}},
		{`"\uD834\u0041"`, []uint16{0xd834, 0x0041}},
	} {
		node, err := Parse(test.expression)
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.expression, err)
		}
		literal, ok := node.(Literal)
		encoded, encodedOK := literal.Value.(UTF16String)
		if !ok || !encodedOK || !reflect.DeepEqual(encoded.Units, test.units) {
			t.Fatalf("Parse(%q) = %#v, want UTF-16 units %#v", test.expression, node, test.units)
		}
	}
	node, err := Parse(`"�"`)
	if err != nil {
		t.Fatal(err)
	}
	if literal, ok := node.(Literal); !ok || literal.Value != "�" {
		t.Fatalf("valid U+FFFD literal = %#v", node)
	}
}

func TestDoubleQuotedLiteralWhitespace(t *testing.T) {
	node, err := Parse("\"line1\n\tline2\"")
	if err != nil {
		t.Fatal(err)
	}
	literal, ok := node.(Literal)
	if !ok || literal.Value != "line1\n\tline2" {
		t.Fatalf("got %#v, want decoded whitespace", node)
	}
}

func TestTransformGrammarKeepsDeleteClause(t *testing.T) {
	node, err := Parse(`|Account.Order.Product|{"Total": Price * Quantity}, ["Description", "SKU"]|`)
	if err != nil {
		t.Fatal(err)
	}
	transform, ok := node.(Transform)
	if !ok || transform.Delete == nil {
		t.Fatalf("got %#v, want transform with delete clause", node)
	}
}

func TestTransformParsesAsStandaloneCallableAndApplyOperand(t *testing.T) {
	standalone, err := Parse(`|item|{"total": price * quantity}|`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := standalone.(Transform); !ok {
		t.Fatalf("standalone transform parsed as %T", standalone)
	}

	applied, err := Parse(`$ ~> |item|{"total": price * quantity}|`)
	if err != nil {
		t.Fatal(err)
	}
	apply, ok := applied.(Apply)
	if !ok {
		t.Fatalf("applied transform parsed as %T", applied)
	}
	if _, ok := apply.Right.(Transform); !ok {
		t.Fatalf("apply right operand parsed as %T", apply.Right)
	}
}

func TestParentOperatorDiagnostics(t *testing.T) {
	tests := []struct {
		expr string
		code string
	}{
		{`{'hello': 'world'}.%`, "S0217"},
		{`%`, "S0217"},
		{`(%)`, "S0217"},
		{`(%%)`, "S0211"},
		{`library.loans.%%`, "S0207"},
		{`$.%`, "S0217"},
		{`$$.%`, "S0217"},
		{`library.loans.%.%.%`, "S0217"},
		{`library.%%%`, "S0217"},
	}
	for _, test := range tests {
		t.Run(test.expr, func(t *testing.T) {
			_, err := Parse(test.expr)
			if err == nil || err.Code != test.code {
				t.Fatalf("got %#v, want %s", err, test.code)
			}
		})
	}
}

func TestParentOperatorGroupingAndCallsParse(t *testing.T) {
	for _, expr := range []string{
		`%()`, `%(1)`, `library.(%%%)`, `library.(%% %)`,
		`library.(% %%)`, `library.(% % %)`, `Account.Order.().%`,
		`Account.Order.Product[%.OrderID='order104'].SKU`,
	} {
		t.Run(expr, func(t *testing.T) {
			if node, err := Parse(expr); err != nil || node == nil {
				t.Fatalf("Parse(%q) = %#v, %v", expr, node, err)
			}
		})
	}
}

func TestBindingOrderingDiagnostics(t *testing.T) {
	tests := []struct {
		expr string
		code string
	}{
		{`Account.Order@o.Product`, "S0214"},
		{`Account.Order@$o#i.Product`, "S0214"},
		{`Account.Order[1]@$o.Product`, "S0215"},
		{`Account.Order^(>OrderID)@$o.Product`, "S0216"},
	}
	for _, test := range tests {
		t.Run(test.expr, func(t *testing.T) {
			_, err := Parse(test.expr)
			if err == nil || err.Code != test.code {
				t.Fatalf("got %#v, want %s", err, test.code)
			}
		})
	}
}
