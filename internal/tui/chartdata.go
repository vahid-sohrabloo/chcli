package tui

import "strings"

type colKind int

const (
	kindUnknown colKind = iota
	kindNumeric
	kindTime
	kindCategory
)

// classifyColumn maps a ClickHouse type name to a chart-usable kind.
// Strips Nullable(...) and LowCardinality(...) wrappers, possibly nested.
func classifyColumn(typeName string) colKind {
	t := stripWrappers(typeName)
	switch {
	case isNumericType(t):
		return kindNumeric
	case isTimeType(t):
		return kindTime
	case isCategoryType(t):
		return kindCategory
	default:
		return kindUnknown
	}
}

func stripWrappers(t string) string {
	for {
		switch {
		case strings.HasPrefix(t, "Nullable(") && strings.HasSuffix(t, ")"):
			t = t[len("Nullable(") : len(t)-1]
		case strings.HasPrefix(t, "LowCardinality(") && strings.HasSuffix(t, ")"):
			t = t[len("LowCardinality(") : len(t)-1]
		default:
			return t
		}
	}
}

func isNumericType(t string) bool {
	switch t {
	case "Bool",
		"Int8", "Int16", "Int32", "Int64", "Int128", "Int256",
		"UInt8", "UInt16", "UInt32", "UInt64", "UInt128", "UInt256",
		"Float32", "Float64":
		return true
	}
	return strings.HasPrefix(t, "Decimal")
}

func isTimeType(t string) bool {
	if t == "Date" || t == "Date32" {
		return true
	}
	if t == "DateTime" || strings.HasPrefix(t, "DateTime(") {
		return true
	}
	return strings.HasPrefix(t, "DateTime64")
}

func isCategoryType(t string) bool {
	if t == "String" {
		return true
	}
	if strings.HasPrefix(t, "FixedString") {
		return true
	}
	return strings.HasPrefix(t, "Enum")
}
