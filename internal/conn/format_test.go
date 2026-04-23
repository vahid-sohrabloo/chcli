package conn

import "testing"

func TestTrimFloatZerosPreservesIntegerFloats(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"3600", "3600"},   // integer float — must not be trimmed to "36"
		{"100", "100"},     // regression for the same class of bug
		{"1.2300", "1.23"}, // trailing zeros after a decimal still trim
		{"1.0", "1"},       // single trailing zero + trailing dot both trim
		{"0.5", "0.5"},     // no trailing zeros — unchanged
		{"0", "0"},
		{"1000", "1000"},
		{"1.0001", "1.0001"},
	}
	for _, tc := range cases {
		if got := trimFloatZeros(tc.in); got != tc.want {
			t.Errorf("trimFloatZeros(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatValueFloat64IntegerIsNotMangled(t *testing.T) {
	if got := formatValue(float64(3600)); got != "3600" {
		t.Errorf("formatValue(3600.0) = %q, want 3600", got)
	}
	if got := formatValue(float64(100)); got != "100" {
		t.Errorf("formatValue(100.0) = %q, want 100", got)
	}
	if got := formatValue(float64(1.23)); got != "1.23" {
		t.Errorf("formatValue(1.23) = %q, want 1.23", got)
	}
}
