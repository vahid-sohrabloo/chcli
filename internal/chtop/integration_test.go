// Integration tests for the chtop SQL. Opts in via CHCLI_TEST_HOST so
// `go test ./...` without a server set up is still a no-op. CI sets the env
// var and runs this suite against every ClickHouse version in the matrix.
package chtop_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func clickhouseAvailable() bool {
	return os.Getenv("CHCLI_TEST_HOST") != ""
}

func testConnStr() string {
	if s := os.Getenv("CHCLI_TEST_CONNSTR"); s != "" {
		return s
	}
	return "clickhouse://default@localhost:9000/default"
}

func mustConnect(t *testing.T) *conn.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := conn.Connect(ctx, testConnStr())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// Unique schema name per test run so parallel matrix jobs don't collide even
// if they somehow hit the same server.
func seedSchema() string {
	return "chtop_it_" + strings.ReplaceAll(time.Now().Format("20060102_150405.000"), ".", "_")
}

// seedStorage creates a small database + table + rows so Storage drilldown
// has something to find. Returns the database name and a cleanup func.
func seedStorage(t *testing.T, c *conn.Conn) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := seedSchema()
	stmts := []string{
		"CREATE DATABASE IF NOT EXISTS " + db,
		"CREATE TABLE " + db + ".seed (id UInt64, t DateTime DEFAULT now()) " +
			"ENGINE = MergeTree() PARTITION BY toYYYYMM(t) ORDER BY id",
		"INSERT INTO " + db + ".seed (id) SELECT number FROM numbers(100)",
	}
	for _, s := range stmts {
		if err := c.Exec(ctx, s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}
	return db, func() {
		dctx, dcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dcancel()
		_ = c.Exec(dctx, "DROP DATABASE IF EXISTS "+db+" SYNC")
	}
}

// ---------- Fetcher (system.processes + header) ----------

func TestIntegrationFetcher(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)
	f := chtop.NewFetcher(c)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	snap, rates, err := f.Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Header.Uptime <= 0 {
		t.Errorf("Uptime = %v, want > 0", snap.Header.Uptime)
	}
	if snap.Header.Version == "" {
		t.Errorf("Version is empty")
	}
	if rates != (chtop.Rates{}) {
		t.Errorf("first-tick Rates should be zero, got %+v", rates)
	}

	// Second tick: Rates become derivable from counters.
	time.Sleep(100 * time.Millisecond)
	_, rates2, err := f.Fetch(ctx)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	// Rates may still be zero on an otherwise-idle server, so we just
	// assert the derivation doesn't go negative.
	if rates2.QueriesPerSec < 0 || rates2.InsertRowsPerSec < 0 {
		t.Errorf("negative rate after second tick: %+v", rates2)
	}
}

// ---------- Merges + Mutations ----------

func TestIntegrationFetchMerges(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// The test server is typically quiet, so we don't assert non-empty —
	// we just assert the queries run and parse.
	merges, muts, err := chtop.FetchMerges(ctx, c)
	if err != nil {
		t.Fatalf("FetchMerges: %v", err)
	}
	// Sanity: if merges is non-empty each row has a database + table.
	for i, m := range merges {
		if m.Database == "" || m.Table == "" {
			t.Errorf("merge row %d missing database/table: %+v", i, m)
		}
	}
	for i, m := range muts {
		if m.MutationID == "" {
			t.Errorf("mutation row %d missing id: %+v", i, m)
		}
	}
}

// ---------- Charts (system.dashboards) ----------

func TestIntegrationLoadDashboardOverview(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	panels, err := chtop.LoadDashboard(ctx, c, "Overview")
	if err != nil {
		// Pre-24.x ClickHouse: system.dashboards missing or empty — the
		// data layer surfaces ErrNoDashboards. That's an expected path.
		if errors.Is(err, chtop.ErrNoDashboards) {
			t.Skipf("system.dashboards has no Overview panels on this server; skipping panel fetch test")
			return
		}
		t.Fatalf("LoadDashboard: %v", err)
	}
	if len(panels) == 0 {
		t.Fatalf("panels = 0, want > 0")
	}
	for i, p := range panels {
		if p.Title == "" {
			t.Errorf("panel %d has empty title: %+v", i, p)
		}
		if !strings.Contains(strings.ToLower(p.SQL), "select") {
			t.Errorf("panel %d SQL doesn't look like a SELECT: %q", i, p.SQL)
		}
	}
}

func TestIntegrationFetchPanelSeries(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	panels, err := chtop.LoadDashboard(ctx, c, "Overview")
	if err != nil {
		if errors.Is(err, chtop.ErrNoDashboards) {
			t.Skip("no Overview dashboard on this server")
		}
		t.Fatalf("LoadDashboard: %v", err)
	}

	// Run the first panel's SQL. On a near-empty test server the series may
	// be empty; we're just verifying the parameterized query runs and parses.
	pts, err := chtop.FetchPanelSeries(ctx, c, panels[0].SQL, 60, 3600)
	if err != nil {
		t.Fatalf("FetchPanelSeries(%q): %v", panels[0].Title, err)
	}
	for i, p := range pts {
		if p.T.IsZero() {
			t.Errorf("point %d has zero time", i)
		}
	}
}

// ---------- Storage drilldown ----------

func TestIntegrationStorageDrilldown(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)

	db, cleanup := seedStorage(t, c)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// L0: database list should contain the seed db.
	dbs, err := chtop.FetchDatabases(ctx, c)
	if err != nil {
		t.Fatalf("FetchDatabases: %v", err)
	}
	found := false
	for _, d := range dbs {
		if d.Name == db {
			found = true
			if d.Tables < 1 {
				t.Errorf("seed db has %d tables, want >= 1", d.Tables)
			}
			if d.Bytes == 0 {
				t.Errorf("seed db has 0 bytes, want > 0")
			}
		}
	}
	if !found {
		t.Fatalf("seed db %q not found in FetchDatabases; got %d dbs", db, len(dbs))
	}

	// L1: tables in the seed db.
	tables, err := chtop.FetchTables(ctx, c, db)
	if err != nil {
		t.Fatalf("FetchTables: %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "seed" {
		t.Fatalf("FetchTables = %+v, want 1 row named seed", tables)
	}
	if tables[0].Rows < 100 {
		t.Errorf("seed table has %d rows, want >= 100", tables[0].Rows)
	}

	// L2: partitions under the seed table.
	parts, err := chtop.FetchPartitions(ctx, c, db, "seed")
	if err != nil {
		t.Fatalf("FetchPartitions: %v", err)
	}
	if len(parts) < 1 {
		t.Fatalf("FetchPartitions returned 0 partitions")
	}
	partition := parts[0].Name

	// L3: individual parts within that partition.
	individualParts, err := chtop.FetchParts(ctx, c, db, "seed", partition)
	if err != nil {
		t.Fatalf("FetchParts: %v", err)
	}
	if len(individualParts) < 1 {
		t.Fatalf("FetchParts returned 0 parts for partition %q", partition)
	}
	for i, p := range individualParts {
		if p.Name == "" {
			t.Errorf("part %d has empty name", i)
		}
		if p.Rows == 0 {
			t.Errorf("part %d has 0 rows", i)
		}
	}
}

// ---------- Reconnect-on-close behavior ----------

func TestIntegrationReconnectsAfterBrokenSQL(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}
	c := mustConnect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// A bad query should fail AND typically tear down the chconn socket.
	_, err := c.QueryAll(ctx, "SELECT nonexistent_column FROM system.numbers LIMIT 1")
	if err == nil {
		t.Fatal("expected an error from bad SQL")
	}

	// A subsequent good query should succeed because QueryAll reconnects on
	// IsClosed and retries.
	res, err := c.QueryAll(ctx, "SELECT 42 AS answer")
	if err != nil {
		t.Fatalf("second query after bad SQL: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "42" {
		t.Errorf("unexpected second query result: %+v", res)
	}
}
