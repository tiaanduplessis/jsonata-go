package regex

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
)

func TestCompileLiteralUsesRE2ForPortablePatterns(t *testing.T) {
	pattern, err := CompileLiteral(`/a+/`)
	if err != nil {
		t.Fatal(err)
	}
	if pattern.re2 == nil {
		t.Fatal("portable pattern did not use the RE2 fast path")
	}
	if pattern.contextJS != nil {
		t.Fatal("portable pattern unnecessarily built a bounded fallback engine")
	}
	matched, err := pattern.MatchString("baaa")
	if err != nil || !matched {
		t.Fatalf("match = %v, err=%v", matched, err)
	}
}

func TestRE2FastPathIsConservative(t *testing.T) {
	for _, literal := range []string{`/./`, `/^a$/`, `/[a]/`, `/\s/`, `/é/`, `/a/m`} {
		pattern, err := CompileLiteral(literal)
		if err != nil {
			t.Fatalf("%s: %v", literal, err)
		}
		if pattern.re2 != nil {
			t.Errorf("%s used the RE2 fast path", literal)
		}
	}
}

func TestASCIIIgnoreCaseRE2StaticPath(t *testing.T) {
	for _, literal := range []string{`/a/i`, `/order-[0-9]{4}/i`, `/transaction-[0-9]{8}-approved/i`} {
		pattern, err := CompileLiteral(literal)
		if err != nil {
			t.Fatal(err)
		}
		if pattern.asciiRe2 == nil {
			t.Fatalf("%s did not compile the ASCII-only RE2 product", literal)
		}
	}
	for _, literal := range []string{`/./i`, `/[a]/im`, `/[^a]/i`, `/\\w/i`, `/^a$/i`, `/a/m`} {
		pattern, err := CompileLiteral(literal)
		if err != nil {
			t.Fatal(err)
		}
		if pattern.asciiRe2 != nil {
			t.Fatalf("%s unexpectedly compiled the ASCII-only RE2 product", literal)
		}
	}
}

func TestASCIIIgnoreCaseRE2MatchesFallbackOnlyForASCII(t *testing.T) {
	for _, test := range []struct {
		literal string
		input   string
	}{
		{literal: `/order-[0-9]{4}/i`, input: "Accepted ORDER-2048"},
		{literal: `/transaction-[0-9]{8}-approved/i`, input: "TRANSACTION-20260822-APPROVED"},
		{literal: `/k/i`, input: "K"},
		{literal: `/k/i`, input: "K"},
		{literal: `/s/i`, input: "ſ"},
	} {
		pattern, err := CompileLiteral(test.literal)
		if err != nil {
			t.Fatal(err)
		}
		static, staticErr := pattern.MatchStringStatic(test.input)
		fallback, fallbackErr := pattern.MatchStringContext(context.Background(), test.input)
		if (staticErr == nil) != (fallbackErr == nil) || static != fallback {
			t.Fatalf("%s input=%q: static=(%t,%v), fallback=(%t,%v)", test.literal, test.input, static, staticErr, fallback, fallbackErr)
		}
	}
}

func TestRE2FastPathReportsUTF16Indexes(t *testing.T) {
	pattern, err := CompileLiteral(`/x/`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pattern.FindAllStringSubmatchUTF16Index("💩x", -1)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{2, 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indexes = %#v, want %#v", got, want)
	}
}

func TestCompileLiteralUsesECMAScriptFallback(t *testing.T) {
	pattern, err := CompileLiteral(`/(?<=foo)bar/`)
	if err != nil {
		t.Fatal(err)
	}
	if pattern.re2 != nil {
		t.Fatal("lookbehind pattern used the RE2 fast path")
	}
	indexes, err := pattern.FindStringSubmatchIndex("foobar")
	if err != nil || len(indexes) < 2 || indexes[0] != 3 || indexes[1] != 6 {
		t.Fatalf("indexes = %#v, err=%v", indexes, err)
	}
}

func TestWithTimeoutReusesMatchingFallbackEngine(t *testing.T) {
	pattern, err := CompileLiteral(`/a/i`)
	if err != nil {
		t.Fatal(err)
	}
	bounded := pattern.WithTimeout(10 * time.Millisecond)
	if bounded.js != pattern.contextJS {
		t.Fatal("default bounded timeout did not use the prebuilt fallback engine")
	}
	reused := bounded.WithTimeout(10 * time.Millisecond)
	if reused.js != bounded.js {
		t.Fatal("matching timeout rebuilt the fallback engine")
	}
	if reused.js.MatchTimeout != 10*time.Millisecond {
		t.Fatalf("cached timeout = %s, want 10ms", reused.js.MatchTimeout)
	}
}

func TestWithTimeoutCachedEngineIsSafeConcurrently(t *testing.T) {
	pattern, err := CompileLiteral(`/a/i`)
	if err != nil {
		t.Fatal(err)
	}
	bounded := pattern.WithTimeout(100 * time.Millisecond)
	if bounded.contextBound().js != bounded.js {
		t.Fatal("context-aware matching did not preserve the explicit timeout engine")
	}
	const workers = 16
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				matched, matchErr := bounded.WithTimeout(100*time.Millisecond).MatchStringContext(context.Background(), "a")
				if matchErr != nil || !matched {
					t.Errorf("match=%v err=%v, want true", matched, matchErr)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestCompileLiteralUsesBalancedDelimiterDepth(t *testing.T) {
	for _, literal := range []string{`/(a/b)/`, `/({a/b}[c/d])/`, `/[/]/`, `/[a/b]/`, `/\//`} {
		if _, err := CompileLiteral(literal); err != nil {
			t.Errorf("CompileLiteral(%q): %v", literal, err)
		}
	}
	for _, literal := range []string{`/(a/b/`, `/[a/b/`, `/)/`, `/[(]/`} {
		_, err := CompileLiteral(literal)
		var compileErr *CompileError
		if !errors.As(err, &compileErr) || compileErr.Code != "S0302" {
			t.Errorf("CompileLiteral(%q) error = %v, want S0302", literal, err)
		}
	}
}

func TestUTF16Index(t *testing.T) {
	if got := UTF16Index("a💩b", len("a💩")); got != 3 {
		t.Fatalf("UTF16Index = %d, want 3", got)
	}
	if got := ByteIndex("a💩b", 3); got != len("a💩") {
		t.Fatalf("ByteIndex = %d, want %d", got, len("a💩"))
	}
	if got, exact := ByteIndexExact("a💩b", 2); got != 1 || exact {
		t.Fatalf("ByteIndexExact inside surrogate = %d, %t; want 1, false", got, exact)
	}
}

func TestFallbackUsesECMAScriptUTF16CodeUnits(t *testing.T) {
	tests := []struct {
		literal string
		input   string
		want    [][]int
	}{
		{literal: `/é/`, input: "aé", want: [][]int{{1, 3}}},
		{literal: `/./`, input: "é", want: [][]int{{0, 2}}},
		{literal: `/./`, input: "e\u0301", want: [][]int{{0, 1}, {1, 3}}},
		{literal: `/../`, input: "💩", want: [][]int{{0, 4}}},
		{literal: `/💩/`, input: "x💩y", want: [][]int{{1, 5}}},
		{literal: `/(💩)\1/`, input: "💩💩", want: [][]int{{0, 8, 0, 4}}},
		{literal: `/(?<=💩)x/`, input: "💩x", want: [][]int{{4, 5}}},
	}
	for _, test := range tests {
		pattern, err := CompileLiteral(test.literal)
		if err != nil {
			t.Fatalf("CompileLiteral(%q): %v", test.literal, err)
		}
		got, err := pattern.FindAllStringSubmatchIndex(test.input, -1)
		if err != nil {
			t.Fatalf("%s against %q: %v", test.literal, test.input, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s against %q = %#v, want %#v", test.literal, test.input, got, test.want)
		}
	}
}

func TestFallbackEscapedAstralLiteralsUseUTF16CodeUnits(t *testing.T) {
	for _, test := range []struct {
		literal string
		want    [][]int
	}{
		{literal: `/\💩/`, want: [][]int{{0, 2}}},
		{literal: `/[\💩]/`, want: [][]int{{0, 1}, {1, 2}}},
	} {
		pattern, err := CompileLiteral(test.literal)
		if err != nil {
			t.Fatalf("CompileLiteral(%q): %v", test.literal, err)
		}
		got, err := pattern.FindAllStringSubmatchUTF16Index("💩", -1)
		if err != nil {
			t.Fatalf("%s: %v", test.literal, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s = %#v, want %#v", test.literal, got, test.want)
		}
	}
}

func TestLegacyIgnoreCaseDoesNotCollideWithPrivateUseCharacters(t *testing.T) {
	for _, test := range []struct {
		literal string
		input   string
	}{
		{literal: `/ſ/i`, input: "\ue001"},
		{literal: `/K/i`, input: "\ue000"},
		{literal: `/\\u017f/i`, input: `\u017f`},
	} {
		pattern, err := CompileLiteral(test.literal)
		if err != nil {
			t.Fatalf("CompileLiteral(%q): %v", test.literal, err)
		}
		matched, err := pattern.MatchString(test.input)
		if err != nil {
			t.Fatalf("%s against %q: %v", test.literal, test.input, err)
		}
		want := test.literal == `/\\u017f/i` && test.input == `\u017f`
		if matched != want {
			t.Errorf("%s against %q = %t, want %t", test.literal, test.input, matched, want)
		}
	}
}

func TestFallbackRejectsUnrepresentableLoneSurrogateResults(t *testing.T) {
	for _, literal := range []string{`/./`, `/[💩]/`} {
		pattern, err := CompileLiteral(literal)
		if err != nil {
			t.Fatal(err)
		}
		matched, err := pattern.MatchString("💩")
		if err != nil || !matched {
			t.Errorf("%s boolean match = %t, %v; want true", literal, matched, err)
		}
		_, err = pattern.FindAllStringSubmatchIndex("💩", -1)
		var representationErr *UTF16RepresentationError
		if !errors.As(err, &representationErr) {
			t.Errorf("%s result error = %v, want UTF16RepresentationError", literal, err)
		}
	}
}

func TestCompileLiteralDiagnosticsAndFallbackCancellation(t *testing.T) {
	for _, test := range []struct {
		literal string
		code    string
	}{
		{literal: `//`, code: "S0301"},
		{literal: `/a/s`, code: "S0302"},
		{literal: `/a/ii`, code: "S0302"},
	} {
		_, err := CompileLiteral(test.literal)
		var compileErr *CompileError
		if !errors.As(err, &compileErr) || compileErr.Code != test.code {
			t.Errorf("CompileLiteral(%q) error = %v, want %s", test.literal, err, test.code)
		}
	}

	pattern, err := CompileLiteral(`/(?<=foo)bar/`)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pattern.FindAllStringSubmatchIndexContext(canceled, "foobar", -1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled match error = %v, want context cancellation", err)
	}
}

func TestFallbackCancellationDoesNotDetachMatchWork(t *testing.T) {
	pattern, err := CompileLiteral(`/(a+)+$/`)
	if err != nil {
		t.Fatal(err)
	}
	pattern = pattern.WithTimeout(10 * time.Millisecond)
	input := strings.Repeat("a", 4096) + "!"
	before := runtime.NumGoroutine()
	for range 4 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		_, matchErr := pattern.MatchStringContext(ctx, input)
		cancel()
		if matchErr == nil {
			t.Fatal("adversarial fallback unexpectedly completed without an error")
		}
	}
	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Fatalf("fallback matches left %d goroutines; regexp2 permits only its one shared timeout clock", after-before)
	}
	// regexp2's single process-wide timeout clock is unavoidable, but unlike
	// match work it is bounded and explicitly stoppable after tests.
	regexp2.StopTimeoutClock()
}

func TestContextCancellationBoundsUntimedFallback(t *testing.T) {
	pattern, err := CompileLiteral(`/(a+)+$/`)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = pattern.MatchStringContext(ctx, strings.Repeat("a", 4096)+"!")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("untimed fallback error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("untimed fallback cancellation took %s", elapsed)
	}
}

func TestFallbackDotUsesECMAScriptLineTerminators(t *testing.T) {
	pattern, err := CompileLiteral(`/./`)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"\n", "\r", "\u2028", "\u2029"} {
		matched, err := pattern.MatchString(input)
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Errorf("dot matched ECMAScript line terminator %q", input)
		}
	}
}

func TestFallbackMultilineUsesECMAScriptLineTerminators(t *testing.T) {
	pattern, err := CompileLiteral(`/^b/m`)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"a\nb", "a\rb", "a\u2028b", "a\u2029b"} {
		matched, err := pattern.MatchString(input)
		if err != nil {
			t.Fatal(err)
		}
		if !matched {
			t.Errorf("multiline anchor did not match after %q", input[1:len(input)-1])
		}
	}
}

func TestFallbackIgnoreCaseUsesECMAScriptCanonicalization(t *testing.T) {
	for _, test := range []struct {
		literal string
		input   string
	}{
		{literal: `/k/i`, input: "K"},
		{literal: `/s/i`, input: "ſ"},
	} {
		pattern, err := CompileLiteral(test.literal)
		if err != nil {
			t.Fatal(err)
		}
		matched, err := pattern.MatchString(test.input)
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Errorf("%s matched non-ASCII fold %q", test.literal, test.input)
		}
	}
}

func BenchmarkFallbackContextMatrixIgnoreCase(b *testing.B) {
	for _, test := range []struct {
		name    string
		literal string
		input   string
	}{
		{name: "ascii", literal: `/a/i`, input: "a"},
		{name: "kelvin-fold", literal: `/k/i`, input: "K"},
		{name: "long-s-fold", literal: `/s/i`, input: "ſ"},
		{name: "private-use-collision", literal: `/ſ/i`, input: "\ue001"},
	} {
		b.Run(test.name, func(b *testing.B) {
			pattern, err := CompileLiteral(test.literal)
			if err != nil {
				b.Fatal(err)
			}
			bounded := pattern.WithTimeout(10 * time.Millisecond)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := bounded.FindAllStringSubmatchIndex(test.input, -1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFallbackContextTimeoutSetup(b *testing.B) {
	for _, literal := range []string{`/k/i`, `/s/i`, `/ſ/i`} {
		b.Run(literal, func(b *testing.B) {
			pattern, err := CompileLiteral(literal)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				_ = pattern.WithTimeout(10 * time.Millisecond)
			}
		})
	}
}
