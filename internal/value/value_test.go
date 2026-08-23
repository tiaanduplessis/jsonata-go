package value

import (
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestDecodeJSONPreservesObjectMemberOrder(t *testing.T) {
	decoded, err := DecodeJSON([]byte(`{"books":1,"loans":2,"customers":{"id":3}}`))
	if err != nil {
		t.Fatal(err)
	}
	object, ok := decoded.(OrderedObject)
	if !ok {
		t.Fatalf("decoded type = %T, want OrderedObject", decoded)
	}
	if want := []string{"books", "loans", "customers"}; !reflect.DeepEqual(object.Order, want) {
		t.Fatalf("order = %#v, want %#v", object.Order, want)
	}
	nested, ok := object.Fields["customers"].(OrderedObject)
	if !ok || !reflect.DeepEqual(nested.Order, []string{"id"}) {
		t.Fatalf("nested object = %#v, want ordered id field", object.Fields["customers"])
	}
}

func TestDecodeJSONDuplicateKeyKeepsFirstPositionAndLastValue(t *testing.T) {
	decoded, err := DecodeJSON([]byte(`{"safe":1,"constructor":2,"safe":3}`))
	if err != nil {
		t.Fatal(err)
	}
	object := decoded.(OrderedObject)
	if want := []string{"safe", "constructor"}; !reflect.DeepEqual(object.Order, want) {
		t.Fatalf("order = %#v, want %#v", object.Order, want)
	}
	if got := object.Fields["safe"]; got != jsonNumber("3") {
		t.Fatalf("safe = %#v, want last value 3", got)
	}
}

func TestDecodeJSONUsesECMAScriptPropertyOrder(t *testing.T) {
	decoded, err := DecodeJSON([]byte(`{"2":"b","1":"a","01":"leading","4294967294":"max index","4294967295":"not index","x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	object := decoded.(OrderedObject)
	want := []string{"1", "2", "4294967294", "01", "4294967295", "x"}
	if !reflect.DeepEqual(object.Order, want) {
		t.Fatalf("order = %#v, want ECMAScript order %#v", object.Order, want)
	}
}

func TestParseArrayIndex(t *testing.T) {
	valid := map[string]uint32{
		"0":          0,
		"1":          1,
		"9":          9,
		"10":         10,
		"4294967293": 4294967293,
		"4294967294": 4294967294,
	}
	for key, want := range valid {
		t.Run("valid_"+key, func(t *testing.T) {
			got, ok := parseArrayIndex(key)
			if !ok || got != want {
				t.Fatalf("parseArrayIndex(%q) = %d, %v; want %d, true", key, got, ok, want)
			}
		})
	}

	invalid := []string{
		"", "00", "01", "0000000000", "-0", "+0", " 0", "0 ",
		"1.0", "1e0", "1_0", "１２", "4294967295", "4294967296",
		"9999999999", "10000000000", "18446744073709551615",
	}
	for _, key := range invalid {
		t.Run("invalid_"+key, func(t *testing.T) {
			if got, ok := parseArrayIndex(key); ok {
				t.Fatalf("parseArrayIndex(%q) = %d, true; want false", key, got)
			}
		})
	}
}

func TestParseArrayIndexMatchesECMAScriptReference(t *testing.T) {
	check := func(key string) {
		t.Helper()
		got, gotOK := parseArrayIndex(key)
		want, wantOK := referenceArrayIndex(key)
		if gotOK != wantOK || gotOK && got != want {
			t.Fatalf("parseArrayIndex(%q) = %d, %v; reference = %d, %v", key, got, gotOK, want, wantOK)
		}
	}

	for index := uint64(0); index <= 100000; index++ {
		canonical := strconv.FormatUint(index, 10)
		check(canonical)
		check("0" + canonical)
		check("+" + canonical)
		check(canonical + "x")
	}
	for index := uint64(math.MaxUint32) - 128; index <= uint64(math.MaxUint32)+128; index++ {
		check(strconv.FormatUint(index, 10))
	}
}

func TestParseArrayIndexDoesNotAllocate(t *testing.T) {
	for _, key := range []string{"0", "4294967294", "ordinary", "4294967295", "18446744073709551615"} {
		t.Run(key, func(t *testing.T) {
			var index uint32
			var ok bool
			if allocations := testing.AllocsPerRun(1000, func() {
				index, ok = parseArrayIndex(key)
			}); allocations != 0 {
				t.Fatalf("parseArrayIndex(%q) allocated %.1f times per call", key, allocations)
			}
			_, _ = index, ok
		})
	}
}

func referenceArrayIndex(key string) (uint32, bool) {
	index, err := strconv.ParseUint(key, 10, 32)
	if err != nil || index == math.MaxUint32 || strconv.FormatUint(index, 10) != key {
		return 0, false
	}
	return uint32(index), true
}

func TestCanonicalObjectOrder(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want []string
	}{
		{
			name: "sorts indexes before stable ordinary keys",
			keys: []string{"beta", "10", "01", "2", "alpha", "0", "4294967294", "4294967295"},
			want: []string{"0", "2", "10", "4294967294", "beta", "01", "alpha", "4294967295"},
		},
		{
			name: "keeps canonical input",
			keys: []string{"0", "2", "4294967294", "beta", "01", "4294967295", "alpha"},
			want: []string{"0", "2", "4294967294", "beta", "01", "4294967295", "alpha"},
		},
		{
			name: "keeps non-index near misses stable",
			keys: []string{"4294967295", "00", "-1", "1.0", "", "01"},
			want: []string{"4294967295", "00", "-1", "1.0", "", "01"},
		},
		{
			name: "sorts adjacent upper boundary",
			keys: []string{"4294967294", "4294967293", "4294967292"},
			want: []string{"4294967292", "4294967293", "4294967294"},
		},
		{
			name: "empty",
			keys: []string{},
			want: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CanonicalObjectOrder(test.keys)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("order = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCanonicalObjectOrderDoesNotExposeInputAliasing(t *testing.T) {
	for _, keys := range [][]string{
		{"0", "1", "alpha", "beta"},
		{"beta", "1", "alpha", "0"},
	} {
		input := append([]string(nil), keys...)
		got := CanonicalObjectOrder(input)
		if len(got) > 0 && &got[0] == &input[0] {
			t.Fatalf("CanonicalObjectOrder returned caller-owned backing array for %#v", keys)
		}
		got[0] = "result mutation"
		if !reflect.DeepEqual(input, keys) {
			t.Fatalf("result mutation changed input: got %#v, want %#v", input, keys)
		}
		input[len(input)-1] = "input mutation"
		if got[len(got)-1] == "input mutation" {
			t.Fatalf("input mutation changed result: %#v", got)
		}
	}
}

func TestCanonicalObjectOrderReusesOwnedCanonicalInput(t *testing.T) {
	keys := []string{"0", "2", "4294967294", "alpha", "01", "4294967295"}
	got := canonicalObjectOrderOwned(keys)
	if &got[0] != &keys[0] {
		t.Fatal("owned canonical order was copied")
	}
	if !reflect.DeepEqual(got, keys) {
		t.Fatalf("order = %#v, want %#v", got, keys)
	}
}

func BenchmarkDecodeJSONObjectOrder(b *testing.B) {
	for _, test := range []struct {
		name string
		json []byte
	}{
		{
			name: "ordinary-keys",
			json: []byte(`{"alpha":1,"bravo":2,"charlie":3,"delta":4,"echo":5,"foxtrot":6,"golf":7,"hotel":8}`),
		},
		{
			name: "mixed-array-index-keys",
			json: []byte(`{"8":8,"alpha":1,"2":2,"bravo":2,"4294967294":3,"01":4,"0":0,"4294967295":5}`),
		},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := DecodeJSON(test.json); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func jsonNumber(text string) any {
	decoded, err := DecodeJSON([]byte(text))
	if err != nil {
		panic(err)
	}
	return decoded
}
