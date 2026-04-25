package tui

import "testing"

// AtFirstLine should return false when the cursor is on a wrapped row of
// logical line 0 (i.e. there's still content visually above it). Otherwise
// Up-arrow incorrectly fires history navigation and the user loses their
// in-progress input.
func TestAtFirstLine_WrappedRow(t *testing.T) {
	m := NewInputModel("> ", nil)
	m.SetWidth(20)
	m.SetValue("SELECT this_is_a_very_long_column_name FROM users")
	m.textarea.MoveToEnd() // cursor on a wrapped portion of logical line 0

	if m.textarea.Line() != 0 {
		t.Fatalf("setup wrong: Line()=%d", m.textarea.Line())
	}
	li := m.textarea.LineInfo()
	t.Logf("Line=%d Col=%d RowOffset=%d Height=%d",
		m.textarea.Line(), m.textarea.Column(), li.RowOffset, li.Height)

	if li.RowOffset == 0 {
		t.Fatalf("setup wrong: RowOffset=0 means line did not wrap; widen width or lengthen text")
	}
	if m.AtFirstLine() {
		t.Errorf("AtFirstLine()=true on wrapped row %d/%d — Up will trigger history nav",
			li.RowOffset, li.Height)
	}
}

// AtLastLine equivalent: cursor on a wrapped row that's NOT the last visual
// row of the last logical line should NOT report at-last (otherwise Down
// triggers history nav instead of moving down within the input).
func TestAtLastLine_WrappedRow(t *testing.T) {
	m := NewInputModel("> ", nil)
	m.SetWidth(20)
	m.SetValue("short\nthis_is_a_very_long_column_name FROM t")
	// Move cursor to start of the wrapped logical line 1 — first visual row
	// of a multi-row line.
	m.textarea.MoveToEnd()
	li := m.textarea.LineInfo()
	if li.Height < 2 {
		t.Fatalf("setup wrong: line did not wrap (Height=%d)", li.Height)
	}
	// Walk up to the first visual row of logical line 1.
	for m.textarea.LineInfo().RowOffset > 0 {
		m.textarea.CursorUp()
	}
	if m.AtLastLine() {
		t.Errorf("AtLastLine()=true on a non-final wrapped row — Down will trigger history nav")
	}
}
