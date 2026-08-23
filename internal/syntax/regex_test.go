package syntax

import "testing"

func TestRegexLiteralClassAndEscapes(t *testing.T) {
	for _, source := range []string{
		`/[/]/`, `/[a/b]/`, `/(a/b)/`, `/({a/b}[c/d])/`, `/\//`, `/[\/]/`, `/[\]]/`, `/\(/`,
	} {
		node, err := Parse(source)
		if err != nil {
			t.Fatalf("Parse(%q): %v", source, err)
		}
		literal, ok := node.(Literal)
		if !ok || literal.Kind != Regex || literal.Value != source {
			t.Fatalf("Parse(%q) = %#v, want regex literal", source, node)
		}
	}
}

func TestRegexLiteralDiagnosticsHappenDuringParsing(t *testing.T) {
	for _, test := range []struct {
		source string
		code   string
	}{
		{source: `//`, code: "S0301"},
		{source: `/[/`, code: "S0302"},
		{source: `/(abc/`, code: "S0302"},
		{source: `/(a/b/`, code: "S0302"},
		{source: `/[a/b/`, code: "S0302"},
		{source: `/)/`, code: "S0302"},
		{source: `/[(]/`, code: "S0302"},
		{source: `/a/g`, code: "S0201"},
		{source: `/a/ii`, code: "S0302"},
	} {
		_, err := Parse(test.source)
		if err == nil || err.Code != test.code {
			t.Errorf("Parse(%q) error = %v, want %s", test.source, err, test.code)
		}
	}
}

func TestRegexLiteralDiagnosticMetadata(t *testing.T) {
	for _, test := range []struct {
		source, code, token string
		position            int
	}{
		{source: `//`, code: "S0301", token: `//`, position: 1},
		{source: `/[/`, code: "S0302", position: 3},
		{source: `/a/g`, code: "S0201", token: "g", position: 4},
	} {
		_, err := Parse(test.source)
		if err == nil {
			t.Fatalf("Parse(%q) succeeded", test.source)
		}
		if err.Code != test.code || err.Token != test.token || err.Value != test.token || err.Position != test.position {
			t.Errorf("Parse(%q) error = %#v, want code=%s token=%q position=%d", test.source, err, test.code, test.token, test.position)
		}
	}
}

func TestTrailingSemicolonOnlyFailsOutsideBlock(t *testing.T) {
	if _, err := Parse(`(1;)`); err != nil {
		t.Fatalf("trailing block semicolon: %v", err)
	}
	_, err := Parse(`1;`)
	if err == nil || err.Code != "S0201" || err.Token != ";" || err.Position != 2 {
		t.Fatalf("top-level trailing semicolon error = %#v", err)
	}
}
