package evaluator

import (
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
)

func BenchmarkCachedNumericLiteral(b *testing.B) {
	node, parseErr := syntax.Parse("$sum([1, 2, 3, 4, 5, 6, 7, 8])")
	if parseErr != nil {
		b.Fatal(parseErr)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := eval(node, state{runtime: newEvalRuntime(Options{})}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedRegexLiteral(b *testing.B) {
	node, parseErr := syntax.Parse(`$contains("the quick brown fox", /quick|fox/)`)
	if parseErr != nil {
		b.Fatal(parseErr)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := eval(node, state{runtime: newEvalRuntime(Options{})}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedRegexFallbackLiteral(b *testing.B) {
	node, parseErr := syntax.Parse(`$contains("a", /a/i)`)
	if parseErr != nil {
		b.Fatal(parseErr)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := eval(node, state{runtime: newEvalRuntime(Options{})}); err != nil {
			b.Fatal(err)
		}
	}
}
