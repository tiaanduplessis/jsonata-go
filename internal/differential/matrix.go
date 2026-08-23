package differential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
)

type FeatureMatrix struct {
	SchemaVersion         int                    `json:"schemaVersion"`
	ReferenceName         string                 `json:"referenceName"`
	ReferenceCommit       string                 `json:"referenceCommit"`
	SuiteCases            int                    `json:"suiteCases"`
	Features              []FeatureClaim         `json:"features"`
	Library               []LibraryClaim         `json:"library"`
	BehavioralDifferences []BehavioralDifference `json:"behavioralDifferences"`
}

type FeatureClaim struct {
	ID       string       `json:"id"`
	Status   string       `json:"status"`
	Cases    int          `json:"cases"`
	Fixture  string       `json:"fixture"`
	Evidence string       `json:"evidence"`
	Outcome  GroupOutcome `json:"outcome"`
}

type GroupOutcome struct {
	Enabled   int `json:"enabled"`
	Passes    int `json:"passes"`
	Failures  int `json:"failures"`
	Skips     int `json:"skips"`
	Remaining int `json:"remaining"`
}

type LibraryClaim struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
}

type BehavioralDifference struct {
	ID                  string   `json:"id"`
	Feature             string   `json:"feature"`
	Status              string   `json:"status"`
	ReferenceBehavior   string   `json:"referenceBehavior"`
	LibraryBehavior     string   `json:"libraryBehavior"`
	Boundary            string   `json:"boundary"`
	FutureCompatibility string   `json:"futureCompatibility"`
	Evidence            []string `json:"evidence"`
}

func GenerateFeatureMatrix(suiteRoot, reportPath string) (FeatureMatrix, error) {
	suite, err := conformance.LoadSuite(suiteRoot)
	if err != nil {
		return FeatureMatrix{}, err
	}
	report, err := loadCompleteConformanceReport(reportPath, suite)
	if err != nil {
		return FeatureMatrix{}, err
	}
	discovered := make(map[string]int, len(report.Discovered))
	for _, group := range report.Discovered {
		discovered[group.Name] = group.Cases
	}
	enabled := make(map[string]bool, len(report.EnabledGroups))
	for _, name := range report.EnabledGroups {
		enabled[name] = true
	}
	features := make([]FeatureClaim, 0, len(suite.Groups))
	total := 0
	for _, group := range suite.Groups {
		if discovered[group.Name] != len(group.Cases) || !enabled[group.Name] {
			return FeatureMatrix{}, fmt.Errorf("conformance report does not prove group %q", group.Name)
		}
		features = append(features, FeatureClaim{
			ID:       group.Name,
			Status:   "supported",
			Cases:    len(group.Cases),
			Fixture:  fmt.Sprintf("testdata/reference/jsonata-js-v2.2.2/test/test-suite/groups/%s", group.Name),
			Evidence: "reports/conformance/report.json",
			Outcome:  GroupOutcome{Enabled: len(group.Cases), Passes: len(group.Cases)},
		})
		total += len(group.Cases)
	}
	return FeatureMatrix{
		SchemaVersion:   3,
		ReferenceName:   ReferenceName,
		ReferenceCommit: ReferenceCommit,
		SuiteCases:      total,
		Features:        features,
		Library: []LibraryClaim{
			{ID: "complete-language-suite", Status: "supported", Evidence: []string{"internal/conformance/full_suite_test.go", "reports/conformance/report.json"}},
			{ID: "structured-errors", Status: "supported", Evidence: []string{"internal/differential/corpus_test.go", "testdata/differential/oracle.json"}},
			{ID: "deterministic-differential-corpus", Status: "supported", Evidence: []string{"internal/differential/corpus_test.go", "testdata/differential/cases.json"}},
			{ID: "bounded-generated-differential-campaign", Status: "supported", Evidence: []string{"internal/differential/corpus_test.go", "testdata/differential/fuzz-cases.json", "testdata/differential/fuzz-oracle.json"}},
			{ID: "context-and-resource-guardrails", Status: "supported", Evidence: []string{"internal/evaluator/phase3_test.go", "internal/security/regression_test.go"}},
			{ID: "concurrent-evaluation", Status: "supported", Evidence: []string{"jsonata_test.go", "internal/security/regression_test.go"}},
			{ID: "safe-extension-registration", Status: "supported", Evidence: []string{"extensions_normalization_test.go", "internal/security/fuzz_test.go"}},
			{ID: "security-regressions", Status: "supported", Evidence: []string{"reports/security-regressions.json", "internal/security/regression_test.go"}},
		},
		BehavioralDifferences: []BehavioralDifference{
			{
				ID:                  "regex-unpaired-surrogate-results",
				Feature:             "regex",
				Status:              "documented",
				ReferenceBehavior:   "ECMAScript strings can contain unpaired UTF-16 surrogate code units.",
				LibraryBehavior:     "Boolean matches remain exact; a match, capture, replacement, split part, or callback argument containing an unpaired surrogate returns U1002.",
				Boundary:            "Go strings and encoding/json cannot preserve an unpaired surrogate without invalid UTF-8 or replacement.",
				FutureCompatibility: "Exact representation requires an explicit opt-in UTF-16 result API.",
				Evidence:            []string{"internal/regex/regex_test.go", "internal/evaluator/builtins_regex_test.go"},
			},
			{
				ID:                  "unicode-case-mapping-runtime",
				Feature:             "lowercase-uppercase",
				Status:              "documented",
				ReferenceBehavior:   "Node.js 24.19.0 with Unicode 17.0 uppercases U+019B to U+A7DC.",
				LibraryBehavior:     "Supported Go 1.26.6 builds with golang.org/x/text v0.39.0 use Unicode 15.0.0 tables and leave U+019B unchanged; supported Go 1.27.0 builds use Unicode 17.0.0 tables and uppercase U+019B to U+A7DC.",
				Boundary:            "ECMAScript casing follows the host runtime; this library's mapping is toolchain-dependent, with Go 1.26.6 differing at this boundary and Go 1.27.0 matching the pinned Node.js 24.19.0 result.",
				FutureCompatibility: "A supported Go runtime or x/text Unicode-table change requires rerunning the pinned Node.js oracle comparison and updating the version-specific claim.",
				Evidence:            []string{"go.mod", "internal/evaluator/builtins_string_test.go"},
			},
		},
	}, nil
}

func loadCompleteConformanceReport(path string, suite conformance.Suite) (conformance.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return conformance.Report{}, fmt.Errorf("read conformance report: %w", err)
	}
	var report conformance.Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return conformance.Report{}, fmt.Errorf("decode conformance report: %w", err)
	}
	if report.ReferenceName != ReferenceName || report.ReferenceCommit != ReferenceCommit || report.ReferenceCommit != suite.ReferenceCommit {
		return conformance.Report{}, fmt.Errorf("conformance report metadata does not match the pinned reference")
	}
	suiteCases := 0
	for _, group := range suite.Groups {
		suiteCases += len(group.Cases)
	}
	if len(report.Discovered) != len(suite.Groups) || len(report.EnabledGroups) != len(suite.Groups) || report.EnabledCases != suiteCases {
		return conformance.Report{}, fmt.Errorf("conformance report accounting does not match the pinned suite")
	}
	if report.EnabledCases != report.Passes || len(report.Failures) != 0 || len(report.Skips) != 0 || len(report.RemainingCases) != 0 || len(report.RemainingGroups) != 0 {
		return conformance.Report{}, fmt.Errorf("conformance report is incomplete")
	}
	return report, nil
}
