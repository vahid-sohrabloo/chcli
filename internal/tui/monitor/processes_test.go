package monitor

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/tui"
)

func TestProcessesTabTitle(t *testing.T) {
	p := NewProcessesTab(nil)
	if p.Title() != "Processes" {
		t.Errorf("Title = %q, want Processes", p.Title())
	}
}

func TestProcessesTabHasActiveModalReflectsTopModel(t *testing.T) {
	tv := tui.NewTopViewForTest(80, 24)
	p := NewProcessesTab(tv)
	if p.HasActiveModal() {
		t.Error("expected HasActiveModal()=false in ModeNormal")
	}
	tv.SetModeForTest(tui.ModeFilter)
	if !p.HasActiveModal() {
		t.Error("expected HasActiveModal()=true in ModeFilter")
	}
}

func TestProcessesTabForwardsUpdate(t *testing.T) {
	tv := tui.NewTopViewForTest(80, 24)
	p := NewProcessesTab(tv)
	tui.SeedProcessesForTest(tv, 3)
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tv.CursorForTest() != 1 {
		t.Errorf("cursor = %d after Down, want 1", tv.CursorForTest())
	}
}
