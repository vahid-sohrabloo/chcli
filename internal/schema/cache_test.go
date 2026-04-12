package schema_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/schema"
)

func clickhouseAvailable() bool {
	return os.Getenv("CHCLI_TEST_HOST") != ""
}

func testConnStr() string {
	connStr := os.Getenv("CHCLI_TEST_CONNSTR")
	if connStr == "" {
		connStr = "clickhouse://default@localhost:9000/default"
	}
	return connStr
}

func TestCacheRefresh(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := schema.New(testConnStr())
	result := c.Refresh(context.Background())
	if result.HasErrors() {
		t.Fatalf("Refresh errors: %v", result.Errors)
	}

	if len(c.Databases) < 1 {
		t.Errorf("expected at least 1 database, got %d", len(c.Databases))
	}

	found := slices.Contains(c.Databases, "system")
	if !found {
		t.Errorf("expected 'system' database; got %v", c.Databases)
	}

	if len(c.Functions) < 1 {
		t.Errorf("expected at least 1 function, got %d", len(c.Functions))
	}
}

func TestCacheTablesForDatabase(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := schema.New(testConnStr())
	c.Refresh(context.Background())

	tables := c.TablesForDatabase("system")
	if len(tables) < 1 {
		t.Errorf("expected at least 1 table in system, got %d", len(tables))
	}
}

func TestCacheColumnsForTable(t *testing.T) {
	if !clickhouseAvailable() {
		t.Skip("CHCLI_TEST_HOST not set")
	}

	c := schema.New(testConnStr())
	c.Refresh(context.Background())

	cols := c.ColumnsForTable("system", "databases")
	if len(cols) < 1 {
		t.Errorf("expected at least 1 column in system.databases, got %d", len(cols))
	}

	found := false
	for _, col := range cols {
		if col.Name == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'name' column; got %v", cols)
	}
}
