package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

func TestConformanceOptionsTranslateMetadata(t *testing.T) {
	tests := []struct {
		name             string
		timeLimit, depth int
		wantTimeout      time.Duration
		wantMaxCallDepth int
	}{
		{name: "empty", wantTimeout: 0, wantMaxCallDepth: 0},
		{name: "tail fixture", timeLimit: 1000, depth: 302, wantTimeout: 0, wantMaxCallDepth: 0},
		{name: "small depth", depth: 2, wantMaxCallDepth: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := conformanceOptions(test.timeLimit, test.depth)
			if got.Timeout != test.wantTimeout || got.MaxCallDepth != test.wantMaxCallDepth || got.MaxOperations != 5_000_000 {
				t.Fatalf("options = timeout %s, depth %d, operations %d; want timeout %s, depth %d, operations 5000000", got.Timeout, got.MaxCallDepth, got.MaxOperations, test.wantTimeout, test.wantMaxCallDepth)
			}
		})
	}
}

func TestEvaluateConformanceCaseUsesPublicOptions(t *testing.T) {
	expression := jsonata.MustCompile("$x")
	bindings := map[string]any{"x": 7.0}
	got, err := evaluateConformanceCase(expression, nil, true, bindings, 1000, 302)
	if err != nil || !reflect.DeepEqual(got, 7.0) {
		t.Fatalf("got %#v, err %v", got, err)
	}
}

func TestEvaluateConformanceCaseRejectsUnknownExpression(t *testing.T) {
	_, err := evaluateConformanceCase(testConformanceExpression{}, nil, true, nil, 1, 1)
	if err == nil || !strings.Contains(err.Error(), "requires *jsonata.Expr") {
		t.Fatalf("error = %v, want unknown-expression error", err)
	}
}

func TestConformanceCompleteRejectsIncompleteReportsWithoutFailures(t *testing.T) {
	tests := []struct {
		name   string
		report conformance.Report
	}{
		{name: "skip", report: conformance.Report{Skips: []conformance.CaseRef{{Group: "sample", ID: "case000"}}}},
		{name: "remaining case", report: conformance.Report{RemainingCases: []conformance.CaseRef{{Group: "sample", ID: "case000"}}}},
		{name: "remaining group", report: conformance.Report{RemainingGroups: []string{"sample"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if conformanceComplete(test.report) {
				t.Fatalf("report with %s was accepted as complete", test.name)
			}
		})
	}
	if !conformanceComplete(conformance.Report{EnabledCases: 1, Passes: 1}) {
		t.Fatal("complete report was rejected")
	}
	if conformanceComplete(conformance.Report{Failures: []conformance.Failure{{Message: "failed"}}}) {
		t.Fatal("report with a failure was accepted as complete")
	}
}

type testConformanceExpression struct{}

func (testConformanceExpression) Eval(any) (any, error) { return nil, nil }

var _ conformance.Expression = testConformanceExpression{}
