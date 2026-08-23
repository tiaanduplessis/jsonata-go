package evaluator

import "testing"

func TestHigherOrderBuiltinSignatures(t *testing.T) {
	want := map[string]string{
		"filter": "<af>",
		"map":    "<af>",
		"reduce": "<afj?:j>",
		"single": "<af?>",
	}
	for name, signature := range want {
		spec, ok := builtinSpecFor(name)
		if !ok {
			t.Fatalf("builtin %q is missing", name)
		}
		if spec.signature != signature {
			t.Errorf("builtin %q signature = %q, want %q", name, spec.signature, signature)
		}
	}
}
