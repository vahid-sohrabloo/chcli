// Package monitor implements the \monitor multi-tab dashboard. A monitorModel
// owns a bar of tabs and delegates key/mouse events to the active tab through
// a small `tab` interface.
package monitor

import (
	tea "charm.land/bubbletea/v2"
)

// keyHint is a single key → description pair shown in the help overlay and
// in per-tab footer hints.
type keyHint struct {
	Key, Desc string
}

// tab is the contract every tab in the monitor implements.
type tab interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View(width, height int) string
	Title() string
	HelpKeys() []keyHint
	HasActiveModal() bool
}
