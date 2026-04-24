package tui

import "testing"

func TestClassifyColumn(t *testing.T) {
	tests := []struct {
		typeName string
		want     colKind
	}{
		// numerics
		{"Int8", kindNumeric},
		{"Int16", kindNumeric},
		{"Int32", kindNumeric},
		{"Int64", kindNumeric},
		{"Int128", kindNumeric},
		{"Int256", kindNumeric},
		{"UInt8", kindNumeric},
		{"UInt16", kindNumeric},
		{"UInt32", kindNumeric},
		{"UInt64", kindNumeric},
		{"UInt128", kindNumeric},
		{"UInt256", kindNumeric},
		{"Float32", kindNumeric},
		{"Float64", kindNumeric},
		{"Bool", kindNumeric},
		{"Decimal(9,2)", kindNumeric},
		{"Decimal32(4)", kindNumeric},
		// times
		{"Date", kindTime},
		{"Date32", kindTime},
		{"DateTime", kindTime},
		{"DateTime('Europe/Berlin')", kindTime},
		{"DateTime64(3)", kindTime},
		{"DateTime64(6, 'UTC')", kindTime},
		// categories
		{"String", kindCategory},
		{"FixedString(16)", kindCategory},
		{"Enum8('a'=1,'b'=2)", kindCategory},
		{"Enum16('x'=1)", kindCategory},
		// wrappers
		{"Nullable(Int64)", kindNumeric},
		{"Nullable(DateTime)", kindTime},
		{"LowCardinality(String)", kindCategory},
		{"Nullable(LowCardinality(String))", kindCategory},
		{"LowCardinality(Nullable(String))", kindCategory},
		// unknown
		{"Array(UInt32)", kindUnknown},
		{"Tuple(UInt32, String)", kindUnknown},
		{"Map(String, UInt32)", kindUnknown},
		{"UUID", kindUnknown},
		{"IPv4", kindUnknown},
		{"", kindUnknown},
	}
	for _, tc := range tests {
		if got := classifyColumn(tc.typeName); got != tc.want {
			t.Errorf("classifyColumn(%q) = %v, want %v", tc.typeName, got, tc.want)
		}
	}
}
