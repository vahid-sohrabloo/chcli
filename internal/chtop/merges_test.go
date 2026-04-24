package chtop

import (
	"context"
	"strings"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func TestParseMergeRows(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: make([]conn.ResultColumn, 10),
		Rows: [][]string{
			{"events", "hits_raw", "18.2", "0.62", "5", "['a','b']", "202604_0_9999_2", "104857600", "1200000", "0"},
			{"logs", "access", "3.1", "0.18", "3", "['c']", "p0_0_123_1", "10485760", "540000", "0"},
		},
	}
	merges, err := parseMerges(qr)
	if err != nil {
		t.Fatalf("parseMerges: %v", err)
	}
	if len(merges) != 2 {
		t.Fatalf("len = %d, want 2", len(merges))
	}
	if merges[0].Database != "events" || merges[0].Elapsed != 18.2 || merges[0].Progress != 0.62 {
		t.Errorf("row 0 mismatch: %+v", merges[0])
	}
	if merges[0].IsMutation {
		t.Error("row 0 IsMutation = true, want false")
	}
}

func TestParseMutationRows(t *testing.T) {
	qr := &conn.QueryResult{
		Columns: make([]conn.ResultColumn, 9),
		Rows: [][]string{
			{"events", "hits_raw", "mutation-00023",
				"DELETE WHERE user_id = 'x'",
				"2026-04-22 12:00:00",
				"7", "0", "", ""},
			{"logs", "access", "mutation-00024",
				"ALTER UPDATE foo = 1 WHERE id = 2",
				"2026-04-22 12:05:00",
				"3", "0", "p0_1_2_0", "DB::Exception: part is missing"},
		},
	}
	muts, err := parseMutations(qr)
	if err != nil {
		t.Fatalf("parseMutations: %v", err)
	}
	if len(muts) != 2 {
		t.Fatalf("len = %d, want 2", len(muts))
	}
	if muts[1].LatestFailReason == "" || !strings.Contains(muts[1].LatestFailReason, "missing") {
		t.Errorf("row 1 LatestFailReason = %q", muts[1].LatestFailReason)
	}
}

// capturingQuerier records the last SQL sent to QueryAll and delegates to next.
type capturingQuerier struct {
	next *fakeQuerier
	into *string
}

func (c *capturingQuerier) QueryAll(ctx context.Context, sql string) (*conn.QueryResult, error) {
	if c.into != nil && !strings.HasPrefix(strings.TrimLeft(sql, "\n "), "SELECT\n    database, table, mutation_id") {
		*c.into = sql
	}
	return c.next.QueryAll(ctx, sql)
}

func TestFetchMergesUsesSettingsSuffix(t *testing.T) {
	var gotSQL string
	fq := &fakeQuerier{}
	capturing := &capturingQuerier{next: fq, into: &gotSQL}
	_, _, err := FetchMerges(context.Background(), capturing)
	if err != nil {
		t.Fatalf("FetchMerges: %v", err)
	}
	if !strings.Contains(gotSQL, "log_queries = 0") {
		t.Errorf("expected log_queries=0 settings suffix in SQL, got: %q", gotSQL)
	}
}
