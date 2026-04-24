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

func TestPicker_OpenAndCancel(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "ts", Type: "DateTime"},
			{Name: "a", Type: "Float64"},
			{Name: "b", Type: "Float64"},
		},
		Rows: [][]string{{"2026-04-24 10:00:00", "1", "2"}},
	}
	c := newChartSubview(res, 80, 24, false)
	origX, origY := c.xIdx, append([]int(nil), c.yIdxs...)

	c.openPicker()
	if !c.pickerOpen() {
		t.Fatal("pickerOpen = false after openPicker")
	}

	// move cursor and cancel: no state change
	c.picker.cursor = 2
	c.picker.xIdx = 1
	c.cancelPicker()
	if c.pickerOpen() {
		t.Fatal("picker still open after cancel")
	}
	if c.xIdx != origX {
		t.Errorf("xIdx changed on cancel: got %d, want %d", c.xIdx, origX)
	}
	if len(c.yIdxs) != len(origY) || c.yIdxs[0] != origY[0] {
		t.Errorf("yIdxs changed on cancel: got %v, want %v", c.yIdxs, origY)
	}
}

func TestPicker_CommitUpdatesSelection(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "ts", Type: "DateTime"},
			{Name: "a", Type: "Float64"},
			{Name: "b", Type: "Float64"},
		},
		Rows: [][]string{{"2026-04-24 10:00:00", "1", "2"}},
	}
	c := newChartSubview(res, 80, 24, false)
	c.openPicker()
	c.picker.xIdx = 0
	c.picker.ySet = map[int]bool{1: true, 2: true}
	c.commitPicker()

	if c.pickerOpen() {
		t.Fatal("picker still open after commit")
	}
	if c.xIdx != 0 {
		t.Errorf("xIdx = %d, want 0", c.xIdx)
	}
	if len(c.yIdxs) != 2 {
		t.Errorf("yIdxs = %v, want 2 elements", c.yIdxs)
	}
	// reparse should have produced one row with two Y series
	if len(c.parsed.ys) != 2 || len(c.parsed.ys[0]) != 1 {
		t.Errorf("parsed not refreshed; ys=%v", c.parsed.ys)
	}
}

func TestPicker_RejectsSameColumnAsXAndY(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "a", Type: "Float64"},
			{Name: "b", Type: "Float64"},
		},
		Rows: [][]string{{"1", "2"}},
	}
	c := newChartSubview(res, 80, 24, false)
	c.openPicker()
	c.picker.xIdx = 0
	// toggleY on the current X row should no-op
	c.picker.toggleY(0)
	if c.picker.ySet[0] {
		t.Error("toggleY on X row should be rejected, but ySet[0]=true")
	}
}

func TestPicker_RejectsNonNumericY(t *testing.T) {
	res := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "s", Type: "String"},
			{Name: "n", Type: "UInt32"},
		},
		Rows: [][]string{{"x", "1"}},
	}
	c := newChartSubview(res, 80, 24, false)
	c.openPicker()
	c.picker.toggleY(0) // s is String, not numeric
	if c.picker.ySet[0] {
		t.Error("toggleY on non-numeric should be rejected")
	}
}
