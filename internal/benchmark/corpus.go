// Package benchmark contains the correctness-gated benchmark harness.
package benchmark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ReferenceCommit is the immutable jsonata-js v2.2.2 oracle revision used to
// generate the checked-in corpus.
const ReferenceCommit = "6c7e95fdbf4405a1e741852a7cd8cd985b4305bb"

const corpusSchemaVersion = 3

// Mode identifies an operation in the benchmark matrix.
type Mode string

const (
	ModeCompile  Mode = "compile"
	ModeDecoded  Mode = "decoded"
	ModeBytes    Mode = "bytes"
	ModeParallel Mode = "parallel"
)

// ImplementationReference pins one implementation in the comparison.
type ImplementationReference struct {
	ID      string `json:"id"`
	Module  string `json:"module"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
}

// OracleError is the stable, machine-readable subset of a JSONata error used
// for differential correctness checks. Empty fields are not compared.
type OracleError struct {
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Token    string `json:"token,omitempty"`
	Value    string `json:"value,omitempty"`
	Position int    `json:"position,omitempty"`
}

// Oracle is one reference result. Exactly one of Value and Error is present.
type Oracle struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error *OracleError    `json:"error,omitempty"`
}

// Case is a frozen representative workload. Input is JSON so decoded and raw
// input measurements use exactly the same document.
type Case struct {
	ID             string      `json:"id"`
	Category       string      `json:"category"`
	SizeTier       string      `json:"size_tier"`
	Dimensions     []string    `json:"dimensions"`
	Expression     string      `json:"expression"`
	Input          string      `json:"input"`
	Modes          []Mode      `json:"modes"`
	CustomFunction string      `json:"custom_function,omitempty"`
	Source         *CaseSource `json:"source,omitempty"`
	Oracle         Oracle      `json:"oracle"`
}

// CaseSource identifies an input copied from the pinned upstream suite.
type CaseSource struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
	SHA256 string `json:"sha256"`
}

// Coverage defines the dimensions required before a scoped performance claim
// can be considered.
type Coverage struct {
	RequiredDimensions map[Mode][]string `json:"required_dimensions"`
}

// RunConfig makes local collection and the statistical claim gate
// reproducible.
type RunConfig struct {
	Count          int     `json:"count"`
	Benchtime      string  `json:"benchtime"`
	Warmup         int     `json:"warmup"`
	Confidence     float64 `json:"confidence"`
	MinimumSamples int     `json:"minimum_samples"`
}

// Corpus is the versioned benchmark input and oracle manifest.
type Corpus struct {
	SchemaVersion   int                       `json:"schema_version"`
	Reference       Reference                 `json:"reference"`
	Implementations []ImplementationReference `json:"implementations"`
	Run             RunConfig                 `json:"run"`
	Coverage        Coverage                  `json:"coverage"`
	Cases           []Case                    `json:"cases"`
}

// Reference identifies the source and revision that produced the oracle.
type Reference struct {
	Implementation string         `json:"implementation"`
	Version        string         `json:"version"`
	Commit         string         `json:"commit"`
	Generator      string         `json:"generator"`
	Generation     GenerationPins `json:"generation"`
}

// GenerationPins bind the oracle to every input used by its generator.
type GenerationPins struct {
	MatrixSHA256      string `json:"matrix_sha256"`
	GeneratorSHA256   string `json:"generator_sha256"`
	PackageLockSHA256 string `json:"package_lock_sha256"`
}

// MachineMetadata records the host and toolchain details needed to interpret
// benchmark output.
type MachineMetadata struct {
	GoVersion  string `json:"go_version"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	Compiler   string `json:"compiler"`
	CPUModel   string `json:"cpu_model"`
	CPUCount   int    `json:"cpu_count"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	PowerMode  string `json:"power_mode"`
}

// RepositoryMetadata identifies the library revision used for a local run.
type RepositoryMetadata struct {
	Revision string `json:"revision"`
	Dirty    bool   `json:"dirty"`
}

// CurrentMachineMetadata returns the relevant metadata for this process.
func CurrentMachineMetadata() MachineMetadata {
	return MachineMetadata{
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Compiler:   runtime.Compiler,
		CPUModel:   cpuModel(),
		CPUCount:   runtime.NumCPU(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		PowerMode:  powerMode(),
	}
}

// CurrentRepositoryMetadata reports the current Git revision and whether
// tracked or untracked files differ from it.
func CurrentRepositoryMetadata() RepositoryMetadata {
	return CurrentRepositoryMetadataExcluding()
}

// CurrentRepositoryMetadataExcluding reports source state while ignoring
// generated evidence paths supplied by the caller. The revision is the latest
// commit that changed non-evidence source, so checking in generated reports
// does not immediately make their provenance stale.
func CurrentRepositoryMetadataExcluding(paths ...string) RepositoryMetadata {
	metadata := RepositoryMetadata{Revision: "unknown"}
	relativePaths := repositoryRelativePaths(paths)
	revisionArguments := []string{"log", "-1", "--format=%H", "--", "."}
	for _, path := range relativePaths {
		revisionArguments = append(revisionArguments, ":(exclude)"+filepath.ToSlash(path)+"/**")
	}
	if output, err := exec.Command("git", revisionArguments...).Output(); err == nil {
		metadata.Revision = strings.TrimSpace(string(output))
	}
	arguments := []string{"status", "--porcelain", "--untracked-files=normal", "--", "."}
	for _, path := range relativePaths {
		arguments = append(arguments, ":(exclude)"+filepath.ToSlash(path)+"/**")
	}
	if output, err := exec.Command("git", arguments...).Output(); err != nil || len(strings.TrimSpace(string(output))) != 0 {
		metadata.Dirty = true
	}
	return metadata
}

func repositoryRelativePaths(paths []string) []string {
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil
	}
	root := strings.TrimSpace(string(rootOutput))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		absolute := path
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(root, absolute)
		}
		relative, err := filepath.Rel(root, filepath.Clean(absolute))
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		result = append(result, relative)
	}
	return result
}

func cpuModel() string {
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if key, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(key) == "model name" {
				return strings.TrimSpace(value)
			}
		}
	}
	return "unknown"
}

func powerMode() string {
	if configured := strings.TrimSpace(os.Getenv("BENCH_POWER_MODE")); configured != "" {
		return configured
	}
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("pmset", "-g", "batt").Output(); err == nil {
			first, _, _ := strings.Cut(string(output), "\n")
			if strings.Contains(first, "AC Power") {
				return "AC power"
			}
			if strings.Contains(first, "Battery Power") {
				return "battery"
			}
		}
	}
	if data, err := os.ReadFile("/sys/class/power_supply/AC/online"); err == nil {
		if online, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if online == 1 {
				return "AC power"
			}
			return "battery"
		}
	}
	return "unknown"
}

// LoadCorpus loads and validates a checked-in corpus manifest.
func LoadCorpus(path string) (Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, fmt.Errorf("read benchmark corpus: %w", err)
	}
	var corpus Corpus
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return Corpus{}, fmt.Errorf("decode benchmark corpus: %w", err)
	}
	if err := ValidateCorpus(corpus); err != nil {
		return Corpus{}, err
	}
	return corpus, nil
}

// ValidateCorpus enforces the frozen corpus invariants before any evaluator is
// invoked.
func ValidateCorpus(corpus Corpus) error {
	if corpus.SchemaVersion != corpusSchemaVersion {
		return fmt.Errorf("unsupported benchmark corpus schema %d", corpus.SchemaVersion)
	}
	if corpus.Reference.Implementation != "jsonata-js" || corpus.Reference.Version != "2.2.2" || corpus.Reference.Commit != ReferenceCommit {
		return fmt.Errorf("benchmark corpus is not pinned to jsonata-js v2.2.2 commit %s", ReferenceCommit)
	}
	wantImplementations := map[string]ImplementationReference{
		"jsonata-go": {ID: "jsonata-go", Module: "github.com/tiaanduplessis/jsonata-go", Version: "workspace"},
		"blues":      {ID: "blues", Module: "github.com/blues/jsonata-go", Version: "v1.5.4", Commit: "e0d39c06990dd541e7d6dbac338853bef894b8f4"},
		"gnata":      {ID: "gnata", Module: "github.com/recolabs/gnata", Version: "v0.2.3", Commit: "8abdb304a3c096c88a43760fd1160ada1851b29d"},
	}
	if len(corpus.Implementations) != len(wantImplementations) {
		return fmt.Errorf("benchmark corpus must pin exactly %d implementations", len(wantImplementations))
	}
	for _, implementation := range corpus.Implementations {
		want, ok := wantImplementations[implementation.ID]
		if !ok || implementation != want {
			return fmt.Errorf("unexpected implementation pin %q at %q", implementation.ID, implementation.Version)
		}
		delete(wantImplementations, implementation.ID)
	}
	if len(wantImplementations) != 0 {
		return fmt.Errorf("benchmark corpus is missing implementation pins")
	}
	if corpus.Run.Count < 6 || corpus.Run.Warmup < 1 || corpus.Run.Benchtime == "" || corpus.Run.Confidence != 0.95 || corpus.Run.MinimumSamples < 6 {
		return fmt.Errorf("benchmark run configuration does not meet the repeated-measurement policy")
	}
	if corpus.Reference.Generator != "testdata/benchmark/generate-oracle.mjs" || !validSHA256(corpus.Reference.Generation.MatrixSHA256) || !validSHA256(corpus.Reference.Generation.GeneratorSHA256) || !validSHA256(corpus.Reference.Generation.PackageLockSHA256) {
		return fmt.Errorf("benchmark corpus has incomplete oracle generation pins")
	}
	for _, mode := range []Mode{ModeDecoded, ModeBytes, ModeParallel} {
		if len(corpus.Coverage.RequiredDimensions[mode]) == 0 {
			return fmt.Errorf("benchmark coverage has no required dimensions for %s", mode)
		}
	}
	if len(corpus.Cases) == 0 {
		return fmt.Errorf("benchmark corpus has no cases")
	}
	seen := make(map[string]struct{}, len(corpus.Cases))
	officialSources := make(map[string]struct{})
	for _, c := range corpus.Cases {
		if err := validateCase(c, seen); err != nil {
			return err
		}
		if c.Source != nil {
			officialSources[c.Source.Path] = struct{}{}
		}
	}
	if len(officialSources) < 2 {
		return fmt.Errorf("benchmark corpus must include at least two pinned official jsonata-js datasets")
	}
	return nil
}

func validateCase(c Case, seen map[string]struct{}) error {
	if c.ID == "" || c.Category == "" || c.SizeTier == "" || c.Expression == "" {
		return fmt.Errorf("benchmark case has an empty required field")
	}
	if _, ok := seen[c.ID]; ok {
		return fmt.Errorf("duplicate benchmark case %q", c.ID)
	}
	seen[c.ID] = struct{}{}
	if len(c.Dimensions) == 0 {
		return fmt.Errorf("case %q has no coverage dimensions", c.ID)
	}
	var input any
	if err := json.Unmarshal([]byte(c.Input), &input); err != nil {
		return fmt.Errorf("case %q has invalid JSON input: %w", c.ID, err)
	}
	inputSize := len(c.Input)
	switch c.SizeTier {
	case "small":
		if inputSize > 128 {
			return fmt.Errorf("case %q labels %d-byte input small; limit is 128", c.ID, inputSize)
		}
	case "medium":
		if inputSize < 129 || inputSize > 512 {
			return fmt.Errorf("case %q labels %d-byte input medium; range is 129-512", c.ID, inputSize)
		}
	case "large":
		if inputSize < 513 {
			return fmt.Errorf("case %q labels %d-byte input large; minimum is 513", c.ID, inputSize)
		}
	default:
		return fmt.Errorf("case %q has unknown size tier %q", c.ID, c.SizeTier)
	}
	if !HasDimension(c, c.SizeTier) {
		return fmt.Errorf("case %q does not include its %q size dimension", c.ID, c.SizeTier)
	}
	if len(c.Modes) == 0 {
		return fmt.Errorf("case %q has no benchmark modes", c.ID)
	}
	seenModes := make(map[Mode]struct{}, len(c.Modes))
	for _, mode := range c.Modes {
		if mode != ModeCompile && mode != ModeDecoded && mode != ModeBytes && mode != ModeParallel {
			return fmt.Errorf("case %q has unknown mode %q", c.ID, mode)
		}
		if _, ok := seenModes[mode]; ok {
			return fmt.Errorf("case %q repeats mode %q", c.ID, mode)
		}
		seenModes[mode] = struct{}{}
	}
	if _, ok := seenModes[ModeCompile]; !ok {
		return fmt.Errorf("case %q does not measure compilation", c.ID)
	}
	if c.CustomFunction != "" && c.CustomFunction != "double" {
		return fmt.Errorf("case %q names unsupported custom function %q", c.ID, c.CustomFunction)
	}
	if c.Source != nil {
		if c.Source.Kind != "jsonata-js-dataset" || c.Source.Commit != ReferenceCommit || !strings.HasPrefix(c.Source.Path, "test/test-suite/datasets/") || filepath.Ext(c.Source.Path) != ".json" || !validSHA256(c.Source.SHA256) {
			return fmt.Errorf("case %q has invalid official dataset provenance", c.ID)
		}
		if filepath.Clean(c.Source.Path) != c.Source.Path || strings.Contains(c.Source.Path, "..") {
			return fmt.Errorf("case %q has unsafe official dataset path", c.ID)
		}
	}
	if len(c.Oracle.Value) == 0 && c.Oracle.Error == nil {
		return fmt.Errorf("case %q has no oracle value or error", c.ID)
	}
	if len(c.Oracle.Value) > 0 && c.Oracle.Error != nil {
		return fmt.Errorf("case %q has both oracle value and error", c.ID)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// HasMode reports whether a case participates in an operation.
func HasMode(c Case, wanted Mode) bool {
	for _, mode := range c.Modes {
		if mode == wanted {
			return true
		}
	}
	return false
}

// HasDimension reports whether a case contributes to a coverage dimension.
func HasDimension(c Case, wanted string) bool {
	for _, dimension := range c.Dimensions {
		if dimension == wanted {
			return true
		}
	}
	return false
}
