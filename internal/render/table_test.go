package render

import (
	"strings"
	"testing"
)

func TestRenderTable(t *testing.T) {
	columns := []string{"Name", "Age", "City"}
	rows := [][]string{
		{"Alice", "30", "New York"},
		{"Bob", "25", "London"},
	}

	got := RenderTable(columns, rows, 80)

	for _, col := range columns {
		if !strings.Contains(got, col) {
			t.Errorf("expected output to contain column %q, got:\n%s", col, got)
		}
	}

	for _, row := range rows {
		for _, val := range row {
			if !strings.Contains(got, val) {
				t.Errorf("expected output to contain value %q, got:\n%s", val, got)
			}
		}
	}
}

func TestRenderTableEmpty(t *testing.T) {
	columns := []string{"ID", "Status"}
	got := RenderTable(columns, nil, 80)

	for _, col := range columns {
		if !strings.Contains(got, col) {
			t.Errorf("expected output to contain header %q even with no rows, got:\n%s", col, got)
		}
	}
}

func TestRenderTableNullValues(t *testing.T) {
	columns := []string{"Key", "Value"}
	rows := [][]string{
		{"missing_field", "NULL"},
		{"present_field", "hello"},
	}

	got := RenderTable(columns, rows, 80)

	if !strings.Contains(got, "NULL") {
		t.Errorf("expected output to contain %q, got:\n%s", "NULL", got)
	}
}
