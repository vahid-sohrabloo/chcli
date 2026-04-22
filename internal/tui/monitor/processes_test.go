package monitor

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type fakeTopModel struct {
	fetchCalls int
	tickCalls  int
	updates    int
	modal      bool
	width      int
	height     int
}

func (f *fakeTopModel) FetchCmd() tea.Cmd              { f.fetchCalls++; return nil }
func (f *fakeTopModel) TickCmd() tea.Cmd               { f.tickCalls++; return nil }
func (f *fakeTopModel) Update(tea.Msg) (tea.Cmd, bool) { f.updates++; return nil, false }
func (f *fakeTopModel) View() string                   { return "proc-view" }
func (f *fakeTopModel) SetSize(w, h int)               { f.width, f.height = w, h }
func (f *fakeTopModel) InModal() bool                  { return f.modal }

func TestProcessesTabTitle(t *testing.T) {
	p := NewProcessesTab(nil)
	if p.Title() != "Processes" {
		t.Errorf("Title = %q, want Processes", p.Title())
	}
}

func TestProcessesTabHasActiveModalReflectsMode(t *testing.T) {
	f := &fakeTopModel{}
	p := NewProcessesTab(f)
	if p.HasActiveModal() {
		t.Error("Mode=0 should not be modal")
	}
	f.modal = true
	if !p.HasActiveModal() {
		t.Error("Mode!=0 should be modal")
	}
}

func TestProcessesTabForwardsUpdate(t *testing.T) {
	f := &fakeTopModel{}
	p := NewProcessesTab(f)
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if f.updates != 1 {
		t.Errorf("updates = %d, want 1", f.updates)
	}
}

func TestProcessesTabInitFiresFetchAndTick(t *testing.T) {
	f := &fakeTopModel{}
	p := NewProcessesTab(f)
	p.Init()
	if f.fetchCalls != 1 {
		t.Errorf("fetchCalls = %d, want 1", f.fetchCalls)
	}
	if f.tickCalls != 1 {
		t.Errorf("tickCalls = %d, want 1", f.tickCalls)
	}
}

func TestProcessesTabViewPassesSize(t *testing.T) {
	f := &fakeTopModel{}
	p := NewProcessesTab(f)
	p.View(120, 40)
	if f.width != 120 || f.height != 40 {
		t.Errorf("SetSize = %dx%d, want 120x40", f.width, f.height)
	}
}
