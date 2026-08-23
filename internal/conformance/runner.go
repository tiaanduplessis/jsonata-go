package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

// EvaluateFunc evaluates one compiled case. hasInput distinguishes the empty
// input sequence from explicit JSON null. timeLimit is expressed in
// milliseconds and depth is the reference suite's maximum evaluation depth.
// By default the callback is only used for cases that declare either limit;
// Options.EvaluateAll routes every selected case through it.
type EvaluateFunc func(expression Expression, data any, hasInput bool, bindings map[string]any, timeLimit, depth int) (any, error)

// Options supplies evaluator-specific behavior without importing the public
// package (which keeps this package usable by loader-only tests). Undefined
// results must be represented by this exact error, or a wrapped equivalent.
type Options struct {
	UndefinedError error
	Evaluate       EvaluateFunc
	// EvaluateAll lets complete-suite runners apply one evaluator policy,
	// including an operation budget suitable for performance fixtures.
	EvaluateAll bool
}

// Run executes exactly the cases in manifest. Cases outside it are retained
// as remaining, including unsupported reference-only fixtures. An evaluator
// error from an enabled case is always a failure.
func Run(suite Suite, compiler Compiler, manifest Manifest) Report {
	return RunWithOptions(suite, compiler, manifest, Options{})
}

// RunWithOptions is Run with explicit public evaluator sentinel handling.
func RunWithOptions(suite Suite, compiler Compiler, manifest Manifest, options Options) Report {
	report := Report{
		ReferenceName:   ReferenceName,
		ReferenceCommit: suite.ReferenceCommit,
		EnabledGroups:   make([]string, 0),
		RemainingGroups: make([]string, 0),
		RemainingCases:  make([]CaseRef, 0),
		Failures:        make([]Failure, 0),
		Skips:           make([]CaseRef, 0),
	}
	knownGroups := make(map[string]struct{}, len(suite.Groups))
	for _, group := range suite.Groups {
		knownGroups[group.Name] = struct{}{}
		report.Discovered = append(report.Discovered, GroupSummary{Name: group.Name, Cases: len(group.Cases)})
		selected := manifest[group.Name]
		validSelected := 0
		for id := range selected {
			found := false
			for _, c := range group.Cases {
				if c.ID == id {
					found = true
					break
				}
			}
			if !found {
				report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: group.Name, ID: id}, Message: "manifest selected an unknown case"})
			} else {
				validSelected++
			}
		}
		if len(selected) > 0 {
			report.EnabledGroups = append(report.EnabledGroups, group.Name)
		}
		for _, testCase := range group.Cases {
			if _, enabled := selected[testCase.ID]; !enabled {
				report.RemainingCases = append(report.RemainingCases, CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source, Reason: testCase.UnsupportedWhy})
				continue
			}
			report.EnabledCases++
			if !testCase.SupportedInput {
				report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: "manifest selected an unsupported fixture"})
				continue
			}
			data, err := caseData(suite, testCase)
			if err != nil {
				report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: err.Error()})
				continue
			}
			expression, err := compiler.Compile(testCase.Expression)
			if err != nil {
				if testCase.ExpectedKind == ExpectedError && expectedError(err, testCase) {
					report.Passes++
				} else {
					report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: fmt.Sprintf("compile: %v", err)})
				}
				continue
			}
			actual, err := evalCaseWithOptions(expression, data, testCase, options)
			if testCase.ExpectedKind == ExpectedError {
				if err != nil && expectedError(err, testCase) {
					report.Passes++
				} else if err == nil {
					report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: "expected error " + testCase.ExpectedCode + ", got a value"})
				} else {
					report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: fmt.Sprintf("expected error %s, got %v", testCase.ExpectedCode, err)})
				}
				continue
			}
			if testCase.ExpectedKind == ExpectedUndefined {
				if err != nil && isUndefined(err, options) {
					report.Passes++
				} else if err == nil {
					report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: "expected the configured undefined error, got a value"})
				} else {
					report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: fmt.Sprintf("expected the configured undefined error, got %v", err)})
				}
				continue
			}
			if err != nil {
				report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: fmt.Sprintf("eval: %v", err)})
				continue
			}
			if jsonEqualCase(actual, testCase.Expected, testCase.Unordered) {
				report.Passes++
			} else {
				report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: testCase.Group, ID: testCase.ID, Source: testCase.Source}, Message: fmt.Sprintf("want %s, got %s", valueString(testCase.Expected), valueString(actual))})
			}
		}
		if validSelected == 0 && len(selected) > 0 {
			// Keep the group visible as a failure, while retaining every case as
			// remaining rather than allowing a typo to create an empty gate.
			report.RemainingGroups = append(report.RemainingGroups, group.Name)
		}
	}
	for name, selected := range manifest {
		if _, ok := knownGroups[name]; !ok {
			report.Failures = append(report.Failures, Failure{CaseRef: CaseRef{Group: name}, Message: "manifest selected an unknown group"})
		}
		if len(selected) == 0 {
			continue
		}
	}
	report.RemainingGroups = remainingGroups(suite, manifest)
	return report
}

func evalCase(expression Expression, data any, hasInput bool, bindings map[string]any) (any, error) {
	if !hasInput {
		if withBindings, ok := expression.(interface {
			EvalNoInputBindings(map[string]any) (any, error)
		}); ok {
			return withBindings.EvalNoInputBindings(bindings)
		}
		if len(bindings) != 0 {
			return nil, errors.New("conformance expression does not support absent input with evaluation bindings")
		}
		if withoutInput, ok := expression.(interface {
			EvalNoInput() (any, error)
		}); ok {
			return withoutInput.EvalNoInput()
		}
		return nil, errors.New("conformance expression does not support absent input")
	}
	if withBindings, ok := expression.(interface {
		EvalBindings(any, map[string]any) (any, error)
	}); ok {
		return withBindings.EvalBindings(data, bindings)
	}
	if len(bindings) != 0 {
		return nil, errors.New("conformance expression does not support evaluation bindings")
	}
	return expression.Eval(data)
}

func evalCaseWithOptions(expression Expression, data any, testCase Case, options Options) (any, error) {
	if !options.EvaluateAll && testCase.TimeLimit == 0 && testCase.Depth == 0 {
		return evalCase(expression, data, testCase.HasInput(), testCase.Bindings)
	}
	if options.Evaluate == nil {
		return nil, fmt.Errorf("case declares timelimit=%dms and depth=%d but no evaluator callback was configured", testCase.TimeLimit, testCase.Depth)
	}
	return options.Evaluate(expression, data, testCase.HasInput(), testCase.Bindings, testCase.TimeLimit, testCase.Depth)
}

func isUndefined(err error, options Options) bool {
	return options.UndefinedError != nil && errors.Is(err, options.UndefinedError)
}

func caseData(suite Suite, testCase Case) (any, error) {
	if testCase.HasData {
		return testCase.Data, nil
	}
	if !testCase.HasDataset || testCase.Dataset == "" {
		return nil, nil
	}
	path := filepath.Join(suite.Root, "test", "test-suite", "datasets", testCase.Dataset+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dataset %q: %w", testCase.Dataset, err)
	}
	data, err := value.DecodeJSON(b)
	if err != nil {
		return nil, fmt.Errorf("dataset %q: %w", testCase.Dataset, err)
	}
	return data, nil
}

func remainingGroups(suite Suite, manifest Manifest) []string {
	var remaining []string
	for _, group := range suite.Groups {
		selected := manifest[group.Name]
		selectedCount := 0
		for _, testCase := range group.Cases {
			if _, ok := selected[testCase.ID]; ok {
				selectedCount++
			}
		}
		if selectedCount < len(group.Cases) {
			remaining = append(remaining, group.Name)
		}
	}
	return remaining
}

func expectedError(err error, testCase Case) bool {
	if err == nil {
		return false
	}
	var coded interface{ JSONataCode() string }
	if !errors.As(err, &coded) || coded.JSONataCode() != testCase.ExpectedCode {
		return false
	}
	return expectedErrorFields(err, testCase)
}

func expectedErrorFields(err error, testCase Case) bool {
	if testCase.ExpectedToken == "" && !testCase.HasExpectedPosition {
		return true
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		v := reflect.ValueOf(current)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				continue
			}
			v = v.Elem()
		}
		if !v.IsValid() || v.Kind() != reflect.Struct {
			continue
		}
		if testCase.ExpectedToken != "" {
			token := v.FieldByName("Token")
			if !token.IsValid() || token.Kind() != reflect.String || token.String() != testCase.ExpectedToken {
				continue
			}
		}
		if testCase.HasExpectedPosition {
			position := v.FieldByName("Position")
			if !position.IsValid() || !isInteger(position.Kind()) || int(position.Int()) != testCase.ExpectedPosition {
				continue
			}
		}
		return true
	}
	return false
}

func jsonEqual(left, right any) bool {
	return equalValue(reflect.ValueOf(left), reflect.ValueOf(right))
}

func jsonEqualCase(left, right any, unordered bool) bool {
	if !unordered {
		return jsonEqual(left, right)
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if !isArrayValue(leftValue) || !isArrayValue(rightValue) {
		return jsonEqual(left, right)
	}
	if leftValue.Len() != rightValue.Len() {
		return false
	}
	matched := make([]bool, rightValue.Len())
	for i := 0; i < leftValue.Len(); i++ {
		found := false
		for j := 0; j < rightValue.Len(); j++ {
			if matched[j] || !equalValue(leftValue.Index(i), rightValue.Index(j)) {
				continue
			}
			matched[j] = true
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

func isArrayValue(value reflect.Value) bool {
	if !value.IsValid() {
		return false
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	return value.Kind() == reflect.Slice || value.Kind() == reflect.Array
}

func equalValue(left, right reflect.Value) bool {
	if !left.IsValid() || !right.IsValid() {
		return !left.IsValid() && !right.IsValid()
	}
	if isNumber(left.Kind()) && isNumber(right.Kind()) {
		l, lok := numberValue(left)
		r, rok := numberValue(right)
		return lok && rok && l.Cmp(r) == 0
	}
	if left.Kind() != right.Kind() {
		return false
	}
	switch left.Kind() {
	case reflect.Interface, reflect.Pointer:
		return equalValue(left.Elem(), right.Elem())
	case reflect.Map:
		if left.Len() != right.Len() {
			return false
		}
		for _, key := range left.MapKeys() {
			other := right.MapIndex(key)
			if !other.IsValid() || !equalValue(left.MapIndex(key), other) {
				return false
			}
		}
		return true
	case reflect.Slice, reflect.Array:
		if left.Len() != right.Len() {
			return false
		}
		for i := 0; i < left.Len(); i++ {
			if !equalValue(left.Index(i), right.Index(i)) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left.Interface(), right.Interface())
	}
}

func isNumber(kind reflect.Kind) bool {
	return (kind >= reflect.Int && kind <= reflect.Float64) || kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 || kind == reflect.Uint32 || kind == reflect.Uint64
}

func numberValue(value reflect.Value) (*big.Rat, bool) {
	switch value.Kind() {
	case reflect.Float32, reflect.Float64:
		v := value.Float()
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		return new(big.Rat).SetFloat64(v), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return new(big.Rat).SetInt64(value.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return new(big.Rat).SetUint64(value.Uint()), true
	default:
		return nil, false
	}
}

func isInteger(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

func valueString(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(b)
}

// WriteJSON writes an indented, timestamp-free report. The output is stable
// across runs because all source traversal and report slices are sorted.
func WriteJSON(path string, report Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func Summary(report Report) string {
	return fmt.Sprintf("jsonata conformance %s @ %s: groups=%d cases=%d enabled=%d passes=%d failures=%d skips=%d remaining-groups=%d remaining-cases=%d", ReferenceName, report.ReferenceCommit, len(report.Discovered), reportCaseCount(report), report.EnabledCases, report.Passes, len(report.Failures), len(report.Skips), len(report.RemainingGroups), len(report.RemainingCases))
}

func reportCaseCount(report Report) int {
	total := 0
	for _, group := range report.Discovered {
		total += group.Cases
	}
	return total
}

// Ensure deterministic output if callers construct a report manually.
func (r *Report) Sort() {
	sort.Strings(r.EnabledGroups)
	sort.Strings(r.RemainingGroups)
}
