package monitor

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

func TestStorageTabDefaults(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	if s.level() != storageLevelRoot {
		t.Errorf("level = %v, want root", s.level())
	}
	if s.sortBy != storageSortSize {
		t.Errorf("sortBy = %v, want size", s.sortBy)
	}
}

func TestStorageTabDrillsDown(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	s.width, s.height = 120, 40
	s.dbs = []chtop.DBRow{{Name: "events", Bytes: 100}, {Name: "logs", Bytes: 10}}
	s.rebuildTable() // populate visibleNames via sort+filter pipeline
	s.table.SetCursor(0)
	s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.level() != storageLevelTables {
		t.Errorf("level = %v, want tables", s.level())
	}
	if len(s.path) != 1 || s.path[0] != "events" {
		t.Errorf("path = %v, want [events]", s.path)
	}
}

func TestStorageTabBackspaceGoesUp(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	s.path = []string{"events", "hits"}
	s.tables = []chtop.TableRow{{Name: "hits"}}
	s.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if len(s.path) != 1 {
		t.Errorf("path = %v, want [events]", s.path)
	}
}

func TestStorageTabSortCycle(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	s.Update(tea.KeyPressMsg{Code: 's'})
	if s.sortBy != storageSortRows {
		t.Errorf("after 1 's': %v, want rows", s.sortBy)
	}
	s.Update(tea.KeyPressMsg{Code: 's'})
	if s.sortBy != storageSortName {
		t.Errorf("after 2 's': %v, want name", s.sortBy)
	}
	s.Update(tea.KeyPressMsg{Code: 's'})
	if s.sortBy != storageSortSize {
		t.Errorf("after 3 's': %v, want size (wrapped)", s.sortBy)
	}
}

func TestStorageTabFilterModeAndHasActiveModal(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	if s.HasActiveModal() {
		t.Error("initial HasActiveModal should be false")
	}
	s.Update(tea.KeyPressMsg{Code: '/'})
	if !s.HasActiveModal() {
		t.Error("HasActiveModal should be true after '/'")
	}
	s.Update(tea.KeyPressMsg{Code: 'a'})
	s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.filter != "a" || s.inFilter {
		t.Errorf("expected committed filter='a' and inFilter=false, got %q/%v", s.filter, s.inFilter)
	}
}

func TestStorageTabViewShowsDatabases(t *testing.T) {
	s := NewStorageTab(nil).(*storageTab)
	s.dbs = []chtop.DBRow{{Name: "events", Bytes: 1 << 30}, {Name: "logs", Bytes: 1 << 28}}
	out := stripANSI(s.View(120, 24))
	if !contains(out, "events") || !contains(out, "logs") {
		t.Errorf("View missing db names:\n%s", out)
	}
}
