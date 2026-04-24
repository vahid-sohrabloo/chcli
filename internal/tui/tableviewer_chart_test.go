package tui

import (
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func TestChartSubview_AutoDetectOnNew(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "ts", Type: "DateTime"},
			{Name: "v", Type: "Float64"},
		},
		Rows: [][]string{
			{"2026-04-24 10:00:00", "1.0"},
			{"2026-04-24 10:01:00", "2.0"},
		},
	}
	c := newChartSubview(res, 80, 24, false)
	if c.xIdx != 0 {
		t.Errorf("xIdx = %d, want 0", c.xIdx)
	}
	if len(c.yIdxs) != 1 || c.yIdxs[0] != 1 {
		t.Errorf("yIdxs = %v, want [1]", c.yIdxs)
	}
	if c.chartType != chartLine {
		t.Errorf("chartType = %v, want chartLine", c.chartType)
	}
	if len(c.parsed.xTimes) != 2 {
		t.Errorf("parsed.xTimes len = %d, want 2", len(c.parsed.xTimes))
	}
}

func TestChartSubview_NonChartable(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{{Name: "n", Type: "String"}},
		Rows:    [][]string{{"a"}, {"b"}},
	}
	c := newChartSubview(res, 80, 24, false)
	if len(c.yIdxs) != 0 {
		t.Errorf("yIdxs = %v, want empty", c.yIdxs)
	}
	if !c.nonChartable {
		t.Error("nonChartable = false, want true")
	}
}
