package conn_test

import (
	"context"
	"os"
	"strings"
	"testing"

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
	ctx := context.Background()
	c, err := conn.Connect(ctx, testConnStr())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestConnect(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)

	result, err := c.Query(context.Background(), "SELECT 1 AS num, 'hello' AS greeting")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0].Name != "num" {
		t.Errorf("col[0].Name = %q, want %q", result.Columns[0].Name, "num")
	}
	if result.Columns[1].Name != "greeting" {
		t.Errorf("col[1].Name = %q, want %q", result.Columns[1].Name, "greeting")
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0][0] != "1" {
		t.Errorf("row[0][0] = %q, want %q", result.Rows[0][0], "1")
	}
	if result.Rows[0][1] != "hello" {
		t.Errorf("row[0][1] = %q, want %q", result.Rows[0][1], "hello")
	}
}

func TestExec(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	if err := c.Exec(context.Background(), "SELECT 1"); err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestDataTypes(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		query string
		check func(t *testing.T, val string)
	}{
		{
			name:  "UInt64",
			query: "SELECT toUInt64(42) AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "42") },
		},
		{
			name:  "Int64 negative",
			query: "SELECT toInt64(-100) AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "-100") },
		},
		{
			name:  "Float64",
			query: "SELECT toFloat64(3.14) AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "3.14") },
		},
		{
			name:  "String",
			query: "SELECT 'hello world' AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "hello world") },
		},
		{
			name:  "Date",
			query: "SELECT toDate('2024-01-15') AS v",
			check: func(t *testing.T, val string) { assertContains(t, val, "2024-01-15") },
		},
		{
			name:  "DateTime",
			query: "SELECT toDateTime('2024-06-15 12:30:45') AS v",
			check: func(t *testing.T, val string) { assertContains(t, val, "2024-06-15") },
		},
		{
			name:  "UUID",
			query: "SELECT toUUID('12345678-1234-5678-1234-567812345678') AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "12345678-1234-5678-1234-567812345678") },
		},
		{
			name:  "Array",
			query: "SELECT [1, 2, 3] AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "[1, 2, 3]") },
		},
		{
			name:  "Nullable NULL",
			query: "SELECT NULL AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "NULL") },
		},
		{
			name:  "Boolean true",
			query: "SELECT true AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "true") },
		},
		{
			name:  "Boolean false",
			query: "SELECT false AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "false") },
		},
		{
			name:  "IPv4",
			query: "SELECT toIPv4('192.168.1.1') AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "192.168.1.1") },
		},
		{
			name:  "Empty string",
			query: "SELECT '' AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "") },
		},
		{
			name:  "Large number",
			query: "SELECT toUInt64(18446744073709551615) AS v",
			check: func(t *testing.T, val string) { assertEqual(t, val, "18446744073709551615") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.Query(ctx, tt.query)
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(result.Rows) != 1 || len(result.Rows[0]) != 1 {
				t.Fatalf("expected 1x1 result, got %dx%d", len(result.Rows), len(result.Rows[0]))
			}
			tt.check(t, result.Rows[0][0])
		})
	}
}

func TestQueryAll(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	result, err := c.QueryAll(ctx, "SELECT number FROM system.numbers LIMIT 100")
	if err != nil {
		t.Fatalf("QueryAll: %v", err)
	}
	if len(result.Rows) != 100 {
		t.Errorf("expected 100 rows, got %d", len(result.Rows))
	}
	if result.Rows[0][0] != "0" {
		t.Errorf("first row = %q, want %q", result.Rows[0][0], "0")
	}
	if result.Rows[99][0] != "99" {
		t.Errorf("last row = %q, want %q", result.Rows[99][0], "99")
	}
}

func TestQueryTruncation(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	// Query more than MaxRows (2000) — result should be truncated
	result, err := c.Query(ctx, "SELECT number FROM system.numbers LIMIT 3000")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Rows) > conn.MaxRows {
		t.Errorf("expected at most %d rows, got %d", conn.MaxRows, len(result.Rows))
	}
}

func TestQueryWithProgress(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	prog := &conn.Progress{}
	result, err := c.Query(ctx, "SELECT number FROM system.numbers LIMIT 1000", prog)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.Rows) != 1000 {
		t.Errorf("expected 1000 rows, got %d", len(result.Rows))
	}
	if prog.Elapsed <= 0 {
		t.Errorf("expected positive elapsed time, got %v", prog.Elapsed)
	}
}

func TestQueryError(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	_, err := c.Query(ctx, "SELECT * FROM nonexistent_table_xyz")
	if err == nil {
		t.Fatal("expected error for invalid table, got nil")
	}
}

func TestMultipleQueries(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, err := c.Query(ctx, "SELECT 1")
		if err != nil {
			t.Fatalf("Query %d: %v", i, err)
		}
		if len(result.Rows) != 1 {
			t.Errorf("Query %d: expected 1 row, got %d", i, len(result.Rows))
		}
	}
}

func TestServerVersion(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	v := c.ServerVersion()
	if v == "" {
		t.Fatal("ServerVersion returned empty string")
	}
	// Should be in format like "24.8.1"
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		t.Errorf("ServerVersion %q doesn't look like a version", v)
	}
}

func TestSystemTables(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	// These system queries are used by chcli's schema cache and should work on all LTS versions
	systemQueries := []struct {
		name  string
		query string
	}{
		{"databases", "SELECT name FROM system.databases ORDER BY name"},
		{"tables", "SELECT database, name, engine FROM system.tables WHERE database = 'system' LIMIT 10"},
		{"columns", "SELECT database, table, name, type FROM system.columns WHERE database = 'system' AND table = 'databases'"},
		{"data_type_families", "SELECT name FROM system.data_type_families ORDER BY name LIMIT 10"},
		{"settings", "SELECT name FROM system.settings LIMIT 10"},
		{"functions", "SELECT name, is_aggregate FROM system.functions LIMIT 10"},
		{"processes", "SELECT query_id, query FROM system.processes LIMIT 5"},
	}

	for _, sq := range systemQueries {
		t.Run(sq.name, func(t *testing.T) {
			result, err := c.QueryAll(ctx, sq.query)
			if err != nil {
				t.Fatalf("system.%s query failed: %v", sq.name, err)
			}
			if len(result.Columns) == 0 {
				t.Errorf("system.%s returned no columns", sq.name)
			}
		})
	}
}

func TestCreateAndQueryTempTable(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	if err := c.Exec(ctx, "CREATE TEMPORARY TABLE test_chcli (id UInt64, name String, ts DateTime)"); err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	if err := c.Exec(ctx, "INSERT INTO test_chcli VALUES (1, 'alice', '2024-01-01 00:00:00'), (2, 'bob', '2024-06-15 12:30:00')"); err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	result, err := c.Query(ctx, "SELECT id, name, ts FROM test_chcli ORDER BY id")
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if result.Rows[0][0] != "1" || result.Rows[0][1] != "alice" {
		t.Errorf("row 0 = %v, want [1 alice ...]", result.Rows[0])
	}
	if result.Rows[1][0] != "2" || result.Rows[1][1] != "bob" {
		t.Errorf("row 1 = %v, want [2 bob ...]", result.Rows[1])
	}
}

func TestExplainAST(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	result, err := c.QueryAll(ctx, "EXPLAIN AST SELECT 1")
	if err != nil {
		t.Fatalf("EXPLAIN AST: %v", err)
	}
	if len(result.Rows) == 0 {
		t.Error("EXPLAIN AST returned no rows")
	}
}

func TestKillQuery(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)

	// KillQuery on a non-existent ID should not error (it just matches nothing)
	if err := c.KillQuery("nonexistent-query-id-12345"); err != nil {
		t.Fatalf("KillQuery: %v", err)
	}
}

func TestReconnect(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := mustConnect(t)
	ctx := context.Background()

	// Query should work before reconnect
	if _, err := c.Query(ctx, "SELECT 1"); err != nil {
		t.Fatalf("pre-reconnect query: %v", err)
	}

	// Reconnect
	if err := c.Reconnect(ctx); err != nil {
		t.Fatalf("Reconnect: %v", err)
	}

	// Query should work after reconnect
	result, err := c.Query(ctx, "SELECT 2 AS v")
	if err != nil {
		t.Fatalf("post-reconnect query: %v", err)
	}
	if result.Rows[0][0] != "2" {
		t.Errorf("post-reconnect got %q, want %q", result.Rows[0][0], "2")
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func assertContains(t *testing.T, got, substr string) {
	t.Helper()
	if !strings.Contains(got, substr) {
		t.Errorf("got %q, want substring %q", got, substr)
	}
}
