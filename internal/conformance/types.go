// Package conformance provides deterministic loading and execution of the
// pinned, language-neutral JSONata test suite.
package conformance

import "fmt"

const (
	ReferenceCommit = "6c7e95fdbf4405a1e741852a7cd8cd985b4305bb"
	ReferenceName   = "jsonata-js-v2.2.2"
)

// Expression is the small part of the public evaluator API needed by the
// conformance runner. Keeping this interface here lets loader tests run
// without coupling them to evaluator internals.
type Expression interface {
	Eval(data any) (any, error)
}

// Compiler compiles an expression into an immutable Expression.
type Compiler interface {
	Compile(expression string) (Expression, error)
}

type ExpectedKind string

const (
	ExpectedResult    ExpectedKind = "result"
	ExpectedUndefined ExpectedKind = "undefined"
	ExpectedError     ExpectedKind = "error"
)

// Case is one JSONata language-neutral test case. Source is relative to the
// suite root and is stable, so it can be used to identify a failure.
type Case struct {
	Group               string
	ID                  string
	Source              string
	Expression          string
	Data                any
	HasData             bool
	Dataset             string
	HasDataset          bool
	Bindings            map[string]any
	Unordered           bool
	TimeLimit           int
	Depth               int
	ExpectedKind        ExpectedKind
	Expected            any
	ExpectedCode        string
	ExpectedToken       string
	ExpectedPosition    int
	HasExpectedPosition bool
	SupportedInput      bool
	UnsupportedWhy      string
}

func (c Case) Reference() string { return fmt.Sprintf("%s/%s", c.Group, c.ID) }

// HasInput reports whether the fixture supplies an input value. A data field
// containing JSON null is input; a dataset field containing null is the
// reference suite's representation of no input.
func (c Case) HasInput() bool {
	return c.HasData || (c.HasDataset && c.Dataset != "")
}

type Group struct {
	Name  string
	Cases []Case
}

type Suite struct {
	Root            string
	ReferenceCommit string
	Groups          []Group
}

func (s Suite) CaseCount() int {
	total := 0
	for _, g := range s.Groups {
		total += len(g.Cases)
	}
	return total
}

type Manifest map[string]map[string]struct{}

// Phase1Manifest is deliberately case-specific. A group can contain
// features from later phases, so enabling a whole group would turn a later
// failure into an accidental Phase 1 gate.
var Phase1Manifest = Manifest{
	"literals": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {}, "case006": {}, "case007": {},
		"case009": {}, "case010": {}, "case011": {}, "case012": {},
		"case013": {}, "case014": {}, "case018": {}, "case019": {},
	},
	"fields": {
		"case000": {}, "case001": {},
	},
	"array-constructor": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {},
	},
	"object-constructor": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {},
	},
}

// Phase2Manifest enables the complete audited Phase 1-2 slice.  Selection is
// case-specific because the upstream groups intentionally contain later-phase
// functions, bindings, and implementation fixtures.
var Phase2Manifest = Manifest{
	"array-constructor": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {},
		"case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {},
		"array-sequences#000": {}, "array-sequences#001": {}, "array-sequences#002": {}, "array-sequences#003": {}, "array-sequences#004": {},
	},
	"boolean-expresssions": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {},
		"case012": {}, "case013": {}, "case014": {}, "case015": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case026": {},
	},
	"coalescing-operator": {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}},
	"comparison-operators": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {},
		"deep-equals#000": {}, "deep-equals#001": {}, "deep-equals#002": {}, "deep-equals#003": {}, "deep-equals#004": {}, "deep-equals#005": {}, "deep-equals#006": {}, "deep-equals#007": {}, "deep-equals#008": {}, "deep-equals#009": {}, "deep-equals#010": {}, "deep-equals#011": {}, "deep-equals#012": {}, "deep-equals#013": {}, "deep-equals#014": {}, "deep-equals#015": {}, "deep-equals#016": {}, "deep-equals#017": {},
	},
	"conditionals": {"case006": {}, "case007": {}, "case008": {}},
	"default-operator": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {},
	},
	"descendent-operator": {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}},
	"errors":              {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case014": {}, "case015": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}},
	"fields":              {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}},
	"flattening": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case026": {}, "case027": {}, "case028": {}, "case029": {}, "case030": {}, "case031": {}, "case032": {}, "case033": {}, "case034": {}, "case035": {}, "case036": {}, "case037": {}, "case038": {}, "case039": {}, "case040": {}, "case041": {}, "case042": {}, "case043": {}, "case045": {},
		"array-inputs#000": {}, "array-inputs#001": {}, "array-inputs#002": {}, "array-inputs#003": {}, "array-inputs#004": {}, "array-inputs#005": {}, "array-inputs#006": {}, "array-inputs#007": {},
		"sequence-of-arrays#000": {}, "sequence-of-arrays#001": {}, "sequence-of-arrays#002": {}, "sequence-of-arrays#003": {},
	},
	"inclusion-operator":       {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}},
	"literals":                 {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}},
	"missing-paths":            {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}},
	"multiple-array-selectors": {"case000": {}, "case001": {}, "case002": {}},
	"null":                     {"case000": {}, "case001": {}, "case002": {}, "case004": {}, "case005": {}, "case006": {}},
	"numeric-operators":        {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}},
	"object-constructor":       {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case007": {}, "case008": {}, "case009": {}, "case013": {}, "case014": {}, "case017": {}, "case018": {}, "case026": {}},
	"parentheses":              {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}},
	"parent-operator": {
		"errors#000": {}, "errors#001": {}, "errors#002": {}, "errors#003": {}, "errors#004": {}, "errors#005": {}, "errors#006": {}, "errors#007": {}, "errors#008": {}, "errors#009": {}, "errors#010": {}, "errors#011": {}, "errors#012": {}, "errors#013": {}, "errors#014": {}, "errors#015": {},
		"parent#002": {}, "parent#003": {}, "parent#004": {}, "parent#005": {}, "parent#006": {}, "parent#007": {}, "parent#008": {}, "parent#009": {}, "parent#010": {}, "parent#011": {}, "parent#012": {}, "parent#018": {}, "parent#019": {}, "parent#020": {}, "parent#021": {},
	},
	"predicates":             {"case000": {}, "case001": {}, "case002": {}},
	"quoted-selectors":       {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}},
	"range-operator":         {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}},
	"simple-array-selectors": {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}},
	"sorting":                {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}},
	"string-concat":          {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}},
	"token-conversion":       {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"transforms":             {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}},
	"wildcards":              {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}},
	"joins": {
		"employee-map-reduce#000": {}, "employee-map-reduce#001": {}, "employee-map-reduce#004": {}, "employee-map-reduce#005": {}, "employee-map-reduce#006": {}, "employee-map-reduce#007": {}, "employee-map-reduce#008": {}, "employee-map-reduce#009": {}, "employee-map-reduce#010": {}, "employee-map-reduce#011": {},
		"errors#000": {}, "errors#001": {}, "errors#002": {}, "errors#003": {},
		"index#000": {}, "index#001": {}, "index#002": {}, "index#003": {}, "index#004": {}, "index#005": {}, "index#006": {}, "index#007": {}, "index#008": {}, "index#009": {}, "index#010": {}, "index#011": {}, "index#012": {}, "index#013": {}, "index#014": {},
	},
}

// Phase3Manifest adds the audited functional-language cases to the complete
// Phase 1-2 slice. It is derived through a deep copy so later test setup
// cannot mutate either staged manifest.
var Phase3Manifest = extendManifest(Phase2Manifest, Manifest{
	"closures": {
		"case000": {}, "case001": {},
	},
	"blocks": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {}, "case006": {},
	},
	"variables": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {}, "case006": {}, "case007": {},
		"case008": {}, "case009": {}, "case010": {}, "case011": {},
		"case012": {},
	},
	"lambdas": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {}, "case006": {}, "case007": {},
		"case008": {}, "case009": {}, "case010": {}, "case011": {},
		"case012": {}, "case013": {},
	},
	"function-applications": {"case020": {}},
	"partial-application": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {},
	},
	"higher-order-functions": {
		"case000": {}, "case001": {}, "case002": {},
	},
	"tail-recursion": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {}, "case006": {}, "case007": {},
		"case008": {}, "case009": {},
	},
	"function-eval": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
		"case004": {}, "case005": {},
		"case006": {}, "case007": {},
		"case008#000": {}, "case008#001": {}, "case008#002": {}, "case008#003": {},
	},
	"function-signatures": {"case026": {}, "case034": {}},
})

// Phase4Manifest adds the portable standard-library and higher-order function
// cases to the complete Phase 3 slice. Regex, picture-string, date/time, and
// Phase 3-owned functional-language cases remain outside this manifest.
var Phase4Manifest = extendManifest(Phase3Manifest, Manifest{
	"context":  {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"encoding": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-abs": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
	},
	"function-append": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {},
	},
	"function-assert": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {},
	},
	"function-average": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {},
	},
	"function-boolean": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {},
	},
	"function-ceil": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
	},
	"function-count": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {},
	},
	"function-decodeUrl":          {"case000": {}, "case001": {}, "case002": {}},
	"function-decodeUrlComponent": {"case000": {}, "case001": {}, "case002": {}},
	"function-distinct": {
		"distinct#000": {}, "distinct#001": {}, "distinct#002": {}, "distinct#003": {}, "distinct#004": {}, "distinct#005": {}, "distinct#006": {}, "distinct#007": {},
	},
	"function-each": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
	},
	"function-encodeUrl":          {"case000": {}, "case001": {}, "case002": {}},
	"function-encodeUrlComponent": {"case000": {}, "case001": {}, "case002": {}},
	"function-error": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
	},
	"function-exists": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {},
	},
	"function-floor": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {},
	},
	"function-join": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {},
	},
	"function-keys": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {},
	},
	"function-length": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {},
	},
	"function-lookup":    {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-lowercase": {"case000": {}, "case001": {}},
	"function-max": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case026": {},
	},
	"function-merge": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {},
	},
	"function-number": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case026": {}, "case027": {}, "case028": {}, "case029": {}, "case030": {}, "case031": {}, "case032": {}, "case033": {},
	},
	"function-pad": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {},
	},
	"function-power":   {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}},
	"function-reverse": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-round": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {},
	},
	"function-shuffle": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-sift":    {"case000": {}, "case001": {}, "case003": {}, "case004": {}},
	"function-sort": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
	},
	"function-spread": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-sqrt":   {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-string": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case026": {}, "case027": {}, "case028": {}, "case029": {}, "case030": {},
	},
	"function-substring": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {},
	},
	"function-substringAfter":  {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}},
	"function-substringBefore": {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}},
	"function-sum":             {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}},
	"function-trim":            {"case000": {}, "case001": {}, "case002": {}},
	"function-typeOf": {
		"case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {},
	},
	"function-uppercase": {"case000": {}, "case001": {}},
	"function-zip":       {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}},
	"hof-filter":         {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"hof-map": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case0010": {}, "case0011": {},
	},
	"hof-reduce": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
	},
	"hof-single": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {},
	},
	"hof-zip-map": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"function-signatures": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {}, "case027": {}, "case028": {}, "case029": {}, "case030": {}, "case031": {}, "case032": {}, "case033": {}, "case035": {}, "case036": {}, "case037": {}, "case038": {}, "case039": {}, "case040": {},
	},
	"function-applications": {
		"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}, "case006": {}, "case007": {}, "case008": {}, "case009": {}, "case010": {}, "case011": {}, "case012": {}, "case013": {}, "case014": {}, "case015": {}, "case016": {}, "case017": {}, "case018": {}, "case019": {},
	},
})

// Phase5Manifest adds regex, formatting, and date/time cases to the complete
// Phase 4 slice. The two mixed application cases are assigned here because
// their expressions contain regex literals.
var Phase5Manifest = extendManifest(Phase4Manifest, Manifest{
	"regex":                  numberedCases("case%03d", 0, 39),
	"matchers":               numberedCases("case%03d", 0, 2),
	"function-contains":      numberedCases("case%03d", 0, 8),
	"function-replace":       numberedCases("case%03d", 0, 12),
	"function-split":         numberedCases("case%03d", 0, 19),
	"function-formatBase":    numberedCases("case%03d", 0, 9),
	"function-formatInteger": numberedCases("formatInteger#%03d", 0, 65),
	"function-formatNumber": mergeCaseSets(
		numberedCases("case%03d", 0, 37),
		numberedCases("issue785#%03d", 0, 4),
		numberedCases("issue786#%03d", 0, 4),
	),
	"function-parseInteger": numberedCases("parseInteger#%03d", 0, 61),
	"function-fromMillis": mergeCaseSets(
		numberedCases("case%03d", 0, 3),
		numberedCases("formatDateTime#%03d", 0, 71),
		numberedCases("isoWeekDate#%03d", 0, 19),
	),
	"function-tomillis": mergeCaseSets(
		numberedCases("case%03d", 1, 13),
		numberedCases("parseDateTime#%03d", 0, 47),
	),
	"function-sift":         {"case002": {}},
	"function-applications": {"case021": {}},
})

// ParserResidualManifest contains the parser diagnostics and comment cases
// that were verified separately from the staged evaluator manifests.
var ParserResidualManifest = Manifest{
	"comments": {"case000": {}, "case001": {}, "case002": {}, "case003": {}},
	"errors": {
		"case005": {}, "case006": {}, "case012": {}, "case013": {},
		"case016": {}, "case017": {}, "case018": {}, "case019": {},
		"case024": {}, "case025": {}, "case026": {},
	},
}

// CoreResidualManifest is the exact audited Phase 7 language-core slice.
// It is intentionally separate from the staged Phase 5 gate until every
// residual group is independently verified.
var CoreResidualManifest = Manifest{
	"array-constructor": {"case006": {}},
	"boolean-expresssions": {
		"case010": {}, "case011": {}, "case016": {}, "case027": {}, "case028": {}, "case029": {}, "case030": {},
	},
	"comparison-operators": {"case026": {}, "case027": {}, "case028": {}, "deep-equals#018": {}},
	"conditionals":         {"case000": {}, "case001": {}, "case002": {}, "case003": {}, "case004": {}, "case005": {}},
	"default-operator":     {"case011": {}},
	"inclusion-operator":   {"case008": {}},
	"literals":             {"array-inputs#000": {}, "array-inputs#001": {}, "array-inputs#002": {}, "array-inputs#003": {}},
	"missing-paths":        {"case005": {}},
	"null":                 {"case003": {}},
	"numeric-operators":    {"case017": {}, "case018": {}},
	"predicates":           {"case003": {}},
	"range-operator":       {"case019": {}, "case020": {}, "case021": {}, "case022": {}, "case023": {}, "case024": {}},
}

// ProjectionResidualManifest is the exact audited Phase 7 projection and
// object-construction slice. The large flattening fixtures use the evaluator
// callback so their declared time and depth limits remain visible to tests.
var ProjectionResidualManifest = Manifest{
	"flattening": {
		"case034a": {}, "case044": {}, "large#000": {}, "large#001": {},
	},
	"object-constructor": {
		"case006": {}, "case010": {}, "case011": {}, "case012": {},
		"case015": {}, "case016": {}, "case019": {}, "case020": {},
		"case021": {}, "case022": {}, "case023": {}, "case024": {}, "case025": {},
	},
	"wildcards": {"case010#000": {}, "case010#001": {}, "case010#002": {}},
}

// ParentJoinResidualManifest is the exact audited Phase 7 parent-lineage,
// join, and index-binding slice.
var ParentJoinResidualManifest = Manifest{
	"parent-operator": {
		"parent#000": {}, "parent#001": {},
		"parent#013": {}, "parent#014": {}, "parent#015": {}, "parent#016": {}, "parent#017": {},
		"parent#022": {}, "parent#023": {}, "parent#024": {}, "parent#025": {}, "parent#026": {}, "parent#027": {},
	},
	"joins": {
		"employee-map-reduce#002": {}, "employee-map-reduce#003": {},
		"index#015":         {},
		"library-joins#000": {}, "library-joins#001": {}, "library-joins#002": {}, "library-joins#003": {}, "library-joins#004": {},
		"library-joins#005": {}, "library-joins#006": {}, "library-joins#007": {}, "library-joins#008": {}, "library-joins#009": {},
		"library-joins#010": {}, "library-joins#011": {}, "library-joins#012": {},
	},
}

// TransformManifest and PerformanceManifest are complete pinned groups.
var TransformManifest = Manifest{
	"transform": numberedCases("case%03d", 0, 104),
}

var PerformanceManifest = Manifest{
	"performance": {"case000": {}, "case001": {}},
}

// FullManifest is the canonical union of every independently audited slice
// in the pinned jsonata-js v2.2.2 suite.
var FullManifest = combineManifests(
	Phase5Manifest,
	ParserResidualManifest,
	CoreResidualManifest,
	ProjectionResidualManifest,
	ParentJoinResidualManifest,
	TransformManifest,
	PerformanceManifest,
)

func numberedCases(format string, start, count int) map[string]struct{} {
	cases := make(map[string]struct{}, count)
	for index := start; index < start+count; index++ {
		cases[fmt.Sprintf(format, index)] = struct{}{}
	}
	return cases
}

func mergeCaseSets(sets ...map[string]struct{}) map[string]struct{} {
	merged := make(map[string]struct{})
	for _, set := range sets {
		for id := range set {
			merged[id] = struct{}{}
		}
	}
	return merged
}

func extendManifest(base, additions Manifest) Manifest {
	result := make(Manifest, len(base)+len(additions))
	for group, cases := range base {
		result[group] = make(map[string]struct{}, len(cases))
		for id := range cases {
			result[group][id] = struct{}{}
		}
	}
	for group, cases := range additions {
		if result[group] == nil {
			result[group] = make(map[string]struct{}, len(cases))
		}
		for id := range cases {
			result[group][id] = struct{}{}
		}
	}
	return result
}

func combineManifests(manifests ...Manifest) Manifest {
	combined := make(Manifest)
	for _, manifest := range manifests {
		combined = extendManifest(combined, manifest)
	}
	return combined
}

type GroupSummary struct {
	Name  string `json:"name"`
	Cases int    `json:"cases"`
}

type CaseRef struct {
	Group  string `json:"group"`
	ID     string `json:"id"`
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}

func (c CaseRef) Reference() string { return fmt.Sprintf("%s/%s", c.Group, c.ID) }

type Failure struct {
	CaseRef
	Message string `json:"message"`
}

type Report struct {
	ReferenceName   string         `json:"referenceName"`
	ReferenceCommit string         `json:"referenceCommit"`
	Discovered      []GroupSummary `json:"discoveredGroups"`
	EnabledGroups   []string       `json:"enabledGroups"`
	EnabledCases    int            `json:"enabledCases"`
	RemainingGroups []string       `json:"remainingGroups"`
	RemainingCases  []CaseRef      `json:"remainingCases"`
	Passes          int            `json:"passes"`
	Failures        []Failure      `json:"failures"`
	Skips           []CaseRef      `json:"skips"`
}

func (r Report) RemainingCount() int { return len(r.RemainingCases) }
