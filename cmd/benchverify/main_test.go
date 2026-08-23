package main

import (
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func TestVerificationReportCountsUnsupportedCells(t *testing.T) {
	corpus := benchmark.Corpus{Cases: []benchmark.Case{{ID: "one"}}}
	matrix := benchmark.VerificationMatrix{Records: []benchmark.VerificationRecord{
		{Status: benchmark.StatusEligible},
		{Status: benchmark.StatusUnsupported, Reason: "unsupported"},
	}}
	report := newVerificationReport(corpus, matrix)
	if report.Eligible != 1 || report.Unsupported != 1 || report.Requested != 2 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
}
