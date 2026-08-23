package evaluator

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestFormatPictureFixtureFamilies(t *testing.T) {
	suite, err := conformance.LoadSuite("../../testdata/reference/jsonata-js-v2.2.2")
	if err != nil {
		t.Fatal(err)
	}
	groups := []string{"function-formatBase", "function-formatInteger", "function-formatNumber", "function-parseInteger"}
	seen := 0
	for _, groupName := range groups {
		group := findConformanceGroup(suite, groupName)
		if group == nil {
			t.Fatalf("missing group %s", groupName)
		}
		for _, fixture := range group.Cases {
			got, gotErr := evalFormatFixture(fixture)
			seen++
			switch fixture.ExpectedKind {
			case conformance.ExpectedUndefined:
				if gotErr != nil || !value.IsUndefined(got) {
					t.Errorf("%s: want undefined, got %#v (%v)", fixture.Reference(), got, gotErr)
				}
			case conformance.ExpectedError:
				if gotErr == nil {
					t.Errorf("%s: want %s, got %#v", fixture.Reference(), fixture.ExpectedCode, got)
					continue
				}
				if code := errorCode(gotErr); code != fixture.ExpectedCode {
					t.Errorf("%s: want %s, got %s (%v)", fixture.Reference(), fixture.ExpectedCode, code, gotErr)
				}
			default:
				if gotErr != nil {
					t.Errorf("%s: unexpected %v", fixture.Reference(), gotErr)
					continue
				}
				if !reflect.DeepEqual(got, fixture.Expected) && !(isNumeric(got) && isNumeric(fixture.Expected) && math.Abs(toTestNumber(got)-toTestNumber(fixture.Expected)) < 1e-9*math.Max(1, math.Abs(toTestNumber(fixture.Expected)))) {
					t.Errorf("%s: want %#v, got %#v", fixture.Reference(), fixture.Expected, got)
				}
			}
		}
	}
	if seen != 180 {
		t.Fatalf("fixture count = %d, want 180", seen)
	}
}

func findConformanceGroup(suite conformance.Suite, name string) *conformance.Group {
	for i := range suite.Groups {
		if suite.Groups[i].Name == name {
			return &suite.Groups[i]
		}
	}
	return nil
}

func evalFormatFixture(fixture conformance.Case) (any, error) {
	open := strings.Index(fixture.Expression, "(")
	close := strings.LastIndex(fixture.Expression, ")")
	if open < 0 || close < open {
		return nil, runtimeError{code: "T1000", msg: "invalid fixture expression"}
	}
	name := strings.TrimPrefix(strings.TrimSpace(fixture.Expression[:open]), "$")
	argsText := splitFormatArguments(fixture.Expression[open+1 : close])
	st := state{root: value.FromJSON(fixture.Data), current: value.FromJSON(fixture.Data), runtime: newEvalRuntime(Options{})}
	args := make([]any, 0, len(argsText))
	for _, text := range argsText {
		arg, err := evalFormatArgument(text, st)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	var function builtinFunc
	switch name {
	case "formatBase":
		function = builtinFormatBase
	case "formatInteger":
		function = builtinFormatInteger
	case "formatNumber":
		function = builtinFormatNumber
	case "parseInteger":
		function = builtinParseInteger
	default:
		return nil, runtimeError{code: "T1000", msg: "unknown picture function"}
	}
	return function(st, args)
}

func splitFormatArguments(text string) []string {
	var result []string
	start, depth := 0, 0
	var quote rune
	for i, char := range text {
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		switch char {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(text[start:]) != "" {
		result = append(result, strings.TrimSpace(text[start:]))
	}
	return result
}

func evalFormatArgument(text string, st state) (any, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "undefined" || trimmed == "nothing" {
		return value.Undefined, nil
	}
	if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return number, nil
	}
	if (strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'")) || (strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) {
		if strings.HasPrefix(trimmed, "'") {
			trimmed = `"` + strings.ReplaceAll(trimmed[1:len(trimmed)-1], `"`, `\"`) + `"`
		}
		var result string
		if err := json.Unmarshal([]byte(trimmed), &result); err == nil {
			return result, nil
		}
	}
	node, err := syntax.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return eval(node, st)
}

func errorCode(err error) string {
	if coded, ok := err.(interface{ JSONataCode() string }); ok {
		return coded.JSONataCode()
	}
	return ""
}

func isNumeric(input any) bool {
	_, ok := strictNumeric(collapse(input))
	return ok
}

func toTestNumber(input any) float64 {
	n, _ := strictNumeric(collapse(input))
	return n
}

func TestFormatPictureCoreRegressions(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	checks := []struct {
		name string
		fn   builtinFunc
		args []any
		want any
	}{
		{"base36", builtinFormatBase, []any{100.0, 36.0}, "2s"},
		{"ordinal", builtinFormatInteger, []any{123.0, "000;o"}, "123rd"},
		{"words", builtinFormatInteger, []any{555.0, "Ww"}, "Five Hundred and Fifty-Five"},
		{"integer unicode", builtinFormatInteger, []any{12340.0, "###١"}, "١٢٣٤٠"},
		{"number grouping", builtinFormatNumber, []any{12345.6, "#,###.00"}, "12,345.60"},
		{"number exponent", builtinFormatNumber, []any{1234.5678, "00.000e000"}, "12.346e002"},
		{"parse roman", builtinParseInteger, []any{"MCMLXXXIV", "I"}, 1984.0},
		{"parse words", builtinParseInteger, []any{"one trillion and one", "w"}, 1000000000001.0},
	}
	for _, check := range checks {
		got, err := check.fn(st, check.args)
		if err != nil {
			t.Errorf("%s: %v", check.name, err)
			continue
		}
		if !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s: want %#v, got %#v", check.name, check.want, got)
		}
	}
}

func TestFormatPictureRuntimeBudget(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{MaxOperations: 1})}
	if _, err := builtinFormatNumber(st, []any{0.000001, "0e0"}); err == nil {
		t.Fatal("expected budget exhaustion")
	}
}

func TestFormatPictureRequiredArgumentsDoNotPanic(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	for _, function := range []builtinFunc{builtinFormatInteger, builtinFormatNumber, builtinParseInteger} {
		if _, err := function(st, []any{1.0}); err == nil {
			t.Errorf("%T accepted a missing picture", function)
		}
	}
}

func TestFormatNumberUnicodeDigitsWithGrouping(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	got, err := builtinFormatNumber(st, []any{12345.0, "#٬###", map[string]any{"zero-digit": "٠", "grouping-separator": "٬"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "١٢٬٣٤٥" {
		t.Fatalf("got %q, want %q", got, "١٢٬٣٤٥")
	}
}

func TestFormatNumberUnicodeDecimalSeparator(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	got, err := builtinFormatNumber(st, []any{12.5, "0٫00", map[string]any{"decimal-separator": "٫"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "12٫50" {
		t.Fatalf("got %q, want %q", got, "12٫50")
	}
}

func TestParseIntegerRejectsMixedUnicodeFamily(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	if _, err := builtinParseInteger(st, []any{"123", "###١"}); err == nil || errorCode(err) != "D3130" {
		t.Fatalf("mixed digit family error = %v, want D3130", err)
	}
}

func TestFormatIntegerLargeRomanDoesNotAllocateUnboundedOutput(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	if _, err := builtinFormatInteger(st, []any{1e100, "I"}); err == nil || errorCode(err) != "D3130" {
		t.Fatalf("large Roman error = %v, want D3130", err)
	}
}

func TestParseIntegerLargeAlphabeticReturnsFiniteError(t *testing.T) {
	st := state{runtime: newEvalRuntime(Options{})}
	if _, err := builtinParseInteger(st, []any{strings.Repeat("Z", 256), "A"}); err == nil || errorCode(err) != "D1001" {
		t.Fatalf("large alphabetic error = %v, want D1001", err)
	}
}
