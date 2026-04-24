package tui

import (
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

const tvMaxCellWidth = 50

type tableSubview struct {
	table     table.Model
	allCols   []table.Column
	allRows   []table.Row
	colOffset int
}

func newTableSubview(result *conn.QueryResult) *tableSubview {
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

	return &tableSubview{allCols: cols, allRows: rows}
}

func (t *tableSubview) rebuild(width, height int, isWarp bool) {
	cols := t.allCols
	rows := t.allRows
	if t.colOffset > 0 && t.colOffset < len(cols) {
		cols = cols[t.colOffset:]
		newRows := make([]table.Row, len(rows))
		for i, r := range rows {
			if t.colOffset < len(r) {
				newRows[i] = r[t.colOffset:]
			}
		}
		rows = newRows
	}

	h := height - 3
	if isWarp {
		h -= 4
	}
	if h < 5 {
		h = 5
	}

	t.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(h),
		table.WithWidth(width),
		table.WithStyles(TableStyles()),
		table.WithFocused(true),
	)
}

// Update returns (handled, cmd). handled=false means "parent should apply fallbacks".
func (t *tableSubview) Update(msg tea.Msg, width, height int, isWarp bool) (bool, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.Code {
		case tea.KeyLeft:
			if t.colOffset > 0 {
				t.colOffset--
				t.rebuild(width, height, isWarp)
			}
			return true, nil
		case tea.KeyRight:
			if t.colOffset < len(t.allCols)-1 {
				t.colOffset++
				t.rebuild(width, height, isWarp)
			}
			return true, nil
		}
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return true, cmd
}

func (t *tableSubview) View() string {
	return t.table.View()
}

func (t *tableSubview) ColOffset() int { return t.colOffset }
func (t *tableSubview) TotalCols() int { return len(t.allCols) }
