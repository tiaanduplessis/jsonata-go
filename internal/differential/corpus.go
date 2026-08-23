// Package differential defines the deterministic compatibility corpus used to
// compare this implementation with the pinned jsonata-js reference.
package differential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
)

const (
	ReferenceName    = "jsonata-js-v2.2.2"
	ReferenceVersion = "2.2.2"
	ReferenceCommit  = "6c7e95fdbf4405a1e741852a7cd8cd985b4305bb"
	CorpusSeed       = uint64(0x6c7e95f22)
	CorpusSize       = 256
	// FuzzCorpusSeed and FuzzCorpusSize define the separately versioned
	// generated campaign. The campaign is larger than the smoke corpus but
	// remains bounded so its oracle can be reviewed and replayed offline.
	FuzzCorpusSeed = uint64(0x4f4950467a11)
	FuzzCorpusSize = 512
)

type Corpus struct {
	SchemaVersion   int    `json:"schemaVersion"`
	ReferenceName   string `json:"referenceName"`
	ReferenceCommit string `json:"referenceCommit"`
	Seed            uint64 `json:"seed"`
	Cases           []Case `json:"cases"`
}

type Case struct {
	ID         string          `json:"id"`
	Family     string          `json:"family"`
	Expression string          `json:"expression"`
	HasInput   bool            `json:"hasInput"`
	Input      json.RawMessage `json:"input,omitempty"`
}

type Oracle struct {
	SchemaVersion   int            `json:"schemaVersion"`
	ReferenceName   string         `json:"referenceName"`
	ReferenceCommit string         `json:"referenceCommit"`
	Cases           []OracleResult `json:"cases"`
}

type OracleResult struct {
	ID    string           `json:"id"`
	Kind  string           `json:"kind"`
	Value json.RawMessage  `json:"value,omitempty"`
	Error *StructuredError `json:"error,omitempty"`
}

type StructuredError struct {
	Code  string `json:"code"`
	Token string `json:"token,omitempty"`
}

// Generate returns a stable set of valid programs and JSON inputs. The
// grammar deliberately spans values, empty sequences, and dynamic errors.
func Generate() (Corpus, error) {
	rng := rand.New(rand.NewPCG(CorpusSeed, CorpusSeed^0x9e3779b97f4a7c15))
	cases := make([]Case, 0, CorpusSize)
	for index := 0; index < CorpusSize; index++ {
		input := generatedInput(rng, index)
		family, expression, hasInput := generatedExpression(index)
		var raw json.RawMessage
		if hasInput {
			encoded, err := json.Marshal(input)
			if err != nil {
				return Corpus{}, fmt.Errorf("encode case %d input: %w", index, err)
			}
			raw = encoded
		}
		cases = append(cases, Case{
			ID:         fmt.Sprintf("generated-%03d", index),
			Family:     family,
			Expression: expression,
			HasInput:   hasInput,
			Input:      raw,
		})
	}
	return Corpus{
		SchemaVersion:   1,
		ReferenceName:   ReferenceName,
		ReferenceCommit: ReferenceCommit,
		Seed:            CorpusSeed,
		Cases:           cases,
	}, nil
}

// GenerateFuzz returns the bounded generated differential campaign. Every
// expression is selected from syntax-valid templates and every case carries a
// JSON document, while the deterministic random choices provide more input
// and literal coverage than the hand-balanced smoke corpus.
func GenerateFuzz() (Corpus, error) {
	rng := rand.New(rand.NewPCG(FuzzCorpusSeed, FuzzCorpusSeed^0x9e3779b97f4a7c15))
	cases := make([]Case, 0, FuzzCorpusSize)
	for index := 0; index < FuzzCorpusSize; index++ {
		input := generatedInput(rng, index)
		family, expression := generatedFuzzExpression(rng)
		encoded, err := json.Marshal(input)
		if err != nil {
			return Corpus{}, fmt.Errorf("encode fuzz case %d input: %w", index, err)
		}
		cases = append(cases, Case{
			ID:         fmt.Sprintf("fuzz-%03d", index),
			Family:     family,
			Expression: expression,
			HasInput:   true,
			Input:      encoded,
		})
	}
	return Corpus{
		SchemaVersion:   1,
		ReferenceName:   ReferenceName,
		ReferenceCommit: ReferenceCommit,
		Seed:            FuzzCorpusSeed,
		Cases:           cases,
	}, nil
}

func Encode(v any) ([]byte, error) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func DecodeCorpus(data []byte) (Corpus, error) {
	return decodeCorpus(data, CorpusSeed, CorpusSize)
}

// DecodeFuzzCorpus validates the generated campaign fixture and its pinned
// provenance without allowing it to be confused with the smoke corpus.
func DecodeFuzzCorpus(data []byte) (Corpus, error) {
	return decodeCorpus(data, FuzzCorpusSeed, FuzzCorpusSize)
}

func decodeCorpus(data []byte, seed uint64, size int) (Corpus, error) {
	var corpus Corpus
	if err := decodeStrict(data, &corpus); err != nil {
		return Corpus{}, err
	}
	if corpus.SchemaVersion != 1 || corpus.ReferenceName != ReferenceName || corpus.ReferenceCommit != ReferenceCommit || corpus.Seed != seed {
		return Corpus{}, fmt.Errorf("differential corpus metadata does not match the pinned reference")
	}
	if len(corpus.Cases) != size {
		return Corpus{}, fmt.Errorf("differential corpus has %d cases; want %d", len(corpus.Cases), size)
	}
	ids := make(map[string]struct{}, len(corpus.Cases))
	for _, testCase := range corpus.Cases {
		if testCase.ID == "" || testCase.Family == "" || testCase.Expression == "" {
			return Corpus{}, fmt.Errorf("differential corpus contains an incomplete case")
		}
		if _, duplicate := ids[testCase.ID]; duplicate {
			return Corpus{}, fmt.Errorf("differential corpus contains duplicate case %q", testCase.ID)
		}
		ids[testCase.ID] = struct{}{}
		if testCase.HasInput != (len(testCase.Input) > 0) || (testCase.HasInput && !json.Valid(testCase.Input)) {
			return Corpus{}, fmt.Errorf("differential corpus case %q has invalid input metadata", testCase.ID)
		}
	}
	return corpus, nil
}

func DecodeOracle(data []byte) (Oracle, error) {
	return decodeOracle(data, CorpusSize)
}

// DecodeFuzzOracle validates the oracle generated for the bounded campaign.
func DecodeFuzzOracle(data []byte) (Oracle, error) {
	return decodeOracle(data, FuzzCorpusSize)
}

func decodeOracle(data []byte, size int) (Oracle, error) {
	var oracle Oracle
	if err := decodeStrict(data, &oracle); err != nil {
		return Oracle{}, err
	}
	if oracle.SchemaVersion != 1 || oracle.ReferenceName != ReferenceName || oracle.ReferenceCommit != ReferenceCommit {
		return Oracle{}, fmt.Errorf("differential oracle metadata does not match the pinned reference")
	}
	if len(oracle.Cases) != size {
		return Oracle{}, fmt.Errorf("differential oracle has %d cases; want %d", len(oracle.Cases), size)
	}
	ids := make(map[string]struct{}, len(oracle.Cases))
	for _, result := range oracle.Cases {
		if result.ID == "" {
			return Oracle{}, fmt.Errorf("differential oracle contains a case without an ID")
		}
		if _, duplicate := ids[result.ID]; duplicate {
			return Oracle{}, fmt.Errorf("differential oracle contains duplicate case %q", result.ID)
		}
		ids[result.ID] = struct{}{}
		switch result.Kind {
		case "value":
			if len(result.Value) == 0 || !json.Valid(result.Value) || result.Error != nil {
				return Oracle{}, fmt.Errorf("differential oracle value case %q has an invalid result shape", result.ID)
			}
		case "undefined":
			if len(result.Value) != 0 || result.Error != nil {
				return Oracle{}, fmt.Errorf("differential oracle undefined case %q has an invalid result shape", result.ID)
			}
		case "error":
			if len(result.Value) != 0 || result.Error == nil || result.Error.Code == "" {
				return Oracle{}, fmt.Errorf("differential oracle error case %q has an invalid result shape", result.ID)
			}
		default:
			return Oracle{}, fmt.Errorf("differential oracle case %q has unknown kind %q", result.ID, result.Kind)
		}
	}
	return oracle, nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

type generatedDocument struct {
	A         int             `json:"a"`
	B         int             `json:"b"`
	Factor    int             `json:"factor"`
	Threshold int             `json:"threshold"`
	Label     string          `json:"label"`
	Suffix    string          `json:"suffix"`
	Numbers   []int           `json:"numbers"`
	Items     []generatedItem `json:"items"`
	Prototype map[string]any  `json:"__proto__"`
	Wildcard  string          `json:"*"`
}

type generatedItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func generatedInput(rng *rand.Rand, index int) generatedDocument {
	width := 2 + rng.IntN(5)
	numbers := make([]int, width)
	items := make([]generatedItem, width)
	for item := range width {
		numbers[item] = rng.IntN(31) - 15
		items[item] = generatedItem{Name: fmt.Sprintf("item-%d-%d", index, item), Value: rng.IntN(21) - 10}
	}
	return generatedDocument{
		A:         rng.IntN(31) - 15,
		B:         rng.IntN(31) - 15,
		Factor:    1 + rng.IntN(5),
		Threshold: rng.IntN(11) - 5,
		Label:     fmt.Sprintf("label-%03d", index),
		Suffix:    []string{"a", "el", "-x", "9"}[rng.IntN(4)],
		Numbers:   numbers,
		Items:     items,
		Prototype: map[string]any{"safe": index},
		Wildcard:  fmt.Sprintf("wild-%03d", index),
	}
}

func generatedFuzzExpression(rng *rand.Rand) (string, string) {
	// Keep the no-input template out of this campaign: its purpose is to vary
	// the JSON document and exercise input-bound evaluation. The remaining
	// templates are syntax-valid programs, including programs whose evaluation
	// intentionally produces a language error.
	selector := rng.IntN(39)
	if selector >= 29 {
		selector++
	}
	variation := rng.IntN(128)
	family, expression, _ := generatedExpression(variation*40 + selector)
	return family, expression
}

func generatedExpression(index int) (string, string, bool) {
	variation := index / 40
	switch index % 40 {
	case 0:
		return "arithmetic", fmt.Sprintf("(a + b) * factor + %d", variation), true
	case 1:
		return "comparison", fmt.Sprintf("a <= b + %d", variation), true
	case 2:
		return "boolean", fmt.Sprintf("(a > %d) and (b <= threshold)", variation), true
	case 3:
		return "conditional", fmt.Sprintf("a > %d ? label : suffix", variation), true
	case 4:
		return "block-binding", fmt.Sprintf("($x := a + %d; $x * factor)", variation), true
	case 5:
		return "array-constructor", fmt.Sprintf("[a, b, %d, numbers]", variation), true
	case 6:
		return "object-constructor", fmt.Sprintf(`{"sum": a + b + %d, "text": label & suffix}`, variation), true
	case 7:
		return "path-projection", fmt.Sprintf("items.(value + %d)", variation), true
	case 8:
		return "predicate", fmt.Sprintf("items[value >= $$.threshold + %d].name", variation), true
	case 9:
		return "order-by", fmt.Sprintf("items^(<value).name[%d]", variation), true
	case 10:
		return "wildcard", fmt.Sprintf(`items.*[$type($)="number"].($ + %d)`, variation), true
	case 11:
		return "descendant", fmt.Sprintf(`items.**[name].(name & "-%d")`, variation), true
	case 12:
		return "sum", fmt.Sprintf("$sum(numbers) + %d", variation), true
	case 13:
		return "map", fmt.Sprintf("$map(numbers, function($v){$v * factor + %d})", variation), true
	case 14:
		return "filter", fmt.Sprintf("$filter(numbers, function($v){$v >= %d})", variation), true
	case 15:
		return "reduce", fmt.Sprintf("$reduce(numbers, function($acc,$v){$acc+$v}, %d)", variation), true
	case 16:
		return "regex-contains", fmt.Sprintf("$contains(label, /%d|label/)", variation), true
	case 17:
		return "replace", fmt.Sprintf(`$replace(label, "label", "item-%d")`, variation), true
	case 18:
		return "split-join", fmt.Sprintf(`$join($split(label & "-%d", "-"), ":")`, variation), true
	case 19:
		return "lookup-undefined", fmt.Sprintf(`$lookup($, "absent-%d")`, variation), true
	case 20:
		return "spread-merge", fmt.Sprintf(`$merge([$spread({"a":a,"b":b}), {"v":%d}])`, variation), true
	case 21:
		return "keys-each", fmt.Sprintf(`$each({"a":a,"b":b}, function($v,$k){$k & ":" & ($v + %d)})`, variation), true
	case 22:
		return "sift", fmt.Sprintf(`$sift({"a":a,"b":b}, function($v){$v >= %d})`, variation), true
	case 23:
		return "append", fmt.Sprintf("$append(numbers, %d)", variation), true
	case 24:
		return "range", fmt.Sprintf("[%d..%d]", variation, variation+3), true
	case 25:
		return "closure", fmt.Sprintf("($f := function($x){$x + %d}; $f(a))", variation), true
	case 26:
		return "chain", fmt.Sprintf("label ~> $substring(%d, 3)", variation), true
	case 27:
		return "default-operator", fmt.Sprintf(`$lookup($, "absent-%d") ?: label`, variation), true
	case 28:
		return "coalescing-operator", fmt.Sprintf(`$lookup($, "absent-%d") ?? suffix`, variation), true
	case 29:
		return "no-input-literal", fmt.Sprintf(`{"generated":%d,"text":"case-%03d"}`, variation, index), false
	case 30:
		return "type-error-T0410", fmt.Sprintf("$sqrt(label & %q)", fmt.Sprintf("-%d", variation)), true
	case 31:
		return "type-error-T2001", fmt.Sprintf("(label & %q) + 1", fmt.Sprintf("-%d", variation)), true
	case 32:
		return "type-error-T2002", fmt.Sprintf("1 + (label & %q)", fmt.Sprintf("-%d", variation)), true
	case 33:
		return "type-error-T0412", fmt.Sprintf(`$sum([%d, label])`, variation), true
	case 34:
		return "dynamic-error-D1009", fmt.Sprintf(`{"duplicate-%d":1,"duplicate-%d":2}`, variation, variation), true
	case 35:
		return "type-error-T1003", fmt.Sprintf("{%d:true}", variation), true
	case 36:
		return "type-error-T1006", fmt.Sprintf("($missing%d)()", variation), true
	case 37:
		return "type-error-T2011", fmt.Sprintf(`$ ~> |$|%d|`, variation), true
	case 38:
		return "type-error-T2012", fmt.Sprintf(`$ ~> |$|{},%d|`, variation), true
	default:
		return "type-error-T0410-map", fmt.Sprintf(`$map(numbers, "not-a-function-%d")`, variation), true
	}
}
