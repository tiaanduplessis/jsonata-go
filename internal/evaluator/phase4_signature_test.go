package evaluator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/conformance"
	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestFunctionSignatureFixtures(t *testing.T) {
	suite, err := conformance.LoadSuite(filepath.Join("..", "..", "testdata", "reference", conformance.ReferenceName))
	if err != nil {
		t.Fatal(err)
	}
	var group conformance.Group
	for _, candidate := range suite.Groups {
		if candidate.Name == "function-signatures" {
			group = candidate
			break
		}
	}
	if len(group.Cases) != 41 {
		t.Fatalf("function-signatures fixture count = %d, want 41", len(group.Cases))
	}
	for _, fixture := range group.Cases {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			data := fixture.Data
			if fixture.HasDataset && fixture.Dataset != "" {
				path := filepath.Join(suite.Root, "test", "test-suite", "datasets", fixture.Dataset+".json")
				contents, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if readErr = json.Unmarshal(contents, &data); readErr != nil {
					t.Fatal(readErr)
				}
			}
			n, parseErr := syntax.Parse(fixture.Expression)
			if parseErr != nil {
				assertSignatureError(t, fixture, parseErr)
				return
			}
			result, evalErr := EvalBindings(n, data, signatureFixtureBindings())
			switch fixture.ExpectedKind {
			case conformance.ExpectedError:
				assertSignatureError(t, fixture, evalErr)
			case conformance.ExpectedUndefined:
				if !IsUndefined(evalErr) {
					t.Fatalf("got result=%#v err=%v, want undefined", result, evalErr)
				}
			default:
				if evalErr != nil {
					t.Fatal(evalErr)
				}
				if !reflect.DeepEqual(result, fixture.Expected) {
					t.Fatalf("got %#v, want %#v", result, fixture.Expected)
				}
			}
		})
	}
}

func TestUserSignatureUndefinedArgumentPreservesEmptySequence(t *testing.T) {
	tests := []string{
		`λ($arg)<b:b>{$not($arg)}(foo)`,
		`λ($arg)<n:n>{$arg}(foo)`,
		`λ($arg)<a<n>:s>{$join($arg)}(foo)`,
	}
	for _, expression := range tests {
		node, parseErr := syntax.Parse(expression)
		if parseErr != nil {
			t.Fatalf("Parse(%q): %v", expression, parseErr)
		}
		result, evalErr := EvalBindings(node, nil, signatureFixtureBindings())
		if !IsUndefined(evalErr) {
			t.Fatalf("%s: got result=%#v err=%v, want undefined", expression, result, evalErr)
		}
	}
	node, parseErr := syntax.Parse(`λ($arg)<b:b>{$not($arg)}(foo)`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	result, evalErr := Eval(node, nil)
	if !IsUndefined(evalErr) {
		t.Fatalf("actual builtin: got result=%#v err=%v, want undefined", result, evalErr)
	}
}

func assertSignatureError(t *testing.T, fixture conformance.Case, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("got success, want %s", fixture.ExpectedCode)
	}
	var parseErr *syntax.ParseError
	if errors.As(err, &parseErr) {
		if parseErr.Code != fixture.ExpectedCode {
			t.Fatalf("got parse error %s, want %s", parseErr.Code, fixture.ExpectedCode)
		}
		return
	}
	var coded interface{ JSONataCode() string }
	if !errors.As(err, &coded) || coded.JSONataCode() != fixture.ExpectedCode {
		actual := "<none>"
		if errors.As(err, &coded) {
			actual = coded.JSONataCode()
		}
		t.Fatalf("got error %v (code %s), want %s", err, actual, fixture.ExpectedCode)
	}
}

// These narrow test doubles isolate signature behavior without adding Phase 4
// standard-library implementations to the evaluator.
func signatureFixtureBindings() map[string]any {
	return map[string]any{
		"$not":       builtinValue{spec: builtinSpec{name: "$not", implementation: signatureTestNot}},
		"$uppercase": builtinValue{spec: builtinSpec{name: "$uppercase", implementation: signatureTestUppercase}},
		"$join":      builtinValue{spec: builtinSpec{name: "$join", implementation: signatureTestJoin}},
		"$number":    builtinValue{spec: builtinSpec{name: "$number", implementation: signatureTestNumber}},
	}
}

func signatureTestNot(st state, args []any) (any, error) {
	if len(args) != 1 {
		return nil, functionArityError("$not", 1, len(args))
	}
	if value.IsUndefined(args[0]) {
		return value.Undefined, nil
	}
	return !ebv(args[0]), nil
}

func signatureTestUppercase(st state, args []any) (any, error) {
	if len(args) != 1 || value.IsUndefined(args[0]) {
		return value.Undefined, nil
	}
	return strings.ToUpper(toString(collapse(args[0]))), nil
}

func signatureTestJoin(st state, args []any) (any, error) {
	if len(args) < 1 || value.IsUndefined(args[0]) {
		return value.Undefined, nil
	}
	items, _ := signatureArrayItems(args[0])
	separator := ""
	if len(args) > 1 && !value.IsUndefined(args[1]) {
		separator = toString(collapse(args[1]))
	}
	parts := make([]string, len(items.Items))
	for i, item := range items.Items {
		parts[i] = toString(collapse(item))
	}
	return strings.Join(parts, separator), nil
}

func signatureTestNumber(st state, args []any) (any, error) {
	if len(args) != 1 || value.IsUndefined(args[0]) {
		return value.Undefined, nil
	}
	if n, ok := numeric(collapse(args[0])); ok {
		return n, nil
	}
	return nil, fmt.Errorf("cannot convert %v to number", args[0])
}
