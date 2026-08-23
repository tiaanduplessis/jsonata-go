package syntax

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestPhase3FixtureSyntax(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "reference", "jsonata-js-v2.2.2", "test", "test-suite", "groups")
	groups := []string{
		"blocks", "closures", "higher-order-functions", "lambdas", "partial-application",
		"tail-recursion", "variables", "function-applications", "function-eval", "function-signatures",
	}
	for _, group := range groups {
		files, err := filepath.Glob(filepath.Join(root, group, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var cases []struct {
				Expr string `json:"expr"`
				Code string `json:"code"`
			}
			if err := json.Unmarshal(data, &cases); err != nil {
				var single struct {
					Expr string `json:"expr"`
					Code string `json:"code"`
				}
				if singleErr := json.Unmarshal(data, &single); singleErr != nil || single.Expr == "" {
					t.Fatal(err)
				}
				cases = []struct {
					Expr string `json:"expr"`
					Code string `json:"code"`
				}{single}
			}
			for index, fixture := range cases {
				fixture := fixture
				t.Run(filepath.Join(group, filepath.Base(file), strconv.Itoa(index)), func(t *testing.T) {
					_, parseErr := Parse(fixture.Expr)
					if len(fixture.Code) > 0 && fixture.Code[0] == 'S' {
						if parseErr == nil || parseErr.Code != fixture.Code {
							t.Fatalf("Parse(%q): got %#v, want syntax error %s", fixture.Expr, parseErr, fixture.Code)
						}
						return
					}
					if parseErr != nil {
						t.Fatalf("Parse(%q): %s at %d token=%q value=%q", fixture.Expr, parseErr.Code, parseErr.Position, parseErr.Token, parseErr.Value)
					}
				})
			}
		}
	}
}

func TestPhase3ExplicitNodes(t *testing.T) {
	checks := []struct {
		expr  string
		check func(t *testing.T, node Node)
	}{
		{`$foo`, func(t *testing.T, node Node) {
			variable, ok := node.(Variable)
			if !ok || variable.Kind != VariableNamed || variable.Name != "$foo" {
				t.Fatalf("got %#v, want named variable", node)
			}
		}},
		{`$`, func(t *testing.T, node Node) {
			variable, ok := node.(Variable)
			if !ok || variable.Kind != VariableFocus {
				t.Fatalf("got %#v, want focus variable", node)
			}
		}},
		{`$$`, func(t *testing.T, node Node) {
			variable, ok := node.(Variable)
			if !ok || variable.Kind != VariableRoot {
				t.Fatalf("got %#v, want root variable", node)
			}
		}},
		{`($a := 1; $a)`, func(t *testing.T, node Node) {
			block, ok := node.(Block)
			if !ok || len(block.Expressions) != 2 {
				t.Fatalf("got %#v, want two-expression block", node)
			}
			if _, ok := block.Expressions[0].(Bind); !ok {
				t.Fatalf("got %#v, want bind", block.Expressions[0])
			}
		}},
		{`function($x){$x}(1)`, func(t *testing.T, node Node) {
			call, ok := node.(Call)
			if !ok {
				t.Fatalf("got %#v, want call", node)
			}
			if _, ok := call.Function.(Lambda); !ok {
				t.Fatalf("got %#v, want lambda procedure", call.Function)
			}
		}},
		{`f(?, 1)`, func(t *testing.T, node Node) {
			call, ok := node.(Call)
			if !ok || !call.Partial {
				t.Fatalf("got %#v, want partial call", node)
			}
			if _, ok := call.Args[0].(Placeholder); !ok {
				t.Fatalf("got %#v, want placeholder", call.Args[0])
			}
		}},
		{`1 ~> f`, func(t *testing.T, node Node) {
			if _, ok := node.(Apply); !ok {
				t.Fatalf("got %#v, want apply", node)
			}
		}},
		{`1 ~> f() = 2`, func(t *testing.T, node Node) {
			equality, ok := node.(Binary)
			if !ok || equality.Op != "=" {
				t.Fatalf("got %#v, want equality", node)
			}
			if _, ok := equality.Left.(Apply); !ok {
				t.Fatalf("got %#v, want application on equality left", equality.Left)
			}
		}},
	}
	for _, check := range checks {
		t.Run(check.expr, func(t *testing.T) {
			node, err := Parse(check.expr)
			if err != nil {
				t.Fatal(err)
			}
			check.check(t, node)
		})
	}
}

func TestPhase3SyntaxDiagnostics(t *testing.T) {
	tests := []struct {
		expr, code, token, value string
		position                 int
	}{
		{`function(x){$x}(3)`, "S0208", "x", "1", 10},
		{`x:=1`, "S0212", "x", "", 1},
		{`2:=1`, "S0212", "2", "", 1},
		{`($a := [1,2]; $a[1]:=3; $a)`, "S0212", "[", "", 17},
		{`λ($arg)<n<n>>{$arg}(5)`, "S0401", "", "n", 10},
		{`λ($arg)<(sa<n>)>>{$arg}([[1]])`, "S0402", "", "sa<n>", 9},
	}
	for _, test := range tests {
		t.Run(test.code+"/"+test.expr, func(t *testing.T) {
			_, err := Parse(test.expr)
			if err == nil {
				t.Fatal("expected parse error")
			}
			if err.Code != test.code || err.Token != test.token || err.Value != test.value || err.Position != test.position {
				t.Fatalf("got code=%s token=%q value=%q position=%d, want code=%s token=%q value=%q position=%d", err.Code, err.Token, err.Value, err.Position, test.code, test.token, test.value, test.position)
			}
		})
	}
}
