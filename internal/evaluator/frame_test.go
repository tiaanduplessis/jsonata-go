package evaluator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/syntax"
	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func evalFrameTest(t *testing.T, expression string, input any) any {
	t.Helper()
	n, parseErr := syntax.Parse(expression)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	result, evalErr := Eval(n, input)
	if evalErr != nil {
		t.Fatal(evalErr)
	}
	return result
}

func fixture(t *testing.T, name string) any {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "reference", "jsonata-js-v2.2.2", "test", "test-suite", "datasets", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestFrameParentLineageSurvivesProjection(t *testing.T) {
	input := map[string]any{
		"Account": map[string]any{
			"Name": "Firefly",
			"Order": []any{map[string]any{
				"OrderID": "order-1",
				"Product": []any{map[string]any{"Name": "Hat"}},
			}},
		},
	}
	want := map[string]any{"name": "Hat", "order": "order-1", "account": "Firefly"}
	got := evalFrameTest(t, `Account.Order.Product.{'name': Name, 'order': %.OrderID, 'account': %.%.Name}`, input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFrameNestedBindingIsolation(t *testing.T) {
	input := map[string]any{"items": []any{
		map[string]any{"id": "a", "children": []any{map[string]any{"id": "a1"}}},
		map[string]any{"id": "b", "children": []any{map[string]any{"id": "b1"}}},
	}}
	want := []any{
		map[string]any{"outer": "a", "inner": "a1"},
		map[string]any{"outer": "b", "inner": "b1"},
	}
	got := evalFrameTest(t, `items@$i.children@$c.{'outer': $i.id, 'inner': $c.id}`, input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFrameEmployeeGroupingAndDescendingSort(t *testing.T) {
	input := fixture(t, "employees")
	grouped := evalFrameTest(t, `Employee@$e.Contact@$c[$c.ssn = $e.SSN]{ $e.(FirstName & ' ' & Surname): $c.Phone[type != 'home'].number[] }`, input)
	wantGrouped := map[string]any{
		"Darren Cruse": []any{"315 782 9279"},
		"Fred Smith":   []any{"01962 001234", "077 7700 1234"},
		"Hugh Jones":   []any{"0280 864 8643", "07735 853535"},
	}
	if !reflect.DeepEqual(grouped, wantGrouped) {
		t.Fatalf("grouped result %#v, want %#v", grouped, wantGrouped)
	}

	sorted := evalFrameTest(t, `Employee@$e.Contact@$c[$e.SSN=$c.ssn]{ $e.Surname: $c.Phone.number^(>$) }`, input)
	wantSorted := map[string]any{
		"Cruse": []any{"315 782 9279", "3146458343"},
		"Jones": []any{"07735 853535", "0280 864 8643", "0280 564 6543"},
		"Smith": []any{"077 7700 1234", "0203 544 1234", "01962 001234"},
	}
	if !reflect.DeepEqual(sorted, wantSorted) {
		t.Fatalf("sorted result %#v, want %#v", sorted, wantSorted)
	}
}

func TestFrameParentAndTupleSortFixtures(t *testing.T) {
	input := fixture(t, "dataset5")
	got := evalFrameTest(t, `Account.Order.Product.Price.%[%.OrderID='order103'].SKU`, input)
	want := []any{"0406654608", "0406634348"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parent selector result %#v, want %#v", got, want)
	}

	got = evalFrameTest(t, `Account.Order.Product.SKU^(%.Price, >%.%.OrderID)`, input)
	want = []any{"0406634348", "040657863", "0406654608", "0406654603"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuple sort result %#v, want %#v", got, want)
	}

	n, parseErr := syntax.Parse(`Account.Order.().%`)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if _, err := Eval(n, input); err == nil {
		t.Fatal("expected an undefined empty parent projection")
	}
}

func TestFrameIndexAndSortShapeFixtures(t *testing.T) {
	input := []any{3, 1, 4, 1, 5, 9}
	checks := []struct {
		expression string
		want       any
	}{
		{`$^($)#$pos[$pos<3]`, []any{1, 1, 3}},
		{`$#$pos[][$pos<3]^($)[-1]`, []any{4}},
		{`$#$pos[][$pos<3]^($)[-1][]`, []any{4}},
		{`$^(age)[0].name`, "Sally"},
	}
	for _, check := range checks {
		var data any = input
		if check.expression == `$^(age)[0].name` {
			data = []any{
				map[string]any{"name": "Bill", "age": 35},
				map[string]any{"name": "Sally", "age": 33},
				map[string]any{"name": "Jim", "age": 42},
			}
		}
		got := evalFrameTest(t, check.expression, data)
		if !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s: got %#v, want %#v", check.expression, got, check.want)
		}
	}
}

func TestFrameSiblingJoinRetainsParentAndGlobalIndex(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "reference", "jsonata-js-v2.2.2", "test", "test-suite", "datasets", "library.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	input, err := value.DecodeJSON(data)
	if err != nil {
		t.Fatal(err)
	}

	got := evalFrameTest(t, `library.loans@$L.books@$B[$L.isbn=$B.isbn].customers[id=$L.customer].{'book':$B.title,'customer':name,'parent':$keys(%.%)}`, input)
	want := []any{
		map[string]any{"book": "Structure and Interpretation of Computer Programs", "customer": "Joe Doe", "parent": []any{"books", "loans", "customers"}},
		map[string]any{"book": "Compilers: Principles, Techniques, and Tools", "customer": "Jason Arthur", "parent": []any{"books", "loans", "customers"}},
		map[string]any{"book": "Structure and Interpretation of Computer Programs", "customer": "Jason Arthur", "parent": []any{"books", "loans", "customers"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sibling parent result %#v, want %#v", got, want)
	}

	got = evalFrameTest(t, `library.loans@$L.books@$B[$L.isbn=$B.isbn]#$i.{'index':$i,'title':$B.title}`, input)
	want = []any{
		map[string]any{"index": 0, "title": "Structure and Interpretation of Computer Programs"},
		map[string]any{"index": 1, "title": "Compilers: Principles, Techniques, and Tools"},
		map[string]any{"index": 2, "title": "Structure and Interpretation of Computer Programs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("global join index result %#v, want %#v", got, want)
	}
}

func TestFrameLineageLookupDoesNotDuplicateDescendants(t *testing.T) {
	input := fixture(t, "dataset0")
	got := evalFrameTest(t, `foo.**.blah`, input)
	want := []any{
		map[string]any{"baz": map[string]any{"fud": "hello"}},
		map[string]any{"baz": map[string]any{"fud": "world"}},
		map[string]any{"bazz": "gotcha"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descendant result %#v, want %#v", got, want)
	}
}

func TestFrameObjectPropertyOrderMatchesECMAScript(t *testing.T) {
	input, err := value.DecodeJSON([]byte(`{"2":"b","1":"a","x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		expression string
		want       any
	}{
		{`$keys($)`, []any{"1", "2", "x"}},
		{`$string($)`, `{"1":"a","2":"b","x":1}`},
		{`$keys({"2":"b","1":"a","x":1})`, []any{"1", "2", "x"}},
		{`$string({"2":"b","1":"a","x":1})`, `{"1":"a","2":"b","x":1}`},
		{`$keys($sift($, function(){true}))`, []any{"1", "2", "x"}},
		{`$keys($sift($, function($v, $k, $o){$k = $keys($o)[0]}))`, "1"},
		{`$keys($merge([$]))`, []any{"1", "2", "x"}},
		{`$each($, function($v, $k, $o){$join($keys($o), ',')})`, []any{"1,2,x", "1,2,x", "1,2,x"}},
	} {
		if got := evalFrameTest(t, check.expression, input); !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s = %#v, want %#v", check.expression, got, check.want)
		}
	}
}

func TestFrameRuntimeErrorCodes(t *testing.T) {
	for _, check := range []struct {
		expression string
		code       string
	}{
		{`%()`, "T1006"},
		{`library.(%%%)`, "T2001"},
	} {
		n, parseErr := syntax.Parse(check.expression)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		_, err := Eval(n, map[string]any{"library": map[string]any{}})
		if err == nil {
			t.Errorf("%s: expected %s", check.expression, check.code)
			continue
		}
		coded, ok := err.(interface{ JSONataCode() string })
		if !ok || coded.JSONataCode() != check.code {
			t.Errorf("%s: got error %v, want %s", check.expression, err, check.code)
		}
	}
}
