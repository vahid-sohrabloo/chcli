package history

import (
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAddAndRecent(t *testing.T) {
	s := testStore(t)

	if err := s.Add("SELECT 1", 10, "default", "dev"); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := s.Add("SELECT 2", 20, "mydb", "prod"); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	entries, err := s.Recent(10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Most recent first
	if entries[0].Query != "SELECT 2" {
		t.Errorf("expected first entry to be 'SELECT 2', got %q", entries[0].Query)
	}
	if entries[1].Query != "SELECT 1" {
		t.Errorf("expected second entry to be 'SELECT 1', got %q", entries[1].Query)
	}
}

func TestSearch(t *testing.T) {
	s := testStore(t)

	if err := s.Add("SELECT * FROM users", 10, "db", "dev"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("SELECT * FROM orders", 20, "db", "dev"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add("SHOW TABLES", 5, "db", "dev"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	results, err := s.Search("users", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Query != "SELECT * FROM users" {
		t.Errorf("expected 'SELECT * FROM users', got %q", results[0].Query)
	}
}

func TestRecentLimit(t *testing.T) {
	s := testStore(t)

	for i := range 5 {
		if err := s.Add("SELECT 1", int64(i*10), "db", "dev"); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}

	entries, err := s.Recent(2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
