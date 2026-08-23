// Command benchmarkrun collects repeated, correctness-gated benchmark output.
package main

import (
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
	outputDir := flag.String("output", "reports/benchmark/raw", "raw benchmark output directory")
	flag.Parse()

	if err := benchmark.ValidateOracleArtifacts(*matrixPath, *corpusPath, *generatorPath, *packageLockPath, *referenceRoot); err != nil {
		fail(err)
	}
	corpus, err := benchmark.LoadCorpus(*corpusPath)
	if err != nil {
		fail(err)
	}
	runtimes := benchmark.Implementations()
	matrix := benchmark.VerifyMatrix(corpus, runtimes)
	if err := matrix.ValidateWorkspace(corpus); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fail(err)
	}
	for _, runtime := range runtimes {
		path := filepath.Join(*outputDir, runtime.ID+".txt")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			fail(err)
		}
	}

	baseCommand := benchmark.BenchmarkCommand(corpus.Run.Benchtime)
	manifest := benchmark.RunManifest{
		RecordedAt: time.Now().UTC().Format(time.RFC3339), Reference: corpus.Reference,
		Implementations: corpus.Implementations, Machine: benchmark.CurrentMachineMetadata(),
		Repository: benchmark.CurrentRepositoryMetadataExcluding("reports/benchmark", *outputDir), Run: corpus.Run,
		Command: baseCommand, WarmupCommand: benchmark.BenchmarkCommand("100ms"),
		RoundOrder: benchmark.ExpectedRoundOrder(corpus.Implementations, corpus.Run.Count),
	}
	for warmup := 0; warmup < corpus.Run.Warmup; warmup++ {
		for _, runtime := range runtimes {
			if _, err := runBenchmark(runtime.ID, benchmark.BenchmarkCommand("100ms")); err != nil {
				fail(fmt.Errorf("warm up %s: %w", runtime.ID, err))
			}
		}
	}
	for round := 0; round < corpus.Run.Count; round++ {
		order := manifest.RoundOrder[round]
		for _, implementation := range order {
			output, err := runBenchmark(implementation, baseCommand)
			if err != nil {
				fail(fmt.Errorf("round %d %s: %w\n%s", round+1, implementation, err, output))
			}
			path := filepath.Join(*outputDir, implementation+".txt")
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				fail(err)
			}
			_, writeErr := fmt.Fprintf(file, "# round %d implementation %s\n%s", round+1, implementation, output)
			closeErr := file.Close()
			if writeErr != nil {
				fail(writeErr)
			}
			if closeErr != nil {
				fail(closeErr)
			}
		}
	}
	manifest.RawFiles, err = benchmark.IdentifyRawFiles(*outputDir, corpus.Implementations)
	if err != nil {
		fail(err)
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail(err)
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(*outputDir, "run.json"), manifestData, 0o644); err != nil {
		fail(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "collected %d rotated rounds for %d implementations in %s\n", corpus.Run.Count, len(runtimes), *outputDir)
}

func runBenchmark(implementation string, arguments []string) (string, error) {
	command := exec.Command(arguments[0], arguments[1:]...)
	command.Env = append(os.Environ(), "BENCH_IMPLEMENTATION="+implementation)
	output, err := command.CombinedOutput()
	return string(output), err
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, strings.TrimSpace(err.Error()))
	os.Exit(1)
}
