package history

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for query history and bookmarks.
type Store struct {
	db *sql.DB
}

// Entry represents a single history entry.
type Entry struct {
	ID         int64
	Query      string
	ExecutedAt time.Time
	DurationMs int64
	Database   string
	Profile    string
}

// Open opens (or creates) a SQLite history database at path and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    query        TEXT    NOT NULL,
    executed_at  DATETIME NOT NULL,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    database_name TEXT,
    profile       TEXT
);
CREATE INDEX IF NOT EXISTS idx_history_query ON history(query);

CREATE TABLE IF NOT EXISTS bookmarks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    query       TEXT    NOT NULL,
    tag         TEXT    NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    description TEXT
);
CREATE INDEX IF NOT EXISTS idx_bookmarks_tag ON bookmarks(tag);
`
	_, err := db.Exec(schema)
	return err
}

// Add inserts a new query execution record into history.
func (s *Store) Add(query string, durationMs int64, database, profile string) error {
	_, err := s.db.Exec(
		`INSERT INTO history (query, executed_at, duration_ms, database_name, profile) VALUES (?, ?, ?, ?, ?)`,
		query,
		time.Now().UTC(),
		durationMs,
		database,
		profile,
	)
	if err != nil {
		return fmt.Errorf("history add: %w", err)
	}
	return nil
}

// Recent returns the most recently executed entries, up to limit.
func (s *Store) Recent(limit int) ([]Entry, error) {
	rows, err := s.db.Query(
		`SELECT id, query, executed_at, duration_ms, database_name, profile FROM history ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("history recent: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Search returns history entries whose query contains term, up to limit.
func (s *Store) Search(term string, limit int) ([]Entry, error) {
	rows, err := s.db.Query(
		`SELECT id, query, executed_at, duration_ms, database_name, profile FROM history WHERE query LIKE ? ORDER BY id DESC LIMIT ?`,
		"%"+term+"%",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("history search: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Queries returns distinct queries for history navigation, most recent first, up to limit.
func (s *Store) Queries(limit int) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT query FROM history ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("history queries: %w", err)
	}
	defer rows.Close()

	var queries []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, fmt.Errorf("history queries scan: %w", err)
		}
		queries = append(queries, q)
	}
	return queries, rows.Err()
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var e Entry
		var dbName, profile sql.NullString
		var executedAt string
		if err := rows.Scan(&e.ID, &e.Query, &executedAt, &e.DurationMs, &dbName, &profile); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		t, err := time.Parse(time.RFC3339, executedAt)
		if err != nil {
			// fallback for other SQLite datetime formats
			t, err = time.Parse("2006-01-02 15:04:05", executedAt)
			if err != nil {
				t = time.Time{}
			}
		}
		e.ExecutedAt = t
		if dbName.Valid {
			e.Database = dbName.String
		}
		if profile.Valid {
			e.Profile = profile.String
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
