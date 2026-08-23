package evaluator

import (
	"reflect"
	"testing"

	"github.com/tiaanduplessis/jsonata-go/internal/value"
)

func TestSignatureContextOnlyCollapsesForContextDefault(t *testing.T) {
	withoutDefault, err := parseFunctionSignature("<s:s>")
	if err != nil {
		t.Fatal(err)
	}
	current := contextual{v: sequence{contextual{v: "context"}}}
	if got := signatureContext(withoutDefault, current); !value.IsUndefined(got) {
		t.Fatalf("signature without context default received %#v", got)
	}

	withDefault, err := parseFunctionSignature("<s-:s>")
	if err != nil {
		t.Fatal(err)
	}
	if got := signatureContext(withDefault, current); got != "context" {
		t.Fatalf("contextual singleton sequence = %#v, want context", got)
	}
	if got := signatureContext(withDefault, value.Undefined); !value.IsUndefined(got) {
		t.Fatalf("undefined context = %#v, want undefined", got)
	}
	if got := signatureContext(withDefault, sequence{"left", "right"}); !reflect.DeepEqual(got, []any{"left", "right"}) {
		t.Fatalf("sequence context = %#v, want collapsed sequence", got)
	}
}

func TestBuiltinSignatureContextBehaviorIsPreserved(t *testing.T) {
	echo := func(_ state, args []any) (any, error) {
		return args[0], nil
	}
	tests := []struct {
		name      string
		signature string
		current   any
		args      []any
		want      any
		wantCode  string
	}{
		{
			name:      "explicit argument without context default",
			signature: "<s:s>",
			current:   contextual{v: sequence{"unused", "context"}},
			args:      []any{"explicit"},
			want:      "explicit",
		},
		{
			name:      "explicit argument with context default",
			signature: "<s-:s>",
			current:   contextual{v: sequence{"unused", "context"}},
			args:      []any{"explicit"},
			want:      "explicit",
		},
		{
			name:      "contextual fallback",
			signature: "<s-:s>",
			current:   contextual{v: "context"},
			want:      "context",
		},
		{
			name:      "sequence fallback",
			signature: "<s-:s>",
			current:   sequence{"context"},
			want:      "context",
		},
		{
			name:      "undefined context",
			signature: "<s-:s>",
			current:   value.Undefined,
			want:      value.Undefined,
		},
		{
			name:      "invalid sequence context",
			signature: "<s-:s>",
			current:   sequence{1.0, 2.0},
			wantCode:  "T0411",
		},
		{
			name:      "missing required explicit argument",
			signature: "<s:s>",
			current:   "unused",
			wantCode:  "T0410",
		},
		{
			name:      "invalid explicit argument",
			signature: "<s-:s>",
			current:   "context",
			args:      []any{1.0},
			wantCode:  "T0410",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			function := builtinValue{spec: builtinSpec{
				name:           "contextTest",
				signature:      test.signature,
				implementation: echo,
			}}
			got, err := function.invoke(state{current: test.current}, test.args)
			if test.wantCode != "" {
				if !hasJSONataCode(err, test.wantCode) {
					t.Fatalf("error = %v, want %s", err, test.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("result = %#v, want %#v", got, test.want)
			}
		})
	}
}
