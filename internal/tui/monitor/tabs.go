package monitor

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/tui"
)

// renderTabBar draws the single-row bar of tab cells. The active tab has an
// inverse style (AccentBlue background, BgDark foreground). Inactive tabs
// are TextMuted.
func (m *Model) renderTabBar() string {
	theme := tui.ActiveTheme
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.BgDark)).
		Background(lipgloss.Color(theme.AccentBlue)).
		Bold(true)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted))
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted)).Render("│")

	var sb strings.Builder
	for i, t := range m.tabs {
		cell := " " + runeDigit(i+1) + " " + t.Title() + " "
		if i == m.activeTab {
			sb.WriteString(activeStyle.Render(cell))
		} else {
			sb.WriteString(inactiveStyle.Render(cell))
		}
		if i < len(m.tabs)-1 {
			sb.WriteString(divider)
		}
	}
	return sb.String()
}

func runeDigit(n int) string {
	if n <= 0 || n > 9 {
		return "?"
	}
	return string(rune('0' + n))
}

// renderTitleRow shows the tool name + a right-aligned help hint.
func (m *Model) renderTitleRow() string {
	theme := tui.ActiveTheme
	name := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true).Render("chmon")
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted)).Render("? help · q quit")
	pad := m.width - lipgloss.Width(name) - lipgloss.Width(hint) - 2
	if pad < 1 {
		pad = 1
	}
	return " " + name + strings.Repeat(" ", pad) + hint
}

// renderHelpOverlay draws a centered modal with global + active-tab keys.
func (m *Model) renderHelpOverlay() string {
	theme := tui.ActiveTheme
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted))

	global := []keyHint{
		{"1-9", "switch tab"},
		{"Tab / Shift+Tab", "next / prev tab"},
		{"?", "toggle help"},
		{"q / Esc", "quit"},
	}
	tabKeys := m.tabs[m.activeTab].HelpKeys()

	var sb strings.Builder
	sb.WriteString(keyStyle.Render("  Global\n"))
	for _, h := range global {
		sb.WriteString("    " + keyStyle.Render(h.Key) + "  " + descStyle.Render(h.Desc) + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(keyStyle.Render("  " + m.tabs[m.activeTab].Title() + "\n"))
	for _, h := range tabKeys {
		sb.WriteString("    " + keyStyle.Render(h.Key) + "  " + descStyle.Render(h.Desc) + "\n")
	}
	return sb.String()
}
