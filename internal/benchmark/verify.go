package benchmark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Compiled is the part of the public expression API needed by the Phase 1
// benchmark. Expressions must be safe for repeated and parallel calls.
type Compiled interface {
	Eval(any) (any, error)
	EvalBytes([]byte) (any, error)
}

// Runtime adapts the public package to the benchmark harness. The error
// normalizer is optional; DefaultNormalizeError handles ordinary structured
// JSONata errors and is useful for the command-line verifier.
type Runtime struct {
	ID             string
	Compile        func(string) (Compiled, error)
	CompileCase    func(Case) (Compiled, error)
	Unsupported    func(Case, Mode) string
	NormalizeError func(error) OracleError
}

func (r Runtime) compile(c Case) (Compiled, error) {
	if r.CompileCase != nil {
		return r.CompileCase(c)
	}
	if r.Compile == nil {
		return nil, fmt.Errorf("benchmark runtime has no compiler")
	}
	return r.Compile(c.Expression)
}

// UnsupportedReason explains a known API or concurrency limitation before
// invoking an implementation. Empty means the operation must pass the dynamic
// correctness gate before it may be timed.
func (r Runtime) UnsupportedReason(c Case, mode Mode) string {
	if r.Unsupported == nil {
		return ""
	}
	return r.Unsupported(c, mode)
}

func (r Runtime) normalizeError(err error) OracleError {
	if r.NormalizeError != nil {
		return r.NormalizeError(err)
	}
	return DefaultNormalizeError(err)
}

// VerificationError explains why a sample was not eligible for timing.
type VerificationError struct {
	CaseID string
	Mode   Mode
	Reason string
}

func (e *VerificationError) Error() string {
	return fmt.Sprintf("benchmark correctness gate rejected case %q (%s): %s", e.CaseID, e.Mode, e.Reason)
}

// Compare compares an evaluator result with the pinned oracle. It is exported
// so tests and alternate benchmark front ends can prove that a bad value or a
// bad structured error is rejected before starting a timer.
func Compare(actual any, actualErr error, expected Oracle, normalize func(error) OracleError) error {
	if expected.Error != nil {
		if actualErr == nil {
			return fmt.Errorf("expected structured error %+v, got value %s", *expected.Error, canonical(actual))
		}
		actualError := DefaultNormalizeError(actualErr)
		if normalize != nil {
			actualError = normalize(actualErr)
		}
		if mismatch := compareErrors(*expected.Error, actualError); mismatch != "" {
			return fmt.Errorf("structured error mismatch: %s", mismatch)
		}
		return nil
	}
	if actualErr != nil {
		actualError := DefaultNormalizeError(actualErr)
		if normalize != nil {
			actualError = normalize(actualErr)
		}
		return fmt.Errorf("expected value %s, got structured error %+v", string(expected.Value), actualError)
	}
	want := canonicalJSON(expected.Value)
	got := canonical(actual)
	if !bytes.Equal(want, got) {
		return fmt.Errorf("value mismatch: expected %s, got %s", want, got)
	}
	return nil
}

func compareErrors(want, got OracleError) string {
	if want.Code != "" && want.Code != got.Code {
		return fmt.Sprintf("code expected %q, got %q", want.Code, got.Code)
	}
	if want.Message != "" && want.Message != got.Message {
		return fmt.Sprintf("message expected %q, got %q", want.Message, got.Message)
	}
	if want.Token != "" && want.Token != got.Token {
		return fmt.Sprintf("token expected %q, got %q", want.Token, got.Token)
	}
	if want.Value != "" && want.Value != got.Value {
		return fmt.Sprintf("value expected %q, got %q", want.Value, got.Value)
	}
	if want.Position != 0 && want.Position != got.Position {
		return fmt.Sprintf("position expected %d, got %d", want.Position, got.Position)
	}
	return ""
}

// VerifyCase compiles and evaluates a case in the selected representation.
// Callers must invoke it with timers stopped; a non-nil error means timing is
// forbidden for that sample.
func VerifyCase(runtime Runtime, c Case, mode Mode) (Compiled, error) {
	if reason := runtime.UnsupportedReason(c, mode); reason != "" {
		return nil, &VerificationError{CaseID: c.ID, Mode: mode, Reason: reason}
	}
	expr, err := runtime.compile(c)
	if err != nil {
		return nil, &VerificationError{CaseID: c.ID, Mode: mode, Reason: "compile failed: " + err.Error()}
	}
	if mode == ModeCompile {
		mode = firstEvaluationMode(c)
		if mode == "" {
			return nil, &VerificationError{CaseID: c.ID, Mode: ModeCompile, Reason: "case has no evaluation mode for the compile correctness gate"}
		}
	}
	inputBytes := []byte(c.Input)
	var input any
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return nil, &VerificationError{CaseID: c.ID, Mode: mode, Reason: "decode failed: " + err.Error()}
	}
	var value any
	var evalErr error
	switch mode {
	case ModeDecoded:
		value, evalErr = expr.Eval(input)
	case ModeBytes:
		value, evalErr = expr.EvalBytes(inputBytes)
		if evalErr == nil {
			switch output := value.(type) {
			case []byte:
				value, evalErr = decodeJSONResult(output)
			case json.RawMessage:
				value, evalErr = decodeJSONResult(output)
			}
		}
	case ModeParallel:
		return verifyParallel(runtime, c, expr, input)
	default:
		return nil, &VerificationError{CaseID: c.ID, Mode: mode, Reason: "unknown mode"}
	}
	if err := Compare(value, evalErr, c.Oracle, runtime.normalizeError); err != nil {
		return nil, &VerificationError{CaseID: c.ID, Mode: mode, Reason: err.Error()}
	}
	return expr, nil
}

func firstEvaluationMode(c Case) Mode {
	for _, mode := range []Mode{ModeDecoded, ModeBytes, ModeParallel} {
		if HasMode(c, mode) {
			return mode
		}
	}
	return ""
}

func verifyParallel(runtime Runtime, c Case, expr Compiled, input any) (Compiled, error) {
	const workers = 8
	const evaluations = 16
	var wait sync.WaitGroup
	errorsByWorker := make(chan error, workers)
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range evaluations {
				value, err := expr.Eval(input)
				if compareErr := Compare(value, err, c.Oracle, runtime.normalizeError); compareErr != nil {
					errorsByWorker <- compareErr
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errorsByWorker)
	if err := <-errorsByWorker; err != nil {
		return nil, &VerificationError{CaseID: c.ID, Mode: ModeParallel, Reason: err.Error()}
	}
	return expr, nil
}

func decodeJSONResult(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode evaluator result: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, fmt.Errorf("decode evaluator result: trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode evaluator result: %w", err)
	}
	return value, nil
}

func canonicalJSON(data []byte) []byte {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&value); err != nil {
		return bytes.TrimSpace(data)
	}
	return canonical(value)
}

func canonical(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte(fmt.Sprintf("<unmarshalable:%v>", err))
	}
	return data
}

// DefaultNormalizeError extracts the stable fields used by JSONata errors.
// It accepts both JSON-marshaling error structs and ordinary errors, allowing
// the benchmark package to remain independent from the evaluator's concrete
// error type.
func DefaultNormalizeError(err error) OracleError {
	if err == nil {
		return OracleError{}
	}
	var shaped interface{ JSONataError() OracleError }
	if errors.As(err, &shaped) {
		return shaped.JSONataError()
	}
	var marshaler json.Marshaler
	if errors.As(err, &marshaler) {
		if shape, ok := decodeErrorShape(marshaler); ok {
			return shape
		}
	}
	if shape, ok := decodeErrorShape(err); ok {
		return shape
	}
	return OracleError{Message: err.Error()}
}

func decodeErrorShape(value any) (OracleError, bool) {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || data[0] != '{' {
		return OracleError{}, false
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return OracleError{}, false
	}
	shape := OracleError{}
	shape.Code = stringField(fields, "code", "Code")
	shape.Message = stringField(fields, "message", "Message", "error")
	shape.Token = stringField(fields, "token", "Token")
	shape.Value = stringField(fields, "value", "Value")
	shape.Position = intField(fields, "position", "Position")
	if shape.Code == "" && shape.Message == "" && shape.Token == "" && shape.Value == "" && shape.Position == 0 {
		return OracleError{}, false
	}
	return shape, true
}

func stringField(fields map[string]any, names ...string) string {
	for _, name := range names {
		for key, value := range fields {
			if strings.EqualFold(key, name) {
				if text, ok := value.(string); ok {
					return text
				}
			}
		}
	}
	return ""
}

func intField(fields map[string]any, names ...string) int {
	for _, name := range names {
		for key, value := range fields {
			if strings.EqualFold(key, name) {
				switch number := value.(type) {
				case float64:
					return int(number)
				case json.Number:
					var result int
					if _, err := fmt.Sscan(number.String(), &result); err == nil {
						return result
					}
				}
			}
		}
	}
	return 0
}
