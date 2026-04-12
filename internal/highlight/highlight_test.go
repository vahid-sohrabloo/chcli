package highlight

import (
	"regexp"
	"strings"
	"testing"
)

// ansiEscapeRe matches ANSI escape sequences.
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes all ANSI color/style escape sequences from s.
func stripANSI(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

func TestHighlight(t *testing.T) {
	h := NewHighlighter("monokai")
	input := "SELECT id FROM users WHERE id = 1"
	out := h.Highlight(input)

	// Output should contain ANSI escapes (i.e. it was actually highlighted).
	if !strings.Contains(out, "\x1b[") {
		t.Error("expected ANSI escape sequences in output, got none")
	}

	// Output should still visibly contain the keyword SELECT.
	if !strings.Contains(out, "SELECT") {
		t.Error("expected 'SELECT' to appear in output")
	}
}

func TestHighlightPreservesText(t *testing.T) {
	h := NewHighlighter("monokai")
	input := "SELECT id FROM users WHERE id = 1"
	out := h.Highlight(input)

	stripped := stripANSI(out)
	if stripped != input {
		t.Errorf("stripped output does not equal input:\nwant: %q\n got: %q", input, stripped)
	}
}

func TestSetTheme(t *testing.T) {
	h := NewHighlighter("monokai")
	h.SetTheme("github")

	input := "SELECT id FROM users"
	out := h.Highlight(input)

	// Should still produce highlighted output after theme change.
	if !strings.Contains(out, "\x1b[") {
		t.Error("expected ANSI escape sequences after theme change, got none")
	}
}
