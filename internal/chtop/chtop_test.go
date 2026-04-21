package chtop

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func TestSnapshotZeroValue(t *testing.T) {
	var s Snapshot
	if s.Processes != nil {
		t.Fatalf("zero Snapshot.Processes should be nil, got %v", s.Processes)
	}
}

func TestParseProcesses(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "query_id"}, {Name: "user"}, {Name: "initial_address"},
			{Name: "client"}, {Name: "database"}, {Name: "elapsed"},
			{Name: "read_rows"}, {Name: "read_bytes"}, {Name: "memory_usage"},
			{Name: "query"},
		},
		Rows: [][]string{
			{"abc-1", "alice", "10.0.0.1", "native 24.8", "default", "12.4", "1200000", "48000000", "50331648", "SELECT * FROM t"},
			{"abc-2", "bob", "10.0.0.2", "http 1.0", "", "0.001", "0", "0", "0", "INSERT INTO s VALUES"},
		},
	}

	procs, err := parseProcesses(qr)
	if err != nil {
		t.Fatalf("parseProcesses: %v", err)
	}
	if len(procs) != 2 {
		t.Fatalf("got %d procs, want 2", len(procs))
	}
	got := procs[0]
	if got.QueryID != "abc-1" || got.User != "alice" || got.Elapsed != 12.4 ||
		got.ReadRows != 1_200_000 || got.MemoryUsage != 50_331_648 ||
		got.Query != "SELECT * FROM t" {
		t.Fatalf("row 0 mismatch: %+v", got)
	}
}

func TestParseHeader(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "uptime_s"}, {Name: "version"}, {Name: "active_queries"},
			{Name: "queries_total"}, {Name: "inserted_rows_total"},
			{Name: "mem_used"}, {Name: "mem_total"}, {Name: "q_running"},
			{Name: "merges_running"}, {Name: "mutations_running"},
			{Name: "replica_max_delay"},
		},
		Rows: [][]string{
			{"3600", "24.8.3.5", "5", "1000000", "2500000",
				"8589934592", "68719476736", "3", "2", "0", "0.00"},
		},
	}
	h, err := parseHeader(qr)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if h.Uptime != time.Hour {
		t.Errorf("Uptime = %v, want 1h", h.Uptime)
	}
	if h.Version != "24.8.3.5" || h.ActiveQueries != 5 ||
		h.QueriesTotal != 1_000_000 || h.InsertedRowsTotal != 2_500_000 ||
		h.MemUsed != 8_589_934_592 || h.MemTotal != 68_719_476_736 ||
		h.QRunning != 3 || h.MergesRunning != 2 || h.MutationsRunning != 0 ||
		h.ReplicaMaxDelay != 0.0 {
		t.Errorf("header mismatch: %+v", h)
	}
}

func TestParseHeaderNoReplicas(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: make([]conn.ResultColumn, 11),
		Rows: [][]string{
			{"0", "", "0", "0", "0", "0", "0", "0", "0", "0", "-1"},
		},
	}
	h, err := parseHeader(qr)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if h.ReplicaMaxDelay != -1 {
		t.Errorf("ReplicaMaxDelay = %v, want -1", h.ReplicaMaxDelay)
	}
}

func TestParseHeaderEmpty(t *testing.T) {
	qr := &conn.QueryResult{Columns: make([]conn.ResultColumn, 11)}
	_, err := parseHeader(qr)
	if err == nil {
		t.Fatal("expected error on empty rows, got nil")
	}
}

func TestDeriveRates(t *testing.T) {
	prev := Header{QueriesTotal: 1000, InsertedRowsTotal: 5000}
	curr := Header{QueriesTotal: 1200, InsertedRowsTotal: 5500}
	r := deriveRates(&prev, curr, 2*time.Second)
	if r.QueriesPerSec != 100 {
		t.Errorf("QueriesPerSec = %v, want 100", r.QueriesPerSec)
	}
	if r.InsertRowsPerSec != 250 {
		t.Errorf("InsertRowsPerSec = %v, want 250", r.InsertRowsPerSec)
	}
}

func TestDeriveRatesFirstTick(t *testing.T) {
	curr := Header{QueriesTotal: 1200, InsertedRowsTotal: 5500}
	r := deriveRates(nil, curr, time.Second)
	if r != (Rates{}) {
		t.Errorf("first-tick rates = %+v, want zero", r)
	}
}

func TestDeriveRatesCounterReset(t *testing.T) {
	prev := Header{QueriesTotal: 1000}
	curr := Header{QueriesTotal: 10}
	r := deriveRates(&prev, curr, time.Second)
	if r.QueriesPerSec != 0 {
		t.Errorf("counter-reset QueriesPerSec = %v, want 0", r.QueriesPerSec)
	}
}

func TestDeriveRatesZeroDuration(t *testing.T) {
	prev := Header{QueriesTotal: 1000}
	curr := Header{QueriesTotal: 1200}
	r := deriveRates(&prev, curr, 0)
	if r.QueriesPerSec != 0 {
		t.Errorf("zero-duration rate should be 0, got %v", r.QueriesPerSec)
	}
}

// fakeQuerier returns canned results keyed by SQL prefix.
type fakeQuerier struct {
	calls     int
	processes *conn.QueryResult
	header    *conn.QueryResult
	err       error
}

func (f *fakeQuerier) QueryAll(_ context.Context, sql string) (*conn.QueryResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	s := strings.TrimLeft(sql, "\n ")
	switch {
	case strings.HasPrefix(s, "SELECT\n    query_id"):
		return f.processes, nil
	case strings.HasPrefix(s, "SELECT\n    (SELECT uptime())"):
		return f.header, nil
	}
	return nil, fmt.Errorf("unexpected sql: %q", s[:min(40, len(s))])
}

func TestFetcherFirstAndSecondTick(t *testing.T) {
	fq := &fakeQuerier{
		processes: &conn.QueryResult{
			Columns: make([]conn.ResultColumn, 10),
			Rows: [][]string{
				{"id", "u", "a", "c", "", "1.5", "100", "200", "300", "SELECT 1"},
			},
		},
		header: &conn.QueryResult{
			Columns: make([]conn.ResultColumn, 11),
			Rows: [][]string{
				{"60", "24.8", "1", "1000", "500", "100", "1000", "1", "0", "0", "-1"},
			},
		},
	}
	f := NewFetcher(fq)

	ctx := context.Background()
	snap1, rates1, err := f.Fetch(ctx)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if rates1 != (Rates{}) {
		t.Errorf("first-tick rates = %+v, want zero", rates1)
	}
	if len(snap1.Processes) != 1 || snap1.Header.Version != "24.8" {
		t.Errorf("snap1 = %+v", snap1)
	}

	// Advance the header counters and re-fetch.
	fq.header.Rows[0][3] = "1200" // queries_total 1000 -> 1200
	fq.header.Rows[0][4] = "700"  // inserted_rows_total 500 -> 700

	// Guarantee dt > 0 between Fetch calls.
	time.Sleep(10 * time.Millisecond)
	_, rates2, err := f.Fetch(ctx)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if rates2.QueriesPerSec <= 0 {
		t.Errorf("expected positive QueriesPerSec, got %v", rates2.QueriesPerSec)
	}
	if rates2.InsertRowsPerSec <= 0 {
		t.Errorf("expected positive InsertRowsPerSec, got %v", rates2.InsertRowsPerSec)
	}
}

func TestFetcherErrorPropagates(t *testing.T) {
	fq := &fakeQuerier{err: fmt.Errorf("boom")}
	f := NewFetcher(fq)
	_, _, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseProcessesBadElapsed(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: []conn.ResultColumn{
			{Name: "query_id"}, {Name: "user"}, {Name: "initial_address"},
			{Name: "client"}, {Name: "database"}, {Name: "elapsed"},
			{Name: "read_rows"}, {Name: "read_bytes"}, {Name: "memory_usage"},
			{Name: "query"},
		},
		Rows: [][]string{
			{"id", "u", "a", "c", "d", "not-a-number", "0", "0", "0", "q"},
		},
	}
	_, err := parseProcesses(qr)
	if err == nil {
		t.Fatal("expected error on bad elapsed, got nil")
	}
}
