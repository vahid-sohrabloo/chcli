// Package monitor implements the \monitor multi-tab dashboard. A Model owns
// a bar of tabs and delegates key/mouse events to the active tab through a
// small `tab` interface.
package monitor

import (
	"time"

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

// Model is the alt-screen container for the \monitor view.
type Model struct {
	tabs      []tab
	activeTab int
	initDone  []bool // one entry per tab; true after the first Init()

	width, height int

	helpVisible bool
	lastActive  []time.Time // last time each tab was visited
}

// NewModel constructs a Model with the given tabs; startIndex is clamped
// into [0, len(tabs)-1]. Width/height set initial size (can be overridden
// by a later WindowSizeMsg).
func NewModel(tabs []tab, startIndex, width, height int) *Model {
	if startIndex < 0 || startIndex >= len(tabs) {
		startIndex = 0
	}
	return &Model{
		tabs:       tabs,
		activeTab:  startIndex,
		initDone:   make([]bool, len(tabs)),
		width:      width,
		height:     height,
		lastActive: make([]time.Time, len(tabs)),
	}
}

// Init fires the active tab's Init() and marks it as initialized so the
// container doesn't re-Init on the first switch back to it.
func (m *Model) Init() tea.Cmd {
	m.initDone[m.activeTab] = true
	m.lastActive[m.activeTab] = time.Now()
	return m.tabs[m.activeTab].Init()
}
