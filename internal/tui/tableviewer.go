package tui

import (
	"fmt"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

const tvMaxCellWidth = 50

// tableViewerModel is a full-screen interactive table viewer.
// Activated by F2 after running a query. Esc to exit back to input.
type tableViewerModel struct {
	table     table.Model
	query     string
	footer    string
	allCols   []table.Column
	allRows   []table.Row
	colOffset int
	width     int
	height    int
	isWarp    bool
}

func newTableViewer(result *conn.QueryResult, query string, width, height int) *tableViewerModel {
	cols := make([]table.Column, len(result.Columns))
	for i, c := range result.Columns {
		w := len(c.Name)
		for _, row := range result.Rows {
			if i < len(row) && len(row[i]) > w {
				w = len(row[i])
			}
		}
		if w > tvMaxCellWidth {
			w = tvMaxCellWidth
		}
		cols[i] = table.Column{Title: c.Name, Width: w}
	}

	rows := make([]table.Row, len(result.Rows))
	for i, row := range result.Rows {
		r := make(table.Row, len(row))
		for j, cell := range row {
			if len(cell) > tvMaxCellWidth {
				r[j] = cell[:tvMaxCellWidth-1] + "…"
			} else {
				r[j] = cell
			}
		}
		rows[i] = r
	}

	footer := formatResultFooter(result.TotalRows, result.Elapsed, result.Truncated)

	tv := &tableViewerModel{
		query:   query,
		footer:  footer,
		allCols: cols,
		allRows: rows,
		width:   width,
		height:  height,
	}
	tv.rebuildTable()
	return tv
}

func (tv *tableViewerModel) rebuildTable() {
	cols := tv.allCols
	rows := tv.allRows
	if tv.colOffset > 0 && tv.colOffset < len(cols) {
		cols = cols[tv.colOffset:]
		newRows := make([]table.Row, len(rows))
		for i, r := range rows {
			if tv.colOffset < len(r) {
				newRows[i] = r[tv.colOffset:]
			}
		}
		rows = newRows
	}

	// Table gets almost full height: total - header(1) - footer(1) - hint(1).
	h := tv.height - 3
	if tv.isWarp {
		h -= 4 // extra padding for Warp's block input
	}
	if h < 5 {
		h = 5
	}

	tv.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(h),
		table.WithWidth(tv.width),
		table.WithStyles(TableStyles()),
		table.WithFocused(true),
	)
}

func (tv *tableViewerModel) Update(msg tea.Msg) (closed bool, cmd tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.Code {
		case tea.KeyEscape, 'q':
			return true, nil
		case tea.KeyLeft:
			if tv.colOffset > 0 {
				tv.colOffset--
				tv.rebuildTable()
			}
			return false, nil
		case tea.KeyRight:
			if tv.colOffset < len(tv.allCols)-1 {
				tv.colOffset++
				tv.rebuildTable()
			}
			return false, nil
		}
	}

	// Forward to table for scrolling.
	var cmd2 tea.Cmd
	tv.table, cmd2 = tv.table.Update(msg)
	return false, cmd2
}

func (tv *tableViewerModel) View() string {
	t := ActiveTheme

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.AccentBlue)).
		Bold(true).
		Render("  TABLE VIEWER") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
			Render("  "+tv.query)

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
		Render("  "+tv.footer) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).
			Render("  │  ↑↓ scroll  ←→ columns  q/Esc exit")

	if tv.colOffset > 0 {
		footer += lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentYellow)).
			Render(fmt.Sprintf("  │  col %d/%d", tv.colOffset+1, len(tv.allCols)))
	}

	return header + "\n" + tv.table.View() + "\n" + footer
}
