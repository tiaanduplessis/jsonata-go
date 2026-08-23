package jlib_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestExportedSurfaceMatchesUpstreamV154 records the v1.5.4 export inventory
// so accidental compatibility drift is caught independently of behavior
// tests. Signatures are compiled by the package itself and remain copied from
// the same upstream source snapshot.
func TestExportedSurfaceMatchesUpstreamV154(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate jlib source directory")
	}
	checks := []struct {
		directory string
		want      []string
	}{
		{filepath.Dir(file), []string{
			"Append", "Average", "Base64Decode", "Base64Encode", "Boolean", "Contains", "Count",
			"DecodeURL", "Distinct", "Each", "EncodeURL", "EncodeURLComponent", "ErrNaNInf", "ErrType",
			"Error", "Exists", "Filter", "FormatBase", "FormatNumber", "FromMillis", "Join", "Keys",
			"Map", "Match", "Max", "Merge", "Min", "Not", "Number", "Pad", "Power", "Random",
			"Reduce", "Replace", "Reverse", "Round", "Shuffle", "Sift", "Single", "Sort", "Split",
			"Spread", "Sqrt", "String", "StringCallable", "StringNumberBool", "Substring", "SubstringAfter",
			"SubstringBefore", "Sum", "ToMillis", "Trim", "TypeOf", "Zip",
		}},
		{filepath.Join(filepath.Dir(file), "jxpath"), []string{"DecimalFormat", "FormatNumber", "FormatTime", "NewDecimalFormat"}},
	}
	for _, check := range checks {
		got := exportedNames(t, check.directory)
		sort.Strings(check.want)
		if len(got) != len(check.want) {
			t.Fatalf("%s exported count = %d, want %d; got %v", check.directory, len(got), len(check.want), got)
		}
		for i := range got {
			if got[i] != check.want[i] {
				t.Fatalf("%s exported surface = %v, want %v", check.directory, got, check.want)
			}
		}
	}
}

func exportedNames(t *testing.T, directory string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	set := make(map[string]struct{})
	for _, file := range files {
		if filepath.Ext(file) != ".go" || strings.HasSuffix(filepath.Base(file), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil && ast.IsExported(declaration.Name.Name) {
					set[declaration.Name.Name] = struct{}{}
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(spec.Name.Name) {
							set[spec.Name.Name] = struct{}{}
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if ast.IsExported(name.Name) {
								set[name.Name] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
	got := make([]string, 0, len(set))
	for name := range set {
		got = append(got, name)
	}
	sort.Strings(got)
	return got
}
