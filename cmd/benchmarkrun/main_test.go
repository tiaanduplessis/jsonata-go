package main

import (
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func TestRotatedOrderChangesFirstImplementation(t *testing.T) {
	implementations := []benchmark.ImplementationReference{{ID: "one"}, {ID: "two"}, {ID: "three"}}
	want := [][]string{{"one", "two", "three"}, {"two", "three", "one"}, {"three", "one", "two"}}
	for round := range want {
		if got := benchmark.ExpectedRoundOrder(implementations, len(want))[round]; !reflect.DeepEqual(got, want[round]) {
			t.Fatalf("round %d order = %v, want %v", round, got, want[round])
		}
	}
}

func TestBenchmarkCommandPinsCollectionControls(t *testing.T) {
	got := benchmark.BenchmarkCommand("250ms")
	want := []string{"go", "test", "./internal/benchmark", "-run", "^$", "-bench", "^BenchmarkMatrix$", "-benchmem", "-benchtime", "250ms", "-count", "1", "-timeout", "20m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %v, want %v", got, want)
	}
}
