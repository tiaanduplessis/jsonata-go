package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

var ErrInvalidSuite = errors.New("invalid JSONata reference suite")

// LoadSuite verifies provenance and loads every group and case in root. It
// does not apply a feature manifest: unsupported material must remain visible
// to the caller.
func LoadSuite(root string) (Suite, error) {
	var suite Suite
	commitBytes, err := os.ReadFile(filepath.Join(root, "REFERENCE_COMMIT"))
	if err != nil {
		return suite, fmt.Errorf("%w: read REFERENCE_COMMIT: %v", ErrInvalidSuite, err)
	}
	commit := strings.TrimSpace(string(commitBytes))
	if commit != ReferenceCommit {
		return suite, fmt.Errorf("%w: reference commit %q does not match pinned %q", ErrInvalidSuite, commit, ReferenceCommit)
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return suite, fmt.Errorf("%w: read LICENSE: %v", ErrInvalidSuite, err)
	}
	if !strings.Contains(strings.ToLower(string(license)), "mit license") {
		return suite, fmt.Errorf("%w: LICENSE does not contain the upstream MIT notice", ErrInvalidSuite)
	}

	groupsRoot := filepath.Join(root, "test", "test-suite", "groups")
	entries, err := os.ReadDir(groupsRoot)
	if err != nil {
		return suite, fmt.Errorf("%w: read groups: %v", ErrInvalidSuite, err)
	}
	groupNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			groupNames = append(groupNames, entry.Name())
		}
	}
	sort.Strings(groupNames)
	suite = Suite{Root: root, ReferenceCommit: commit}
	for _, groupName := range groupNames {
		group, err := loadGroup(groupsRoot, groupName)
		if err != nil {
			return Suite{}, err
		}
		suite.Groups = append(suite.Groups, group)
	}
	return suite, nil
}

func loadGroup(groupsRoot, name string) (Group, error) {
	dir := filepath.Join(groupsRoot, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Group{}, fmt.Errorf("%w: read group %s: %v", ErrInvalidSuite, name, err)
	}
	files := make([]string, 0, len(entries))
	expressionFiles := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".jsonata")) {
			files = append(files, entry.Name())
			if strings.HasSuffix(entry.Name(), ".jsonata") {
				expressionFiles[entry.Name()] = struct{}{}
			}
		}
	}
	sort.Strings(files)
	group := Group{Name: name}
	usedExpressionFiles := make(map[string]struct{})
	for _, file := range files {
		path := filepath.Join(dir, file)
		rel, _ := filepath.Rel(filepath.Dir(groupsRoot), path)
		if strings.HasSuffix(file, ".jsonata") {
			continue
		}
		cases, references, err := loadJSONCasesWithReferences(path, name, filepath.ToSlash(rel))
		if err != nil {
			return Group{}, err
		}
		group.Cases = append(group.Cases, cases...)
		for reference := range references {
			usedExpressionFiles[reference] = struct{}{}
		}
	}
	for reference := range usedExpressionFiles {
		if _, ok := expressionFiles[reference]; !ok {
			return Group{}, fmt.Errorf("%w: group %s references missing expression file %s", ErrInvalidSuite, name, reference)
		}
	}
	for file := range expressionFiles {
		if _, ok := usedExpressionFiles[file]; !ok {
			return Group{}, fmt.Errorf("%w: group %s has unreferenced expression file %s", ErrInvalidSuite, name, file)
		}
	}
	seen := make(map[string]string, len(group.Cases))
	for _, testCase := range group.Cases {
		if previous, ok := seen[testCase.ID]; ok {
			return Group{}, fmt.Errorf("%w: group %s has duplicate semantic case ID %s in %s and %s", ErrInvalidSuite, name, testCase.ID, previous, testCase.Source)
		}
		seen[testCase.ID] = testCase.Source
	}
	sort.Slice(group.Cases, func(i, j int) bool {
		if group.Cases[i].ID != group.Cases[j].ID {
			return group.Cases[i].ID < group.Cases[j].ID
		}
		return group.Cases[i].Source < group.Cases[j].Source
	})
	return group, nil
}

func loadJSONCases(path, group, source string) ([]Case, error) {
	cases, _, err := loadJSONCasesWithReferences(path, group, source)
	return cases, err
}

func loadJSONCasesWithReferences(path, group, source string) ([]Case, map[string]struct{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: read %s: %v", ErrInvalidSuite, source, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("%w: decode %s: %v", ErrInvalidSuite, source, err)
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, nil, fmt.Errorf("%w: trailing JSON in %s", ErrInvalidSuite, source)
		}
		return nil, nil, fmt.Errorf("%w: decode trailing %s: %v", ErrInvalidSuite, source, err)
	}
	var values []json.RawMessage
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '[' {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, nil, fmt.Errorf("%w: decode cases in %s: %v", ErrInvalidSuite, source, err)
		}
	} else {
		values = []json.RawMessage{raw}
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("%w: %s contains no cases", ErrInvalidSuite, source)
	}
	cases := make([]Case, 0, len(values))
	expressionFiles := make(map[string]struct{})
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for i, value := range values {
		id := base
		if len(values) > 1 {
			id = fmt.Sprintf("%s#%03d", base, i)
		}
		caseValue, err := decodeCase(value, group, id, source, filepath.Dir(path))
		if err != nil {
			return nil, nil, err
		}
		cases = append(cases, caseValue)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(value, &fields); err == nil {
			if expressionFile, ok := fields["expr-file"]; ok {
				var reference string
				if err := json.Unmarshal(expressionFile, &reference); err == nil && reference != "" {
					expressionFiles[reference] = struct{}{}
				}
			}
		}
	}
	return cases, expressionFiles, nil
}

func decodeCase(raw json.RawMessage, group, id, source, groupDir string) (Case, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Case{}, fmt.Errorf("%w: %s/%s must be a JSON object", ErrInvalidSuite, group, id)
	}
	var c Case
	c.Group, c.ID, c.Source, c.SupportedInput = group, id, source, true
	if expression, ok := fields["expr"]; ok {
		if err := unmarshalExpression(expression, &c.Expression); err != nil || c.Expression == "" {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid expr", ErrInvalidSuite, group, id)
		}
	} else {
		var expressionFile string
		if fileErr := unmarshalRequired(fields, "expr-file", &expressionFile); fileErr != nil || expressionFile == "" {
			return Case{}, fmt.Errorf("%w: %s/%s requires a non-empty expr or expr-file", ErrInvalidSuite, group, id)
		}
		b, readErr := os.ReadFile(filepath.Join(groupDir, expressionFile))
		if readErr != nil {
			return Case{}, fmt.Errorf("%w: %s/%s expression file: %v", ErrInvalidSuite, group, id, readErr)
		}
		c.Expression = string(b)
	}
	if strings.TrimSpace(c.Expression) == "" {
		return Case{}, fmt.Errorf("%w: %s/%s requires a non-empty expr", ErrInvalidSuite, group, id)
	}
	dataRaw, hasData := fields["data"]
	datasetRaw, hasDataset := fields["dataset"]
	if hasData && hasDataset {
		return Case{}, fmt.Errorf("%w: %s/%s must not contain both data and dataset", ErrInvalidSuite, group, id)
	}
	if data, ok := dataRaw, hasData; ok {
		c.HasData = true
		decoded, err := value.DecodeJSON(data)
		if err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid data: %v", ErrInvalidSuite, group, id, err)
		}
		c.Data = decoded
	} else if dataset, ok := datasetRaw, hasDataset; ok {
		c.HasDataset = true
		if err := json.Unmarshal(dataset, &c.Dataset); err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid dataset: %v", ErrInvalidSuite, group, id, err)
		}
	} else {
		return Case{}, fmt.Errorf("%w: %s/%s requires data or dataset", ErrInvalidSuite, group, id)
	}
	if bindings, ok := fields["bindings"]; ok {
		if err := json.Unmarshal(bindings, &c.Bindings); err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid bindings: %v", ErrInvalidSuite, group, id, err)
		}
	}
	if unordered, ok := fields["unordered"]; ok {
		trimmed := bytes.TrimSpace(unordered)
		if !bytes.Equal(trimmed, []byte("true")) && !bytes.Equal(trimmed, []byte("false")) {
			return Case{}, fmt.Errorf("%w: %s/%s unordered must be a boolean", ErrInvalidSuite, group, id)
		}
		if err := json.Unmarshal(trimmed, &c.Unordered); err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid unordered: %v", ErrInvalidSuite, group, id, err)
		}
	}
	result, hasResult := fields["result"]
	undefined, hasUndefined := fields["undefinedResult"]
	code, hasCode := fields["code"]
	errorValue, hasError := fields["error"]
	expectedCount := 0
	if hasResult {
		expectedCount++
		c.ExpectedKind = ExpectedResult
		if err := json.Unmarshal(result, &c.Expected); err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid result: %v", ErrInvalidSuite, group, id, err)
		}
	}
	if hasUndefined {
		expectedCount++
		var flag bool
		if err := json.Unmarshal(undefined, &flag); err != nil || !flag {
			return Case{}, fmt.Errorf("%w: %s/%s undefinedResult must be true", ErrInvalidSuite, group, id)
		}
		c.ExpectedKind = ExpectedUndefined
	}
	if hasCode {
		expectedCount++
		if err := json.Unmarshal(code, &c.ExpectedCode); err != nil || c.ExpectedCode == "" {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid code", ErrInvalidSuite, group, id)
		}
		c.ExpectedKind = ExpectedError
	}
	if hasError {
		expectedCount++
		var expected struct {
			Code     string `json:"code"`
			Token    string `json:"token"`
			Position *int   `json:"position"`
		}
		if err := json.Unmarshal(errorValue, &expected); err != nil || expected.Code == "" {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid error", ErrInvalidSuite, group, id)
		}
		c.ExpectedKind, c.ExpectedCode, c.ExpectedToken = ExpectedError, expected.Code, expected.Token
		if expected.Position != nil {
			c.ExpectedPosition, c.HasExpectedPosition = *expected.Position, true
		}
	}
	if expectedCount != 1 {
		return Case{}, fmt.Errorf("%w: %s/%s must have exactly one expected result, undefinedResult, or error", ErrInvalidSuite, group, id)
	}
	if token, ok := fields["token"]; ok && c.ExpectedToken == "" {
		_ = json.Unmarshal(token, &c.ExpectedToken)
	}
	if position, ok := fields["position"]; ok {
		if err := json.Unmarshal(position, &c.ExpectedPosition); err != nil {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid position: %v", ErrInvalidSuite, group, id, err)
		}
		c.HasExpectedPosition = true
	}
	if timelimit, ok := fields["timelimit"]; ok {
		if err := json.Unmarshal(timelimit, &c.TimeLimit); err != nil || c.TimeLimit < 0 {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid timelimit", ErrInvalidSuite, group, id)
		}
	}
	if depth, ok := fields["depth"]; ok {
		if err := json.Unmarshal(depth, &c.Depth); err != nil || c.Depth < 0 {
			return Case{}, fmt.Errorf("%w: %s/%s has invalid depth", ErrInvalidSuite, group, id)
		}
	}
	return c, nil
}

// unmarshalExpression keeps UTF-16 surrogate escapes in expression source so
// the JSONata parser can classify them. encoding/json otherwise replaces an
// unpaired surrogate with U+FFFD before the language parser sees it.
func unmarshalExpression(raw json.RawMessage, destination *string) error {
	preserved := make([]byte, 0, len(raw)+8)
	for index := 0; index < len(raw); index++ {
		if raw[index] != '\\' || index+1 >= len(raw) {
			preserved = append(preserved, raw[index])
			continue
		}
		if raw[index+1] == '\\' {
			preserved = append(preserved, raw[index], raw[index+1])
			index++
			continue
		}
		if raw[index+1] == 'u' && index+5 < len(raw) {
			code, err := strconv.ParseUint(string(raw[index+2:index+6]), 16, 16)
			if err == nil && code >= 0xd800 && code <= 0xdfff {
				preserved = append(preserved, '\\', '\\')
				preserved = append(preserved, raw[index+1:index+6]...)
				index += 5
				continue
			}
		}
		preserved = append(preserved, raw[index])
	}
	return json.Unmarshal(preserved, destination)
}

func unmarshalRequired(fields map[string]json.RawMessage, key string, target *string) error {
	raw, ok := fields[key]
	if !ok {
		return errors.New("missing field")
	}
	return json.Unmarshal(raw, target)
}
