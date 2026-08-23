package evaluator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/cases"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestStringEncodingAndContextFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{
		"context": true, "encoding": true,
		"function-string": true, "function-substring": true,
		"function-substringAfter": true, "function-substringBefore": true,
		"function-lowercase": true, "function-uppercase": true,
		"function-pad": true, "function-trim": true,
		"function-encodeUrl": true, "function-decodeUrl": true,
		"function-encodeUrlComponent": true, "function-decodeUrlComponent": true,
	}
	var fixtures []conformance.Case
	for _, group := range suite.Groups {
		if groups[group.Name] {
			fixtures = append(fixtures, group.Cases...)
		}
	}
	if len(fixtures) != 100 {
		t.Fatalf("owned fixture count = %d, want 100", len(fixtures))
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Reference(), func(t *testing.T) {
			data := fixtureData(t, suite, fixture)
			node, parseErr := syntax.Parse(fixture.Expression)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			var actual any
			var evalErr error
			if fixture.HasInput() {
				actual, evalErr = EvalBindings(node, data, fixture.Bindings)
			} else {
				actual, evalErr = EvalNoInputBindingsWithOptions(node, fixture.Bindings, Options{})
			}
			switch fixture.ExpectedKind {
			case conformance.ExpectedUndefined:
				if !IsUndefined(evalErr) {
					t.Fatalf("got value=%#v err=%v, want undefined", actual, evalErr)
				}
			case conformance.ExpectedError:
				if evalErr == nil {
					t.Fatalf("got value=%#v, want error %s", actual, fixture.ExpectedCode)
				}
				var coded interface{ JSONataCode() string }
				if !errors.As(evalErr, &coded) || coded.JSONataCode() != fixture.ExpectedCode {
					t.Fatalf("got error %v, want %s", evalErr, fixture.ExpectedCode)
				}
			default:
				if evalErr != nil {
					t.Fatal(evalErr)
				}
				if !reflect.DeepEqual(actual, fixture.Expected) {
					t.Fatalf("got %#v, want %#v", actual, fixture.Expected)
				}
			}
		})
	}
}

func TestStringURLComponentEscaping(t *testing.T) {
	tests := []struct {
		expression string
		want       string
	}{
		{expression: `$encodeUrlComponent("a b+c")`, want: "a%20b%2Bc"},
		{expression: `$decodeUrlComponent("a+b%2Bc")`, want: "a+b+c"},
	}
	for _, test := range tests {
		node, err := syntax.Parse(test.expression)
		if err != nil {
			t.Fatal(err)
		}
		got, evalErr := Eval(node, nil)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		if got != test.want {
			t.Fatalf("%s = %#v, want %#v", test.expression, got, test.want)
		}
	}
}

func TestStringUnicodeAndEncodingParity(t *testing.T) {
	tests := []struct {
		expression string
		want       any
	}{
		{`$uppercase("ß")`, "SS"},
		{`$uppercase("ﬃ")`, "FFI"},
		{`$lowercase("İ")`, "i̇"},
		{`$lowercase("ΟΣ")`, "ος"},
		{`$lowercase("ΟΣΟΣ")`, "οσος"},
		{`$trim("   a\u000b  b  ")`, "  a\u000b b  "},
		{`$base64encode("é")`, "6Q=="},
		{`$base64decode("6Q==")`, "é"},
		{`$base64decode("w6k=")`, "Ã©"},
		{`$base64decode("YQ==Yg==")`, "a"},
		{`$base64decode("YQ=Yg")`, "a"},
		{`$base64decode("abcd=efgh")`, "i·\u001d"},
		{`$base64encode("💩")`, "Pak="},
		{`$base64encode("Ā")`, "AA=="},
		{`$encodeUrl("https://x/a b?x=1&y=2#z")`, "https://x/a%20b?x=1&y=2#z"},
		{`$encodeUrlComponent("a b+c/?#&=%")`, "a%20b%2Bc%2F%3F%23%26%3D%25"},
		{`$decodeUrl("https://x/a%20b?x=1&y=2#z")`, "https://x/a b?x=1&y=2#z"},
		{`$decodeUrlComponent("a%20b%2Bc%2F%3F%23%26%3D%25")`, "a b+c/?#&=%"},
	}
	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			node, err := syntax.Parse(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			got, evalErr := Eval(node, nil)
			if evalErr != nil {
				t.Fatal(evalErr)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestStringContextDefaultAndExplicitUndefined(t *testing.T) {
	functions := []string{
		"lowercase", "uppercase", "trim", "base64encode", "base64decode",
		"encodeUrl", "decodeUrl", "encodeUrlComponent", "decodeUrlComponent",
	}
	for _, name := range functions {
		node, err := syntax.Parse("$" + name + "()")
		if err != nil {
			t.Fatal(err)
		}
		if _, evalErr := EvalNoInputWithOptions(node, Options{}); !IsUndefined(evalErr) {
			t.Fatalf("$%s() without input error = %v, want undefined", name, evalErr)
		}
		for _, input := range []any{nil, 12.0, map[string]any{}, []any{}} {
			if _, evalErr := Eval(node, input); !hasJSONataCode(evalErr, "T0411") {
				t.Fatalf("$%s() with context %#v error = %v, want T0411", name, input, evalErr)
			}
		}

		explicitUndefined, err := syntax.Parse("$" + name + "(missing)")
		if err != nil {
			t.Fatal(err)
		}
		if _, evalErr := Eval(explicitUndefined, "context"); !IsUndefined(evalErr) {
			t.Fatalf("$%s(missing) error = %v, want undefined", name, evalErr)
		}
	}
}

func TestStringCaseUnicodeTarget(t *testing.T) {
	want := map[string]string{
		"15.0.0": "ƛ",
		"17.0.0": "Ƛ",
	}[cases.UnicodeVersion]
	if want == "" {
		t.Fatalf("unsupported x/text casing tables: %s", cases.UnicodeVersion)
	}
	if got := jsonataUppercase.String("ƛ"); got != want {
		t.Fatalf("Unicode %s upper-case boundary for U+019B = %q, want %q", cases.UnicodeVersion, got, want)
	}
}

func TestStringURLMalformedAndReservedParity(t *testing.T) {
	tests := []struct {
		expression string
		want       string
	}{
		{`$encodeUrl("a%20b")`, "a%2520b"},
		{`$encodeUrl("a#b?c&d=e")`, "a#b?c&d=e"},
		{`$encodeUrl("http://x/?b=2&a=1")`, "http://x/?b=2&a=1"},
		{`$decodeUrl("%C3%A9")`, "é"},
		{`$decodeUrl("%3B%2f%26")`, "%3B%2f%26"},
		{`$decodeUrlComponent("%3B%2f%26")`, ";/&"},
	}
	for _, test := range tests {
		node, err := syntax.Parse(test.expression)
		if err != nil {
			t.Fatal(err)
		}
		got, evalErr := Eval(node, nil)
		if evalErr != nil || got != test.want {
			t.Fatalf("%s = %#v, err=%v, want %#v", test.expression, got, evalErr, test.want)
		}
	}
	for _, expression := range []string{`$decodeUrl("a%ZZ")`, `$decodeUrlComponent("a%ZZ")`, `$decodeUrl("%E9")`, `$decodeUrlComponent("%C3%28")`} {
		node, err := syntax.Parse(expression)
		if err != nil {
			t.Fatal(err)
		}
		if _, evalErr := Eval(node, nil); !hasJSONataCode(evalErr, "D3140") {
			t.Fatalf("%s error = %v, want D3140", expression, evalErr)
		}
	}
}

func TestStringSplitUTF16RepresentationPolicy(t *testing.T) {
	node, err := syntax.Parse(`$split("A💩B", "")`)
	if err != nil {
		t.Fatal(err)
	}
	if _, evalErr := Eval(node, nil); !hasJSONataCode(evalErr, "U1002") {
		t.Fatalf("split error = %v, want U1002", evalErr)
	}
}

func TestStringNumericFormattingAndPathSubstring(t *testing.T) {
	data := map[string]any{
		"Employment": map[string]any{
			"Role":                   "Senior Physician",
			"Executive.Compensation": float64(1400000),
		},
	}
	tests := []struct {
		expression string
		want       string
	}{
		{expression: `$lowercase("COMPENSATION IS : " & Employment."Executive.Compensation")`, want: "compensation is : 1400000"},
		{expression: `$string(1400000)`, want: "1400000"},
		{expression: `$substring(Employment.Role, 7, 4)`, want: "Phys"},
		{expression: `$substring(Employment.Role, -4, 4)`, want: "cian"},
	}
	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			node, parseErr := syntax.Parse(test.expression)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			got, evalErr := Eval(node, data)
			if evalErr != nil || got != test.want {
				t.Fatalf("got %#v, err=%v, want %q", got, evalErr, test.want)
			}
		})
	}
}

func TestStringResourceAndTypeErrors(t *testing.T) {
	largePad, err := syntax.Parse(`$pad("x", 1000000000)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvalWithOptions(largePad, nil, Options{MaxOperations: 100}); !hasJSONataCode(err, "U1001") {
		t.Fatalf("large pad error = %v, want U1001", err)
	}

	largeSubstring, err := syntax.Parse(`$substring($, -1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvalWithOptions(largeSubstring, strings.Repeat("x", 1000), Options{MaxOperations: 10}); !hasJSONataCode(err, "U1001") {
		t.Fatalf("substring budget error = %v, want U1001", err)
	}

	stringify, err := syntax.Parse(`$string($)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Eval(stringify, map[string]any{"unsupported": make(chan int)}); !hasJSONataCode(err, "T0412") {
		t.Fatalf("unsupported value error = %v, want T0412", err)
	}
}

func TestStringOperationsCheckResourcesBeforeAllocation(t *testing.T) {
	large := strings.Repeat("x", 4096)
	limited := func() state {
		return state{runtime: newEvalRuntime(Options{MaxOperations: 8})}
	}
	tests := map[string]func(state) error{
		"lowercase": func(st state) error { _, err := builtinLowercase(st, []any{large}); return err },
		"uppercase": func(st state) error { _, err := builtinUppercase(st, []any{large}); return err },
		"trim":      func(st state) error { _, err := trimJSONata(st, large); return err },
		"base64encode": func(st state) error {
			_, err := base64EncodeBinary(st, large)
			return err
		},
		"base64decode": func(st state) error {
			_, err := base64DecodeBinary(st, strings.Repeat("YQ==", 1024))
			return err
		},
		"encodeUrl": func(st state) error { _, err := encodeURIValue(st, large, true); return err },
		"decodeUrl": func(st state) error { _, err := decodeURIValue(st, large, true); return err },
		"split": func(st state) error {
			_, err := splitString(st, large, "", -1)
			return err
		},
	}
	for name, run := range tests {
		t.Run(name+"/budget", func(t *testing.T) {
			if err := run(limited()); !hasJSONataCode(err, "U1001") {
				t.Fatalf("error = %v, want U1001", err)
			}
		})
		t.Run(name+"/cancel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			st := state{runtime: newEvalRuntime(Options{Context: ctx, MaxOperations: 1_000_000})}
			if err := run(st); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestStringPreservesConstructedObjectOrderAndNonFiniteCodes(t *testing.T) {
	ordered, err := syntax.Parse(`$string({"b": 2, "a": 1})`)
	if err != nil {
		t.Fatal(err)
	}
	got, evalErr := Eval(ordered, nil)
	if evalErr != nil || got != `{"b":2,"a":1}` {
		t.Fatalf("ordered string = %#v, err=%v", got, evalErr)
	}

	for expression, want := range map[string]string{
		`$string(1/0)`:        "D3001",
		`$string({"n": 1/0})`: "D1001",
		`1/0`:                 "T2002",
	} {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatalf("%s: %v", expression, parseErr)
		}
		if _, evalErr := Eval(node, nil); !hasJSONataCode(evalErr, want) {
			t.Fatalf("%s error = %v, want %s", expression, evalErr, want)
		}
	}

	object, err := syntax.Parse(`{"b": 2, "a": 1}`)
	if err != nil {
		t.Fatal(err)
	}
	public, evalErr := Eval(object, nil)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	if _, ok := public.(map[string]any); !ok {
		t.Fatalf("public object type = %T, want map[string]any", public)
	}
}

func hasJSONataCode(err error, want string) bool {
	if err == nil {
		return false
	}
	var coded interface{ JSONataCode() string }
	return errors.As(err, &coded) && coded.JSONataCode() == want
}

func fixtureData(t *testing.T, suite conformance.Suite, fixture conformance.Case) any {
	t.Helper()
	if fixture.HasData {
		return fixture.Data
	}
	if !fixture.HasDataset || fixture.Dataset == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(suite.Root, "test", "test-suite", "datasets", fixture.Dataset+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}
