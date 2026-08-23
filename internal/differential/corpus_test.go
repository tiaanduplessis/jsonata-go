package differential_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	jsonata "github.com/tiaanduplessis/jsonata-go"
	"github.com/tiaanduplessis/jsonata-go/internal/differential"
)

func TestGeneratedCorpusAndPinnedOracle(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "differential")
	corpusBytes := readFixture(t, filepath.Join(root, "cases.json"))
	corpus, err := differential.DecodeCorpus(corpusBytes)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := differential.Generate()
	if err != nil {
		t.Fatal(err)
	}
	generatedBytes, err := differential.Encode(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(corpusBytes, generatedBytes) {
		t.Fatal("committed differential corpus is stale; run go run ./cmd/differential")
	}

	oracle, err := differential.DecodeOracle(readFixture(t, filepath.Join(root, "oracle.json")))
	if err != nil {
		t.Fatal(err)
	}
	if len(oracle.Cases) != len(corpus.Cases) {
		t.Fatalf("oracle has %d cases for a %d-case corpus", len(oracle.Cases), len(corpus.Cases))
	}
	results := make(map[string]differential.OracleResult, len(oracle.Cases))
	kinds := make(map[string]int)
	errorCodes := make(map[string]int)
	for _, result := range oracle.Cases {
		if _, duplicate := results[result.ID]; duplicate {
			t.Fatalf("oracle contains duplicate case %q", result.ID)
		}
		if result.Kind != "value" && result.Kind != "undefined" && result.Kind != "error" {
			t.Fatalf("oracle case %q has unknown kind %q", result.ID, result.Kind)
		}
		results[result.ID] = result
		kinds[result.Kind]++
		if result.Kind == "error" && result.Error != nil {
			errorCodes[result.Error.Code]++
		}
	}
	for _, required := range []string{"value", "undefined", "error"} {
		if kinds[required] == 0 {
			t.Fatalf("oracle has no %s cases", required)
		}
	}
	if len(errorCodes) < 8 {
		t.Fatalf("oracle covers %d structured error codes; want at least 8", len(errorCodes))
	}

	families := make(map[string]int)
	expressions := make(map[string]struct{})
	for _, testCase := range corpus.Cases {
		if testCase.Family == "" {
			t.Fatalf("corpus case %q has no grammar family", testCase.ID)
		}
		families[testCase.Family]++
		expressions[testCase.Expression] = struct{}{}
	}
	if len(families) < 40 {
		t.Fatalf("corpus covers %d grammar families; want at least 40", len(families))
	}
	if len(expressions) < 240 {
		t.Fatalf("corpus has %d unique expressions; want at least 240", len(expressions))
	}
	for family, count := range families {
		if count > 7 {
			t.Fatalf("grammar family %q occupies %d cases; want at most 7", family, count)
		}
	}

	for _, testCase := range corpus.Cases {
		testCase := testCase
		t.Run(testCase.ID, func(t *testing.T) {
			t.Parallel()
			want, ok := results[testCase.ID]
			if !ok {
				t.Fatalf("oracle result is missing")
			}
			actual, evalErr := evaluate(testCase)
			switch want.Kind {
			case "value":
				if evalErr != nil {
					t.Fatalf("evaluate: %v", evalErr)
				}
				if !jsonEqual(actual, want.Value) {
					t.Fatalf("want %s, got %s", want.Value, actual)
				}
			case "undefined":
				if !errors.Is(evalErr, jsonata.ErrUndefined) {
					t.Fatalf("got %s, %v; want undefined", actual, evalErr)
				}
			case "error":
				assertStructuredError(t, evalErr, want.Error)
			}
		})
	}
}

func TestGeneratedFuzzCampaignAndPinnedOracle(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "differential")
	corpusBytes := readFixture(t, filepath.Join(root, "fuzz-cases.json"))
	corpus, err := differential.DecodeFuzzCorpus(corpusBytes)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := differential.GenerateFuzz()
	if err != nil {
		t.Fatal(err)
	}
	generatedBytes, err := differential.Encode(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(corpusBytes, generatedBytes) {
		t.Fatal("committed generated fuzz corpus is stale; run go run ./cmd/differential")
	}
	oracle, err := differential.DecodeFuzzOracle(readFixture(t, filepath.Join(root, "fuzz-oracle.json")))
	if err != nil {
		t.Fatal(err)
	}
	if len(oracle.Cases) != len(corpus.Cases) {
		t.Fatalf("fuzz oracle has %d cases for a %d-case corpus", len(oracle.Cases), len(corpus.Cases))
	}
	results := make(map[string]differential.OracleResult, len(oracle.Cases))
	kinds := make(map[string]int)
	for _, result := range oracle.Cases {
		if _, duplicate := results[result.ID]; duplicate {
			t.Fatalf("fuzz oracle contains duplicate case %q", result.ID)
		}
		results[result.ID] = result
		kinds[result.Kind]++
	}
	for _, required := range []string{"value", "undefined", "error"} {
		if kinds[required] == 0 {
			t.Fatalf("fuzz oracle has no %s cases", required)
		}
	}
	for _, testCase := range corpus.Cases {
		testCase := testCase
		want, ok := results[testCase.ID]
		if !ok {
			t.Fatalf("fuzz oracle result is missing for %q", testCase.ID)
		}
		if !testCase.HasInput {
			t.Fatalf("fuzz case %q has no JSON input", testCase.ID)
		}
		t.Run(testCase.ID, func(t *testing.T) {
			actual, evalErr := evaluate(testCase)
			assertOracleResult(t, actual, evalErr, want)
		})
	}
}

func assertOracleResult(t *testing.T, actual []byte, evalErr error, want differential.OracleResult) {
	t.Helper()
	switch want.Kind {
	case "value":
		if evalErr != nil {
			t.Fatalf("evaluate: %v", evalErr)
		}
		if !jsonEqual(actual, want.Value) {
			t.Fatalf("want %s, got %s", want.Value, actual)
		}
	case "undefined":
		if !errors.Is(evalErr, jsonata.ErrUndefined) {
			t.Fatalf("got %s, %v; want undefined", actual, evalErr)
		}
	case "error":
		assertStructuredError(t, evalErr, want.Error)
	default:
		t.Fatalf("unknown oracle result kind %q", want.Kind)
	}
}

func evaluate(testCase differential.Case) ([]byte, error) {
	compiled, err := jsonata.Compile(testCase.Expression)
	if err != nil {
		return nil, err
	}
	if testCase.HasInput {
		return compiled.EvalBytes(testCase.Input)
	}
	result, err := compiled.EvalNoInput()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func assertStructuredError(t *testing.T, err error, want *differential.StructuredError) {
	t.Helper()
	if want == nil || want.Code == "" {
		t.Fatal("oracle error is missing its code")
	}
	var actual *jsonata.Error
	if !errors.As(err, &actual) {
		t.Fatalf("got %v; want structured error %s", err, want.Code)
	}
	if actual.Code != want.Code || actual.Token != want.Token {
		t.Fatalf("got %s token %q; want %s token %q", actual.Code, actual.Token, want.Code, want.Token)
	}
}

func jsonEqual(actual, expected []byte) bool {
	var left, right any
	leftDecoder := json.NewDecoder(bytes.NewReader(actual))
	leftDecoder.UseNumber()
	rightDecoder := json.NewDecoder(bytes.NewReader(expected))
	rightDecoder.UseNumber()
	return leftDecoder.Decode(&left) == nil && rightDecoder.Decode(&right) == nil && reflect.DeepEqual(left, right)
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
