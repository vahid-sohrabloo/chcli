package chtop

import (
	"context"
	"strings"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func TestParseDatabaseRowsOrdersByBytes(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: make([]conn.ResultColumn, 4),
		Rows: [][]string{
			{"events", "87", "34521481216", "1234567890"},
			{"analytics", "28", "9345678912", "234567890"},
		},
	}
	dbs, err := parseDatabases(qr)
	if err != nil {
		t.Fatalf("parseDatabases: %v", err)
	}
	if len(dbs) != 2 || dbs[0].Name != "events" || dbs[0].Bytes != 34_521_481_216 {
		t.Errorf("rows = %+v", dbs)
	}
}

func TestFetchTablesUsesDBParam(t *testing.T) {
	q := &fakeParamQuerier{panel: &conn.QueryResult{Columns: make([]conn.ResultColumn, 6)}}
	_, err := FetchTables(context.Background(), q, "events")
	if err != nil {
		t.Fatalf("FetchTables: %v", err)
	}
	if !strings.Contains(q.lastSQL, "{db:String}") {
		t.Errorf("expected {db:String} in SQL, got %q", q.lastSQL)
	}
	found := false
	for _, p := range q.lastParams {
		if p().Name == "db" {
			found = true
		}
	}
	if !found {
		t.Error("expected db parameter to be passed")
	}
}

func TestFetchPartitionsUsesTwoParams(t *testing.T) {
	q := &fakeParamQuerier{panel: &conn.QueryResult{Columns: make([]conn.ResultColumn, 4)}}
	_, err := FetchPartitions(context.Background(), q, "events", "hits")
	if err != nil {
		t.Fatalf("FetchPartitions: %v", err)
	}
	if len(q.lastParams) != 2 {
		t.Errorf("params = %d, want 2", len(q.lastParams))
	}
}

func TestFetchPartsUsesThreeParams(t *testing.T) {
	q := &fakeParamQuerier{panel: &conn.QueryResult{Columns: make([]conn.ResultColumn, 8)}}
	_, err := FetchParts(context.Background(), q, "events", "hits", "20260422")
	if err != nil {
		t.Fatalf("FetchParts: %v", err)
	}
	if len(q.lastParams) != 3 {
		t.Errorf("params = %d, want 3", len(q.lastParams))
	}
}
