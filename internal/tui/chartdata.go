package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

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

type parsedData struct {
	xTimes  []time.Time
	xCats   []string
	xNums   []float64
	ys      [][]float64
	dropped int
}

// parseRows converts result rows into typed X/Y slices for charting.
// A row is dropped (counted in dropped) if any selected Y is NULL or unparseable,
// or if the X cell cannot be parsed for its kind.
func parseRows(res *conn.QueryResult, xIdx int, yIdxs []int) parsedData {
	xKind := classifyColumn(res.Columns[xIdx].Type)
	p := parsedData{ys: make([][]float64, len(yIdxs))}

	for _, row := range res.Rows {
		xOk := true
		var xT time.Time
		var xN float64
		var xC string

		switch xKind {
		case kindTime:
			xT, xOk = parseTimeCell(row[xIdx])
		case kindNumeric:
			xN, xOk = parseFloatCell(row[xIdx])
		default:
			xC = row[xIdx]
		}
		if !xOk {
			p.dropped++
			continue
		}

		ys := make([]float64, len(yIdxs))
		yOk := true
		for i, yi := range yIdxs {
			v, ok := parseFloatCell(row[yi])
			if !ok {
				yOk = false
				break
			}
			ys[i] = v
		}
		if !yOk {
			p.dropped++
			continue
		}

		switch xKind {
		case kindTime:
			p.xTimes = append(p.xTimes, xT)
		case kindNumeric:
			p.xNums = append(p.xNums, xN)
		default:
			p.xCats = append(p.xCats, xC)
		}
		for i := range yIdxs {
			p.ys[i] = append(p.ys[i], ys[i])
		}
	}
	return p
}

var timeFormats = []string{
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseTimeCell(s string) (time.Time, bool) {
	if s == "" || s == "NULL" {
		return time.Time{}, false
	}
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseFloatCell(s string) (float64, bool) {
	if s == "" || s == "NULL" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

type chartType int

const (
	chartLine chartType = iota
	chartBar
)

// autoDetect picks initial X, Y, and chart type for a query result.
//
// Priority for X:
//  1. first kindTime column                  -> chartLine
//  2. first kindCategory or kindUnknown      -> chartBar
//  3. fallback: column 0                     -> chartBar
//
// Y is the first kindNumeric column other than X (single series).
// If no numeric column exists, Y is empty (non-chartable result).
func autoDetect(res *conn.QueryResult) (xIdx int, yIdxs []int, ct chartType) {
	kinds := make([]colKind, len(res.Columns))
	for i, c := range res.Columns {
		kinds[i] = classifyColumn(c.Type)
	}

	xIdx = -1
	for i, k := range kinds {
		if k == kindTime {
			xIdx = i
			ct = chartLine
			break
		}
	}
	if xIdx < 0 {
		for i, k := range kinds {
			if k == kindCategory || k == kindUnknown {
				xIdx = i
				ct = chartBar
				break
			}
		}
	}
	// Still nothing? Fall back to the first column.
	// If the result has any numeric column, that X will still produce a
	// plottable chart (the numeric Y we pick below is a different column).
	if xIdx < 0 && len(res.Columns) > 0 {
		xIdx = 0
		ct = chartBar
	}

	for i, k := range kinds {
		if i == xIdx {
			continue
		}
		if k == kindNumeric {
			yIdxs = []int{i}
			break
		}
	}
	return xIdx, yIdxs, ct
}

// isChartable reports whether the result has at least one numeric column.
func isChartable(res *conn.QueryResult) bool {
	for _, c := range res.Columns {
		if classifyColumn(c.Type) == kindNumeric {
			return true
		}
	}
	return false
}
