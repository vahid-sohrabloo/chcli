package tui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// UITheme defines all colors for the chcli TUI as hex strings.
type UITheme struct {
	Name                                                           string
	BgDark, BgMain, BgSubtle, BgOverlay                            string
	TextPrimary, TextSecondary, TextMuted                          string
	AccentBlue, AccentGreen, AccentYellow, AccentOrange, AccentRed string
	Border                                                         string
	// Matching chroma syntax theme name.
	SyntaxTheme string
}

var uiThemes = map[string]UITheme{
	"tokyo-night": {
		Name: "tokyo-night", BgDark: "#16161e", BgMain: "#1a1b26", BgSubtle: "#1e2030", BgOverlay: "#24283b",
		TextPrimary: "#a9b1d6", TextSecondary: "#565f89", TextMuted: "#3b4261",
		AccentBlue: "#7aa2f7", AccentGreen: "#9ece6a", AccentYellow: "#e0af68", AccentOrange: "#ff9e64", AccentRed: "#f7768e",
		Border: "#3b4261", SyntaxTheme: "monokai",
	},
	"dracula": {
		Name: "dracula", BgDark: "#21222c", BgMain: "#282a36", BgSubtle: "#2d2f3f", BgOverlay: "#44475a",
		TextPrimary: "#f8f8f2", TextSecondary: "#6272a4", TextMuted: "#44475a",
		AccentBlue: "#8be9fd", AccentGreen: "#50fa7b", AccentYellow: "#f1fa8c", AccentOrange: "#ffb86c", AccentRed: "#ff5555",
		Border: "#44475a", SyntaxTheme: "dracula",
	},
	"nord": {
		Name: "nord", BgDark: "#2e3440", BgMain: "#3b4252", BgSubtle: "#434c5e", BgOverlay: "#4c566a",
		TextPrimary: "#eceff4", TextSecondary: "#d8dee9", TextMuted: "#4c566a",
		AccentBlue: "#88c0d0", AccentGreen: "#a3be8c", AccentYellow: "#ebcb8b", AccentOrange: "#d08770", AccentRed: "#bf616a",
		Border: "#4c566a", SyntaxTheme: "nord",
	},
	"gruvbox": {
		Name: "gruvbox", BgDark: "#1d2021", BgMain: "#282828", BgSubtle: "#3c3836", BgOverlay: "#504945",
		TextPrimary: "#ebdbb2", TextSecondary: "#a89984", TextMuted: "#504945",
		AccentBlue: "#83a598", AccentGreen: "#b8bb26", AccentYellow: "#fabd2f", AccentOrange: "#fe8019", AccentRed: "#fb4934",
		Border: "#504945", SyntaxTheme: "gruvbox",
	},
	"catppuccin": {
		Name: "catppuccin", BgDark: "#11111b", BgMain: "#1e1e2e", BgSubtle: "#313244", BgOverlay: "#45475a",
		TextPrimary: "#cdd6f4", TextSecondary: "#a6adc8", TextMuted: "#45475a",
		AccentBlue: "#89b4fa", AccentGreen: "#a6e3a1", AccentYellow: "#f9e2af", AccentOrange: "#fab387", AccentRed: "#f38ba8",
		Border: "#45475a", SyntaxTheme: "catppuccin-mocha",
	},
	"solarized": {
		Name: "solarized", BgDark: "#002b36", BgMain: "#073642", BgSubtle: "#073642", BgOverlay: "#586e75",
		TextPrimary: "#839496", TextSecondary: "#657b83", TextMuted: "#586e75",
		AccentBlue: "#268bd2", AccentGreen: "#859900", AccentYellow: "#b58900", AccentOrange: "#cb4b16", AccentRed: "#dc322f",
		Border: "#586e75", SyntaxTheme: "solarized-dark",
	},
}

// TableStyles returns themed styles for bubbles/table.
func TableStyles() table.Styles {
	t := ActiveTheme
	return table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.AccentBlue)).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Border)).
			BorderBottom(true),
		Cell: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextPrimary)).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextPrimary)).
			Background(lipgloss.Color(t.BgOverlay)).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.AccentBlue)).
			BorderLeft(true),
	}
}

// ActiveTheme is the current UI theme.
var ActiveTheme = uiThemes["tokyo-night"]

// SetUITheme changes the active UI theme. Returns false if not found.
func SetUITheme(name string) bool {
	if t, ok := uiThemes[name]; ok {
		ActiveTheme = t
		return true
	}
	return false
}
