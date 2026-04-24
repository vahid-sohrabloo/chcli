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
	chart *chartSubview
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
			if tv.mode == modeChart && tv.chart != nil && tv.chart.pickerOpen() {
				tv.chart.closePicker()
				return false, nil
			}
			return true, nil
		case 'c':
			if tv.mode == modeTable {
				if tv.chart == nil {
					tv.chart = newChartSubview(tv.result, tv.width, tv.height, tv.isWarp)
					if tv.chart.nonChartable {
						tv.chart.openPicker()
					}
				}
				tv.mode = modeChart
			} else {
				tv.mode = modeTable
			}
			return false, nil
		}
	}
	switch tv.mode {
	case modeChart:
		if tv.chart != nil {
			_, c := tv.chart.Update(msg, tv.width, tv.height, tv.isWarp)
			return false, c
		}
	case modeTable:
		_, c := tv.table.Update(msg, tv.width, tv.height, tv.isWarp)
		return false, c
	}
	return false, nil
}

func (tv *tableViewerModel) View() string {
	t := ActiveTheme
	label := "  TABLE VIEWER"
	if tv.mode == modeChart {
		label = "  CHART VIEWER"
	}
	queryEcho := collapseWhitespace(tv.query)
	if budget := tv.width - len(label) - 4; budget > 0 && len(queryEcho) > budget {
		queryEcho = queryEcho[:budget-1] + "…"
	}
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.AccentBlue)).
		Bold(true).
		Render(label) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
			Render("  "+queryEcho)

	var body, footer string
	switch tv.mode {
	case modeChart:
		body = tv.chart.View()
		footer = tv.chart.footer(tv)
	default:
		body = tv.table.View()
		footer = tv.tableFooter()
	}
	return header + "\n" + body + "\n" + footer
}

func (tv *tableViewerModel) tableFooter() string {
	t := ActiveTheme
	out := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
		Render("  "+tv.footer) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).
			Render("  │  ↑↓ scroll  ←→ columns  c chart  q/Esc exit")
	if tv.table != nil && tv.table.ColOffset() > 0 {
		out += lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentYellow)).
			Render(fmt.Sprintf("  │  col %d/%d", tv.table.ColOffset()+1, tv.table.TotalCols()))
	}
	return out
}
