// Package chtop polls ClickHouse system tables to feed a top-like live view.
// It has no TUI dependencies so it can be unit-tested with a fake Querier.
package chtop

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// Querier is the subset of *conn.Conn that chtop uses. Taking an interface
// here (instead of a concrete *conn.Conn) lets tests pass a canned fake.
type Querier interface {
	QueryAll(ctx context.Context, sql string) (*conn.QueryResult, error)
}

// Snapshot is one tick's worth of data: server-level metrics plus the list
// of in-flight queries.
type Snapshot struct {
	At        time.Time
	Header    Header
	Processes []Process
}

// Header holds server-level gauges and monotonic counters. Rates are derived
// by the Fetcher from consecutive Headers, not stored here.
type Header struct {
	Uptime            time.Duration
	Version           string
	ActiveQueries     int
	QueriesTotal      uint64  // monotonic; counter for Query events
	InsertedRowsTotal uint64  // monotonic; counter for InsertedRows events
	MemUsed           uint64
	MemTotal          uint64
	QRunning          int
	MergesRunning     int
	MutationsRunning  int
	ReplicaMaxDelay   float64 // seconds; -1 when no replicated tables
}

// Process is one row from system.processes.
type Process struct {
	QueryID        string
	User           string
	InitialAddress string
	Client         string
	Database       string
	Elapsed        float64 // seconds
	ReadRows       uint64
	ReadBytes      uint64
	MemoryUsage    int64
	Query          string // pre-collapsed to a single line
}

// Rates are per-second derivatives computed client-side from two Headers.
type Rates struct {
	QueriesPerSec    float64
	InsertRowsPerSec float64
}

// Fetcher runs the polling SQLs and derives Rates across ticks.
type Fetcher struct {
	q      Querier
	prev   *Header
	prevAt time.Time
}

// NewFetcher returns a Fetcher bound to q. One Fetcher per topview lifetime.
func NewFetcher(q Querier) *Fetcher {
	return &Fetcher{q: q}
}

// parseProcesses turns a processes-query QueryResult into typed Process rows.
// Column order must match sqlProcesses. Errors on malformed numeric columns
// because silent 0-fallbacks would hide bugs in our own SQL.
func parseProcesses(qr *conn.QueryResult) ([]Process, error) {
	out := make([]Process, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 10 {
			return nil, fmt.Errorf("row %d: expected 10 columns, got %d", i, len(row))
		}
		elapsed, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("row %d elapsed: %w", i, err)
		}
		readRows, err := parseUint64Lenient(row[6])
		if err != nil {
			return nil, fmt.Errorf("row %d read_rows: %w", i, err)
		}
		readBytes, err := parseUint64Lenient(row[7])
		if err != nil {
			return nil, fmt.Errorf("row %d read_bytes: %w", i, err)
		}
		mem, err := strconv.ParseInt(row[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("row %d memory_usage: %w", i, err)
		}
		out = append(out, Process{
			QueryID:        row[0],
			User:           row[1],
			InitialAddress: row[2],
			Client:         row[3],
			Database:       row[4],
			Elapsed:        elapsed,
			ReadRows:       readRows,
			ReadBytes:      readBytes,
			MemoryUsage:    mem,
			Query:          row[9],
		})
	}
	return out, nil
}

// parseUint64Lenient strips thousand separators / whitespace and parses. Empty
// string parses to 0 so "" values (which chconn's formatter can emit for
// Nullable NULLs) don't break the header path.
func parseUint64Lenient(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseUint(s, 10, 64)
}
