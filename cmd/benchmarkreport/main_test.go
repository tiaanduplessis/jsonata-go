package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func TestMarkdownNeverClaimsFastestWhenGateFails(t *testing.T) {
	document := reportDocument{Analysis: benchmark.Analysis{Claim: benchmark.ClaimGate{
		Met: false, Scope: "test", Reasons: []string{"not supported"},
	}}}
	report := string(renderMarkdown(document))
	if !strings.Contains(report, "No fastest-library claim") || !strings.Contains(report, "Met: **false**") {
		t.Fatalf("report did not make failed claim explicit:\n%s", report)
	}
}

func TestLoadRunEvidenceRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(path, []byte(`{"recorded_at":"2026-08-22T00:00:00Z","unknown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRunEvidence(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict manifest decode failure, got %v", err)
	}
}

func TestLoadReportRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(`{"generated_at":"2026-08-22T00:00:00Z","evidence_at":"2026-08-22T00:00:00Z","unknown":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadReport(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict report decode failure, got %v", err)
	}
}

func TestMarkdownStatesUnsupportedCellsAreUntimed(t *testing.T) {
	document := reportDocument{Analysis: benchmark.Analysis{Unsupported: []benchmark.VerificationRecord{{
		Implementation: "competitor", CaseID: "case", Mode: benchmark.ModeBytes, Class: "api", Reason: "not supported",
	}}}}
	report := string(renderMarkdown(document))
	if !strings.Contains(report, "were not timed and were not counted as wins") {
		t.Fatalf("unsupported policy missing from report:\n%s", report)
	}
}
