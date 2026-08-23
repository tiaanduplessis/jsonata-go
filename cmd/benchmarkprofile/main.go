// Command benchmarkprofile captures reproducible runtime profiles and their
// complete source and host metadata.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

func main() {
	corpusPath := flag.String("corpus", "testdata/benchmark/corpus.json", "frozen benchmark corpus path")
	matrixPath := flag.String("matrix", "testdata/benchmark/matrix.json", "benchmark source matrix path")
	generatorPath := flag.String("generator", "testdata/benchmark/generate-oracle.mjs", "oracle generator path")
	packageLockPath := flag.String("package-lock", "testdata/benchmark/package-lock.json", "oracle dependency lock path")
	referenceRoot := flag.String("reference-root", "testdata/reference/jsonata-js-v2.2.2", "vendored jsonata-js reference root")
	outputDir := flag.String("output", "reports/benchmark/profiles", "profile output directory")
	check := flag.Bool("check", false, "validate existing profile metadata and artifacts without collecting profiles")
	flag.Parse()

	if err := benchmark.ValidateOracleArtifacts(*matrixPath, *corpusPath, *generatorPath, *packageLockPath, *referenceRoot); err != nil {
		fail(err)
	}
	corpus, err := benchmark.LoadCorpus(*corpusPath)
	if err != nil {
		fail(err)
	}
	machine := benchmark.CurrentMachineMetadata()
	repository := benchmark.CurrentRepositoryMetadataExcluding("reports/benchmark", *outputDir)
	if *check {
		manifest, err := loadManifest(filepath.Join(*outputDir, "metadata.json"))
		if err != nil {
			fail(err)
		}
		if err := benchmark.ValidateProfileManifest(manifest, corpus, *outputDir, machine, repository); err != nil {
			fail(err)
		}
		_, _ = fmt.Fprintln(os.Stdout, "profile evidence: current")
		return
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fail(err)
	}
	binaryPath := filepath.Join(*outputDir, ".benchmark-profile.test")
	defer os.Remove(binaryPath)
	commands := benchmark.ExpectedProfileCommands(*outputDir)
	if err := run(commands[0].Arguments, commands[0].Environment, ""); err != nil {
		fail(err)
	}
	for _, command := range commands[1:] {
		if err := run(command.Arguments, command.Environment, command.Stdout); err != nil {
			fail(err)
		}
	}
	files, err := benchmark.IdentifyProfileFiles(*outputDir)
	if err != nil {
		fail(err)
	}
	manifest := benchmark.ProfileManifest{
		RecordedAt: time.Now().UTC().Format(time.RFC3339), Reference: corpus.Reference,
		Machine: machine, Repository: repository, Commands: commands, Files: files,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(*outputDir, "metadata.json"), data, 0o644); err != nil {
		fail(err)
	}
}

func loadManifest(path string) (benchmark.ProfileManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return benchmark.ProfileManifest{}, fmt.Errorf("read profile metadata: %w", err)
	}
	var manifest benchmark.ProfileManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return benchmark.ProfileManifest{}, fmt.Errorf("decode profile metadata: %w", err)
	}
	return manifest, nil
}

func run(arguments, environment []string, outputPath string) error {
	command := exec.Command(arguments[0], arguments[1:]...)
	command.Env = append(os.Environ(), environment...)
	if outputPath == "" {
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("run %s: %w\n%s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("run %s: %w\n%s", strings.Join(arguments, " "), err, strings.TrimSpace(stderr.String()))
	}
	if err := os.WriteFile(outputPath, output, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	return nil
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
