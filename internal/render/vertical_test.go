package render

import (
	"strings"
	"testing"
)

func TestRenderVertical(t *testing.T) {
	columns := []string{"id", "name", "email"}
	rows := [][]string{
		{"1", "Alice", "alice@example.com"},
		{"2", "Bob", "bob@example.com"},
	}

	got := RenderVertical(columns, rows)

	// Check for row markers
	if !strings.Contains(got, "Row 1") {
		t.Errorf("expected output to contain %q, got:\n%s", "Row 1", got)
	}
	if !strings.Contains(got, "Row 2") {
		t.Errorf("expected output to contain %q, got:\n%s", "Row 2", got)
	}

	// Check for column labels
	for _, col := range columns {
		if !strings.Contains(got, col) {
			t.Errorf("expected output to contain column label %q, got:\n%s", col, got)
		}
	}

	// Check for cell values
	for _, row := range rows {
		for _, val := range row {
			if !strings.Contains(got, val) {
				t.Errorf("expected output to contain value %q, got:\n%s", val, got)
			}
		}
	}
}
