package schema

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// TableInfo holds metadata about a ClickHouse table.
type TableInfo struct {
	Name       string
	Database   string
	Engine     string
	TotalRows  uint64
	TotalBytes uint64
	Comment    string
}

// ColumnInfo holds metadata about a ClickHouse table column.
type ColumnInfo struct {
	Name        string
	Type        string
	DefaultKind string
	DefaultExpr string
	Comment     string
}

// FunctionInfo holds metadata about a ClickHouse function.
type FunctionInfo struct {
	Name          string
	IsAggregate   bool
	Description   string
	Syntax        string
	Arguments     string
	ReturnedValue string
}

// Cache holds introspected ClickHouse schema metadata.
// Function metadata is served from internal/functions (embedded), not loaded here.
type Cache struct {
	connStr   string // connection string for creating dedicated connections
	Databases []string
	Tables    map[string][]TableInfo  // db name → tables
	Columns   map[string][]ColumnInfo // "db.table" → columns
	Types     []string
	Settings  []string
}

// New creates a Cache. Uses connStr to open dedicated connections for loading.
func New(connStr string) *Cache {
	return &Cache{
		connStr: connStr,
		Tables:  make(map[string][]TableInfo),
		Columns: make(map[string][]ColumnInfo),
	}
}

// RefreshResult holds the outcome of each schema loading step.
type RefreshResult struct {
	Databases int
	Tables    int
	Columns   int
	Types     int
	Settings  int
	Errors    []string
}

// HasErrors reports whether the refresh encountered any errors.
func (r RefreshResult) HasErrors() bool { return len(r.Errors) > 0 }

// Summary returns a one-line description of what was loaded.
func (r RefreshResult) Summary() string {
	s := fmt.Sprintf("Schema: %d databases, %d tables, %d columns, %d types, %d settings",
		r.Databases, r.Tables, r.Columns, r.Types, r.Settings)
	if len(r.Errors) > 0 {
		s += fmt.Sprintf(" (%d errors)", len(r.Errors))
	}
	return s
}

// Refresh repopulates all cache fields using a dedicated connection.
// Each step is independent — failures are collected but don't stop other loads.
func (c *Cache) Refresh(ctx context.Context) *RefreshResult {
	result := &RefreshResult{}

	// Open a dedicated connection for schema loading.
	schemaConn, err := conn.Connect(ctx, c.connStr)
	if err != nil {
		result.Errors = append(result.Errors, "connect: "+err.Error())
		return result
	}
	defer schemaConn.Close()

	if err := c.loadDatabases(ctx, schemaConn); err != nil {
		result.Errors = append(result.Errors, "databases: "+err.Error())
	} else {
		result.Databases = len(c.Databases)
	}

	if err := c.loadTables(ctx, schemaConn); err != nil {
		result.Errors = append(result.Errors, "tables: "+err.Error())
	} else {
		for _, tables := range c.Tables {
			result.Tables += len(tables)
		}
	}

	if err := c.loadColumns(ctx, schemaConn); err != nil {
		result.Errors = append(result.Errors, "columns: "+err.Error())
	} else {
		for _, cols := range c.Columns {
			result.Columns += len(cols)
		}
	}

	// Functions: skip loading from server — use embedded data instead.
	// Embedded functions have better metadata (syntax, description).
	// UDFs could be loaded separately if needed in the future.

	if err := c.loadTypes(ctx, schemaConn); err != nil {
		result.Errors = append(result.Errors, "types: "+err.Error())
	} else {
		result.Types = len(c.Types)
	}

	if err := c.loadSettings(ctx, schemaConn); err != nil {
		result.Errors = append(result.Errors, "settings: "+err.Error())
	} else {
		result.Settings = len(c.Settings)
	}

	return result
}

func (c *Cache) loadDatabases(ctx context.Context, sc *conn.Conn) error {
	result, err := sc.QueryAll(ctx, "SELECT name FROM system.databases ORDER BY name")
	if err != nil {
		return err
	}
	c.Databases = make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			c.Databases = append(c.Databases, row[0])
		}
	}
	return nil
}

func (c *Cache) loadTables(ctx context.Context, sc *conn.Conn) error {
	result, err := sc.QueryAll(ctx,
		"SELECT database, name, engine, total_rows, total_bytes, comment "+
			"FROM system.tables "+
			"WHERE database NOT IN ('INFORMATION_SCHEMA', 'information_schema') "+
			"ORDER BY database, name")
	if err != nil {
		return err
	}
	c.Tables = make(map[string][]TableInfo)
	for _, row := range result.Rows {
		if len(row) < 6 {
			continue
		}
		totalRows, _ := strconv.ParseUint(row[3], 10, 64)
		totalBytes, _ := strconv.ParseUint(row[4], 10, 64)
		info := TableInfo{
			Database:   row[0],
			Name:       row[1],
			Engine:     row[2],
			TotalRows:  totalRows,
			TotalBytes: totalBytes,
			Comment:    row[5],
		}
		c.Tables[info.Database] = append(c.Tables[info.Database], info)
	}
	return nil
}

func (c *Cache) loadColumns(ctx context.Context, sc *conn.Conn) error {
	result, err := sc.QueryAll(ctx,
		"SELECT database, table, name, type, default_kind, default_expression, comment "+
			"FROM system.columns "+
			"WHERE database NOT IN ('INFORMATION_SCHEMA', 'information_schema') "+
			"ORDER BY database, table, position")
	if err != nil {
		return err
	}
	c.Columns = make(map[string][]ColumnInfo)
	for _, row := range result.Rows {
		if len(row) < 7 {
			continue
		}
		key := row[0] + "." + row[1]
		info := ColumnInfo{
			Name:        row[2],
			Type:        row[3],
			DefaultKind: row[4],
			DefaultExpr: row[5],
			Comment:     row[6],
		}
		c.Columns[key] = append(c.Columns[key], info)
	}
	return nil
}

func (c *Cache) loadTypes(ctx context.Context, sc *conn.Conn) error {
	result, err := sc.QueryAll(ctx, "SELECT name FROM system.data_type_families ORDER BY name")
	if err != nil {
		return err
	}
	c.Types = make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			c.Types = append(c.Types, row[0])
		}
	}
	return nil
}

func (c *Cache) loadSettings(ctx context.Context, sc *conn.Conn) error {
	result, err := sc.QueryAll(ctx, "SELECT name FROM system.settings ORDER BY name")
	if err != nil {
		return err
	}
	c.Settings = make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			c.Settings = append(c.Settings, row[0])
		}
	}
	return nil
}

// TablesForDatabase returns the tables for the given database.
func (c *Cache) TablesForDatabase(db string) []TableInfo {
	return c.Tables[db]
}

// ColumnsForTable returns the columns for the given database and table.
func (c *Cache) ColumnsForTable(db, table string) []ColumnInfo {
	return c.Columns[db+"."+table]
}

// TableNames returns the names of all tables in the given database.
func (c *Cache) TableNames(db string) []string {
	tables := c.Tables[db]
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	return names
}

// ColumnNames returns the names of all columns for the given database and table.
func (c *Cache) ColumnNames(db, table string) []string {
	cols := c.Columns[db+"."+table]
	names := make([]string, len(cols))
	for i, col := range cols {
		names[i] = col.Name
	}
	return names
}

// AllTableNames returns the names of all tables across all databases.
func (c *Cache) AllTableNames() []string {
	var names []string
	for _, tables := range c.Tables {
		for _, t := range tables {
			names = append(names, t.Name)
		}
	}
	return names
}

// AllColumnNames returns deduplicated column names across all tables.
func (c *Cache) AllColumnNames() []string {
	seen := make(map[string]struct{})
	var names []string
	for _, cols := range c.Columns {
		for _, col := range cols {
			if _, ok := seen[col.Name]; !ok {
				seen[col.Name] = struct{}{}
				names = append(names, col.Name)
			}
		}
	}
	return names
}
