// Package chtop polls ClickHouse system tables to feed a top-like live view.
// It has no TUI dependencies so it can be unit-tested with a fake Querier.
package chtop

import (
	"context"
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
