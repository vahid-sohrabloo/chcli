package history

import (
	"database/sql"
	"fmt"
	"time"
)

// Bookmark represents a saved query bookmark.
type Bookmark struct {
	ID          int64
	Query       string
	Tag         string
	CreatedAt   time.Time
	Description string
}

// AddBookmark saves a query as a bookmark with an optional tag and description.
func (s *Store) AddBookmark(query, tag, description string) error {
	_, err := s.db.Exec(
		`INSERT INTO bookmarks (query, tag, created_at, description) VALUES (?, ?, ?, ?)`,
		query,
		tag,
		time.Now().UTC(),
		description,
	)
	if err != nil {
		return fmt.Errorf("add bookmark: %w", err)
	}
	return nil
}

// ListBookmarks returns all bookmarks. If tag is non-empty, only bookmarks with that tag are returned.
func (s *Store) ListBookmarks(tag string) ([]Bookmark, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if tag == "" {
		rows, err = s.db.Query(
			`SELECT id, query, tag, created_at, description FROM bookmarks ORDER BY id ASC`,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, query, tag, created_at, description FROM bookmarks WHERE tag = ? ORDER BY id ASC`,
			tag,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		var desc sql.NullString
		var createdAt string
		if err := rows.Scan(&b.ID, &b.Query, &b.Tag, &createdAt, &desc); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", createdAt)
			if err != nil {
				t = time.Time{}
			}
		}
		b.CreatedAt = t
		if desc.Valid {
			b.Description = desc.String
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, rows.Err()
}

// DeleteBookmark removes the bookmark with the given ID.
func (s *Store) DeleteBookmark(id int64) error {
	_, err := s.db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark %d: %w", id, err)
	}
	return nil
}
