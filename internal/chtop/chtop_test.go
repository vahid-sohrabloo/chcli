package chtop

import (
	"testing"

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
