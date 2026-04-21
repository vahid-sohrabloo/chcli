package chtop

import (
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
