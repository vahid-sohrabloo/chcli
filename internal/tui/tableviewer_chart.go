package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

type chartSubview struct {
	result *conn.QueryResult
	kinds  []colKind

	xIdx         int
	yIdxs        []int
	chartType    chartType
	nonChartable bool
	parsed       parsedData

	width, height int
	isWarp        bool
}

func newChartSubview(res *conn.QueryResult, width, height int, isWarp bool) *chartSubview {
	c := &chartSubview{
		result: res,
		width:  width,
		height: height,
		isWarp: isWarp,
	}
	c.kinds = make([]colKind, len(res.Columns))
	for i, col := range res.Columns {
		c.kinds[i] = classifyColumn(col.Type)
	}
	c.xIdx, c.yIdxs, c.chartType = autoDetect(res)
	c.nonChartable = !isChartable(res)
	c.reparse()
	return c
}

// reparse rebuilds parsedData from the current xIdx/yIdxs selection.
// Skips when the result is non-chartable or no Ys selected.
func (c *chartSubview) reparse() {
	c.parsed = parsedData{}
	if c.nonChartable || len(c.yIdxs) == 0 {
		return
	}
	c.parsed = parseRows(c.result, c.xIdx, c.yIdxs)
}

func (c *chartSubview) Update(msg tea.Msg, width, height int, isWarp bool) (bool, tea.Cmd) {
	c.width, c.height, c.isWarp = width, height, isWarp
	return true, nil
}

func (c *chartSubview) View() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.TextMuted)).
		Render(fmt.Sprintf("  (chart placeholder — X=%d Y=%v)", c.xIdx, c.yIdxs))
}

func (c *chartSubview) footer(tv *tableViewerModel) string {
	t := ActiveTheme
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary)).
		Render("  "+tv.footer) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).
			Render("  │  c table  q/Esc exit")
}

func (c *chartSubview) pickerOpen() bool { return false }
func (c *chartSubview) closePicker()     {}
