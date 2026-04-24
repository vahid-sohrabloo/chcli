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
type Tab interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View(width, height int) string
	Title() string
	HelpKeys() []keyHint
	HasActiveModal() bool
}

// Model is the alt-screen container for the \monitor view.
type Model struct {
	tabs      []Tab
	activeTab int
	initDone  []bool // one entry per tab; true after the first Init()

	width, height int

	helpVisible bool
	closed      bool
	lastActive  []time.Time // last time each tab was visited
}

// Closed reports whether the user has asked to exit the monitor. The outer
// Model should observe this and drop the view.
func (m *Model) Closed() bool { return m.closed }

// NewModel constructs a Model with the given tabs; startIndex is clamped
// into [0, len(tabs)-1]. Width/height set initial size (can be overridden
// by a later WindowSizeMsg).
func NewModel(tabs []Tab, startIndex, width, height int) *Model {
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

// Update is bubbletea's update. Returns (model, cmd) — global keys are
// consumed here; everything else is forwarded to the active tab.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		var cmds []tea.Cmd
		for _, t := range m.tabs {
			if c := t.Update(v); c != nil {
				cmds = append(cmds, c)
			}
		}
		return m, tea.Batch(cmds...)

	case tea.MouseClickMsg:
		if idx := m.tabIndexAtX(v.X); v.Y == tabBarY && idx >= 0 {
			return m, m.switchTo(idx)
		}
		cmd := m.tabs[m.activeTab].Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		if cmd, handled := m.handleGlobalKey(v); handled {
			return m, cmd
		}
	}
	cmd := m.tabs[m.activeTab].Update(msg)
	return m, cmd
}

// tabBarY is the row index the tab bar is rendered on inside the monitor
// screen. The title row is row 0, tab bar is row 1.
const tabBarY = 1

// tabBarCellStart returns the x column where the label of tab i starts. The
// layout is: " N Title " cells separated by "│".
func (m *Model) tabBarCellStart(i int) int {
	x := 0
	for j, t := range m.tabs {
		if j == i {
			return x
		}
		x += tabCellWidth(j+1, t.Title()) + 1 // +1 for the divider
	}
	return x
}

// tabIndexAtX returns the tab index whose cell contains screen-column x, or
// -1 if x lands on a divider or outside any tab.
func (m *Model) tabIndexAtX(x int) int {
	if x < 0 {
		return -1
	}
	start := 0
	for i, t := range m.tabs {
		w := tabCellWidth(i+1, t.Title())
		if x >= start && x < start+w {
			return i
		}
		start += w + 1
	}
	return -1
}

// tabCellWidth returns the rendered width of a tab cell " N Title ".
func tabCellWidth(_ int, title string) int {
	return len(title) + 4
}

// View composes title · tab bar · content · (help overlay if visible).
func (m *Model) View() string {
	title := m.renderTitleRow()
	bar := m.renderTabBar()

	if m.helpVisible {
		return title + "\n" + bar + "\n\n" + m.renderHelpOverlay()
	}

	// Content area = everything below the title + tab bar + a one-row gap.
	contentHeight := max(m.height-3, 1)
	body := m.tabs[m.activeTab].View(m.width, contentHeight)
	return title + "\n" + bar + "\n" + body
}

func (m *Model) handleGlobalKey(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	// Help overlay: any key closes it. Consumed; does not forward.
	if m.helpVisible {
		m.helpVisible = false
		return nil, true
	}

	// Number keys 1..9 jump to that tab (1-indexed).
	if kp.Code >= '1' && kp.Code <= '9' && kp.Mod == 0 {
		idx := int(kp.Code - '1')
		if idx < len(m.tabs) {
			return m.switchTo(idx), true
		}
		return nil, true
	}

	switch kp.Code {
	case tea.KeyTab:
		if kp.Mod == tea.ModShift {
			return m.switchTo((m.activeTab - 1 + len(m.tabs)) % len(m.tabs)), true
		}
		return m.switchTo((m.activeTab + 1) % len(m.tabs)), true

	case '?':
		m.helpVisible = true
		return nil, true

	case 'q', tea.KeyEscape:
		// Modal-aware: if the active tab has a modal open, forward so it can
		// cancel. Only close the monitor when no modal is active.
		if m.tabs[m.activeTab].HasActiveModal() {
			return nil, false
		}
		m.closed = true
		return nil, true
	}
	return nil, false
}

// Resettable is an optional extension implemented by tabs that want to
// reset their state when re-activated (e.g. Storage clears its drilldown
// cache so the user always lands at the L0 databases list).
type Resettable interface {
	Reset() tea.Cmd
}

// switchTo activates idx, running Init() on the first activation and Reset()
// (when implemented) on subsequent activations.
func (m *Model) switchTo(idx int) tea.Cmd {
	if idx == m.activeTab {
		return nil
	}
	m.activeTab = idx
	m.lastActive[idx] = time.Now()
	if !m.initDone[idx] {
		m.initDone[idx] = true
		return m.tabs[idx].Init()
	}
	if r, ok := m.tabs[idx].(Resettable); ok {
		return r.Reset()
	}
	return nil
}
