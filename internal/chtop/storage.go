package chtop

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// DBRow / TableRow / PartitionRow / PartRow are the typed rows for each
// level of the Storage tab's drilldown.
type DBRow struct {
	Name   string
	Tables int
	Bytes  uint64
	Rows   uint64
}

type TableRow struct {
	Name        string
	Parts       int
	Bytes       uint64
	Rows        uint64
	Marks       uint64
	BytesPerRow float64
}

type PartitionRow struct {
	Name  string
	Parts int
	Bytes uint64
	Rows  uint64
}

type PartRow struct {
	Name             string
	Rows             uint64
	Bytes            uint64
	Level            int
	MinTime, MaxTime string
	Marks            uint64
	ModificationTime string
}

const sqlDatabases = `
SELECT database,
       count(DISTINCT table) AS tables,
       sum(bytes_on_disk)    AS bytes,
       sum(rows)             AS rows
FROM   system.parts WHERE active
GROUP  BY database ORDER BY bytes DESC
` + suppressLogging

// sqlTables — bytes_per_row is computed client-side; ClickHouse rejects
// referencing an aggregate alias (e.g. sum(rows) AS rows) from inside
// another aggregate expression.
const sqlTables = `
SELECT table,
       count()             AS parts,
       sum(bytes_on_disk)  AS bytes,
       sum(rows)           AS row_count,
       sum(marks)          AS marks
FROM   system.parts WHERE active AND database = {db:String}
GROUP  BY table ORDER BY bytes DESC
` + suppressLogging

const sqlPartitions = `
SELECT partition,
       count()             AS parts,
       sum(bytes_on_disk)  AS bytes,
       sum(rows)           AS rows
FROM   system.parts
WHERE  active AND database = {db:String} AND table = {t:String}
GROUP  BY partition ORDER BY partition DESC
` + suppressLogging

const sqlParts = `
SELECT name, rows, bytes_on_disk, level,
       toString(min_time)          AS min_time,
       toString(max_time)          AS max_time,
       marks,
       toString(modification_time) AS modification_time
FROM   system.parts
WHERE  active AND database = {db:String} AND table = {t:String} AND partition = {p:String}
ORDER  BY bytes_on_disk DESC
LIMIT  1000
` + suppressLogging

func FetchDatabases(ctx context.Context, q ParamQuerier) ([]DBRow, error) {
	res, err := q.QueryAllWithParams(ctx, sqlDatabases)
	if err != nil {
		return nil, fmt.Errorf("databases: %w", err)
	}
	return parseDatabases(res)
}

func FetchTables(ctx context.Context, q ParamQuerier, db string) ([]TableRow, error) {
	res, err := q.QueryAllWithParams(ctx, sqlTables, conn.StringParam("db", db))
	if err != nil {
		return nil, fmt.Errorf("tables: %w", err)
	}
	return parseTables(res)
}

func FetchPartitions(ctx context.Context, q ParamQuerier, db, table string) ([]PartitionRow, error) {
	res, err := q.QueryAllWithParams(ctx, sqlPartitions,
		conn.StringParam("db", db), conn.StringParam("t", table))
	if err != nil {
		return nil, fmt.Errorf("partitions: %w", err)
	}
	return parsePartitions(res)
}

func FetchParts(ctx context.Context, q ParamQuerier, db, table, partition string) ([]PartRow, error) {
	res, err := q.QueryAllWithParams(ctx, sqlParts,
		conn.StringParam("db", db),
		conn.StringParam("t", table),
		conn.StringParam("p", partition))
	if err != nil {
		return nil, fmt.Errorf("parts: %w", err)
	}
	return parseParts(res)
}

func parseDatabases(qr *conn.QueryResult) ([]DBRow, error) {
	out := make([]DBRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("row %d: expected 4 cols, got %d", i, len(row))
		}
		tables, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, fmt.Errorf("row %d tables: %w", i, err)
		}
		bytes, err := parseUint64Lenient(row[2])
		if err != nil {
			return nil, fmt.Errorf("row %d bytes: %w", i, err)
		}
		rows, err := parseUint64Lenient(row[3])
		if err != nil {
			return nil, fmt.Errorf("row %d rows: %w", i, err)
		}
		out = append(out, DBRow{Name: row[0], Tables: tables, Bytes: bytes, Rows: rows})
	}
	return out, nil
}

func parseTables(qr *conn.QueryResult) ([]TableRow, error) {
	out := make([]TableRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 5 {
			return nil, fmt.Errorf("row %d: expected 5 cols, got %d", i, len(row))
		}
		parts, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, fmt.Errorf("row %d parts: %w", i, err)
		}
		bytes, _ := parseUint64Lenient(row[2])
		rows, _ := parseUint64Lenient(row[3])
		marks, _ := parseUint64Lenient(row[4])
		var bpr float64
		if rows > 0 {
			bpr = float64(bytes) / float64(rows)
		}
		out = append(out, TableRow{
			Name: row[0], Parts: parts, Bytes: bytes, Rows: rows,
			Marks: marks, BytesPerRow: bpr,
		})
	}
	return out, nil
}

func parsePartitions(qr *conn.QueryResult) ([]PartitionRow, error) {
	out := make([]PartitionRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("row %d: expected 4 cols, got %d", i, len(row))
		}
		parts, _ := strconv.Atoi(row[1])
		bytes, _ := parseUint64Lenient(row[2])
		rows, _ := parseUint64Lenient(row[3])
		out = append(out, PartitionRow{Name: row[0], Parts: parts, Bytes: bytes, Rows: rows})
	}
	return out, nil
}

func parseParts(qr *conn.QueryResult) ([]PartRow, error) {
	out := make([]PartRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 8 {
			return nil, fmt.Errorf("row %d: expected 8 cols, got %d", i, len(row))
		}
		rows, _ := parseUint64Lenient(row[1])
		bytes, _ := parseUint64Lenient(row[2])
		level, _ := strconv.Atoi(row[3])
		marks, _ := parseUint64Lenient(row[6])
		out = append(out, PartRow{
			Name: row[0], Rows: rows, Bytes: bytes, Level: level,
			MinTime: row[4], MaxTime: row[5], Marks: marks,
			ModificationTime: row[7],
		})
	}
	return out, nil
}
