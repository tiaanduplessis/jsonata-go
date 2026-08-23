package differential_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/differential"
)

func TestFeatureMatrixIsCompleteAndCurrent(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	reportPath := filepath.Join(repositoryRoot, "reports", "conformance", "report.json")
	matrix, err := differential.GenerateFeatureMatrix(filepath.Join(repositoryRoot, "testdata", "reference", "jsonata-js-v2.2.2"), reportPath)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := differential.Encode(matrix)
	if err != nil {
		t.Fatal(err)
	}
	committed := readFixture(t, filepath.Join(repositoryRoot, "reports", "feature-matrix.json"))
	if !bytes.Equal(encoded, committed) {
		t.Fatal("feature matrix is stale; run go run ./cmd/differential")
	}

	var decoded differential.FeatureMatrix
	decoder := json.NewDecoder(bytes.NewReader(committed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 3 || decoded.ReferenceName != differential.ReferenceName || decoded.ReferenceCommit != differential.ReferenceCommit {
		t.Fatal("feature matrix is not tied to the pinned reference")
	}
	if len(decoded.Features) == 0 || decoded.SuiteCases != differentialSuiteSize(t, repositoryRoot) {
		t.Fatalf("feature matrix suite total = %d", decoded.SuiteCases)
	}
	for _, feature := range decoded.Features {
		if feature.Status != "supported" || feature.Cases < 1 || feature.Fixture == "" || feature.Evidence != "reports/conformance/report.json" {
			t.Fatalf("incomplete feature claim: %#v", feature)
		}
		if feature.Outcome != (differential.GroupOutcome{Enabled: feature.Cases, Passes: feature.Cases}) {
			t.Fatalf("feature %q outcome does not prove complete support: %#v", feature.ID, feature.Outcome)
		}
		if _, err := os.Stat(filepath.Join(repositoryRoot, feature.Evidence)); err != nil {
			t.Fatalf("feature %q evidence: %v", feature.ID, err)
		}
		if _, err := os.Stat(filepath.Join(repositoryRoot, feature.Fixture)); err != nil {
			t.Fatalf("feature %q fixture: %v", feature.ID, err)
		}
	}

	required := map[string]bool{
		"complete-language-suite":                 false,
		"structured-errors":                       false,
		"deterministic-differential-corpus":       false,
		"bounded-generated-differential-campaign": false,
		"context-and-resource-guardrails":         false,
		"concurrent-evaluation":                   false,
		"safe-extension-registration":             false,
		"security-regressions":                    false,
	}
	for _, claim := range decoded.Library {
		if claim.Status != "supported" || len(claim.Evidence) == 0 {
			t.Fatalf("incomplete library claim: %#v", claim)
		}
		if _, ok := required[claim.ID]; !ok {
			t.Fatalf("undocumented library claim %q", claim.ID)
		}
		required[claim.ID] = true
		for _, evidence := range claim.Evidence {
			if _, err := os.Stat(filepath.Join(repositoryRoot, evidence)); err != nil {
				t.Fatalf("library claim %q evidence %q: %v", claim.ID, evidence, err)
			}
		}
	}
	for id, present := range required {
		if !present {
			t.Fatalf("feature matrix is missing library claim %q", id)
		}
	}
	if len(decoded.BehavioralDifferences) != 2 {
		t.Fatalf("feature matrix behavioral differences = %d, want 2 documented runtime boundaries", len(decoded.BehavioralDifferences))
	}
	wantDifferences := map[string]string{
		"regex-unpaired-surrogate-results": "regex",
		"unicode-case-mapping-runtime":     "lowercase-uppercase",
	}
	for _, difference := range decoded.BehavioralDifferences {
		feature, ok := wantDifferences[difference.ID]
		if !ok || difference.Feature != feature || difference.Status != "documented" || difference.ReferenceBehavior == "" || difference.LibraryBehavior == "" || difference.Boundary == "" || difference.FutureCompatibility == "" || len(difference.Evidence) == 0 {
			t.Fatalf("incomplete behavioral difference: %#v", difference)
		}
		if difference.ID == "unicode-case-mapping-runtime" {
			for _, want := range []string{"Go 1.26.6", "Unicode 15.0.0", "U+019B unchanged", "Go 1.27.0", "Unicode 17.0.0", "U+019B to U+A7DC"} {
				if !strings.Contains(difference.LibraryBehavior, want) {
					t.Fatalf("Unicode casing boundary is missing %q: %s", want, difference.LibraryBehavior)
				}
			}
		}
		delete(wantDifferences, difference.ID)
		for _, evidence := range difference.Evidence {
			if _, err := os.Stat(filepath.Join(repositoryRoot, evidence)); err != nil {
				t.Fatalf("behavioral difference %q evidence %q: %v", difference.ID, evidence, err)
			}
		}
	}
	if len(wantDifferences) != 0 {
		t.Fatalf("missing behavioral differences: %#v", wantDifferences)
	}
}

func TestFeatureMatrixRejectsIncompleteConformanceEvidence(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	data := readFixture(t, filepath.Join(repositoryRoot, "reports", "conformance", "report.json"))
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	report["passes"] = float64(1685)
	mutated, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = differential.GenerateFeatureMatrix(filepath.Join(repositoryRoot, "testdata", "reference", "jsonata-js-v2.2.2"), path)
	if err == nil {
		t.Fatal("incomplete conformance evidence was accepted")
	}
}

func differentialSuiteSize(t *testing.T, repositoryRoot string) int {
	t.Helper()
	data := readFixture(t, filepath.Join(repositoryRoot, "reports", "conformance", "report.json"))
	var report struct {
		ReferenceCommit string `json:"referenceCommit"`
		EnabledCases    int    `json:"enabledCases"`
		Passes          int    `json:"passes"`
		Failures        []any  `json:"failures"`
		Skips           []any  `json:"skips"`
		RemainingCases  []any  `json:"remainingCases"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.ReferenceCommit != differential.ReferenceCommit || report.EnabledCases != report.Passes || len(report.Failures) != 0 || len(report.Skips) != 0 || len(report.RemainingCases) != 0 {
		t.Fatalf("conformance report does not prove complete support: %#v", report)
	}
	return report.EnabledCases
}
