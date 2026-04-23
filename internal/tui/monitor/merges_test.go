package monitor

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

func TestMergesTabTitle(t *testing.T) {
	tab := NewMergesTab(nil)
	if tab.Title() != "Merges" {
		t.Errorf("Title = %q, want Merges", tab.Title())
	}
}

func TestMergesTabOnSnapshotStoresRows(t *testing.T) {
	tab := NewMergesTab(nil).(*mergesTab)
	tab.onSnapshot([]chtop.MergeRow{{Database: "events"}}, []chtop.MutationRow{})
	if len(tab.merges) != 1 || tab.merges[0].Database != "events" {
		t.Errorf("merges not stored: %+v", tab.merges)
	}
}

func TestMergesTabCursorClamps(t *testing.T) {
	tab := NewMergesTab(nil).(*mergesTab)
	tab.width, tab.height = 120, 40
	tab.onSnapshot([]chtop.MergeRow{{Database: "a"}, {Database: "b"}}, nil)

	tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if tab.table.Cursor() != 1 {
		t.Errorf("cursor = %d after 3 downs with 2 rows, want 1", tab.table.Cursor())
	}
	tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if tab.table.Cursor() != 0 {
		t.Errorf("cursor = %d after 3 ups, want 0", tab.table.Cursor())
	}
}

func TestMergesTabViewShowsContent(t *testing.T) {
	tab := NewMergesTab(nil)
	tab.(*mergesTab).onSnapshot(
		[]chtop.MergeRow{{Database: "events", Table: "hits", Progress: 0.5, Elapsed: 2.0}},
		[]chtop.MutationRow{{Database: "events", Table: "hits", MutationID: "m1", Command: "DELETE WHERE x=1"}},
	)
	out := stripANSI(tab.View(120, 24))
	for _, want := range []string{"events", "hits", "DELETE", "merge", "mutation"} {
		if !contains(out, want) {
			t.Errorf("View missing %q in:\n%s", want, out)
		}
	}
}

var _ = time.Second
