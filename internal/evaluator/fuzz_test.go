package evaluator

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

// FuzzSyntaxAndEvaluation checks that malformed user input is reported rather
// than panicking, which is an important embedding boundary for expression
// services.
func FuzzSyntaxAndEvaluation(f *testing.F) {
	for _, seed := range []string{
		"1", "foo.bar", "[1, {\"x\": true}]", "\"text\"",
		`($f := function($x){$x}; $f($))`,
		`$lookup($, "__proto__")`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expression string) {
		expression = fuzzBound(expression, 256)
		n, err := syntax.Parse(expression)
		if err != nil {
			return
		}
		_, _ = EvalWithOptions(n, hostileFuzzValue([]byte(expression)), Options{
			MaxCallDepth:  32,
			MaxOperations: 2_000,
		})
	})
}

func FuzzJSONDecode(f *testing.F) {
	for _, seed := range []string{
		`null`, `{"x":1}`, `[1,2]`,
		`{"__proto__":{"constructor":"blocked"},"*":{"keep":true}}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		data = fuzzBoundBytes(data, 4*1024)
		decoded, err := DecodeJSON(data)
		if err != nil {
			return
		}
		n, parseErr := syntax.Parse("$")
		if parseErr == nil {
			_, _ = EvalWithOptions(n, decoded, Options{MaxCallDepth: 16, MaxOperations: 256})
		}
	})
}

// FuzzRegexLiteralAndReplacement exercises both syntax-level regex literals
// and the evaluator's replacement path with bounded inputs.
func FuzzRegexLiteralAndReplacement(f *testing.F) {
	for _, seed := range []struct {
		pattern     string
		replacement string
	}{
		{"a+", "<$&>"},
		{"(?:)", "x"},
		{`(?<=a)b`, "$&"},
		{`(a)\1`, "$1"},
	} {
		f.Add(seed.pattern, seed.replacement)
	}
	f.Fuzz(func(t *testing.T, pattern, replacement string) {
		pattern = fuzzBound(pattern, 128)
		replacement = fuzzBound(replacement, 128)
		n, err := syntax.Parse("/" + pattern + "/")
		if err == nil {
			_, _ = EvalWithOptions(n, "aab", Options{MaxCallDepth: 16, MaxOperations: 512, Timeout: 10 * time.Millisecond})
		}
		replacementJSON, marshalErr := json.Marshal(replacement)
		if marshalErr != nil {
			return
		}
		expression := `$replace("aab", /` + pattern + `/, ` + string(replacementJSON) + `)`
		n, err = syntax.Parse(expression)
		if err == nil {
			_, _ = EvalWithOptions(n, "aab", Options{MaxCallDepth: 16, MaxOperations: 512, Timeout: 10 * time.Millisecond})
		}
	})
}

func FuzzDateTimePictures(f *testing.F) {
	for _, seed := range []string{
		"[Y0001]-[M01]-[D01]", "[H01]:[m01]:[s01]", "[[literal]]", "[",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, picture string) {
		picture = fuzzBound(picture, 256)
		st := state{runtime: newEvalRuntime(Options{MaxOperations: 512})}
		_, _ = analyseDateTimePicture(st, picture)
		_, _, _ = parseDateTime(st, "2024-01-01", picture)
	})
}

func FuzzFunctionSignatures(f *testing.F) {
	for _, seed := range []string{"<s:s>", "<a<o-f>:a<o>>", "<", "<af?>", "<x-:x>"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, signature string) {
		_, _ = parseFunctionSignature(fuzzBound(signature, 256))
	})
}

func FuzzHostileNestedEvaluation(f *testing.F) {
	for _, seed := range []string{"", "{}", "__proto__", "constructor", "wildcard"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		input := hostileFuzzValue(data)
		for _, expression := range []string{
			"$", "*", "**", "$keys($)", "$spread($)",
			`$lookup($, "constructor")`, `$lookup($, "__proto__")`,
		} {
			n, err := syntax.Parse(expression)
			if err != nil {
				continue
			}
			_, _ = EvalWithOptions(n, input, Options{MaxCallDepth: 16, MaxOperations: 256})
		}
	})
}

func TestGHSA2943CraftedLookupCannotExposeCallable(t *testing.T) {
	const expression = `(
 $hasOwnProperty := $spread($string);
 $__proto__ := $constructor;
 $constructor("return process.getBuiltinModule('child_process').execSync('true')")()
)`
	n, err := syntax.Parse(expression)
	if err != nil {
		t.Fatal(err)
	}
	result, evalErr := EvalWithOptions(n, map[string]any{}, Options{MaxCallDepth: 16, MaxOperations: 512})
	if !hasJSONataCode(evalErr, "T1006") {
		t.Fatalf("crafted lookup returned %#v, %v; want T1006", result, evalErr)
	}
	if result != nil {
		t.Fatalf("crafted lookup returned a value with an error: %#v", result)
	}
}

func TestPrototypePropertiesAreNotInheritedFromHostObjects(t *testing.T) {
	input := map[string]any{"foo": map[string]any{"bar": "baz"}}

	prototype, err := syntax.Parse("foo.__proto__")
	if err != nil {
		t.Fatal(err)
	}
	if result, evalErr := EvalWithOptions(prototype, input, Options{MaxCallDepth: 16, MaxOperations: 256}); result != nil || !errors.Is(evalErr, errUndefined) {
		t.Fatalf("foo.__proto__ returned %#v, %v; want undefined", result, evalErr)
	}

	toString, err := syntax.Parse("foo.toString()")
	if err != nil {
		t.Fatal(err)
	}
	if result, evalErr := EvalWithOptions(toString, input, Options{MaxCallDepth: 16, MaxOperations: 256}); result != nil || !hasJSONataCode(evalErr, "T1006") {
		t.Fatalf("foo.toString() returned %#v, %v; want T1006", result, evalErr)
	}
}

func TestPrototypeNamedKeysRemainPlainJSONData(t *testing.T) {
	input := map[string]any{
		"__proto__":   map[string]any{"constructor": "blocked"},
		"constructor": "blocked",
		"prototype":   map[string]any{"invoke": "blocked"},
		"*":           map[string]any{"keep": true},
	}
	for _, test := range []struct {
		expression string
		want       any
	}{
		{expression: `$lookup($, "__proto__")`, want: map[string]any{"constructor": "blocked"}},
		{expression: `$lookup($, "constructor")`, want: "blocked"},
		{expression: `$lookup($, "prototype")`, want: map[string]any{"invoke": "blocked"}},
		{expression: `$lookup($, "*")`, want: map[string]any{"keep": true}},
	} {
		n, err := syntax.Parse(test.expression)
		if err != nil {
			t.Fatal(err)
		}
		result, evalErr := EvalWithOptions(n, input, Options{MaxCallDepth: 16, MaxOperations: 256})
		if evalErr != nil || !reflect.DeepEqual(result, test.want) {
			t.Fatalf("%s returned %#v, %v; want %#v", test.expression, result, evalErr, test.want)
		}
		if containsCallable(result) {
			t.Fatalf("%s exposed an internal callable: %#v", test.expression, result)
		}
	}
}

func fuzzBound(value string, limit int) string {
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func fuzzBoundBytes(value []byte, limit int) []byte {
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func hostileFuzzValue(data []byte) any {
	if len(data) > 128 {
		data = data[:128]
	}
	return hostileFuzzObject(data, 0)
}

func hostileFuzzObject(data []byte, depth int) any {
	if depth >= 5 || len(data) < 2 {
		return string(data)
	}
	keys := []string{"__proto__", "constructor", "prototype", "*", "keep", "$internal"}
	width := int(data[0]%3) + 1
	result := make(map[string]any, width)
	for index := 0; index < width; index++ {
		key := keys[(int(data[(index+1)%len(data)])+index)%len(keys)]
		child := data[1:]
		if len(child) > 32 {
			child = child[:32]
		}
		if (data[0]+byte(index))%2 == 0 {
			result[key] = hostileFuzzObject(child, depth+1)
		} else {
			result[key] = []any{hostileFuzzObject(child, depth+1), string(child)}
		}
	}
	return result
}
