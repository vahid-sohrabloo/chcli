package functions

import (
	"testing"
)

func TestForVersionEmpty(t *testing.T) {
	all := ForVersion("")
	if len(all) == 0 {
		t.Fatal("ForVersion('') returned no functions")
	}
	if len(all) != len(BuiltinFunctions) {
		t.Errorf("ForVersion('') returned %d, want all %d", len(all), len(BuiltinFunctions))
	}
}

func TestForVersionFilters(t *testing.T) {
	all := ForVersion("")
	// An older version should have fewer or equal functions
	old := ForVersion("21.1.0")
	if len(old) > len(all) {
		t.Errorf("old version (%d) has more functions than all (%d)", len(old), len(all))
	}
}

func TestForVersionInvalid(t *testing.T) {
	// Invalid version should return all functions
	result := ForVersion("garbage")
	if len(result) != len(BuiltinFunctions) {
		t.Errorf("invalid version returned %d, want all %d", len(result), len(BuiltinFunctions))
	}
}

func TestLookupExists(t *testing.T) {
	// These functions exist in every ClickHouse version
	for _, name := range []string{"count", "sum", "toDate", "toString", "now"} {
		f := Lookup(name)
		if f == nil {
			t.Errorf("Lookup(%q) returned nil", name)
		}
	}
}

func TestLookupCaseInsensitive(t *testing.T) {
	lower := Lookup("count")
	upper := Lookup("COUNT")
	mixed := Lookup("Count")

	if lower == nil || upper == nil || mixed == nil {
		t.Fatal("Lookup should be case-insensitive")
	}
	if lower.Name != upper.Name || lower.Name != mixed.Name {
		t.Errorf("case mismatch: %q vs %q vs %q", lower.Name, upper.Name, mixed.Name)
	}
}

func TestLookupNotFound(t *testing.T) {
	f := Lookup("this_function_does_not_exist_xyz")
	if f != nil {
		t.Errorf("expected nil for nonexistent function, got %+v", f)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantMajor   int
		wantMinor   int
	}{
		{"24.3.1", 24, 3},
		{"25.8", 25, 8},
		{"21.1.0", 21, 1},
		{"", 0, 0},
		{"garbage", 0, 0},
	}

	for _, tt := range tests {
		major, minor := parseVersion(tt.input)
		if major != tt.wantMajor || minor != tt.wantMinor {
			t.Errorf("parseVersion(%q) = (%d, %d), want (%d, %d)", tt.input, major, minor, tt.wantMajor, tt.wantMinor)
		}
	}
}

func TestBuiltinFunctionsNotEmpty(t *testing.T) {
	if len(BuiltinFunctions) < 100 {
		t.Errorf("expected 100+ builtin functions, got %d", len(BuiltinFunctions))
	}
}
