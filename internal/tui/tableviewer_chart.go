package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

type chartSubview struct {
	result        *conn.QueryResult
	width, height int
	isWarp        bool
}

func newChartSubview(res *conn.QueryResult, width, height int, isWarp bool) *chartSubview {
	return &chartSubview{result: res, width: width, height: height, isWarp: isWarp}
}

func (c *chartSubview) Update(msg tea.Msg, width, height int, isWarp bool) (bool, tea.Cmd) {
	c.width, c.height, c.isWarp = width, height, isWarp
	return true, nil
}

func (c *chartSubview) View() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.TextMuted)).
		Render(fmt.Sprintf("  (chart placeholder — %d rows)", len(c.result.Rows)))
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
