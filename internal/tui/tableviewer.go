package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

type viewerMode int

const (
	modeTable viewerMode = iota
	modeChart
)

type tableViewerModel struct {
	result *conn.QueryResult
	query  string
	footer string

	width, height int
	isWarp        bool

	mode  viewerMode
	table *tableSubview
}

func newTableViewer(result *conn.QueryResult, query string, width, height int) *tableViewerModel {
	tv := &tableViewerModel{
		result: result,
		query:  query,
		footer: formatResultFooter(result.TotalRows, result.Elapsed, result.Truncated),
		width:  width,
		height: height,
		mode:   modeTable,
		table:  newTableSubview(result),
	}
	tv.table.rebuild(width, height, false)
	return tv
}

func (tv *tableViewerModel) rebuildTable() {
	tv.table.rebuild(tv.width, tv.height, tv.isWarp)
}

func (tv *tableViewerModel) Update(msg tea.Msg) (closed bool, cmd tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.Code {
		case tea.KeyEscape, 'q':
			return true, nil
		}
	}
	switch tv.mode {
	case modeTable:
		_, c := tv.table.Update(msg, tv.width, tv.height, tv.isWarp)
		return false, c
	}
	return false, nil
}

func (tv *tableViewerModel) View() string {
	t := ActiveTheme
	label := "  TABLE VIEWER"
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.AccentBlue)).
		Bold(true).
		Render(label) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
			Render("  "+tv.query)

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
		Render("  "+tv.footer) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).
			Render("  │  ↑↓ scroll  ←→ columns  q/Esc exit")

	if tv.table != nil && tv.table.ColOffset() > 0 {
		footer += lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentYellow)).
			Render(fmt.Sprintf("  │  col %d/%d", tv.table.ColOffset()+1, tv.table.TotalCols()))
	}

	return header + "\n" + tv.table.View() + "\n" + footer
}
