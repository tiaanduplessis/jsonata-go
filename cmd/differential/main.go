package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiaanduplessis/jsonata-go/internal/differential"
)

func main() {
	output := flag.String("output", "testdata/differential/cases.json", "generated corpus output path")
	fuzzOutput := flag.String("fuzz-output", "testdata/differential/fuzz-cases.json", "generated bounded differential campaign output path")
	matrixOutput := flag.String("matrix-output", "reports/feature-matrix.json", "generated feature matrix output path")
	suiteRoot := flag.String("suite", "testdata/reference/jsonata-js-v2.2.2", "pinned language-neutral suite path")
	conformanceReport := flag.String("conformance-report", "reports/conformance/report.json", "complete pinned conformance report path")
	check := flag.Bool("check", false, "verify that the committed corpus is current")
	flag.Parse()

	corpus, err := differential.Generate()
	if err != nil {
		fatal(err)
	}
	encoded, err := differential.Encode(corpus)
	if err != nil {
		fatal(err)
	}
	fuzzCorpus, err := differential.GenerateFuzz()
	if err != nil {
		fatal(err)
	}
	fuzzEncoded, err := differential.Encode(fuzzCorpus)
	if err != nil {
		fatal(err)
	}
	matrix, err := differential.GenerateFeatureMatrix(*suiteRoot, *conformanceReport)
	if err != nil {
		fatal(err)
	}
	matrixEncoded, err := differential.Encode(matrix)
	if err != nil {
		fatal(err)
	}
	if *check {
		checkCurrent(*output, encoded)
		checkCurrent(*fuzzOutput, fuzzEncoded)
		checkCurrent(*matrixOutput, matrixEncoded)
		return
	}
	write(*output, encoded)
	write(*fuzzOutput, fuzzEncoded)
	write(*matrixOutput, matrixEncoded)
}

func checkCurrent(path string, want []byte) {
	current, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	if string(current) != string(want) {
		fatal(fmt.Errorf("%s is stale; regenerate it with go run ./cmd/differential", path))
	}
}

func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
