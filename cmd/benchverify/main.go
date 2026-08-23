// Command benchverify proves benchmark eligibility against the pinned oracle.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tiaanduplessis/jsonata-go/internal/benchmark"
)

type verificationReport struct {
	RecordedAt      string                              `json:"recorded_at"`
	Reference       benchmark.Reference                 `json:"reference"`
	Implementations []benchmark.ImplementationReference `json:"implementations"`
	Run             benchmark.RunConfig                 `json:"run"`
	Machine         benchmark.MachineMetadata           `json:"machine"`
	Repository      benchmark.RepositoryMetadata        `json:"repository"`
	Cases           int                                 `json:"cases"`
	Requested       int                                 `json:"requested_cells"`
	Eligible        int                                 `json:"eligible_cells"`
	Unsupported     int                                 `json:"unsupported_cells"`
	Records         []benchmark.VerificationRecord      `json:"records"`
}

func main() {
	path := flag.String("corpus", "testdata/benchmark/corpus.json", "frozen benchmark corpus path")
	matrixPath := flag.String("matrix", "testdata/benchmark/matrix.json", "benchmark source matrix path")
	generatorPath := flag.String("generator", "testdata/benchmark/generate-oracle.mjs", "oracle generator path")
	packageLockPath := flag.String("package-lock", "testdata/benchmark/package-lock.json", "oracle dependency lock path")
	referenceRoot := flag.String("reference-root", "testdata/reference/jsonata-js-v2.2.2", "vendored jsonata-js reference root")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()

	if err := benchmark.ValidateOracleArtifacts(*matrixPath, *path, *generatorPath, *packageLockPath, *referenceRoot); err != nil {
		fail(err)
	}
	corpus, err := benchmark.LoadCorpus(*path)
	if err != nil {
		fail(err)
	}
	runtimes := benchmark.Implementations()
	matrix := benchmark.VerifyMatrix(corpus, runtimes)
	if err := matrix.ValidateWorkspace(corpus); err != nil {
		fail(err)
	}
	report := newVerificationReport(corpus, matrix)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, data, 0o644); err != nil {
			fail(fmt.Errorf("write verification report: %w", err))
		}
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fail(err)
	}
}

func newVerificationReport(corpus benchmark.Corpus, matrix benchmark.VerificationMatrix) verificationReport {
	eligible := 0
	for _, record := range matrix.Records {
		if record.Status == benchmark.StatusEligible {
			eligible++
		}
	}
	return verificationReport{
		RecordedAt:      time.Now().UTC().Format(time.RFC3339),
		Reference:       corpus.Reference,
		Implementations: corpus.Implementations,
		Run:             corpus.Run,
		Machine:         benchmark.CurrentMachineMetadata(),
		Repository:      benchmark.CurrentRepositoryMetadata(),
		Cases:           len(corpus.Cases),
		Requested:       len(matrix.Records),
		Eligible:        eligible,
		Unsupported:     len(matrix.Records) - eligible,
		Records:         matrix.Records,
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
