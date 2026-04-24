package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

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

func TestParseRows_TimeXNumericY(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "ts", Type: "DateTime"},
			{Name: "count", Type: "UInt64"},
			{Name: "mean", Type: "Float64"},
		},
		Rows: [][]string{
			{"2026-04-24 10:00:00", "42", "1.5"},
			{"2026-04-24 10:01:00", "NULL", "2.5"},
			{"2026-04-24 10:02:00", "100", "not-a-number"},
			{"2026-04-24 10:03:00", "7", "3.0"},
		},
	}
	p := parseRows(res, 0, []int{1, 2})

	if got := len(p.xTimes); got != 2 {
		t.Fatalf("xTimes len = %d, want 2 (two rows should be dropped)", got)
	}
	wantT, _ := time.Parse("2006-01-02 15:04:05", "2026-04-24 10:00:00")
	if !p.xTimes[0].Equal(wantT) {
		t.Errorf("xTimes[0] = %v, want %v", p.xTimes[0], wantT)
	}
	if p.ys[0][0] != 42 || p.ys[0][1] != 7 {
		t.Errorf("ys[0] = %v, want [42 7]", p.ys[0])
	}
	if p.ys[1][0] != 1.5 || p.ys[1][1] != 3.0 {
		t.Errorf("ys[1] = %v, want [1.5 3]", p.ys[1])
	}
	if p.dropped != 2 {
		t.Errorf("dropped = %d, want 2", p.dropped)
	}
}

func TestParseRows_CategoryX(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "country", Type: "String"},
			{Name: "n", Type: "UInt32"},
		},
		Rows: [][]string{
			{"US", "100"},
			{"DE", "50"},
			{"FR", "25"},
		},
	}
	p := parseRows(res, 0, []int{1})
	if got := p.xCats; len(got) != 3 || got[0] != "US" || got[2] != "FR" {
		t.Errorf("xCats = %v, want [US DE FR]", got)
	}
	if p.ys[0][0] != 100 || p.ys[0][2] != 25 {
		t.Errorf("ys[0] = %v", p.ys[0])
	}
	if p.dropped != 0 {
		t.Errorf("dropped = %d, want 0", p.dropped)
	}
}

func TestParseRows_DateAndDateTime64(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "d", Type: "Date"},
			{Name: "v", Type: "Int32"},
		},
		Rows: [][]string{{"2026-04-24", "1"}},
	}
	p := parseRows(res, 0, []int{1})
	if len(p.xTimes) != 1 {
		t.Fatalf("xTimes len = %d, want 1", len(p.xTimes))
	}

	res2 := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "ts", Type: "DateTime64(3)"},
			{Name: "v", Type: "Int32"},
		},
		Rows: [][]string{{"2026-04-24 10:00:00.123", "1"}},
	}
	p2 := parseRows(res2, 0, []int{1})
	if len(p2.xTimes) != 1 {
		t.Fatalf("DateTime64 xTimes len = %d, want 1", len(p2.xTimes))
	}
}

func TestAutoDetectDefaults(t *testing.T) {
	type result struct {
		xIdx     int
		yIdxs    []int
		chartKey chartType
	}
	mkRes := func(cols ...string) *conn.QueryResult {
		out := &conn.QueryResult{}
		for _, c := range cols {
			parts := strings.SplitN(c, ":", 2)
			out.Columns = append(out.Columns, conn.ResultColumn{Name: parts[0], Type: parts[1]})
		}
		return out
	}

	tests := []struct {
		name string
		res  *conn.QueryResult
		want result
	}{
		{
			name: "time + 1 numeric",
			res:  mkRes("ts:DateTime", "count:UInt64"),
			want: result{xIdx: 0, yIdxs: []int{1}, chartKey: chartLine},
		},
		{
			name: "time + multi numeric",
			res:  mkRes("ts:DateTime", "p50:Float64", "p95:Float64", "p99:Float64"),
			want: result{xIdx: 0, yIdxs: []int{1}, chartKey: chartLine},
		},
		{
			name: "category + numeric",
			res:  mkRes("country:String", "n:UInt32"),
			want: result{xIdx: 0, yIdxs: []int{1}, chartKey: chartBar},
		},
		{
			name: "only numerics (no time/category) -> first col as X, second as Y",
			res:  mkRes("a:Float64", "b:Float64"),
			want: result{xIdx: 0, yIdxs: []int{1}, chartKey: chartBar},
		},
		{
			name: "all string (no numeric) -> non-chartable",
			res:  mkRes("name:String", "city:String"),
			want: result{xIdx: 0, yIdxs: nil, chartKey: chartBar},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			x, ys, ct := autoDetect(tc.res)
			if x != tc.want.xIdx || ct != tc.want.chartKey {
				t.Errorf("got xIdx=%d chartType=%v, want %d %v",
					x, ct, tc.want.xIdx, tc.want.chartKey)
			}
			if len(ys) != len(tc.want.yIdxs) {
				t.Fatalf("yIdxs len = %d, want %d", len(ys), len(tc.want.yIdxs))
			}
			for i := range ys {
				if ys[i] != tc.want.yIdxs[i] {
					t.Errorf("yIdxs[%d] = %d, want %d", i, ys[i], tc.want.yIdxs[i])
				}
			}
		})
	}
}

func TestIsChartable(t *testing.T) {
	mk := func(types ...string) *conn.QueryResult {
		r := &conn.QueryResult{}
		for i, ty := range types {
			r.Columns = append(r.Columns, conn.ResultColumn{
				Name: "c" + strconv.Itoa(i), Type: ty,
			})
		}
		return r
	}
	cases := []struct {
		res  *conn.QueryResult
		want bool
	}{
		{mk("DateTime", "UInt64"), true},
		{mk("String", "UInt32"), true},
		{mk("String"), false},
		{mk("String", "String"), false},
		{mk("Int64"), true},
	}
	for i, c := range cases {
		if got := isChartable(c.res); got != c.want {
			t.Errorf("case %d: isChartable=%v, want %v", i, got, c.want)
		}
	}
}
