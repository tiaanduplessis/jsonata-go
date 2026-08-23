package benchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

// ArtifactIdentity binds evidence metadata to the exact file that was
// produced by a benchmark command.
type ArtifactIdentity struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// RawFileIdentity binds one implementation to its exact benchmark output.
type RawFileIdentity struct {
	Implementation string `json:"implementation"`
	ArtifactIdentity
}

// RunManifest records all inputs and outputs needed to authenticate one raw
// benchmark run before its measurements can be reported.
type RunManifest struct {
	RecordedAt      string                    `json:"recorded_at"`
	Reference       Reference                 `json:"reference"`
	Implementations []ImplementationReference `json:"implementations"`
	Machine         MachineMetadata           `json:"machine"`
	Repository      RepositoryMetadata        `json:"repository"`
	Run             RunConfig                 `json:"run"`
	Command         []string                  `json:"command"`
	WarmupCommand   []string                  `json:"warmup_command"`
	RoundOrder      [][]string                `json:"round_order"`
	RawFiles        []RawFileIdentity         `json:"raw_files"`
}

// ProfileManifest records the provenance and exact artifacts of one profile
// collection run.
type ProfileManifest struct {
	RecordedAt string             `json:"recorded_at"`
	Reference  Reference          `json:"reference"`
	Machine    MachineMetadata    `json:"machine"`
	Repository RepositoryMetadata `json:"repository"`
	Commands   []ProfileCommand   `json:"commands"`
	Files      []ArtifactIdentity `json:"files"`
}

// ProfileCommand records one exact profiling command and its output file.
type ProfileCommand struct {
	Arguments   []string `json:"arguments"`
	Environment []string `json:"environment,omitempty"`
	Stdout      string   `json:"stdout,omitempty"`
}

var profileArtifactNames = []string{
	"cpu.pprof", "heap.pprof", "mutex.pprof", "block.pprof",
	"cpu.txt", "allocations.txt", "mutex.txt", "block.txt",
}

// ExpectedProfileCommands returns the exact commands used by the profile
// collector. The temporary test binary is removed after collection.
func ExpectedProfileCommands(outputDir string) []ProfileCommand {
	binaryPath := filepath.Join(outputDir, ".benchmark-profile.test")
	commands := []ProfileCommand{{
		Arguments: []string{
			"go", "test", "./internal/benchmark", "-run", "^$",
			"-bench", "^BenchmarkMatrix/(decoded|bytes)/(medium-transform|large-filter-reduce)$",
			"-benchmem", "-benchtime=3s", "-count=1", "-o", binaryPath,
			"-cpuprofile", filepath.Join(outputDir, "cpu.pprof"),
			"-memprofile", filepath.Join(outputDir, "heap.pprof"),
			"-mutexprofile", filepath.Join(outputDir, "mutex.pprof"),
			"-blockprofile", filepath.Join(outputDir, "block.pprof"),
		},
		Environment: []string{"BENCH_IMPLEMENTATION=jsonata-go"},
	}}
	for _, profile := range []struct {
		name string
		args []string
	}{
		{name: "cpu.txt", args: []string{"-top", "-nodecount=30", filepath.Join(outputDir, "cpu.pprof")}},
		{name: "allocations.txt", args: []string{"-top", "-alloc_space", "-nodecount=30", filepath.Join(outputDir, "heap.pprof")}},
		{name: "mutex.txt", args: []string{"-top", "-nodecount=30", filepath.Join(outputDir, "mutex.pprof")}},
		{name: "block.txt", args: []string{"-top", "-nodecount=30", filepath.Join(outputDir, "block.pprof")}},
	} {
		arguments := append([]string{"go", "tool", "pprof"}, profile.args...)
		commands = append(commands, ProfileCommand{Arguments: arguments, Stdout: filepath.Join(outputDir, profile.name)})
	}
	return commands
}

// IdentifyProfileFiles records every required profile artifact.
func IdentifyProfileFiles(outputDir string) ([]ArtifactIdentity, error) {
	files := make([]ArtifactIdentity, 0, len(profileArtifactNames))
	for _, name := range profileArtifactNames {
		identity, err := IdentifyArtifact(filepath.Join(outputDir, name), name)
		if err != nil {
			return nil, err
		}
		files = append(files, identity)
	}
	return files, nil
}

// ValidateProfileManifest rejects stale, relabeled, or modified profile
// evidence.
func ValidateProfileManifest(manifest ProfileManifest, corpus Corpus, outputDir string, currentMachine MachineMetadata, currentRepository RepositoryMetadata) error {
	if _, err := time.Parse(time.RFC3339, manifest.RecordedAt); err != nil {
		return fmt.Errorf("profile evidence has an invalid timestamp: %w", err)
	}
	if !reflect.DeepEqual(manifest.Reference, corpus.Reference) {
		return fmt.Errorf("profile evidence reference differs from the frozen corpus")
	}
	if !reflect.DeepEqual(manifest.Machine, currentMachine) {
		return fmt.Errorf("profile evidence machine metadata differs from the current machine")
	}
	if manifest.Repository.Revision == "" || manifest.Repository.Revision == "unknown" {
		return fmt.Errorf("profile evidence repository revision is missing")
	}
	if manifest.Repository.Dirty {
		return fmt.Errorf("profile evidence was collected from a dirty source checkout")
	}
	if manifest.Repository != currentRepository {
		return fmt.Errorf("profile evidence repository revision or dirty status differs from the current checkout")
	}
	if !reflect.DeepEqual(manifest.Commands, ExpectedProfileCommands(outputDir)) {
		return fmt.Errorf("profile evidence commands differ from the required profile commands")
	}
	wantFiles, err := IdentifyProfileFiles(outputDir)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(manifest.Files, wantFiles) {
		return fmt.Errorf("profile evidence file identities differ from the profile manifest")
	}
	return nil
}

// BenchmarkCommand is the exact command used for each timed implementation.
func BenchmarkCommand(benchtime string) []string {
	return []string{
		"go", "test", "./internal/benchmark", "-run", "^$", "-bench", "^BenchmarkMatrix$",
		"-benchmem", "-benchtime", benchtime, "-count", "1", "-timeout", "20m",
	}
}

// ExpectedRoundOrder returns the required deterministic rotation.
func ExpectedRoundOrder(implementations []ImplementationReference, count int) [][]string {
	result := make([][]string, 0, count)
	for round := 0; round < count; round++ {
		order := make([]string, 0, len(implementations))
		for offset := range len(implementations) {
			order = append(order, implementations[(round+offset)%len(implementations)].ID)
		}
		result = append(result, order)
	}
	return result
}

// IdentifyArtifact computes the stable identity of one evidence file.
func IdentifyArtifact(path, name string) (ArtifactIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ArtifactIdentity{}, fmt.Errorf("read evidence artifact %s: %w", name, err)
	}
	digest := sha256.Sum256(data)
	return ArtifactIdentity{File: name, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(data))}, nil
}

// IdentifyRawFiles records the exact expected raw file for every pinned
// implementation.
func IdentifyRawFiles(rawDir string, implementations []ImplementationReference) ([]RawFileIdentity, error) {
	result := make([]RawFileIdentity, 0, len(implementations))
	for _, implementation := range implementations {
		name := implementation.ID + ".txt"
		identity, err := IdentifyArtifact(filepath.Join(rawDir, name), name)
		if err != nil {
			return nil, err
		}
		result = append(result, RawFileIdentity{Implementation: implementation.ID, ArtifactIdentity: identity})
	}
	return result, nil
}

// ValidateRunManifest rejects stale or relabeled raw benchmark evidence.
func ValidateRunManifest(manifest RunManifest, corpus Corpus, rawDir string, currentMachine MachineMetadata, currentRepository RepositoryMetadata) error {
	if _, err := time.Parse(time.RFC3339, manifest.RecordedAt); err != nil {
		return fmt.Errorf("raw evidence has an invalid timestamp: %w", err)
	}
	if !reflect.DeepEqual(manifest.Reference, corpus.Reference) {
		return fmt.Errorf("raw evidence reference differs from the frozen corpus")
	}
	if !reflect.DeepEqual(manifest.Implementations, corpus.Implementations) {
		return fmt.Errorf("raw evidence implementation pins differ from the frozen corpus")
	}
	if manifest.Run != corpus.Run {
		return fmt.Errorf("raw run configuration differs from the frozen corpus")
	}
	if !reflect.DeepEqual(manifest.Command, BenchmarkCommand(corpus.Run.Benchtime)) {
		return fmt.Errorf("raw evidence command differs from the required benchmark command")
	}
	if !reflect.DeepEqual(manifest.WarmupCommand, BenchmarkCommand("100ms")) {
		return fmt.Errorf("raw evidence warm-up command differs from the required benchmark command")
	}
	if !reflect.DeepEqual(manifest.RoundOrder, ExpectedRoundOrder(corpus.Implementations, corpus.Run.Count)) {
		return fmt.Errorf("raw evidence round order is incomplete or not the required rotation")
	}
	if !reflect.DeepEqual(manifest.Machine, currentMachine) {
		return fmt.Errorf("raw evidence machine metadata differs from the current machine")
	}
	if manifest.Repository.Revision == "" || manifest.Repository.Revision == "unknown" {
		return fmt.Errorf("raw evidence repository revision is missing")
	}
	if manifest.Repository.Dirty {
		return fmt.Errorf("raw evidence was collected from a dirty source checkout")
	}
	if manifest.Repository != currentRepository {
		return fmt.Errorf("raw evidence repository revision or dirty status differs from the current checkout")
	}
	wantFiles, err := IdentifyRawFiles(rawDir, corpus.Implementations)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(manifest.RawFiles, wantFiles) {
		return fmt.Errorf("raw evidence file identities differ from the run manifest")
	}
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return fmt.Errorf("read raw evidence directory: %w", err)
	}
	var textFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".txt" {
			textFiles = append(textFiles, entry.Name())
		}
	}
	sort.Strings(textFiles)
	wantNames := make([]string, 0, len(corpus.Implementations))
	for _, implementation := range corpus.Implementations {
		wantNames = append(wantNames, implementation.ID+".txt")
	}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(textFiles, wantNames) {
		return fmt.Errorf("raw evidence directory contains an unexpected benchmark output set")
	}
	return nil
}
