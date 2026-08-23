package evaluator

import "testing"

func TestBuiltinRegistryRejectsDuplicateNames(t *testing.T) {
	_, err := buildBuiltinRegistry([]builtinSpec{{name: "duplicate"}, {name: "duplicate"}})
	if err == nil {
		t.Fatal("duplicate builtin name was accepted")
	}
}

func TestBuiltinRegistryRejectsUnknownImplementations(t *testing.T) {
	implementation := builtinFunc(func(state, []any) (any, error) {
		return nil, nil
	})
	_, err := buildBuiltinRegistry([]builtinSpec{{name: "future", implementation: implementation}})
	if err == nil {
		t.Fatal("unknown builtin implementation was accepted")
	}
}

func TestBuiltinCatalogHasUniqueNames(t *testing.T) {
	catalog, err := builtinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := buildBuiltinRegistry(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry) != len(catalog) {
		t.Fatalf("registry has %d names for %d specs", len(registry), len(catalog))
	}
	for _, name := range []string{"append", "eval", "substring", "sum", "uppercase"} {
		if _, ok := registry[name]; !ok {
			t.Fatalf("catalog omitted %q", name)
		}
	}
}

func TestBuiltinCatalogParsesSignaturesForReuse(t *testing.T) {
	catalog, err := builtinCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var catalogAbs *functionSignature
	for index := range catalog {
		if catalog[index].signature == "" {
			continue
		}
		if catalog[index].parsedSignature == nil {
			t.Fatalf("%s signature was not parsed during catalog construction", catalog[index].name)
		}
		if catalog[index].name == "abs" {
			catalogAbs = catalog[index].parsedSignature
		}
	}
	if catalogAbs == nil {
		t.Fatal("abs signature was not present in the catalog")
	}

	registry, err := buildBuiltinRegistry(catalog)
	if err != nil {
		t.Fatal(err)
	}
	spec := registry["abs"]
	if spec.parsedSignature != catalogAbs {
		t.Fatal("registry did not retain the catalog's parsed signature")
	}

	// The parsed form is the immutable source of truth after construction. If
	// invocation reparsed the raw text, this deliberately invalid text would
	// produce T0410 instead of calling abs.
	spec.signature = "<-:n>"
	result, err := (builtinValue{spec: spec}).invoke(state{}, []any{2.0})
	if err != nil {
		t.Fatalf("cached signature was not reused: %v", err)
	}
	if result != 2.0 {
		t.Fatalf("abs result = %#v, want 2", result)
	}
}

func TestBuiltinRegistryRejectsInvalidSignaturesAtConstruction(t *testing.T) {
	_, err := buildBuiltinRegistry([]builtinSpec{{name: "abs", signature: "<-:n>"}})
	if err == nil || err.Error() != "invalid function signature" {
		t.Fatalf("invalid signature error = %v, want invalid function signature", err)
	}
}

func TestBuiltinValueUsesSignaturePreparation(t *testing.T) {
	fn, ok := builtinFor("$append")
	if !ok {
		t.Fatal("append was not registered")
	}
	if _, ok := fn.(builtinValue); !ok {
		t.Fatalf("append has type %T, want builtinValue", fn)
	}
	if signature, ok := callableSignature(fn); !ok || len(signature.params) != 2 {
		t.Fatalf("append signature was not exposed to callable matching: %#v", signature)
	}
	if _, err := fn.invoke(state{}, []any{1.0}); err == nil {
		t.Fatal("append accepted too few arguments")
	} else if code, ok := err.(interface{ JSONataCode() string }); !ok || code.JSONataCode() != "T0410" {
		t.Fatalf("append arity error: %v", err)
	}
}
