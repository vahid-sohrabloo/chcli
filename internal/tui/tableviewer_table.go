package tui

import (
	"fmt"

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

// rebuild reconstructs the inner table.Model from current state. Preserves
// the cursor position across rebuilds so horizontal scroll (Left/Right)
// doesn't reset the highlighted row to 0.
func (t *tableSubview) rebuild(width, height int, isWarp bool) {
	// Build the visible columns: a sticky "#" column on the left, then the
	// data columns starting at colOffset.
	rowNumWidth := max(numDigits(len(t.allRows)), 2)
	cols := make([]table.Column, 0, len(t.allCols)+1)
	cols = append(cols, table.Column{Title: "#", Width: rowNumWidth})
	dataCols := t.allCols
	if t.colOffset > 0 && t.colOffset < len(dataCols) {
		dataCols = dataCols[t.colOffset:]
	}
	cols = append(cols, dataCols...)

	rows := make([]table.Row, len(t.allRows))
	for i, r := range t.allRows {
		out := make(table.Row, 0, len(r)+1)
		out = append(out, fmt.Sprintf("%*d", rowNumWidth, i+1))
		dataCells := r
		if t.colOffset > 0 && t.colOffset < len(dataCells) {
			dataCells = dataCells[t.colOffset:]
		}
		out = append(out, dataCells...)
		rows[i] = out
	}

	h := height - 3
	if isWarp {
		h -= 4
	}
	if h < 5 {
		h = 5
	}

	prevCursor := max(t.table.Cursor(), 0)

	t.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(h),
		table.WithWidth(width),
		table.WithStyles(TableStyles()),
		table.WithFocused(true),
	)
	if prevCursor > 0 && prevCursor < len(rows) {
		t.table.SetCursor(prevCursor)
	}
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

func numDigits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}
