package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// StatusBarModel renders the top bar and bottom hints bar.
type StatusBarModel struct {
	host, user, database, version, serverVersion string
	loading                                      bool
	spinnerView                                  string
	resultCount, resultActive                    int
	port                                         uint16
	keymap                                       KeymapMode
	width                                        int
	connected                                    bool
	hintMode                                     HintMode
}

// HintMode determines which keybind hints to show.
type HintMode int

const (
	HintDefault    HintMode = iota // typing mode
	HintCompletion                 // completion popup visible
)

// NewStatusBarModel creates a StatusBarModel for the given connection details.
func NewStatusBarModel(host string, port uint16, user, database string, keymap KeymapMode) *StatusBarModel {
	return &StatusBarModel{
		host:      host,
		port:      port,
		user:      user,
		database:  database,
		keymap:    keymap,
		connected: true,
		version:   "0.1.0",
	}
}

func (m *StatusBarModel) SetDatabase(db string)     { m.database = db }
func (m *StatusBarModel) SetServerVersion(v string) { m.serverVersion = v }
func (m *StatusBarModel) SetWidth(w int)            { m.width = w }
func (m *StatusBarModel) SetConnected(c bool)       { m.connected = c }
func (m *StatusBarModel) SetLoading(l bool)         { m.loading = l }
func (m *StatusBarModel) SetSpinnerView(s string)   { m.spinnerView = s }
func (m *StatusBarModel) SetHintMode(h HintMode)    { m.hintMode = h }

// Theme-aware style accessors using ANSI 0 (black) background for bars.
func barBg() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color("0"))
}
func barStyle(hex string) lipgloss.Style {
	return barBg().Foreground(lipgloss.Color(hex))
}

// TopBarView renders the top bar.
func (m *StatusBarModel) TopBarView() string {
	dot := barStyle("#9ece6a").Render("●")
	if !m.connected {
		dot = barStyle("#f7768e").Render("●")
	}

	sep := barStyle(ActiveTheme.TextSecondary).Render(" │ ")

	left := barStyle(ActiveTheme.AccentBlue).Bold(true).Render(" chcli") + sep +
		dot + " " + barStyle(ActiveTheme.TextPrimary).Render(fmt.Sprintf("%s@%s:%d", m.user, m.host, m.port)) + sep +
		barStyle(ActiveTheme.AccentYellow).Render(m.database)

	if m.loading {
		loadStyle := barStyle(ActiveTheme.AccentYellow)
		left += sep + loadStyle.Render(m.spinnerView+" loading schema...")
	}

	if m.resultCount > 0 {
		badge := barStyle("#e0af68").Render(
			fmt.Sprintf("Q%d/%d", m.resultActive+1, m.resultCount))
		left += sep + badge
		if m.resultCount > 1 {
			hint := barStyle(ActiveTheme.TextSecondary).Render(" Ctrl+P")
			left += hint
		}
	}

	keymapLabel := "emacs"
	if m.keymap == KeymapVi {
		keymapLabel = "vi"
	}
	var rightParts []string
	if m.serverVersion != "" {
		rightParts = append(rightParts, barStyle(ActiveTheme.TextSecondary).Render("CH "+m.serverVersion))
	}
	rightParts = append(rightParts, barStyle(ActiveTheme.TextSecondary).Render("v"+m.version))
	rightParts = append(rightParts, barStyle(ActiveTheme.AccentBlue).Bold(true).Render(keymapLabel))
	right := strings.Join(rightParts, " ") + " "

	// Fill to width.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(m.width-leftWidth-rightWidth, 0)
	fill := barBg().Render(strings.Repeat(" ", gap))

	return left + fill + right
}

// HintsBarView renders the bottom hints bar.
func (m *StatusBarModel) HintsBarView() string {
	var leftHints, rightHints []string

	switch m.hintMode {
	case HintCompletion:
		leftHints = []string{
			hint("↑↓", "navigate"), hint("Tab", "accept"), hint("Esc", "close"),
		}
	default:
		leftHints = []string{
			hint("Tab", "complete"), hint("↑↓", "history"), hint("Ctrl+R", "search"), hint("F2", "table view"),
		}
	}

	rightHints = []string{
		hint("\\help", "commands"), hint("\\x", "vertical"), hint("Ctrl+D", "quit"),
	}

	left := " " + strings.Join(leftHints, "  ")
	right := strings.Join(rightHints, "  ") + " "

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(m.width-leftWidth-rightWidth, 0)
	fill := barBg().Render(strings.Repeat(" ", gap))

	return lipgloss.NewStyle().Render(left) + fill + lipgloss.NewStyle().Render(right)
}

func hint(key, desc string) string {
	return barStyle(ActiveTheme.TextSecondary).Render(key) + " " + barStyle(ActiveTheme.TextMuted).Render(desc)
}

// View renders the old-style status bar (kept for backward compat, delegates to TopBarView).
func (m *StatusBarModel) View() string {
	return m.TopBarView()
}
