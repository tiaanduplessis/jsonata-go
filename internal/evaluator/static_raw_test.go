package evaluator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/evaluator"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func TestStaticRawPathMatchesDecoder(t *testing.T) {
	tests := []struct {
		expression string
		input      string
		want       string
		planned    bool
	}{
		{`a.b`, `{"a":{"b":7}}`, `7`, true},
		{`a.b`, `{"a":{"b":1e2}}`, `100`, true},
		{`a.b`, `{"a":{"b":9007199254740993}}`, `9007199254740992`, true},
		{`a.b`, `{"a":{"b":-0}}`, `-0`, true},
		{`a.b`, `{"a":{"b":"line\nvalue"}}`, `"line\nvalue"`, true},
		{`ab.c`, `{"a\u0062":{"c":1}}`, `1`, true},
		{`a.b`, `{"a":{"b":"\ud83d\ude00"}}`, `"😀"`, true},
		{`a.b`, `{"a":{"b":"\ud800"}}`, `"�"`, true},
		{`a.b`, `{"a":{"b":null}}`, `null`, true},
		{`a.b`, `{"a":{"b":1,"b":2}}`, `2`, true},
		{`a.b`, `{"a":{"b":1},"a":{"b":2}}`, `2`, true},
		{`a.b`, `{"a":{"b":1},"a":{"b":[2]}}`, ``, false},
		{`a.b`, `{"a":{"b":1},"a":{"c":2}}`, ``, false},
		{`a.b`, `{"a":[{"b":1}]}`, ``, false},
		{`a.b`, `{"a":{"c":1}}`, ``, false},
		{`a.b`, `{"a":{"b":1}`, ``, false},
		{`a.b`, `{"a":{"b":1}} {"a":{"b":2}}`, ``, false},
	}
	for _, test := range tests {
		t.Run(test.expression+"/"+test.input, func(t *testing.T) {
			n, err := syntax.Parse(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			plan := evaluator.BuildStaticPathPlan(n)
			if plan == nil {
				t.Fatal("static path plan was not built")
			}
			got, ok := evaluator.EvalStaticPathBytes(plan, []byte(test.input))
			if ok != test.planned {
				t.Fatalf("planned=%v, got %s", ok, got)
			}
			if ok && !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("raw result=%s, want %s", got, test.want)
			}
			expr := jsonata.MustCompile(test.expression)
			full, fullErr := expr.EvalBytesWithOptions([]byte(test.input), jsonata.EvalOptions{MaxOperations: 100_000})
			if test.planned {
				if fullErr != nil || !bytes.Equal(got, full) {
					t.Fatalf("raw=%s, full=%s, err=%v", got, full, fullErr)
				}
			} else if fullErr == nil {
				// A fallback is expected for ambiguous values, not an error. The
				// full result is still checked for valid JSON below.
				var decoded any
				if err := json.Unmarshal(full, &decoded); err != nil {
					t.Fatalf("full output=%s: %v", full, err)
				}
			}
		})
	}
}

func TestStaticRawDecodedPlansMatchForcedFullEvaluation(t *testing.T) {
	cases := []struct {
		name string
		expr string
		data string
	}{
		{"filter-project", `orders[status="paid"].amount`, `{"orders":[{"status":"paid","amount":12},{"status":"pending","amount":9},{"status":"paid","amount":4.5}]}`},
		{"sum", `$sum(orders[status="paid"].amount)`, `{"orders":[{"status":"paid","amount":12},{"status":"pending","amount":9},{"status":"paid","amount":4.5}]}`},
		{"map", `$map(orders, function($item){$item.price * $item.quantity})`, `{"orders":[{"price":2,"quantity":3},{"price":4,"quantity":5}]}`},
		{"descendant-sum", `$sum(payload.**.value)`, `{"payload":{"value":4,"nested":{"value":7},"items":[{"value":5},{"value":6}]}}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			expr := jsonata.MustCompile(test.expr)
			fast, fastErr := expr.EvalBytes([]byte(test.data))
			full, fullErr := expr.EvalBytesWithOptions([]byte(test.data), jsonata.EvalOptions{Context: context.Background()})
			if fastErr != nil || fullErr != nil || !bytes.Equal(fast, full) {
				t.Fatalf("fast=(%s,%v), full=(%s,%v)", fast, fastErr, full, fullErr)
			}
		})
	}
}

func BenchmarkStaticRawDecodedPlans(b *testing.B) {
	cases := []struct {
		name string
		expr string
		data []byte
	}{
		{"filter-project", `orders[status="paid"].amount`, []byte(`{"orders":[{"status":"paid","amount":12},{"status":"pending","amount":9},{"status":"paid","amount":4.5}]}`)},
		{"sum", `$sum(orders[status="paid"].amount)`, []byte(`{"orders":[{"status":"paid","amount":12},{"status":"pending","amount":9},{"status":"paid","amount":4.5}]}`)},
		{"map", `$map(orders, function($item){$item.price * $item.quantity})`, []byte(`{"orders":[{"price":2,"quantity":3},{"price":4,"quantity":5}]}`)},
		{"descendant-sum", `$sum(payload.**.value)`, []byte(`{"payload":{"value":4,"nested":{"value":7},"items":[{"value":5},{"value":6}]}}`)},
	}
	for _, test := range cases {
		expr := jsonata.MustCompile(test.expr)
		b.Run(test.name+"/bridge", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := expr.EvalBytes(test.data); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(test.name+"/full", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := expr.EvalBytesWithOptions(test.data, jsonata.EvalOptions{Context: context.Background()}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestStaticRawDecodedPlansPreserveJSONBoundaries(t *testing.T) {
	expr := jsonata.MustCompile(`$sum(orders[status="paid"].amount)`)
	inputs := []string{
		`{"orders":[{"\u0073tatus":"pending","status":"paid","amount":2}]}`,
		`{"orders":[{"status":"paid","amount":9007199254740993}]}`,
	}
	for _, input := range inputs {
		fast, fastErr := expr.EvalBytes([]byte(input))
		full, fullErr := expr.EvalBytesWithOptions([]byte(input), jsonata.EvalOptions{Context: context.Background()})
		if fastErr != nil || fullErr != nil || !bytes.Equal(fast, full) {
			t.Fatalf("input %s: fast=(%s,%v), full=(%s,%v)", input, fast, fastErr, full, fullErr)
		}
	}
	stringExpr := jsonata.MustCompile(`orders[status="paid"].amount`)
	stringInput := []byte(`{"orders":[{"status":"paid","amount":"\ud800"}]}`)
	fastString, fastStringErr := stringExpr.EvalBytes(stringInput)
	fullString, fullStringErr := stringExpr.EvalBytesWithOptions(stringInput, jsonata.EvalOptions{Context: context.Background()})
	if fastStringErr != nil || fullStringErr != nil || !bytes.Equal(fastString, fullString) {
		t.Fatalf("unpaired surrogate: fast=(%s,%v), full=(%s,%v)", fastString, fastStringErr, fullString, fullStringErr)
	}
	for _, input := range []string{`{"orders":[{"status":"paid","amount":1}]`, `{"orders":[]} {"extra":true}`} {
		_, fastErr := expr.EvalBytes([]byte(input))
		_, fullErr := expr.EvalBytesWithOptions([]byte(input), jsonata.EvalOptions{Context: context.Background()})
		if (fastErr == nil) != (fullErr == nil) {
			t.Fatalf("input %s: fast error=%v, full error=%v", input, fastErr, fullErr)
		}
	}
	deep := `{"orders":[{"status":"paid","amount":1}]}`
	for i := 0; i < 101; i++ {
		deep = `{"nested":` + deep + `}`
	}
	if _, err := expr.EvalBytes([]byte(deep)); err == nil {
		t.Fatal("deep document unexpectedly evaluated without an error")
	}
	large := `{"orders":[{"status":"paid","amount":1}],"padding":{` + strings.Repeat(`"x":0,`, 30_000) + `"last":0}}`
	if _, err := expr.EvalBytes([]byte(large)); err != nil {
		t.Fatalf("large but valid document failed: %v", err)
	}
}

func TestStaticRawComparisonMatchesDecoder(t *testing.T) {
	tests := []struct {
		expression string
		input      string
		want       string
		planned    bool
	}{
		{`customer.profile.tier = "gold"`, `{"customer":{"profile":{"tier":"gold"}}}`, `true`, true},
		{`customer.profile.tier != "gold"`, `{"customer":{"profile":{"tier":"silver"}}}`, `true`, true},
		{`amount = 7`, `{"amount":7e0}`, `true`, true},
		{`amount = 9007199254740993`, `{"amount":9007199254740992}`, `true`, true},
		{`active = true`, `{"active":true}`, `true`, true},
		{`active != false`, `{"active":true}`, `true`, true},
		{`amount = 7`, `{"amount":null}`, ``, false},
		{`amount = 7`, `{"amount":[7]}`, ``, false},
		{`amount = 7`, `{"amount":7,"amount":[]}`, ``, false},
		{`amount = 7`, `{"amount":7}`, `true`, true},
		{`amount = 7`, `{"amount":7} {}`, ``, false},
	}
	for _, test := range tests {
		t.Run(test.expression+"/"+test.input, func(t *testing.T) {
			n, err := syntax.Parse(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			plan := evaluator.BuildStaticComparisonPlan(n)
			if plan == nil {
				t.Fatal("static comparison plan was not built")
			}
			got, ok := evaluator.EvalStaticComparisonBytes(plan, []byte(test.input))
			if ok != test.planned {
				t.Fatalf("planned=%v, got %s", ok, got)
			}
			if ok && !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("raw result=%s, want %s", got, test.want)
			}
			expr := jsonata.MustCompile(test.expression)
			full, fullErr := expr.EvalBytesWithOptions([]byte(test.input), jsonata.EvalOptions{MaxOperations: 100_000})
			if test.planned && (fullErr != nil || !bytes.Equal(got, full)) {
				t.Fatalf("raw=%s, full=%s, err=%v", got, full, fullErr)
			}
		})
	}
}

func TestStaticRawRejectsDepthAndWorkLimits(t *testing.T) {
	n, err := syntax.Parse(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	deep := []byte(`{"a":`)
	for i := 0; i < 101; i++ {
		deep = append(deep, []byte(`{"x":`)...)
	}
	deep = append(deep, []byte(`1`)...)
	for i := 0; i < 102; i++ {
		deep = append(deep, '}')
	}
	if _, ok := evaluator.EvalStaticPathBytes(plan, deep); ok {
		t.Fatal("overly deep input used the raw fast path")
	}

	var large bytes.Buffer
	large.WriteString(`{"a":{"b":1,"unrelated":[`)
	for i := 0; i < 100_001; i++ {
		if i != 0 {
			large.WriteByte(',')
		}
		large.WriteByte('0')
	}
	large.WriteString(`]}}`)
	if _, ok := evaluator.EvalStaticPathBytes(plan, large.Bytes()); ok {
		t.Fatal("overly large input used the raw fast path")
	}
}

func TestStaticRawConcurrentUse(t *testing.T) {
	pathNode, err := syntax.Parse(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	path := evaluator.BuildStaticPathPlan(pathNode)
	comparisonNode, err := syntax.Parse(`a = 7`)
	if err != nil {
		t.Fatal(err)
	}
	comparison := evaluator.BuildStaticComparisonPlan(comparisonNode)
	input := []byte(`{"a":{"b":7}}`)
	comparisonInput := []byte(`{"a":7}`)
	const workers = 16
	const iterations = 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				got, ok := evaluator.EvalStaticPathBytes(path, input)
				if !ok || !bytes.Equal(got, []byte("7")) {
					t.Errorf("path=%s ok=%v", got, ok)
					return
				}
				got, ok = evaluator.EvalStaticComparisonBytes(comparison, comparisonInput)
				if !ok || !bytes.Equal(got, []byte("true")) {
					t.Errorf("comparison=%s ok=%v", got, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkStaticRawPath(b *testing.B) {
	n, err := syntax.Parse(`customer.profile.tier`)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	input := []byte(`{"customer":{"profile":{"tier":"gold"},"unrelated":[1,2,3]}}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, ok := evaluator.EvalStaticPathBytes(plan, input)
		if !ok {
			b.Fatal("raw path was not planned")
		}
		b.SetBytes(int64(len(input)))
		_ = result
	}
}

func BenchmarkStaticRawComparison(b *testing.B) {
	n, err := syntax.Parse(`customer.profile.tier = "gold"`)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticComparisonPlan(n)
	input := []byte(`{"customer":{"profile":{"tier":"gold"},"unrelated":[1,2,3]}}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, ok := evaluator.EvalStaticComparisonBytes(plan, input)
		if !ok {
			b.Fatal("raw comparison was not planned")
		}
		_ = result
	}
}

func TestStaticRawPathOutputIsJSON(t *testing.T) {
	n, err := syntax.Parse(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	got, ok := evaluator.EvalStaticPathBytes(plan, []byte(`{"a":{"b":"x"}}`))
	if !ok {
		t.Fatal("raw path was not planned")
	}
	var value any
	if err := json.Unmarshal(got, &value); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, "x") {
		t.Fatalf("output=%#v", value)
	}
}

func TestStaticRawCanonicalizationAndOwnership(t *testing.T) {
	n, err := syntax.Parse(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "html escaping", input: `{"a":{"b":"<&>"}}`, want: `"\u003c\u0026\u003e"`},
		{name: "small exponent", input: `{"a":{"b":1e-9}}`, want: `1e-9`},
		{name: "fixed cutoff", input: `{"a":{"b":1e-6}}`, want: `0.000001`},
		{name: "large exponent", input: `{"a":{"b":1e21}}`, want: `1e+21`},
		{name: "precision", input: `{"a":{"b":9007199254740993}}`, want: `9007199254740992`},
		{name: "negative zero", input: `{"a":{"b":-0}}`, want: `-0`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := []byte(test.input)
			got, ok := evaluator.EvalStaticPathBytes(plan, input)
			if !ok || !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("got %s, ok=%v, want %s", got, ok, test.want)
			}
			full, err := jsonata.MustCompile(`a.b`).EvalBytes([]byte(test.input))
			if err != nil || !bytes.Equal(got, full) {
				t.Fatalf("raw=%s, full=%s, err=%v", got, full, err)
			}
			for i := range input {
				input[i] = 'x'
			}
			if !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("result aliases input: got %s after input mutation", got)
			}
		})
	}
}

func TestStaticRawScannerPreservesJSONValidation(t *testing.T) {
	n, err := syntax.Parse(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	for _, input := range []string{
		`{"a":{"b":"\q"}}`,
		`{"a":{"b":"\u12x4"}}`,
		"{\"a\":{\"b\":\"\x01\"}}",
		`{"a":{"b":1,}}`,
		`{"a":[1,]}`,
		`{"a":{"b":1}} trailing`,
		`{"a":{"b":1},"unrelated":{"bad":"\q"}}`,
	} {
		t.Run(input, func(t *testing.T) {
			if got, ok := evaluator.EvalStaticPathBytes(plan, []byte(input)); ok {
				t.Fatalf("invalid input used raw path: %s", got)
			}
			if _, err := jsonata.MustCompile(`a.b`).EvalBytes([]byte(input)); err == nil {
				t.Fatal("invalid input unexpectedly evaluated")
			}
		})
	}
	invalidUTF8 := []byte{'{', '"', 'a', '"', ':', '{', '"', 'b', '"', ':', '"', 0xff, '"', '}', '}'}
	got, ok := evaluator.EvalStaticPathBytes(plan, invalidUTF8)
	if !ok || !bytes.Equal(got, []byte(`"�"`)) {
		t.Fatalf("invalid UTF-8 handling: got %s, ok=%v", got, ok)
	}
	full, fullErr := jsonata.MustCompile(`a.b`).EvalBytesWithOptions(invalidUTF8, jsonata.EvalOptions{Context: context.Background()})
	if fullErr != nil || !bytes.Equal(got, full) {
		t.Fatalf("invalid UTF-8 fast=%s, full=(%s,%v)", got, full, fullErr)
	}
}

func BenchmarkStaticRawPathSimple(b *testing.B) {
	benchmarkStaticRawPath(b, `a.b`, []byte(`{"a":{"b":7}}`))
}

func BenchmarkStaticRawContainsASCIIIgnoreCase(b *testing.B) {
	n, err := syntax.Parse(`$contains(message, /order-[0-9]{4}/i)`)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticContainsPlan(n)
	input := []byte(`{"message":"Accepted ORDER-2048 for processing"}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matched, ok := evaluator.EvalStaticContainsBytes(plan, input)
		if !ok || !matched {
			b.Fatal("raw ASCII ignore-case contains was not planned")
		}
	}
}

func BenchmarkStaticRawPathDeep(b *testing.B) {
	benchmarkStaticRawPath(b, `a.b.c.d.e`, []byte(`{"a":{"b":{"c":{"d":{"e":7}}}}}`))
}

func BenchmarkStaticRawPathWide(b *testing.B) {
	benchmarkStaticRawPath(b, `selected.value`, []byte(`{"unrelated":{"one":1,"two":2,"three":3},"selected":{"value":7},"tail":[1,2,3,4,5]}`))
}

func benchmarkStaticRawPath(b *testing.B, expression string, input []byte) {
	n, err := syntax.Parse(expression)
	if err != nil {
		b.Fatal(err)
	}
	plan := evaluator.BuildStaticPathPlan(n)
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result, ok := evaluator.EvalStaticPathBytes(plan, input); !ok {
			b.Fatal("raw path was not planned")
		} else {
			_ = result
		}
	}
}
