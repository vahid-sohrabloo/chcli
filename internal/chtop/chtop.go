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
	QueryID         string
	User            string
	InitialAddress  string
	Client          string
	Database        string
	Elapsed         float64 // seconds
	ReadRows        uint64
	ReadBytes       uint64
	MemoryUsage     int64
	CPUSeconds      float64 // OSCPUVirtualTimeMicroseconds / 1e6
	TotalRowsApprox uint64  // ClickHouse's estimate; 0 when unknown
	Query           string  // pre-collapsed to a single line
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

// Fetch runs the two polling SQLs, parses the rows, derives rates from the
// previous header, and stores the new header for the next call. It returns
// any SQL or parse error unwrapped — the caller decides how to surface it.
func (f *Fetcher) Fetch(ctx context.Context) (Snapshot, Rates, error) {
	now := time.Now()

	procsRes, err := f.q.QueryAll(ctx, sqlProcesses)
	if err != nil {
		return Snapshot{}, Rates{}, fmt.Errorf("processes: %w", err)
	}
	procs, err := parseProcesses(procsRes)
	if err != nil {
		return Snapshot{}, Rates{}, fmt.Errorf("parse processes: %w", err)
	}

	hdrRes, err := f.q.QueryAll(ctx, sqlHeader)
	if err != nil {
		return Snapshot{}, Rates{}, fmt.Errorf("header: %w", err)
	}
	hdr, err := parseHeader(hdrRes)
	if err != nil {
		return Snapshot{}, Rates{}, fmt.Errorf("parse header: %w", err)
	}

	var rates Rates
	if f.prev != nil {
		rates = deriveRates(f.prev, hdr, now.Sub(f.prevAt))
	}
	prev := hdr
	f.prev = &prev
	f.prevAt = now

	return Snapshot{At: now, Header: hdr, Processes: procs}, rates, nil
}

// parseProcesses turns a processes-query QueryResult into typed Process rows.
// Column order must match sqlProcesses. Errors on malformed numeric columns
// because silent 0-fallbacks would hide bugs in our own SQL.
func parseProcesses(qr *conn.QueryResult) ([]Process, error) {
	out := make([]Process, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 12 {
			return nil, fmt.Errorf("row %d: expected 12 columns, got %d", i, len(row))
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
		// Empty string = NULL (chconn's formatter renders NULL as ""); treat
		// as 0 since new queries may not have accumulated ProfileEvents yet.
		cpu := 0.0
		if row[9] != "" {
			cpu, err = strconv.ParseFloat(row[9], 64)
			if err != nil {
				return nil, fmt.Errorf("row %d cpu_seconds: %w", i, err)
			}
		}
		totalRows, err := parseUint64Lenient(row[10])
		if err != nil {
			return nil, fmt.Errorf("row %d total_rows_approx: %w", i, err)
		}
		out = append(out, Process{
			QueryID:         row[0],
			User:            row[1],
			InitialAddress:  row[2],
			Client:          row[3],
			Database:        row[4],
			Elapsed:         elapsed,
			ReadRows:        readRows,
			ReadBytes:       readBytes,
			MemoryUsage:     mem,
			CPUSeconds:      cpu,
			TotalRowsApprox: totalRows,
			Query:           row[11],
		})
	}
	return out, nil
}

// parseHeader turns the single-row header-metrics QueryResult into a Header.
// Column order must match sqlHeader. Uses parseUint64Lenient so cells that
// chconn renders as "" (NULL) parse as 0 — the SQL uses ifNull so this is
// defense in depth.
func parseHeader(qr *conn.QueryResult) (Header, error) {
	if len(qr.Rows) == 0 {
		return Header{}, fmt.Errorf("parseHeader: no rows")
	}
	r := qr.Rows[0]
	if len(r) < 11 {
		return Header{}, fmt.Errorf("parseHeader: expected 11 columns, got %d", len(r))
	}
	uptimeS, err := parseUint64Lenient(r[0])
	if err != nil {
		return Header{}, fmt.Errorf("uptime: %w", err)
	}
	activeQ, err := strconv.Atoi(r[2])
	if err != nil {
		return Header{}, fmt.Errorf("active_queries: %w", err)
	}
	qTot, err := parseUint64Lenient(r[3])
	if err != nil {
		return Header{}, fmt.Errorf("queries_total: %w", err)
	}
	insRows, err := parseUint64Lenient(r[4])
	if err != nil {
		return Header{}, fmt.Errorf("inserted_rows_total: %w", err)
	}
	memUsed, err := parseUint64Lenient(r[5])
	if err != nil {
		return Header{}, fmt.Errorf("mem_used: %w", err)
	}
	memTotal, err := parseUint64Lenient(r[6])
	if err != nil {
		return Header{}, fmt.Errorf("mem_total: %w", err)
	}
	qRun, err := strconv.Atoi(r[7])
	if err != nil {
		return Header{}, fmt.Errorf("q_running: %w", err)
	}
	merges, err := strconv.Atoi(r[8])
	if err != nil {
		return Header{}, fmt.Errorf("merges_running: %w", err)
	}
	muts, err := strconv.Atoi(r[9])
	if err != nil {
		return Header{}, fmt.Errorf("mutations_running: %w", err)
	}
	// Empty string = NULL from chconn's formatter, which maps to the
	// "no replicated tables" sentinel.
	replicaDelay := -1.0
	if r[10] != "" {
		replicaDelay, err = strconv.ParseFloat(r[10], 64)
		if err != nil {
			return Header{}, fmt.Errorf("replica_max_delay: %w", err)
		}
	}
	return Header{
		Uptime:            time.Duration(uptimeS) * time.Second,
		Version:           r[1],
		ActiveQueries:     activeQ,
		QueriesTotal:      qTot,
		InsertedRowsTotal: insRows,
		MemUsed:           memUsed,
		MemTotal:          memTotal,
		QRunning:          qRun,
		MergesRunning:     merges,
		MutationsRunning:  muts,
		ReplicaMaxDelay:   replicaDelay,
	}, nil
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

// deriveRates returns per-second rates across two consecutive headers.
// Returns zero Rates when prev is nil (first tick), when dt is non-positive,
// or when a counter appears to have reset (curr < prev, e.g. server restart).
func deriveRates(prev *Header, curr Header, dt time.Duration) Rates {
	if prev == nil || dt <= 0 {
		return Rates{}
	}
	secs := dt.Seconds()
	return Rates{
		QueriesPerSec:    perSec(prev.QueriesTotal, curr.QueriesTotal, secs),
		InsertRowsPerSec: perSec(prev.InsertedRowsTotal, curr.InsertedRowsTotal, secs),
	}
}

func perSec(prev, curr uint64, secs float64) float64 {
	if curr < prev {
		return 0
	}
	return float64(curr-prev) / secs
}
