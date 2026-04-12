package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/vahid-sohrabloo/chcli/internal/completer"
)

const maxVisible = 10

// CompletionModel manages the state and rendering of the autocomplete popup.
type CompletionModel struct {
	items    []completer.Completion
	maxWidth int
	cursor   int
	visible  bool
	offset   int
}

func NewCompletionModel() *CompletionModel   { return &CompletionModel{} }
func (m *CompletionModel) SetMaxWidth(w int) { m.maxWidth = w }
func (m *CompletionModel) Len() int          { return len(m.items) }
func (m *CompletionModel) Visible() bool     { return m.visible && len(m.items) > 0 }
func (m *CompletionModel) Hide()             { m.visible = false }

func (m *CompletionModel) SetItems(items []completer.Completion) {
	m.items = items
	m.cursor = 0
	m.offset = 0
}

func (m *CompletionModel) Show() {
	if len(m.items) > 0 {
		m.visible = true
	}
}

func (m *CompletionModel) Next() {
	if len(m.items) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(m.items) {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor >= m.offset+maxVisible {
		m.offset = m.cursor - maxVisible + 1
	}
}

func (m *CompletionModel) Prev() {
	if len(m.items) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.items) - 1
		if len(m.items) > maxVisible {
			m.offset = len(m.items) - maxVisible
		} else {
			m.offset = 0
		}
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

func (m *CompletionModel) Selected() string {
	if len(m.items) == 0 {
		return ""
	}
	return m.items[m.cursor].Text
}

func (m *CompletionModel) SelectedKind() completer.CompletionKind {
	if len(m.items) == 0 {
		return completer.KindKeyword
	}
	return m.items[m.cursor].Kind
}

// --- Styles ---

// Popup styles using ANSI colors (0-15) for backgrounds — adapts to terminal theme.
// ANSI 0 = terminal's black, 4 = blue, 7 = white, 8 = bright black (gray).
var (
	selRow = lipgloss.NewStyle().
		Background(lipgloss.Color("4")).  // terminal blue
		Foreground(lipgloss.Color("15")). // bright white
		Bold(true)

	normalRow = lipgloss.NewStyle().
			Background(lipgloss.Color("0")). // terminal black
			Foreground(lipgloss.Color("7"))  // terminal white

	footerRow = lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("8"))
)

func kindIcon(k completer.CompletionKind) string {
	switch k {
	case completer.KindTable:
		return "T"
	case completer.KindColumn:
		return "C"
	case completer.KindFunction:
		return "f()"
	case completer.KindAggFunction:
		return "agg"
	case completer.KindDatabase:
		return "D"
	case completer.KindKeyword:
		return "K"
	case completer.KindSnippet:
		return "S"
	case completer.KindEngine:
		return "E"
	case completer.KindSetting:
		return "s"
	default:
		return " "
	}
}

const maxDetailLen = 30

// ViewAt renders the popup at screenX offset.
func (m *CompletionModel) ViewAt(screenX int) string {
	if !m.Visible() {
		return ""
	}

	end := min(m.offset+maxVisible, len(m.items))

	// Calculate column widths.
	maxNameLen := 0
	for i := m.offset; i < end; i++ {
		if len(m.items[i].Text) > maxNameLen {
			maxNameLen = len(m.items[i].Text)
		}
	}
	if maxNameLen < 12 {
		maxNameLen = 12
	}

	hasDetail := false
	for i := m.offset; i < end; i++ {
		if m.items[i].Detail != "" {
			hasDetail = true
			break
		}
	}

	// Calculate detail width based on available space.
	detailWidth := maxDetailLen
	nameColWidth := maxNameLen + 6
	if m.maxWidth > 0 {
		available := m.maxWidth - screenX - nameColWidth - 6 // border + padding
		if available < 10 {
			hasDetail = false
		} else if available < detailWidth {
			detailWidth = available
		}
	}

	// Build rows.
	// Total row width for consistent background fill.
	totalWidth := nameColWidth + 2
	if hasDetail {
		totalWidth += detailWidth + 3
	}

	var rows []string
	for i := m.offset; i < end; i++ {
		item := m.items[i]
		isSel := i == m.cursor

		icon := fmt.Sprintf("%-3s", kindIcon(item.Kind))
		name := fmt.Sprintf("%-*s", maxNameLen, item.Text)
		rowText := " " + icon + " " + name

		if hasDetail {
			detail := truncateStr(item.Detail, detailWidth)
			padded := fmt.Sprintf("%-*s", detailWidth, detail)
			rowText += "  " + padded
		}

		// Use Width() to force background fill across entire row.
		if isSel {
			rows = append(rows, selRow.Width(totalWidth).Render(rowText))
		} else {
			rows = append(rows, normalRow.Width(totalWidth).Render(rowText))
		}
	}

	// Footer with count and keybind hints.
	var footer string
	countStr := fmt.Sprintf("%d/%d", m.cursor+1, len(m.items))
	hintsStr := "↑↓ navigate  Tab accept  Esc close"
	gap := nameColWidth
	if hasDetail {
		gap += detailWidth + 3
	}
	gap -= len(countStr) - len(hintsStr) - 2
	if gap < 1 {
		gap = 1
	}
	footer = footerRow.Width(totalWidth).Render(
		" " + countStr + strings.Repeat(" ", max(1, gap)) + hintsStr,
	)

	// Wrap in a bordered box with background fill.
	content := strings.Join(rows, "\n") + "\n" + footer
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Background(lipgloss.Color("0")).
		Width(totalWidth + 2). // ensure box fills
		Render(content)

	indent := strings.Repeat(" ", screenX)
	lines := strings.Split(box, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}

	return strings.Join(lines, "\n")
}

func truncateStr(s string, maxLen int) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
