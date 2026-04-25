package tui

import "testing"

// ReplaceWordAtCursor must land the cursor at the end of the inserted text on
// the same logical line where the prefix was, regardless of whether earlier
// lines have soft-wrapped to multiple visual rows. Previously the code rebuilt
// the value via SetValue and stepped down with CursorDown — but bubbles/v2's
// CursorDown moves through visual rows, not logical lines, so wrapped earlier
// content absorbed the steps and the cursor "jumped back" to an earlier line.
func TestReplaceWordAtCursor_MultilineThirdLine(t *testing.T) {
	m := NewInputModel("> ", nil)
	m.SetWidth(80)
	m.SetValue("SELECT *\nFROM users\nWHERE na")
	m.textarea.MoveToEnd()

	m.ReplaceWordAtCursor(2, "name")

	if m.Value() != "SELECT *\nFROM users\nWHERE name" {
		t.Errorf("Value()=%q", m.Value())
	}
	if got := m.textarea.Line(); got != 2 {
		t.Errorf("Line()=%d want 2", got)
	}
	if got, want := m.textarea.Column(), len([]rune("WHERE name")); got != want {
		t.Errorf("Column()=%d want %d", got, want)
	}
}

func TestReplaceWordAtCursor_WrappedFirstLine(t *testing.T) {
	m := NewInputModel("> ", nil)
	m.SetWidth(20) // narrow → first logical line wraps to multiple visual rows

	m.SetValue("SELECT this_is_a_very_long_column_name\nFROM t\nWHERE na")
	m.textarea.MoveToEnd()

	m.ReplaceWordAtCursor(2, "name")

	if got := m.textarea.Line(); got != 2 {
		t.Errorf("Line()=%d want 2 — cursor jumped back to a wrapped earlier line", got)
	}
	if got, want := m.textarea.Column(), len([]rune("WHERE name")); got != want {
		t.Errorf("Column()=%d want %d", got, want)
	}
}
