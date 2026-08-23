package evaluator

import (
	"math"
	"testing"
)

func TestFormatJSONataNumber(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{input: 90.57000000000001, want: "90.57"},
		{input: 245.79000000000002, want: "245.79"},
		{input: 1400000, want: "1400000"},
		{input: 1e-6, want: "0.000001"},
		{input: 1e-7, want: "1e-7"},
		{input: 1e20, want: "100000000000000000000"},
		{input: 1e21, want: "1e+21"},
		{input: 1e-100, want: "1e-100"},
		{input: 1.2345678901234567, want: "1.23456789012346"},
		{input: math.Copysign(0, -1), want: "0"},
	}
	for _, test := range tests {
		if got := formatJSONataNumber(test.input); got != test.want {
			t.Errorf("formatJSONataNumber(%v) = %q, want %q", test.input, got, test.want)
		}
	}
}
