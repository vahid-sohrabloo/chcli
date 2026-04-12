package metacmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/render"
)

// splitDBTable splits "db.table" into (db, table).  If there is no dot, it
// returns (defaultDB, input).
func splitDBTable(input, defaultDB string) (string, string) {
	before, after, ok := strings.Cut(input, ".")
	if !ok {
		return defaultDB, input
	}
	return before, after
}

// escapeSingleQuote replaces every single-quote in s with two single-quotes,
// safe for embedding in a single-quoted SQL string literal.
func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// handleListDatabases lists all databases.
func handleListDatabases(ctx context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	qr, err := hctx.Conn.Query(ctx, "SELECT name FROM system.databases ORDER BY name")
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListTables lists tables in the current (or specified) database.
func handleListTables(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	db := hctx.CurrentDB
	if len(args) > 0 {
		db = args[0]
	}
	sql := fmt.Sprintf(
		"SELECT name, engine FROM system.tables WHERE database = '%s' ORDER BY name",
		escapeSingleQuote(db),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListTablesExtended lists tables with row count and size.
func handleListTablesExtended(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	db := hctx.CurrentDB
	if len(args) > 0 {
		db = args[0]
	}
	sql := fmt.Sprintf(
		"SELECT name, engine, total_rows, formatReadableSize(total_bytes) AS size"+
			" FROM system.tables WHERE database = '%s' ORDER BY name",
		escapeSingleQuote(db),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleDescribeTable describes columns of a table.
func handleDescribeTable(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\d requires a table name")
	}
	db, table := splitDBTable(args[0], hctx.CurrentDB)
	sql := fmt.Sprintf(
		"SELECT name, type, default_kind, default_expression"+
			" FROM system.columns WHERE database='%s' AND table='%s' ORDER BY position",
		escapeSingleQuote(db), escapeSingleQuote(table),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleDescribeTableExtended describes columns including comment and codec.
func handleDescribeTableExtended(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\d+ requires a table name")
	}
	db, table := splitDBTable(args[0], hctx.CurrentDB)
	sql := fmt.Sprintf(
		"SELECT name, type, default_kind, default_expression, comment"+
			" FROM system.columns WHERE database='%s' AND table='%s' ORDER BY position",
		escapeSingleQuote(db), escapeSingleQuote(table),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListDictionaries lists tables with engine=Dictionary.
func handleListDictionaries(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	db := hctx.CurrentDB
	if len(args) > 0 {
		db = args[0]
	}
	sql := fmt.Sprintf(
		"SELECT name, engine FROM system.tables WHERE database='%s' AND engine='Dictionary' ORDER BY name",
		escapeSingleQuote(db),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListMaterializedViews lists tables with engine=MaterializedView.
func handleListMaterializedViews(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	db := hctx.CurrentDB
	if len(args) > 0 {
		db = args[0]
	}
	sql := fmt.Sprintf(
		"SELECT name, engine FROM system.tables WHERE database='%s' AND engine='MaterializedView' ORDER BY name",
		escapeSingleQuote(db),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListViews lists tables with engine in (View, MaterializedView).
func handleListViews(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	db := hctx.CurrentDB
	if len(args) > 0 {
		db = args[0]
	}
	sql := fmt.Sprintf(
		"SELECT name, engine FROM system.tables WHERE database='%s' AND engine IN ('View','MaterializedView') ORDER BY name",
		escapeSingleQuote(db),
	)
	qr, err := hctx.Conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}

// handleListProcesses lists running queries from system.processes.
func handleListProcesses(ctx context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	qr, err := hctx.Conn.Query(ctx,
		"SELECT query_id, user, query, elapsed, read_rows, memory_usage"+
			" FROM system.processes ORDER BY elapsed DESC",
	)
	if err != nil {
		return nil, err
	}
	cols := make([]string, len(qr.Columns))
	for i, c := range qr.Columns {
		cols[i] = c.Name
	}
	return &Result{Output: render.RenderTable(cols, qr.Rows, 0)}, nil
}
