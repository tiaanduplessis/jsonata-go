package benchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

type oracleMatrix struct {
	Cases []matrixCase `json:"cases"`
}

type matrixCase struct {
	ID             string          `json:"id"`
	Category       string          `json:"category"`
	SizeTier       string          `json:"size_tier"`
	Dimensions     []string        `json:"dimensions"`
	Expression     string          `json:"expression"`
	Input          json.RawMessage `json:"input"`
	Modes          []Mode          `json:"modes"`
	CustomFunction string          `json:"custom_function,omitempty"`
	Source         *CaseSource     `json:"source,omitempty"`
}

// ValidateOracleArtifacts performs a read-only drift check over the matrix,
// generator, dependency lock, vendored source datasets, and frozen corpus.
func ValidateOracleArtifacts(matrixPath, corpusPath, generatorPath, packageLockPath, referenceRoot string) error {
	corpus, err := LoadCorpus(corpusPath)
	if err != nil {
		return err
	}
	for path, want := range map[string]string{
		matrixPath:      corpus.Reference.Generation.MatrixSHA256,
		generatorPath:   corpus.Reference.Generation.GeneratorSHA256,
		packageLockPath: corpus.Reference.Generation.PackageLockSHA256,
	} {
		got, err := fileSHA256(path)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("benchmark oracle input %s differs from the frozen generation pin", filepath.Base(path))
		}
	}
	matrixData, err := os.ReadFile(matrixPath)
	if err != nil {
		return fmt.Errorf("read benchmark matrix: %w", err)
	}
	var matrix oracleMatrix
	if err := json.Unmarshal(matrixData, &matrix); err != nil {
		return fmt.Errorf("decode benchmark matrix: %w", err)
	}
	if len(matrix.Cases) != len(corpus.Cases) {
		return fmt.Errorf("benchmark matrix and frozen corpus contain different case counts")
	}
	for index, source := range matrix.Cases {
		generated := corpus.Cases[index]
		if source.ID != generated.ID || source.Category != generated.Category || source.SizeTier != generated.SizeTier || source.Expression != generated.Expression || source.CustomFunction != generated.CustomFunction || !reflect.DeepEqual(source.Dimensions, generated.Dimensions) || !reflect.DeepEqual(source.Modes, generated.Modes) {
			return fmt.Errorf("benchmark case %q metadata differs from the source matrix", generated.ID)
		}
		input := source.Input
		if source.Source != nil {
			if generated.Source == nil || source.Source.Kind != generated.Source.Kind || source.Source.Path != generated.Source.Path || source.Source.Commit != generated.Source.Commit {
				return fmt.Errorf("benchmark case %q source metadata differs from the matrix", generated.ID)
			}
			datasetPath := filepath.Join(referenceRoot, filepath.FromSlash(source.Source.Path))
			datasetHash, err := fileSHA256(datasetPath)
			if err != nil {
				return err
			}
			if datasetHash != generated.Source.SHA256 {
				return fmt.Errorf("benchmark case %q official dataset differs from its frozen hash", generated.ID)
			}
			input, err = os.ReadFile(datasetPath)
			if err != nil {
				return fmt.Errorf("read benchmark source dataset: %w", err)
			}
		} else if generated.Source != nil {
			return fmt.Errorf("benchmark case %q unexpectedly claims an official source", generated.ID)
		}
		var decoded any
		if err := json.Unmarshal(input, &decoded); err != nil {
			return fmt.Errorf("decode benchmark case %q input: %w", generated.ID, err)
		}
		var frozen any
		if err := json.Unmarshal([]byte(generated.Input), &frozen); err != nil {
			return fmt.Errorf("decode frozen benchmark case %q input: %w", generated.ID, err)
		}
		if !reflect.DeepEqual(decoded, frozen) {
			return fmt.Errorf("benchmark case %q input differs from the source matrix or dataset", generated.ID)
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read oracle generation input %s: %w", path, err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
