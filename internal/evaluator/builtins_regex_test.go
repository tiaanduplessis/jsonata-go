package evaluator_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type regexFixtureCompiler struct{}

func (regexFixtureCompiler) Compile(expression string) (conformance.Expression, error) {
	return jsonata.Compile(expression)
}

func TestRegexMatcherObjectAndZeroLengthProtection(t *testing.T) {
	got, err := jsonata.Eval(`$match("aba", /a/)`, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"match": "a", "index": 0.0, "groups": []any{}},
		map[string]any{"match": "a", "index": 2.0, "groups": []any{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("regex matches = %#v, want %#v", got, want)
	}

	_, err = jsonata.Eval(`$match("ab", /(?:)/)`, nil)
	var jsonataErr *jsonata.Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D1004" {
		t.Fatalf("zero-length match error = %v, want D1004", err)
	}
}

func TestCustomMatcherPublicShape(t *testing.T) {
	got, err := jsonata.Eval(`$match("a", function($input){{"match":"a","start":0,"end":1}})`, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{map[string]any{"match": "a", "index": 0.0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("custom matcher result = %#v, want %#v", got, want)
	}
}

func TestArbitraryMatcherContractAcrossBuiltins(t *testing.T) {
	matcher := `function($s){{"match":"b","start":1,"end":2,"groups":[],"next":function(){$missing}}}`
	for _, test := range []struct {
		expression string
		want       any
	}{
		{`$contains("abc", ` + matcher + `)`, true},
		{`$replace("abc", ` + matcher + `, "X")`, "aXc"},
		{`$split("abc", ` + matcher + `)`, []any{"a", "c"}},
	} {
		got, err := jsonata.Eval(test.expression, nil)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s = %#v, %v; want %#v", test.expression, got, err, test.want)
		}
	}
	_, err := jsonata.Eval(`$contains("abc", function($s){{"invalid":true}})`, nil)
	var jsonataErr *jsonata.Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "T1010" {
		t.Fatalf("invalid matcher error = %v, want T1010", err)
	}
}

func TestRegexReplacementTokensAndFractionalLimits(t *testing.T) {
	tests := []struct {
		expression string
		want       any
	}{
		{`$replace("abc", /(b)/, "$&-$` + "`" + `-$'-$01")`, "a$&-$`-$'-b1c"},
		{`$replace("a", "a", "$0")`, "$0"},
		{`$replace("abc", /(b)/, function($m){$m.start & ":" & $m.end & ":" & $exists($m.index)})`, "a1:2:falsec"},
		{`$replace("aaaa", /a/, "x", 2.5)`, "xxxa"},
		{`$split("a,a,a,a", /,/, 2.5)`, []any{"a", "a", "a"}},
	}
	for _, test := range tests {
		got, err := jsonata.Eval(test.expression, nil)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s = %#v, %v; want %#v", test.expression, got, err, test.want)
		}
	}
	got, err := jsonata.Eval(`$match("aaaa", /a/, 2.5).index`, nil)
	if err != nil || !reflect.DeepEqual(got, []any{0.0, 1.0, 2.0}) {
		t.Fatalf("fractional match limit = %#v, %v", got, err)
	}
	_, err = jsonata.Eval(`$match("a", /a/, -1)`, nil)
	var jsonataErr *jsonata.Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D3040" {
		t.Fatalf("negative match limit error = %v, want D3040", err)
	}
}

func TestRegexFallbackUTF16AndResourceBounds(t *testing.T) {
	got, err := jsonata.Eval(`[$match("💩x", /x/).index, $match("foobar", /(?<=foo)bar/).match, $match("zaabb", /(a)\1/).match]`, nil)
	if err != nil || !reflect.DeepEqual(got, []any{2.0, "bar", "aa"}) {
		t.Fatalf("fallback/UTF-16 result = %#v, %v", got, err)
	}

	expression := jsonata.MustCompile(`$match($input, /(a+)+$/)`)
	input := strings.Repeat("a", 4096) + "!"
	_, err = expression.EvalWithOptions(nil, jsonata.EvalOptions{
		Bindings:      map[string]any{"input": input},
		Timeout:       5 * time.Millisecond,
		MaxOperations: 100000,
	})
	var jsonataErr *jsonata.Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D1012" {
		t.Fatalf("adversarial fallback error = %v, want D1012", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = expression.EvalWithOptions(nil, jsonata.EvalOptions{
		Context:  canceled,
		Bindings: map[string]any{"input": input},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fallback error = %v, want context cancellation", err)
	}

	_, err = expression.EvalWithOptions(nil, jsonata.EvalOptions{
		Bindings:      map[string]any{"input": strings.Repeat("a", 100)},
		MaxOperations: 20,
	})
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "U1001" {
		t.Fatalf("budgeted fallback error = %v, want U1001", err)
	}
}

func TestRegexNonASCIIUTF16Parity(t *testing.T) {
	tests := []struct {
		expression string
		want       any
	}{
		{`$contains("💩", /./)`, true},
		{`$contains("💩", /\uD83D/)`, true},
		{`[$match("💩x", /x/).match, $match("💩x", /x/).index]`, []any{"x", 2.0}},
		{`[$match("aé", /é/).match, $match("aé", /é/).index]`, []any{"é", 1.0}},
		{`$match("é", /./).match`, []any{"e", "́"}},
		{`$match("💩", /../).match`, "💩"},
		{`$match("💩", /[💩][💩]/).match`, "💩"},
		{`[$match("💩💩", /(..)\1/).match, $match("💩💩", /(..)\1/).groups[0]]`, []any{"💩💩", "💩"}},
		{`[$match("💩x", /(?<=💩)(x)/).match, $match("💩x", /(?<=💩)(x)/).index, $match("💩x", /(?<=💩)(x)/).groups[0]]`, []any{"x", 2.0, "x"}},
		{`$replace("a💩b", /(💩)/, function($m){$m.start & ":" & $m.end & ":" & $m.groups[0]})`, "a1:3:💩b"},
		{`$replace("💩", /./, "x")`, "xx"},
		{`$replace("💩", /./, "$0")`, "💩"},
		{`$split("💩", /./)`, []any{"", "", ""}},
		{`$split("a💩b", /💩/)`, []any{"a", "b"}},
		{`($r := /(💩)/; $m := $r("a💩"); [$m.match,$m.start,$m.end,$m.groups])`, []any{"💩", 1.0, 3.0, "💩"}},
	}
	for _, test := range tests {
		got, err := jsonata.Eval(test.expression, nil)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s = %#v, %v; want %#v", test.expression, got, err, test.want)
		}
	}
}

func TestRegexLoneSurrogateResultsReturnRepresentationError(t *testing.T) {
	for _, expression := range []string{
		`$match("💩", /./)`,
		`$replace("💩", /./, function($m){"x"})`,
		`$replace("💩", /./, "x", 1)`,
		`$split("💩", /\uD83D/)`,
		`($r := /./; $r("💩"))`,
	} {
		_, err := jsonata.Eval(expression, nil)
		var jsonataErr *jsonata.Error
		if !errors.As(err, &jsonataErr) || jsonataErr.Code != "U1002" || !strings.Contains(jsonataErr.Message, "unpaired UTF-16 surrogate") {
			t.Errorf("%s error = %v, want U1002 representation error", expression, err)
		}
	}

	_, err := jsonata.Eval(`$match("💩", /(?:)/)`, nil)
	var jsonataErr *jsonata.Error
	if !errors.As(err, &jsonataErr) || jsonataErr.Code != "D1004" {
		t.Fatalf("astral zero-length error = %v, want D1004", err)
	}
}

func TestRegexFallbackConcurrentEvaluation(t *testing.T) {
	expression := jsonata.MustCompile(`$match($input, /(?<=foo)bar/).match`)
	const workers = 16
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, err := expression.EvalWithOptions(nil, jsonata.EvalOptions{Bindings: map[string]any{"input": "foobar"}})
			if err != nil || got != "bar" {
				t.Errorf("concurrent fallback = %#v, %v", got, err)
			}
		}()
	}
	wait.Wait()
}

func TestRegexAndStringMatchingFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	wantGroups := map[string]int{
		"regex": 39, "matchers": 2, "function-contains": 8,
		"function-replace": 12, "function-split": 19,
		"function-sift": 1, "function-applications": 1,
	}
	manifest := make(conformance.Manifest)
	for group, want := range wantGroups {
		var found *conformance.Group
		for index := range suite.Groups {
			if suite.Groups[index].Name == group {
				found = &suite.Groups[index]
				break
			}
		}
		if found == nil {
			t.Fatalf("fixture group %q is missing", group)
		}
		if len(found.Cases) != want && group != "function-sift" && group != "function-applications" {
			t.Fatalf("fixture group %q has %d cases, want %d", group, len(found.Cases), want)
		}
		manifest[group] = make(map[string]struct{}, want)
		for _, fixture := range found.Cases {
			if group == "function-sift" && fixture.ID != "case002" {
				continue
			}
			if group == "function-applications" && fixture.ID != "case021" {
				continue
			}
			manifest[group][fixture.ID] = struct{}{}
		}
	}
	if got := regexManifestCaseCount(manifest); got != 82 {
		t.Fatalf("regex fixture count = %d, want 82", got)
	}
	report := conformance.RunWithOptions(suite, regexFixtureCompiler{}, manifest, conformance.Options{
		UndefinedError: jsonata.ErrUndefined,
	})
	if len(report.Failures) != 0 {
		t.Fatalf("regex conformance failures: %+v", report.Failures)
	}
	if report.EnabledCases != 82 || report.Passes != 82 {
		t.Fatalf("regex conformance enabled=%d passes=%d, want 82", report.EnabledCases, report.Passes)
	}
}

func regexManifestCaseCount(manifest conformance.Manifest) int {
	total := 0
	for _, cases := range manifest {
		total += len(cases)
	}
	return total
}
