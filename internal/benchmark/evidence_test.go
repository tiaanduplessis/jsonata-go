package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRunManifestAuthenticatesCompleteEvidence(t *testing.T) {
	corpus := manifestTestCorpus()
	rawDir := t.TempDir()
	for _, implementation := range corpus.Implementations {
		if err := os.WriteFile(filepath.Join(rawDir, implementation.ID+".txt"), []byte(implementation.ID), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rawFiles, err := IdentifyRawFiles(rawDir, corpus.Implementations)
	if err != nil {
		t.Fatal(err)
	}
	machine := MachineMetadata{GoVersion: "go-test", GOOS: "test", GOARCH: "test", Compiler: "gc", CPUModel: "test", CPUCount: 1, GOMAXPROCS: 1, PowerMode: "test"}
	repository := RepositoryMetadata{Revision: strings.Repeat("a", 40)}
	manifest := RunManifest{
		RecordedAt: time.Now().UTC().Format(time.RFC3339), Reference: corpus.Reference,
		Implementations: corpus.Implementations, Machine: machine, Repository: repository,
		Run: corpus.Run, Command: BenchmarkCommand(corpus.Run.Benchtime),
		WarmupCommand: BenchmarkCommand("100ms"),
		RoundOrder:    ExpectedRoundOrder(corpus.Implementations, corpus.Run.Count), RawFiles: rawFiles,
	}
	if err := ValidateRunManifest(manifest, corpus, rawDir, machine, repository); err != nil {
		t.Fatal(err)
	}
	dirty := manifest
	dirty.Repository.Dirty = true
	if err := ValidateRunManifest(dirty, corpus, rawDir, machine, dirty.Repository); err == nil || !strings.Contains(err.Error(), "dirty source checkout") {
		t.Fatalf("expected dirty evidence rejection, got %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*RunManifest)
	}{
		{name: "reference", mutate: func(value *RunManifest) { value.Reference.Commit = "deadbeef" }},
		{name: "implementations", mutate: func(value *RunManifest) { value.Implementations = value.Implementations[:1] }},
		{name: "run", mutate: func(value *RunManifest) { value.Run.Count++ }},
		{name: "command", mutate: func(value *RunManifest) { value.Command = []string{"not", "the", "command"} }},
		{name: "warm-up command", mutate: func(value *RunManifest) { value.WarmupCommand = nil }},
		{name: "round order", mutate: func(value *RunManifest) {
			value.RoundOrder[0][0], value.RoundOrder[0][1] = value.RoundOrder[0][1], value.RoundOrder[0][0]
		}},
		{name: "machine", mutate: func(value *RunManifest) { value.Machine.CPUCount++ }},
		{name: "repository", mutate: func(value *RunManifest) { value.Repository.Dirty = true }},
		{name: "raw identity", mutate: func(value *RunManifest) { value.RawFiles[0].SHA256 = strings.Repeat("0", 64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneManifest(manifest)
			test.mutate(&candidate)
			if err := ValidateRunManifest(candidate, corpus, rawDir, machine, repository); err == nil {
				t.Fatal("expected provenance mismatch")
			}
		})
	}
	if err := os.WriteFile(filepath.Join(rawDir, "extra.txt"), []byte("unexpected"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRunManifest(manifest, corpus, rawDir, machine, repository); err == nil || !strings.Contains(err.Error(), "unexpected benchmark output set") {
		t.Fatalf("expected unexpected raw file rejection, got %v", err)
	}
}

func TestValidateRunManifestRejectsModifiedRawFile(t *testing.T) {
	corpus := manifestTestCorpus()
	rawDir := t.TempDir()
	for _, implementation := range corpus.Implementations {
		if err := os.WriteFile(filepath.Join(rawDir, implementation.ID+".txt"), []byte(implementation.ID), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rawFiles, err := IdentifyRawFiles(rawDir, corpus.Implementations)
	if err != nil {
		t.Fatal(err)
	}
	machine := MachineMetadata{GoVersion: "go-test"}
	repository := RepositoryMetadata{Revision: "revision"}
	manifest := RunManifest{
		RecordedAt: time.Now().UTC().Format(time.RFC3339), Reference: corpus.Reference,
		Implementations: corpus.Implementations, Machine: machine, Repository: repository, Run: corpus.Run,
		Command: BenchmarkCommand(corpus.Run.Benchtime), WarmupCommand: BenchmarkCommand("100ms"),
		RoundOrder: ExpectedRoundOrder(corpus.Implementations, corpus.Run.Count), RawFiles: rawFiles,
	}
	if err := os.WriteFile(filepath.Join(rawDir, corpus.Implementations[0].ID+".txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRunManifest(manifest, corpus, rawDir, machine, repository); err == nil || !strings.Contains(err.Error(), "file identities") {
		t.Fatalf("expected modified raw file rejection, got %v", err)
	}
}

func TestValidateProfileManifestAuthenticatesCommandsAndArtifacts(t *testing.T) {
	corpus := manifestTestCorpus()
	outputDir := t.TempDir()
	for _, name := range profileArtifactNames {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := IdentifyProfileFiles(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	machine := MachineMetadata{GoVersion: "go-test"}
	repository := RepositoryMetadata{Revision: "revision"}
	manifest := ProfileManifest{
		RecordedAt: time.Now().UTC().Format(time.RFC3339), Reference: corpus.Reference,
		Machine: machine, Repository: repository, Commands: ExpectedProfileCommands(outputDir), Files: files,
	}
	if err := ValidateProfileManifest(manifest, corpus, outputDir, machine, repository); err != nil {
		t.Fatal(err)
	}
	dirty := manifest
	dirty.Repository.Dirty = true
	if err := ValidateProfileManifest(dirty, corpus, outputDir, machine, dirty.Repository); err == nil || !strings.Contains(err.Error(), "dirty source checkout") {
		t.Fatalf("expected dirty profile rejection, got %v", err)
	}
	manifest.Commands[0].Arguments[0] = "other"
	if err := ValidateProfileManifest(manifest, corpus, outputDir, machine, repository); err == nil || !strings.Contains(err.Error(), "commands") {
		t.Fatalf("expected profile command rejection, got %v", err)
	}
}

func manifestTestCorpus() Corpus {
	implementations := []ImplementationReference{
		{ID: WorkspaceImplementation}, {ID: BluesImplementation}, {ID: GnataImplementation},
	}
	return Corpus{
		Reference:       Reference{Implementation: "jsonata-js", Version: "2.2.2", Commit: ReferenceCommit},
		Implementations: implementations,
		Run:             RunConfig{Count: 3, Benchtime: "200ms"},
	}
}

func cloneManifest(value RunManifest) RunManifest {
	clone := value
	clone.Implementations = append([]ImplementationReference(nil), value.Implementations...)
	clone.Command = append([]string(nil), value.Command...)
	clone.RawFiles = append([]RawFileIdentity(nil), value.RawFiles...)
	clone.RoundOrder = make([][]string, len(value.RoundOrder))
	for index := range value.RoundOrder {
		clone.RoundOrder[index] = append([]string(nil), value.RoundOrder[index]...)
	}
	return clone
}
