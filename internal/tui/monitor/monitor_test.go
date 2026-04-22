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
	m := NewModel([]tab{a, b}, 0, 80, 24)
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
	m := NewModel([]tab{a, b}, 1, 80, 24)
	if m.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", m.activeTab)
	}
}

func TestNewModelClampsStartIndex(t *testing.T) {
	a := &fakeTab{}
	m := NewModel([]tab{a}, 5, 80, 24)
	if m.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 (clamped)", m.activeTab)
	}
}

func TestInitCallsActiveTabInit(t *testing.T) {
	a, b := &fakeTab{}, &fakeTab{}
	m := NewModel([]tab{a, b}, 1, 80, 24)
	m.Init()
	if a.initCalls != 0 {
		t.Errorf("inactive tab Init calls = %d, want 0", a.initCalls)
	}
	if b.initCalls != 1 {
		t.Errorf("active tab Init calls = %d, want 1", b.initCalls)
	}
}

var _ = time.Second
