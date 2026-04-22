package monitor

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type fakeTab struct {
	title       string
	initCalls   int
	updateCalls int
	width       int
	modal       bool
}

func (f *fakeTab) Init() tea.Cmd              { f.initCalls++; return nil }
func (f *fakeTab) Update(msg tea.Msg) tea.Cmd { f.updateCalls++; return nil }
func (f *fakeTab) View(w, h int) string       { f.width = w; return "content" }
func (f *fakeTab) Title() string              { return f.title }
func (f *fakeTab) HelpKeys() []keyHint        { return nil }
func (f *fakeTab) HasActiveModal() bool       { return f.modal }

func TestKeyHintZeroValue(t *testing.T) {
	var k keyHint
	if k.Key != "" || k.Desc != "" {
		t.Fatalf("zero keyHint should be empty strings")
	}
}

func TestNewModelDefaults(t *testing.T) {
	a, b := &fakeTab{title: "A"}, &fakeTab{title: "B"}
	m := NewModel([]Tab{a, b}, 0, 80, 24)
	if m.width != 80 || m.height != 24 {
		t.Errorf("size = %dx%d, want 80x24", m.width, m.height)
	}
	if m.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0", m.activeTab)
	}
	if len(m.tabs) != 2 {
		t.Errorf("tabs = %d, want 2", len(m.tabs))
	}
}

func TestNewModelRespectsStartIndex(t *testing.T) {
	a, b := &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b}, 1, 80, 24)
	if m.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", m.activeTab)
	}
}

func TestNewModelClampsStartIndex(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 5, 80, 24)
	if m.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 (clamped)", m.activeTab)
	}
}

func TestInitCallsActiveTabInit(t *testing.T) {
	a, b := &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b}, 1, 80, 24)
	m.Init()
	if a.initCalls != 0 {
		t.Errorf("inactive tab Init calls = %d, want 0", a.initCalls)
	}
	if b.initCalls != 1 {
		t.Errorf("active tab Init calls = %d, want 1", b.initCalls)
	}
}

func press(m *Model, code rune, mod tea.KeyMod) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: code, Mod: mod})
	return cmd
}

func TestSwitchTabsByNumberKeys(t *testing.T) {
	a, b, c := &fakeTab{}, &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b, c}, 0, 80, 24)

	press(m, '2', 0)
	if m.activeTab != 1 {
		t.Errorf("after '2': activeTab = %d, want 1", m.activeTab)
	}
	if b.initCalls != 1 {
		t.Errorf("B.Init calls = %d, want 1 (first activation)", b.initCalls)
	}
	press(m, '1', 0)
	if m.activeTab != 0 {
		t.Errorf("after '1': activeTab = %d, want 0", m.activeTab)
	}
	press(m, '2', 0)
	if b.initCalls != 1 {
		t.Errorf("B.Init calls = %d after re-visit, want 1 (Init is once-only)", b.initCalls)
	}
}

func TestSwitchTabsIgnoresOutOfRangeNumberKeys(t *testing.T) {
	a, b := &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b}, 0, 80, 24)
	press(m, '3', 0)
	if m.activeTab != 0 {
		t.Errorf("after '3' with 2 tabs: activeTab = %d, want 0 (no-op)", m.activeTab)
	}
}

func TestCycleForwardWithTab(t *testing.T) {
	a, b, c := &fakeTab{}, &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b, c}, 0, 80, 24)
	press(m, tea.KeyTab, 0)
	if m.activeTab != 1 {
		t.Errorf("after Tab: activeTab = %d, want 1", m.activeTab)
	}
	press(m, tea.KeyTab, 0)
	press(m, tea.KeyTab, 0)
	if m.activeTab != 0 {
		t.Errorf("after 3 Tabs: activeTab = %d, want 0 (wrapped)", m.activeTab)
	}
}

func TestCycleBackwardWithShiftTab(t *testing.T) {
	a, b, c := &fakeTab{}, &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b, c}, 0, 80, 24)
	press(m, tea.KeyTab, tea.ModShift)
	if m.activeTab != 2 {
		t.Errorf("after Shift+Tab: activeTab = %d, want 2 (wrapped)", m.activeTab)
	}
}

func TestQuitClosesMonitorWhenNoModal(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 0, 80, 24)
	press(m, 'q', 0)
	if !m.closed {
		t.Error("expected closed = true after 'q'")
	}
}

func TestEscClosesMonitorWhenNoModal(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 0, 80, 24)
	press(m, tea.KeyEscape, 0)
	if !m.closed {
		t.Error("expected closed = true after Esc")
	}
}

func TestEscForwardedToTabWhenModalActive(t *testing.T) {
	a := &fakeTab{modal: true}
	m := NewModel([]Tab{a}, 0, 80, 24)
	press(m, tea.KeyEscape, 0)
	if m.closed {
		t.Error("modal active; Esc should forward to tab, not close the monitor")
	}
	if a.updateCalls != 1 {
		t.Errorf("tab.Update calls = %d, want 1", a.updateCalls)
	}
}

func TestQuestionMarkTogglesHelp(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 0, 80, 24)
	press(m, '?', 0)
	if !m.helpVisible {
		t.Error("expected helpVisible = true after first '?'")
	}
	press(m, '?', 0)
	if m.helpVisible {
		t.Error("expected helpVisible = false after second '?'")
	}
}

func TestAnyKeyClosesHelpOverlay(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 0, 80, 24)
	m.helpVisible = true
	press(m, 'x', 0)
	if m.helpVisible {
		t.Error("any key should close help overlay")
	}
	if a.updateCalls != 0 {
		t.Errorf("tab.Update calls = %d, want 0 (key consumed by help-close)", a.updateCalls)
	}
}

func TestClosedGetter(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]Tab{a}, 0, 80, 24)
	press(m, 'q', 0)
	if !m.Closed() {
		t.Error("Closed() should return true")
	}
}

func TestWindowResizeUpdatesSizeAndBroadcasts(t *testing.T) {
	a, b := &fakeTab{}, &fakeTab{}
	m := NewModel([]Tab{a, b}, 0, 80, 24)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m.width, m.height)
	}
	if a.updateCalls != 1 || b.updateCalls != 1 {
		t.Errorf("resize update calls: a=%d, b=%d; want 1 each", a.updateCalls, b.updateCalls)
	}
}

func TestTabIndexAtX(t *testing.T) {
	a, b, c := &fakeTab{title: "Alpha"}, &fakeTab{title: "Beta"}, &fakeTab{title: "Gamma"}
	m := NewModel([]Tab{a, b, c}, 0, 120, 24)

	idx := m.tabIndexAtX(m.tabBarCellStart(2))
	if idx != 2 {
		t.Errorf("tabIndexAtX on tab 2 start = %d, want 2", idx)
	}
	idx = m.tabIndexAtX(-1)
	if idx != -1 {
		t.Errorf("tabIndexAtX on -1 = %d, want -1 (miss)", idx)
	}
}

func stripANSI(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b = append(b, s[i])
	}
	return string(b)
}

func TestViewIncludesTabBarAndActiveContent(t *testing.T) {
	a := &fakeTab{title: "Alpha"}
	b := &fakeTab{title: "Beta"}
	m := NewModel([]Tab{a, b}, 0, 120, 24)
	out := stripANSI(m.View())
	for _, want := range []string{"1 Alpha", "2 Beta", "content"} {
		if !contains(out, want) {
			t.Errorf("View missing %q in:\n%s", want, out)
		}
	}
}

func TestViewHelpOverlayShowsKeys(t *testing.T) {
	a := &fakeTab{title: "Alpha"}
	m := NewModel([]Tab{a}, 0, 120, 24)
	m.helpVisible = true
	out := stripANSI(m.View())
	for _, want := range []string{"Tab", "next", "q", "quit"} {
		if !contains(out, want) {
			t.Errorf("help overlay missing %q in:\n%s", want, out)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

var _ = time.Second
