package history

import (
	"testing"
)

func TestAddAndListBookmarks(t *testing.T) {
	s := testStore(t)

	if err := s.AddBookmark("SELECT * FROM users", "users", "user query"); err != nil {
		t.Fatalf("AddBookmark first: %v", err)
	}
	if err := s.AddBookmark("SELECT * FROM orders", "orders", "order query"); err != nil {
		t.Fatalf("AddBookmark second: %v", err)
	}

	all, err := s.ListBookmarks("")
	if err != nil {
		t.Fatalf("ListBookmarks all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(all))
	}

	byTag, err := s.ListBookmarks("users")
	if err != nil {
		t.Fatalf("ListBookmarks by tag: %v", err)
	}
	if len(byTag) != 1 {
		t.Fatalf("expected 1 bookmark with tag 'users', got %d", len(byTag))
	}
	if byTag[0].Query != "SELECT * FROM users" {
		t.Errorf("expected 'SELECT * FROM users', got %q", byTag[0].Query)
	}
}

func TestDeleteBookmark(t *testing.T) {
	s := testStore(t)

	if err := s.AddBookmark("SELECT 1", "misc", "test"); err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}

	bookmarks, err := s.ListBookmarks("")
	if err != nil {
		t.Fatalf("ListBookmarks: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark before delete, got %d", len(bookmarks))
	}

	if err := s.DeleteBookmark(bookmarks[0].ID); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}

	after, err := s.ListBookmarks("")
	if err != nil {
		t.Fatalf("ListBookmarks after delete: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("expected 0 bookmarks after delete, got %d", len(after))
	}
}
