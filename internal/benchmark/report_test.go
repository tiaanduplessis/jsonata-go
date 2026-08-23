package benchmark

import (
	"strings"
	"testing"
)

func TestParseBenchmarkOutput(t *testing.T) {
	raw := `goos: darwin
BenchmarkMatrix/decoded/small-path-10  1000  125.5 ns/op  64 B/op  3 allocs/op
BenchmarkMatrix/bytes/small-path-10  500  250 ns/op  128 B/op  7 allocs/op
PASS
`
	measurements, err := ParseBenchmarkOutput(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(measurements) != 2 || measurements[0].NsPerOp != 125.5 || measurements[1].BytesPerOp != 128 {
		t.Fatalf("unexpected measurements: %+v", measurements)
	}
}

func TestAnalyzeMeasurementsRequiresCompleteEligibleRawData(t *testing.T) {
	corpus, runtimes, matrix, raw := syntheticAnalysisInput()
	delete(raw[BluesImplementation], measurementKey(ModeBytes, "case"))
	_, err := AnalyzeMeasurements(corpus, runtimes, matrix, flattenMeasurements(raw))
	if err == nil || !strings.Contains(err.Error(), "requires exactly 6") {
		t.Fatalf("expected incomplete raw-data error, got %v", err)
	}
}

func TestAnalyzeMeasurementsRejectsTimedUnsupportedCell(t *testing.T) {
	corpus, runtimes, matrix, raw := syntheticAnalysisInput()
	matrix.Records[0].Status = StatusUnsupported
	matrix.buildIndex()
	_, err := AnalyzeMeasurements(corpus, runtimes, matrix, flattenMeasurements(raw))
	if err == nil || !strings.Contains(err.Error(), "timed ineligible cell") {
		t.Fatalf("expected ineligible timing error, got %v", err)
	}
}

func TestClaimRequiresStatisticalSupport(t *testing.T) {
	corpus, runtimes, matrix, raw := syntheticAnalysisInput()
	analysis, err := AnalyzeMeasurements(corpus, runtimes, matrix, flattenMeasurements(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !analysis.Claim.Met {
		t.Fatalf("expected supported claim, got %v", analysis.Claim.Reasons)
	}
	for _, suite := range analysis.Suites {
		if suite.Mode != ModeDecoded && suite.Mode != ModeBytes {
			continue
		}
		for _, comparison := range suite.Comparisons {
			if comparison.Upper95 >= 1 {
				t.Fatalf("expected ratio confidence bound below one: %+v", comparison)
			}
		}
	}
}

func syntheticAnalysisInput() (Corpus, []Runtime, VerificationMatrix, map[string]map[string][]Measurement) {
	corpus := Corpus{
		Run: RunConfig{Count: 6, MinimumSamples: 6},
		Coverage: Coverage{RequiredDimensions: map[Mode][]string{
			ModeDecoded: {"small"},
			ModeBytes:   {"small"},
		}},
		Cases: []Case{{ID: "case", Dimensions: []string{"small"}, Modes: []Mode{ModeDecoded, ModeBytes}}},
	}
	runtimes := []Runtime{{ID: WorkspaceImplementation}, {ID: BluesImplementation}, {ID: GnataImplementation}}
	matrix := VerificationMatrix{}
	for _, runtime := range runtimes {
		for _, mode := range corpus.Cases[0].Modes {
			matrix.Records = append(matrix.Records, VerificationRecord{
				Implementation: runtime.ID, CaseID: "case", Mode: mode, Status: StatusEligible,
			})
		}
	}
	matrix.buildIndex()
	raw := make(map[string]map[string][]Measurement)
	for _, runtime := range runtimes {
		ns := 100.0
		if runtime.ID == WorkspaceImplementation {
			ns = 50
		}
		raw[runtime.ID] = make(map[string][]Measurement)
		for _, mode := range corpus.Cases[0].Modes {
			for range 6 {
				raw[runtime.ID][measurementKey(mode, "case")] = append(raw[runtime.ID][measurementKey(mode, "case")], Measurement{
					Mode: mode, CaseID: "case", NsPerOp: ns, BytesPerOp: 10, AllocsPerOp: 1,
				})
			}
		}
	}
	return corpus, runtimes, matrix, raw
}

func flattenMeasurements(indexed map[string]map[string][]Measurement) map[string][]Measurement {
	result := make(map[string][]Measurement)
	for implementation, byCase := range indexed {
		for _, measurements := range byCase {
			result[implementation] = append(result[implementation], measurements...)
		}
	}
	return result
}
