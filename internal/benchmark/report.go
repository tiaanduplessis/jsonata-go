package benchmark

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var benchmarkLine = regexp.MustCompile(`^BenchmarkMatrix/([^/]+)/(.+)-[0-9]+$`)

// Measurement is one go test benchmark estimate.
type Measurement struct {
	Mode        Mode
	CaseID      string
	NsPerOp     float64
	BytesPerOp  float64
	AllocsPerOp float64
}

// ParseBenchmarkOutput parses the standard output emitted by testing.B.
func ParseBenchmarkOutput(reader io.Reader) ([]Measurement, error) {
	scanner := bufio.NewScanner(reader)
	measurements := make([]Measurement, 0)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		match := benchmarkLine.FindStringSubmatch(fields[0])
		if match == nil {
			continue
		}
		measurement := Measurement{Mode: Mode(match[1]), CaseID: match[2]}
		for i := 2; i+1 < len(fields); i += 2 {
			value, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				return nil, fmt.Errorf("parse benchmark metric %q: %w", fields[i], err)
			}
			switch fields[i+1] {
			case "ns/op":
				measurement.NsPerOp = value
			case "B/op":
				measurement.BytesPerOp = value
			case "allocs/op":
				measurement.AllocsPerOp = value
			}
		}
		if measurement.NsPerOp <= 0 {
			return nil, fmt.Errorf("benchmark %s/%s has no positive ns/op metric", measurement.Mode, measurement.CaseID)
		}
		measurements = append(measurements, measurement)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read benchmark output: %w", err)
	}
	if len(measurements) == 0 {
		return nil, fmt.Errorf("benchmark output contains no BenchmarkMatrix measurements")
	}
	return measurements, nil
}

// ImplementationMetrics summarizes one implementation over an identical set
// of cases.
type ImplementationMetrics struct {
	Implementation       string  `json:"implementation"`
	GeometricMeanNS      float64 `json:"geometric_mean_ns"`
	ArithmeticMeanBytes  float64 `json:"arithmetic_mean_bytes_per_op"`
	ArithmeticMeanAllocs float64 `json:"arithmetic_mean_allocs_per_op"`
}

// StatisticalComparison reports the workspace/competitor latency ratio. Both
// confidence bounds below one are required for a supported faster claim.
type StatisticalComparison struct {
	Competitor          string  `json:"competitor"`
	Ratio               float64 `json:"workspace_ratio"`
	Lower95             float64 `json:"lower_95"`
	Upper95             float64 `json:"upper_95"`
	StatisticallyFaster bool    `json:"statistically_faster"`
}

// SuiteAnalysis describes one complete comparable operation suite.
type SuiteAnalysis struct {
	Mode              Mode                    `json:"mode"`
	ComparableCases   []string                `json:"comparable_cases"`
	MissingDimensions []string                `json:"missing_dimensions,omitempty"`
	Complete          bool                    `json:"complete"`
	Metrics           []ImplementationMetrics `json:"metrics"`
	Comparisons       []StatisticalComparison `json:"comparisons"`
}

// ParallelAnalysis reports aggregate throughput scaling for implementations
// that explicitly support concurrent evaluation.
type ParallelAnalysis struct {
	Implementation  string  `json:"implementation"`
	Cases           int     `json:"cases"`
	SerialGeoMean   float64 `json:"serial_geometric_mean_ns"`
	ParallelGeoMean float64 `json:"parallel_geometric_mean_ns"`
	ThroughputScale float64 `json:"throughput_scale"`
}

// ClaimGate is deliberately scoped to decoded and raw-input evaluation on the
// complete comparable matrix.
type ClaimGate struct {
	Met     bool     `json:"met"`
	Scope   string   `json:"scope"`
	Reasons []string `json:"reasons,omitempty"`
}

// Analysis is the generated machine-readable benchmark conclusion.
type Analysis struct {
	Reference       Reference                 `json:"reference"`
	Implementations []ImplementationReference `json:"implementations"`
	Machine         MachineMetadata           `json:"machine"`
	Run             RunConfig                 `json:"run"`
	Unsupported     []VerificationRecord      `json:"unsupported"`
	Suites          []SuiteAnalysis           `json:"suites"`
	Parallel        []ParallelAnalysis        `json:"parallel"`
	Claim           ClaimGate                 `json:"claim"`
}

// AnalyzeMeasurements validates raw completeness and computes the published
// geometric means and confidence intervals.
func AnalyzeMeasurements(corpus Corpus, runtimes []Runtime, matrix VerificationMatrix, raw map[string][]Measurement) (Analysis, error) {
	byImplementation := make(map[string]map[string][]Measurement, len(runtimes))
	for _, runtime := range runtimes {
		measurements, ok := raw[runtime.ID]
		if !ok {
			return Analysis{}, fmt.Errorf("missing raw benchmark output for %s", runtime.ID)
		}
		indexed := make(map[string][]Measurement)
		for _, measurement := range measurements {
			if !matrix.Eligible(runtime.ID, measurement.CaseID, measurement.Mode) {
				return Analysis{}, fmt.Errorf("raw output timed ineligible cell %s/%s/%s", runtime.ID, measurement.Mode, measurement.CaseID)
			}
			key := measurementKey(measurement.Mode, measurement.CaseID)
			indexed[key] = append(indexed[key], measurement)
		}
		byImplementation[runtime.ID] = indexed
	}
	if err := validateRawCompleteness(corpus, runtimes, matrix, byImplementation); err != nil {
		return Analysis{}, err
	}
	analysis := Analysis{
		Reference:       corpus.Reference,
		Implementations: corpus.Implementations,
		Machine:         CurrentMachineMetadata(),
		Run:             corpus.Run,
		Unsupported:     matrix.Unsupported(),
		Claim: ClaimGate{
			Scope: "lowest statistically supported geometric-mean latency on complete decoded and raw-input comparable suites",
		},
	}
	for _, mode := range []Mode{ModeCompile, ModeDecoded, ModeBytes} {
		analysis.Suites = append(analysis.Suites, analyzeSuite(corpus, runtimes, matrix, byImplementation, mode))
	}
	analysis.Parallel = analyzeParallel(corpus, runtimes, matrix, byImplementation)
	analysis.Claim = evaluateClaim(analysis.Suites)
	return analysis, nil
}

func validateRawCompleteness(corpus Corpus, runtimes []Runtime, matrix VerificationMatrix, raw map[string]map[string][]Measurement) error {
	for _, runtime := range runtimes {
		for _, sample := range corpus.Cases {
			for _, mode := range sample.Modes {
				if !matrix.Eligible(runtime.ID, sample.ID, mode) {
					continue
				}
				measurements := raw[runtime.ID][measurementKey(mode, sample.ID)]
				if len(measurements) != corpus.Run.Count {
					return fmt.Errorf("%s/%s/%s has %d measurements, requires exactly %d", runtime.ID, mode, sample.ID, len(measurements), corpus.Run.Count)
				}
				if len(measurements) < corpus.Run.MinimumSamples {
					return fmt.Errorf("%s/%s/%s has %d measurements, statistical minimum is %d", runtime.ID, mode, sample.ID, len(measurements), corpus.Run.MinimumSamples)
				}
			}
		}
	}
	return nil
}

func measurementKey(mode Mode, caseID string) string { return string(mode) + "\x00" + caseID }

func analyzeSuite(corpus Corpus, runtimes []Runtime, matrix VerificationMatrix, raw map[string]map[string][]Measurement, mode Mode) SuiteAnalysis {
	cases := matrix.ComparableCases(corpus, runtimes, mode)
	suite := SuiteAnalysis{Mode: mode, MissingDimensions: MissingDimensions(corpus, cases, mode)}
	if mode == ModeCompile {
		suite.MissingDimensions = nil
	}
	for _, sample := range cases {
		suite.ComparableCases = append(suite.ComparableCases, sample.ID)
	}
	suite.Complete = len(cases) > 0 && len(suite.MissingDimensions) == 0
	for _, runtime := range runtimes {
		suite.Metrics = append(suite.Metrics, summarizeImplementation(runtime.ID, mode, cases, raw[runtime.ID]))
	}
	workspace := raw[WorkspaceImplementation]
	for _, runtime := range runtimes {
		if runtime.ID == WorkspaceImplementation {
			continue
		}
		ratio, lower, upper := confidenceRatio(mode, cases, workspace, raw[runtime.ID])
		suite.Comparisons = append(suite.Comparisons, StatisticalComparison{
			Competitor: runtime.ID, Ratio: ratio, Lower95: lower, Upper95: upper, StatisticallyFaster: upper < 1,
		})
	}
	return suite
}

func summarizeImplementation(implementation string, mode Mode, cases []Case, raw map[string][]Measurement) ImplementationMetrics {
	logs := make([]float64, 0)
	bytesTotal := 0.0
	allocsTotal := 0.0
	count := 0
	for _, sample := range cases {
		for _, measurement := range raw[measurementKey(mode, sample.ID)] {
			logs = append(logs, math.Log(measurement.NsPerOp))
			bytesTotal += measurement.BytesPerOp
			allocsTotal += measurement.AllocsPerOp
			count++
		}
	}
	metrics := ImplementationMetrics{Implementation: implementation}
	if len(logs) != 0 {
		metrics.GeometricMeanNS = math.Exp(mean(logs))
	}
	if count != 0 {
		metrics.ArithmeticMeanBytes = bytesTotal / float64(count)
		metrics.ArithmeticMeanAllocs = allocsTotal / float64(count)
	}
	return metrics
}

func confidenceRatio(mode Mode, cases []Case, workspace, competitor map[string][]Measurement) (float64, float64, float64) {
	workspaceMean, workspaceVariance := aggregateLogEstimate(mode, cases, workspace)
	competitorMean, competitorVariance := aggregateLogEstimate(mode, cases, competitor)
	difference := workspaceMean - competitorMean
	standardError := math.Sqrt(workspaceVariance + competitorVariance)
	const z95 = 1.959963984540054
	return math.Exp(difference), math.Exp(difference - z95*standardError), math.Exp(difference + z95*standardError)
}

func aggregateLogEstimate(mode Mode, cases []Case, raw map[string][]Measurement) (float64, float64) {
	caseMeans := make([]float64, 0, len(cases))
	variance := 0.0
	for _, sample := range cases {
		measurements := raw[measurementKey(mode, sample.ID)]
		logs := make([]float64, len(measurements))
		for i, measurement := range measurements {
			logs[i] = math.Log(measurement.NsPerOp)
		}
		caseMeans = append(caseMeans, mean(logs))
		variance += sampleVariance(logs) / float64(len(logs))
	}
	if len(caseMeans) == 0 {
		return 0, math.Inf(1)
	}
	divisor := float64(len(caseMeans) * len(caseMeans))
	return mean(caseMeans), variance / divisor
}

func analyzeParallel(corpus Corpus, runtimes []Runtime, matrix VerificationMatrix, raw map[string]map[string][]Measurement) []ParallelAnalysis {
	result := make([]ParallelAnalysis, 0, len(runtimes))
	for _, runtime := range runtimes {
		cases := make([]Case, 0)
		for _, sample := range corpus.Cases {
			if matrix.Eligible(runtime.ID, sample.ID, ModeDecoded) && matrix.Eligible(runtime.ID, sample.ID, ModeParallel) {
				cases = append(cases, sample)
			}
		}
		if len(cases) == 0 {
			continue
		}
		serial := summarizeImplementation(runtime.ID, ModeDecoded, cases, raw[runtime.ID]).GeometricMeanNS
		parallel := summarizeImplementation(runtime.ID, ModeParallel, cases, raw[runtime.ID]).GeometricMeanNS
		result = append(result, ParallelAnalysis{
			Implementation: runtime.ID, Cases: len(cases), SerialGeoMean: serial, ParallelGeoMean: parallel, ThroughputScale: serial / parallel,
		})
	}
	return result
}

func evaluateClaim(suites []SuiteAnalysis) ClaimGate {
	claim := ClaimGate{Scope: "lowest statistically supported geometric-mean latency on complete decoded and raw-input comparable suites"}
	for _, wanted := range []Mode{ModeDecoded, ModeBytes} {
		var suite *SuiteAnalysis
		for i := range suites {
			if suites[i].Mode == wanted {
				suite = &suites[i]
				break
			}
		}
		if suite == nil || !suite.Complete {
			claim.Reasons = append(claim.Reasons, fmt.Sprintf("%s comparable suite is incomplete", wanted))
			continue
		}
		for _, comparison := range suite.Comparisons {
			if !comparison.StatisticallyFaster {
				claim.Reasons = append(claim.Reasons, fmt.Sprintf("%s is not statistically faster than %s for %s (upper 95%% ratio %.3f)", WorkspaceImplementation, comparison.Competitor, wanted, comparison.Upper95))
			}
		}
	}
	claim.Met = len(claim.Reasons) == 0
	return claim
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func sampleVariance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	average := mean(values)
	total := 0.0
	for _, value := range values {
		difference := value - average
		total += difference * difference
	}
	return total / float64(len(values)-1)
}

// SortedModes returns the modes present in a measurement set. It is useful in
// report renderers and tests.
func SortedModes(measurements []Measurement) []Mode {
	seen := make(map[Mode]bool)
	for _, measurement := range measurements {
		seen[measurement.Mode] = true
	}
	modes := make([]Mode, 0, len(seen))
	for mode := range seen {
		modes = append(modes, mode)
	}
	sort.Slice(modes, func(i, j int) bool { return modes[i] < modes[j] })
	return modes
}
