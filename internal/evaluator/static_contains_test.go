package evaluator_test

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestStaticContainsPlanRecognisesOnlyLiteralRegexCalls(t *testing.T) {
	for _, expression := range []string{
		`$contains(label, /quick|fox/)`,
		`$contains(label, /a/i)`,
		`$contains(customer.profile.label, /bar/)`,
	} {
		n, err := syntax.Parse(expression)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expression, err)
		}
		if plan := evaluator.BuildStaticContainsPlan(n); plan == nil {
			t.Fatalf("BuildStaticContainsPlan(%q) returned nil", expression)
		}
	}
	for _, expression := range []string{
		`$contains(label, "fox")`,
		`$contains(label, $pattern)`,
		`$contains(labels[], /fox/)`,
		`$contains(label, function($s){/fox/})`,
		`$contains(label, /fox/, 1)`,
	} {
		n, err := syntax.Parse(expression)
		if err != nil {
			continue
		}
		if plan := evaluator.BuildStaticContainsPlan(n); plan != nil {
			t.Fatalf("BuildStaticContainsPlan(%q) unexpectedly planned", expression)
		}
	}
}

func TestStaticContainsDecodedMatchesFullEvaluator(t *testing.T) {
	cases := []struct {
		expression string
		input      any
		want       any
	}{
		{`$contains(label, /quick|fox/)`, map[string]any{"label": "the quick brown fox"}, true},
		{`$contains(label, /quick|fox/)`, map[string]any{"label": "the slow brown dog"}, false},
		{`$contains(label, /a/i)`, map[string]any{"label": "Café"}, true},
		{`$contains(label, /(?<=foo)bar/)`, map[string]any{"label": "foobar"}, true},
		{`$contains(label, /(a)\1/)`, map[string]any{"label": "zaabb"}, true},
		{`$contains(label, /order-[0-9]{4}/i)`, map[string]any{"label": "Accepted ORDER-2048"}, true},
		{`$contains(label, /transaction-[0-9]{8}-approved/i)`, map[string]any{"label": "TRANSACTION-20260822-APPROVED"}, true},
		{`$contains(label, /k/i)`, map[string]any{"label": "K"}, false},
		{`$contains(label, /s/i)`, map[string]any{"label": "ſ"}, false},
		{`$contains(customer.profile.label, /bar/)`, map[string]any{"customer": map[string]any{"profile": map[string]any{"label": "bar"}}}, true},
	}
	for _, test := range cases {
		expr := jsonata.MustCompile(test.expression)
		fast, fastErr := expr.Eval(test.input)
		full, fullErr := expr.EvalWithOptions(test.input, jsonata.EvalOptions{Context: context.Background()})
		if fastErr != nil || fullErr != nil || !reflect.DeepEqual(fast, test.want) || !reflect.DeepEqual(fast, full) {
			t.Fatalf("%s input=%#v: fast=(%#v,%v), full=(%#v,%v), want=%#v", test.expression, test.input, fast, fastErr, full, fullErr, test.want)
		}
	}
}

func TestStaticContainsFallsBackForAmbiguousAndUnsafeInputs(t *testing.T) {
	expr := jsonata.MustCompile(`$contains(label, /a/)`)
	inputs := []any{
		map[string]any{},
		map[string]any{"label": nil},
		map[string]any{"label": 7},
		map[string]any{"label": []any{"a"}},
		map[string]any{"label": map[string]any{"value": "a"}},
		map[string]any{"label": "b", "bad": math.NaN()},
	}
	for _, input := range inputs {
		fast, fastErr := expr.Eval(input)
		full, fullErr := expr.EvalWithOptions(input, jsonata.EvalOptions{Context: context.Background()})
		if (fastErr == nil) != (fullErr == nil) || !reflect.DeepEqual(fast, full) {
			t.Fatalf("input %#v: fast=(%#v,%v), full=(%#v,%v)", input, fast, fastErr, full, fullErr)
		}
	}
	cycle := map[string]any{}
	cycle["loop"] = cycle
	cycle["label"] = "a"
	fast, fastErr := expr.Eval(cycle)
	full, fullErr := expr.EvalWithOptions(cycle, jsonata.EvalOptions{Context: context.Background()})
	if fast != nil || full != nil || fastErr == nil || fullErr == nil || fastErr.Error() != fullErr.Error() {
		t.Fatalf("cycle: fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expr.EvalWithOptions(map[string]any{"label": "a"}, jsonata.EvalOptions{Context: ctx}); err == nil {
		t.Fatal("canceled context bypassed static contains gating")
	}
	large := map[string]any{"label": strings.Repeat("a", 100_000)}
	fast, fastErr = expr.Eval(large)
	full, fullErr = expr.EvalWithOptions(large, jsonata.EvalOptions{Context: context.Background()})
	if fast != nil || full != nil || fastErr == nil || fullErr == nil || fastErr.Error() != fullErr.Error() {
		t.Fatalf("oversized regex input: fast=(%#v,%v), full=(%#v,%v)", fast, fastErr, full, fullErr)
	}
}

func TestStaticContainsRegistryAndRawGates(t *testing.T) {
	n, err := syntax.Parse(`$contains(label, /a/)`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticContainsPlan(n)
	if plan == nil {
		t.Fatal("missing plan")
	}
	if !plan.RegistryConflict(map[string]any{"contains": func() {}}) || !plan.RegistryConflict(map[string]any{"label": 1}) {
		t.Fatal("relevant registry conflict was not detected")
	}
	if plan.RegistryConflict(map[string]any{"other": 1}) {
		t.Fatal("unrelated registry conflict disabled plan")
	}
	expr := jsonata.MustCompile(`$contains(label, /a/)`)
	for _, test := range []struct {
		input string
		want  []byte
	}{
		{`{"label":"a"}`, []byte("true")},
		{`{"label":"b"}`, []byte("false")},
		{`{"label":"\u0061"}`, []byte("true")},
	} {
		got, gotErr := expr.EvalBytes([]byte(test.input))
		full, fullErr := expr.EvalBytesWithOptions([]byte(test.input), jsonata.EvalOptions{Context: context.Background()})
		if gotErr != nil || fullErr != nil || !bytes.Equal(got, test.want) || !bytes.Equal(got, full) {
			t.Fatalf("input=%s: raw=(%s,%v), full=(%s,%v), want=%s", test.input, got, gotErr, full, fullErr, test.want)
		}
	}
	for _, input := range []string{`{"label":null}`, `{"label":["a"]}`, `{"label":"a","bad":1e999}`,
		`{"label":"a"}`, `{"label":"a"} {}`} {
		_, rawErr := expr.EvalBytes([]byte(input))
		_, fullErr := expr.EvalBytesWithOptions([]byte(input), jsonata.EvalOptions{Context: context.Background()})
		if (rawErr == nil) != (fullErr == nil) {
			t.Fatalf("input=%s: raw error=%v, full error=%v", input, rawErr, fullErr)
		}
	}
	ignoreCase := jsonata.MustCompile(`$contains(label, /order-[0-9]{4}/i)`)
	for _, input := range []string{
		`{"label":"Accepted ORDER-2048"}`,
		`{"label":"Accepted order-2048"}`,
		`{"label":"Accepted ORDER-２０４８"}`,
		`{"label":"Accepted K"}`,
	} {
		got, gotErr := ignoreCase.EvalBytes([]byte(input))
		full, fullErr := ignoreCase.EvalBytesWithOptions([]byte(input), jsonata.EvalOptions{Context: context.Background()})
		if (gotErr == nil) != (fullErr == nil) || !bytes.Equal(got, full) {
			t.Fatalf("input=%s: raw=(%s,%v), full=(%s,%v)", input, got, gotErr, full, fullErr)
		}
	}
}

func TestStaticContainsConcurrentReuse(t *testing.T) {
	expr := jsonata.MustCompile(`$contains(label, /a+/)`)
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				input := map[string]any{"label": strings.Repeat("a", i%7) + "b"}
				got, err := expr.Eval(input)
				if err != nil || got != (i%7 > 0) {
					t.Errorf("input=%#v got=(%#v,%v)", input, got, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkStaticContainsMatrix(b *testing.B) {
	cases := []struct {
		name string
		expr *jsonata.Expr
		data any
	}{
		{"small-re2", jsonata.MustCompile(`$contains(label, /quick|fox/)`), map[string]any{"label": "the quick brown fox"}},
		{"small-ascii-ignore-case", jsonata.MustCompile(`$contains(label, /order-[0-9]{4}/i)`), map[string]any{"label": "Accepted ORDER-2048"}},
		{"small-fallback", jsonata.MustCompile(`$contains(label, /(?<=foo)bar/)`), map[string]any{"label": "foobar"}},
		{"large-re2", jsonata.MustCompile(`$contains(payload.text, /needle/)`), map[string]any{"payload": map[string]any{"text": strings.Repeat("x", 4096) + "needle"}}},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := test.expr.Eval(test.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
