package differential

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ShapeCorpusSeed identifies the shape campaign. The campaign is an
// enumerated grid, so the seed is a fixture label rather than a random seed.
const ShapeCorpusSeed = uint64(0x5ba9e)

// GenerateShapes returns the shape campaign. It crosses value shapes with
// path step constructs and with the contexts that consume a path result, and
// it crosses every deterministic standard function with argument sources. A
// value then reaches a function or an operator through each path construct,
// not only as a literal.
func GenerateShapes() (Corpus, error) {
	var cases []Case
	add := func(family, expression string, input map[string]any) error {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode shape case %d input: %w", len(cases), err)
		}
		cases = append(cases, Case{
			ID:         fmt.Sprintf("shape-%04d", len(cases)),
			Family:     family,
			Expression: expression,
			HasInput:   true,
			Input:      encoded,
		})
		return nil
	}
	for _, path := range shapePaths {
		for _, shape := range shapeValues {
			expression, input := path.build(shape.value, shape.present)
			addDecoys(input)
			for _, consumer := range shapeConsumers {
				family := "path-" + path.name + "-" + shape.name + "-" + consumer.name
				if err := add(family, consumer.wrap(expression), input); err != nil {
					return Corpus{}, err
				}
			}
		}
	}
	for _, call := range shapeCalls {
		for position, argument := range call.args {
			if argument.source != "" {
				continue
			}
			for _, source := range shapeSources {
				if source.name == "literal" && position > 0 {
					continue
				}
				if source.name == "missing" && undefinedArgumentCrashes[fmt.Sprintf("%s/%d", call.name, position)] {
					continue
				}
				expression, input := source.build(argument.value, position)
				parts := make([]string, len(call.args))
				for index, other := range call.args {
					switch {
					case index == position:
						parts[index] = expression
					case other.source != "":
						parts[index] = other.source
					default:
						parts[index] = literal(other.value)
					}
				}
				family := fmt.Sprintf("argument-%s-%d-%s", call.name, position, source.name)
				if err := add(family, call.render(parts), input); err != nil {
					return Corpus{}, err
				}
			}
		}
	}
	for _, ancestor := range ancestorCases {
		if err := add("ancestor", ancestor.expression, ancestor.input); err != nil {
			return Corpus{}, err
		}
	}
	for _, scope := range scopeCases {
		if err := add("scope", scope, map[string]any{"x": 1}); err != nil {
			return Corpus{}, err
		}
	}
	return Corpus{
		SchemaVersion:   1,
		ReferenceName:   ReferenceName,
		ReferenceCommit: ReferenceCommit,
		Seed:            ShapeCorpusSeed,
		Cases:           cases,
	}, nil
}

// DecodeShapeCorpus validates the shape campaign fixture.
func DecodeShapeCorpus(data []byte, size int) (Corpus, error) {
	return decodeCorpus(data, ShapeCorpusSeed, size)
}

// DecodeShapeOracle validates the oracle generated for the shape campaign.
func DecodeShapeOracle(data []byte, size int) (Oracle, error) {
	return decodeOracle(data, size)
}

type shapeValue struct {
	name    string
	value   any
	present bool
}

var shapeValues = []shapeValue{
	{name: "missing"},
	{name: "null", value: nil, present: true},
	{name: "string", value: "s", present: true},
	{name: "empty-string", value: "", present: true},
	{name: "zero", value: 0, present: true},
	{name: "number", value: 1.5, present: true},
	{name: "boolean", value: true, present: true},
	{name: "empty-array", value: []any{}, present: true},
	{name: "singleton", value: []any{"x"}, present: true},
	{name: "pair", value: []any{"x", "y"}, present: true},
	{name: "nested-array", value: []any{[]any{1}, []any{2}}, present: true},
	{name: "object", value: map[string]any{"k": "v"}, present: true},
	{name: "object-singleton", value: []any{map[string]any{"k": "v"}}, present: true},
	{name: "object-pair", value: []any{map[string]any{"k": "v"}, map[string]any{"k": "w"}}, present: true},
}

type shapePath struct {
	name       string
	expression string
	input      func(field map[string]any) map[string]any
}

func (p shapePath) build(v any, present bool) (string, map[string]any) {
	field := map[string]any{}
	if present {
		field["p"] = v
	}
	return p.expression, p.input(field)
}

func under(key string, v any) map[string]any { return map[string]any{key: v} }

// addDecoys gives the root the field names that a path selects below the root.
// A path step must never fall back to a root field of the same name.
func addDecoys(input map[string]any) {
	for _, key := range []string{"p", "k", "b", "q", "z"} {
		if _, exists := input[key]; !exists {
			input[key] = "root " + key
		}
	}
}

func withSibling(field map[string]any, key string, v any) map[string]any {
	out := map[string]any{key: v}
	for name, item := range field {
		out[name] = item
	}
	return out
}

var shapePaths = []shapePath{
	{name: "field", expression: "p", input: func(f map[string]any) map[string]any { return f }},
	{name: "path2", expression: "a.p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "path3", expression: "a.b.p", input: func(f map[string]any) map[string]any { return under("a", under("b", f)) }},
	{name: "array1", expression: "a.p", input: func(f map[string]any) map[string]any { return under("a", []any{f}) }},
	{name: "array2", expression: "a.p", input: func(f map[string]any) map[string]any { return under("a", []any{f, f}) }},
	{name: "array-mixed", expression: "a.p", input: func(f map[string]any) map[string]any {
		return under("a", []any{f, map[string]any{"q": 1}})
	}},
	{name: "index", expression: "a[0].p", input: func(f map[string]any) map[string]any { return under("a", []any{f}) }},
	{name: "predicate", expression: "a[true].p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "block", expression: "a.(p)", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "wildcard", expression: "a.*", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "descendant", expression: "a.**.p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "keep-array", expression: "a.p[]", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "context", expression: "a.p.$", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "first", expression: "a.p[0]", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "last", expression: "a.p[-1]", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "value-filter", expression: `a.p[$ = "x"]`, input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "sort", expression: "a.p^($)", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "middle", expression: "a.p.k", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "middle-block", expression: "a.p.(k)", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "array-constructor", expression: "a.[p]", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "object-constructor", expression: `a.{"r": p}`, input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "focus", expression: "a@$o.$o.p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "focus-context", expression: "a@$o.p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "position", expression: "a#$i.p", input: func(f map[string]any) map[string]any { return under("a", f) }},
	{name: "parent", expression: "a.b.%.p", input: func(f map[string]any) map[string]any {
		return under("a", withSibling(f, "b", map[string]any{"z": 1}))
	}},
	{name: "parent-block", expression: "a.b.(%.p)", input: func(f map[string]any) map[string]any {
		return under("a", withSibling(f, "b", map[string]any{"z": 1}))
	}},
}

// ancestorCases put a field on an ancestor that a step below it does not have.
// A step must not find that field through a binding, a position, or %.
var ancestorCases = []struct {
	expression string
	input      map[string]any
}{
	{"x.y#$i.p", map[string]any{"x": map[string]any{"p": "decoy", "y": []any{map[string]any{}}}}},
	{"x.y@$o.$o.p", map[string]any{"x": map[string]any{"p": "decoy", "y": []any{map[string]any{}}}}},
	{"x.y@$o.z.p", map[string]any{"x": map[string]any{"p": "decoy", "z": map[string]any{}, "y": []any{map[string]any{}}}}},
	{"x.y@$o[k = 1]", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"x.y@$o[$o.k = 2].$o", map[string]any{"x": map[string]any{"y": []any{map[string]any{"k": 2}, map[string]any{"k": 3}}}}},
	{"x.y.%.z.p", map[string]any{"x": map[string]any{"p": "decoy", "y": map[string]any{}, "z": map[string]any{}}}},
	{"x.y@$o", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}, map[string]any{"k": 3}}}}},
	{"y@$o", map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}},
	{"x.y@$o#$i", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}, map[string]any{"k": 3}}}}},
	{"x.(y@$o)", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"x.y@$o^($o.k)", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 3}, map[string]any{"k": 2}}}}},
	{"x.y@$o^($o.k).$o.k", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 3}, map[string]any{"k": 2}}}}},
	{"a#$i^(d).p", map[string]any{"a": []any{map[string]any{"d": 1, "p": []any{1}}}}},
	{"a.(%)", map[string]any{"a": 1}},
	{"a.$$@$x", map[string]any{"a": map[string]any{"d": 1}, "d": 2}},
	{"a.($$@$x)", map[string]any{"a": map[string]any{"d": 1}, "d": 2}},
	{"(x.y@$o).$o", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"(x.y#$i).$i", map[string]any{"x": map[string]any{"y": []any{map[string]any{"k": 2}}}}},
	{"(x.y#$i).k", map[string]any{"x": map[string]any{"y": []any{map[string]any{"k": 2}}}}},
	{"a#$i.($#$i[]).$i", map[string]any{"a": []any{10, 20}}},
	{"(x.y@$o).%", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"(x.y@$o).%.%", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"x.y@$o.%", map[string]any{"x": map[string]any{"k": 1, "y": []any{map[string]any{"k": 2}}}}},
	{"x.loans@$l.books@$b[$l.id = $b.id].missing", map[string]any{"x": map[string]any{
		"missing": "decoy", "loans": []any{map[string]any{"id": 1}}, "books": []any{map[string]any{"id": 1}},
	}}},
	{"x.loans@$l#$i.books@$b[$l.id = $b.id].{\"i\": $i, \"keys\": $keys(%)}", map[string]any{"x": map[string]any{
		"loans": []any{map[string]any{"id": 1}, map[string]any{"id": 2}}, "books": []any{map[string]any{"id": 2}},
	}}},
}

// scopeCases bind a variable in a block. Until its binding runs, the name refers
// to the enclosing scope or to the standard function of that name.
var scopeCases = []string{
	`($string := $string(1); $string)`,
	`($f := function(){ ($uppercase := $uppercase("a"); $uppercase) }; $f())`,
	`($y := 1; ($z := $y; $y := 2; $z))`,
	`($f := function(){ $g() }; $g := function(){ 7 }; $f())`,
	`($x := x; $x)`,
	`($y := 1; ($y := $y + 1; $y))`,
}

type shapeConsumer struct {
	name string
	wrap func(string) string
}

func call(name string) func(string) string {
	return func(expression string) string { return "$" + name + "(" + expression + ")" }
}

var shapeConsumers = []shapeConsumer{
	{name: "result", wrap: func(e string) string { return e }},
	{name: "object", wrap: func(e string) string { return `{"r": ` + e + `}` }},
	{name: "string", wrap: call("string")},
	{name: "type", wrap: call("type")},
	{name: "count", wrap: call("count")},
	{name: "exists", wrap: call("exists")},
	{name: "lambda", wrap: func(e string) string { return "(function($v){$v})(" + e + ")" }},
}

type shapeArgument struct {
	value  any
	source string
}

type shapeCall struct {
	name     string
	template string
	args     []shapeArgument
}

func (c shapeCall) render(parts []string) string {
	if c.template == "" {
		return "$" + c.name + "(" + strings.Join(parts, ", ") + ")"
	}
	replacements := make([]string, 0, 2*len(parts))
	for index, part := range parts {
		replacements = append(replacements, fmt.Sprintf("{%d}", index), part)
	}
	return strings.NewReplacer(replacements...).Replace(c.template)
}

func v(value any) shapeArgument       { return shapeArgument{value: value} }
func src(source string) shapeArgument { return shapeArgument{source: source} }

var shapeCalls = []shapeCall{
	{name: "abs", args: []shapeArgument{v(-3.5)}},
	{name: "append", args: []shapeArgument{v([]any{1, 2}), v([]any{3})}},
	{name: "append", args: []shapeArgument{v("a"), v("b")}},
	{name: "append", args: []shapeArgument{v("a"), v([]any{})}},
	{name: "append", args: []shapeArgument{v([]any{1}), v([]any{})}},
	{name: "assert", args: []shapeArgument{v(true), v("m")}},
	{name: "assert", args: []shapeArgument{v(false), v("assert message")}},
	{name: "average", args: []shapeArgument{v([]any{1, 2, 3})}},
	{name: "base64decode", args: []shapeArgument{v("aGVsbG8=")}},
	{name: "base64encode", args: []shapeArgument{v("hello")}},
	{name: "boolean", args: []shapeArgument{v("x")}},
	{name: "boolean", args: []shapeArgument{v("")}},
	{name: "boolean", args: []shapeArgument{v(0)}},
	{name: "boolean", args: []shapeArgument{v([]any{})}},
	{name: "boolean", args: []shapeArgument{v(map[string]any{"a": 1})}},
	{name: "ceil", args: []shapeArgument{v(2.1)}},
	{name: "contains", args: []shapeArgument{v("hello"), v("ell")}},
	{name: "contains", args: []shapeArgument{v("hello"), src("/l+/")}},
	{name: "count", args: []shapeArgument{v([]any{1, 2})}},
	{name: "count", args: []shapeArgument{v("x")}},
	{name: "decodeUrl", args: []shapeArgument{v("a%20b")}},
	{name: "decodeUrlComponent", args: []shapeArgument{v("a%20b")}},
	{name: "distinct", args: []shapeArgument{v([]any{1, 1, 2})}},
	{name: "distinct", args: []shapeArgument{v([]any{"a", "a"})}},
	{name: "each", args: []shapeArgument{v(map[string]any{"a": 1, "b": 2}), src("function($v,$k){$k & $v}")}},
	{name: "each", args: []shapeArgument{v(map[string]any{"a": "x"}), src("function($v){$uppercase($v)}")}},
	{name: "encodeUrl", args: []shapeArgument{v("a b")}},
	{name: "encodeUrlComponent", args: []shapeArgument{v("a b/c")}},
	{name: "error", args: []shapeArgument{v("boom")}},
	{name: "exists", args: []shapeArgument{v("x")}},
	{name: "exists", args: []shapeArgument{v(nil)}},
	{name: "filter", args: []shapeArgument{v([]any{1, 2, 3}), src("function($v){$v>1}")}},
	{name: "floor", args: []shapeArgument{v(2.7)}},
	{name: "formatBase", args: []shapeArgument{v(255), v(16)}},
	{name: "formatInteger", args: []shapeArgument{v(42), v("w")}},
	{name: "formatNumber", args: []shapeArgument{v(1234.5), v("#,##0.00")}},
	{name: "formatNumber", args: []shapeArgument{v(0.5), v("0%")}},
	{name: "fromMillis", args: []shapeArgument{v(1510067557121)}},
	{name: "fromMillis", args: []shapeArgument{v(0), v("[Y]-[M01]-[D01]")}},
	{name: "join", args: []shapeArgument{v([]any{"a", "b"}), v("-")}},
	{name: "join", args: []shapeArgument{v("a")}},
	{name: "keys", args: []shapeArgument{v(map[string]any{"a": 1, "b": 2})}},
	{name: "length", args: []shapeArgument{v("héllo")}},
	{name: "lowercase", args: []shapeArgument{v("ABC")}},
	{name: "lookup", args: []shapeArgument{v(map[string]any{"a": 1}), v("a")}},
	{name: "lookup", args: []shapeArgument{v(map[string]any{"a": nil}), v("a")}},
	{name: "map", args: []shapeArgument{v([]any{1, 2}), src("function($v){$v*2}")}},
	{name: "map", args: []shapeArgument{v([]any{"a", "b"}), src("function($v){$uppercase($v)}")}},
	{name: "match", args: []shapeArgument{v("abcb"), src("/b/")}},
	{name: "max", args: []shapeArgument{v([]any{1, 3})}},
	{name: "merge", args: []shapeArgument{v([]any{map[string]any{"a": 1}, map[string]any{"b": 2}})}},
	{name: "min", args: []shapeArgument{v([]any{1, 3})}},
	{name: "not", args: []shapeArgument{v(true)}},
	{name: "not", args: []shapeArgument{v("x")}},
	{name: "number", args: []shapeArgument{v("12")}},
	{name: "number", args: []shapeArgument{v(true)}},
	{name: "pad", args: []shapeArgument{v("x"), v(5), v("#")}},
	{name: "pad", args: []shapeArgument{v("x"), v(-3)}},
	{name: "parseInteger", args: []shapeArgument{v("twelve"), v("w")}},
	{name: "power", args: []shapeArgument{v(2), v(3)}},
	{name: "reduce", args: []shapeArgument{v([]any{1, 2, 3}), src("function($a,$b){$a+$b}")}},
	{name: "replace", args: []shapeArgument{v("hello"), v("l"), v("L")}},
	{name: "replace", args: []shapeArgument{v("hello"), src("/l/"), v("L")}},
	{name: "reverse", args: []shapeArgument{v([]any{1, 2})}},
	{name: "round", args: []shapeArgument{v(2.567), v(2)}},
	{name: "sift", args: []shapeArgument{v(map[string]any{"a": 1, "b": 2}), src("function($v){$v>1}")}},
	{name: "single", args: []shapeArgument{v([]any{1, 2, 3}), src("function($v){$v=2}")}},
	{name: "sort", args: []shapeArgument{v([]any{3, 1, 2})}},
	{name: "sort", args: []shapeArgument{v([]any{"b", "a"})}},
	{name: "split", args: []shapeArgument{v("a,b,c"), v(",")}},
	{name: "split", args: []shapeArgument{v("a1b2c"), src("/[0-9]/")}},
	{name: "spread", args: []shapeArgument{v(map[string]any{"a": 1, "b": 2})}},
	{name: "sqrt", args: []shapeArgument{v(16)}},
	{name: "string", args: []shapeArgument{v("abc")}},
	{name: "string", args: []shapeArgument{v(12)}},
	{name: "string", args: []shapeArgument{v(map[string]any{"a": 1})}},
	{name: "string", args: []shapeArgument{v(true)}},
	{name: "string", args: []shapeArgument{v(nil)}},
	{name: "string", args: []shapeArgument{v([]any{"x", 1})}},
	{name: "string", args: []shapeArgument{v(map[string]any{"a": "b"}), v(true)}},
	{name: "substring", args: []shapeArgument{v("hello"), v(1), v(3)}},
	{name: "substringAfter", args: []shapeArgument{v("a-b"), v("-")}},
	{name: "substringBefore", args: []shapeArgument{v("a-b"), v("-")}},
	{name: "sum", args: []shapeArgument{v([]any{1, 2})}},
	{name: "toMillis", args: []shapeArgument{v("2017-11-07T15:07:37.121Z")}},
	{name: "toMillis", args: []shapeArgument{v("2017-11-07"), v("[Y]-[M]-[D]")}},
	{name: "trim", args: []shapeArgument{v("  a  b ")}},
	{name: "type", args: []shapeArgument{v("x")}},
	{name: "type", args: []shapeArgument{v(nil)}},
	{name: "type", args: []shapeArgument{v(1)}},
	{name: "type", args: []shapeArgument{v(map[string]any{"a": 1})}},
	{name: "type", args: []shapeArgument{v([]any{1})}},
	{name: "uppercase", args: []shapeArgument{v("abc")}},
	{name: "zip", args: []shapeArgument{v([]any{1, 2}), v([]any{3, 4})}},
	{name: "eval", args: []shapeArgument{v("1+1")}},
	{name: "eval", args: []shapeArgument{v("a"), v(map[string]any{"a": 5})}},
	{name: "equal", template: "{0} = {1}", args: []shapeArgument{v("abc"), v("abc")}},
	{name: "not-equal", template: "{0} != {1}", args: []shapeArgument{v("abc"), v("abd")}},
	{name: "concatenate", template: "{0} & {1}", args: []shapeArgument{v("ab"), v("cd")}},
	{name: "add", template: "{0} + {1}", args: []shapeArgument{v(1), v(2)}},
	{name: "less", template: "{0} < {1}", args: []shapeArgument{v("a"), v("b")}},
	{name: "in", template: "{0} in {1}", args: []shapeArgument{v("a"), v([]any{"a", "b"})}},
	{name: "condition", template: `{0} ? "y" : "n"`, args: []shapeArgument{v("x")}},
	{name: "object-value", template: `{"k": {0}}`, args: []shapeArgument{v("v")}},
	{name: "object-key", template: "{{0}: 1}", args: []shapeArgument{v("k")}},
	{name: "array-item", template: "[{0}, {1}]", args: []shapeArgument{v("v"), v(2)}},
	{name: "negate", template: "-{0}", args: []shapeArgument{v(3)}},
	{name: "range", template: "[{0}..{1}]", args: []shapeArgument{v(1), v(3)}},
	{name: "and", template: "{0} and {1}", args: []shapeArgument{v("x"), v("")}},
	{name: "elvis", template: `{0} ?: "d"`, args: []shapeArgument{v("")}},
	{name: "coalesce", template: `{0} ?? "d"`, args: []shapeArgument{v(nil)}},
	{name: "order-by", template: "{0}^($)", args: []shapeArgument{v([]any{3, 1, 2})}},
	{name: "filter-predicate", template: `{0}[$ = "b"]`, args: []shapeArgument{v([]any{"a", "b"})}},
	{name: "subscript", template: "{0}[1]", args: []shapeArgument{v([]any{"a", "b"})}},
}

// undefinedArgumentCrashes lists the arguments for which jsonata-js throws an
// uncoded TypeError when the argument is undefined. The reference defines no
// result there, so the campaign does not compare them.
var undefinedArgumentCrashes = map[string]bool{
	"formatInteger/1": true,
	"formatNumber/1":  true,
	"parseInteger/1":  true,
	"replace/1":       true,
	"replace/2":       true,
	"split/1":         true,
}

type shapeSource struct {
	name  string
	build func(value any, position int) (string, map[string]any)
}

func literal(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func nested(position int, template string, value any, wrap func(map[string]any) map[string]any) (string, map[string]any) {
	a, p := fmt.Sprintf("a%d", position), fmt.Sprintf("p%d", position)
	expression := strings.NewReplacer("{a}", a, "{p}", p).Replace(template)
	field := map[string]any{p: value}
	if wrap != nil {
		field = wrap(field)
	}
	return expression, map[string]any{a: field}
}

var shapeSources = []shapeSource{
	{name: "literal", build: func(value any, _ int) (string, map[string]any) { return literal(value), map[string]any{} }},
	{name: "field", build: func(value any, position int) (string, map[string]any) {
		p := fmt.Sprintf("p%d", position)
		return p, map[string]any{p: value}
	}},
	{name: "path2", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}.{p}", value, nil)
	}},
	{name: "path3", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}.b.{p}", value, func(f map[string]any) map[string]any { return under("b", f) })
	}},
	{name: "variable", build: func(value any, position int) (string, map[string]any) {
		expression, input := nested(position, "{a}.{p}", value, nil)
		return "($x := " + expression + "; $x)", input
	}},
	{name: "parameter", build: func(value any, position int) (string, map[string]any) {
		expression, input := nested(position, "{a}.{p}", value, nil)
		return "(function($q){$q})(" + expression + ")", input
	}},
	{name: "block", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}.({p})", value, nil)
	}},
	{name: "predicate", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}[true].{p}", value, nil)
	}},
	{name: "element", build: func(value any, position int) (string, map[string]any) {
		if _, isArray := value.([]any); isArray {
			return nested(position, "{a}.{p}", value, nil)
		}
		return nested(position, "{a}.{p}[0]", []any{value}, nil)
	}},
	{name: "sorted", build: func(value any, position int) (string, map[string]any) {
		expression, input := nested(position, "{a}.{p}", value, nil)
		return "(" + expression + ")^($)", input
	}},
	{name: "focus", build: func(value any, position int) (string, map[string]any) {
		a, p := fmt.Sprintf("a%d", position), fmt.Sprintf("p%d", position)
		return a + "@$o.$o." + p, map[string]any{a: map[string]any{p: value}}
	}},
	{name: "position", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}#$i.{p}", value, nil)
	}},
	{name: "missing", build: func(_ any, position int) (string, map[string]any) {
		a := fmt.Sprintf("a%d", position)
		return a + ".missing", map[string]any{a: map[string]any{}}
	}},
	{name: "parent", build: func(value any, position int) (string, map[string]any) {
		return nested(position, "{a}.b.%.{p}", value, func(f map[string]any) map[string]any {
			return withSibling(f, "b", map[string]any{"z": 1})
		})
	}},
}
