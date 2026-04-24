package chtop

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// DatabaseDetail is the data behind the Level-0 detail pane: the database's
// own metadata from system.databases plus the aggregated part stats that
// give you a "what's in this database" summary.
type DatabaseDetail struct {
	Name         string
	Engine       string
	UUID         string
	Comment      string
	Tables       int
	Bytes        uint64
	Rows         uint64
	Compressed   uint64
	Uncompressed uint64
	FirstModify  string
	LastModify   string
}

// Ratio returns uncompressed/compressed for the whole database.
func (d DatabaseDetail) Ratio() float64 {
	if d.Compressed == 0 {
		return 0
	}
	return float64(d.Uncompressed) / float64(d.Compressed)
}

// TableDetail is the data behind the Level-1 detail pane. Merges
// system.tables metadata (engine, keys, TTL expression) with the
// aggregated system.parts stats for a quick "how is this table laid out"
// view.
type TableDetail struct {
	Database      string
	Table         string
	Engine        string
	UUID          string
	Comment       string
	PartitionKey  string
	SortingKey    string
	PrimaryKey    string
	SamplingKey   string
	StoragePolicy string
	MetadataPath  string
	CreatedAt     string // metadata_modification_time from system.tables

	ActiveParts    int
	InactiveParts  int
	Bytes          uint64
	Rows           uint64
	Compressed     uint64
	Uncompressed   uint64
	Marks          uint64
	PrimaryKeyMem  uint64
	MinTime        string
	MaxTime        string
	OldestModified string
	NewestModified string
}

// Ratio returns uncompressed/compressed for the whole table.
func (d TableDetail) Ratio() float64 {
	if d.Compressed == 0 {
		return 0
	}
	return float64(d.Uncompressed) / float64(d.Compressed)
}

// PartitionDetail is the aggregated Level-2 view. LevelCounts is a
// {merge-level → part-count} map (e.g. {0:2, 1:5, 2:1}) that makes the
// merge-in-flight state obvious.
type PartitionDetail struct {
	Database       string
	Table          string
	Partition      string
	ActiveParts    int
	InactiveParts  int
	Bytes          uint64
	Rows           uint64
	Compressed     uint64
	Uncompressed   uint64
	Marks          uint64
	MinBlockNumber int64
	MaxBlockNumber int64
	MinTime        string
	MaxTime        string
	OldestModified string
	NewestModified string
	LevelCounts    map[int]int
}

// Ratio returns uncompressed/compressed for the whole partition.
func (d PartitionDetail) Ratio() float64 {
	if d.Compressed == 0 {
		return 0
	}
	return float64(d.Uncompressed) / float64(d.Compressed)
}

// PartDetail is the full system.parts row for one part.
type PartDetail struct {
	Database         string
	Table            string
	Partition        string
	Name             string
	PartType         string
	Level            int
	Active           bool
	Rows             uint64
	Bytes            uint64
	Compressed       uint64
	Uncompressed     uint64
	Marks            uint64
	PrimaryKeyMem    uint64
	MinBlockNumber   int64
	MaxBlockNumber   int64
	MinTime          string
	MaxTime          string
	ModificationTime string
	DiskName         string
	Path             string
	Refcount         int
}

// Ratio returns uncompressed/compressed for this part.
func (d PartDetail) Ratio() float64 {
	if d.Compressed == 0 {
		return 0
	}
	return float64(d.Uncompressed) / float64(d.Compressed)
}

// Subquery-heavy form is used throughout these queries so that a single
// round-trip returns one row regardless of whether system.databases /
// system.tables carries the matching row — LEFT JOINs would need a
// GROUP BY that ClickHouse rejects in some versions for aggregate-of-
// aggregate shapes. Subqueries avoid that.
const sqlDatabaseDetail = `
SELECT
    (SELECT engine          FROM system.databases WHERE name = {db:String})                       AS engine,
    (SELECT toString(uuid)  FROM system.databases WHERE name = {db:String})                       AS uuid,
    (SELECT comment         FROM system.databases WHERE name = {db:String})                       AS comment,
    (SELECT count(DISTINCT table)               FROM system.parts WHERE active AND database = {db:String}) AS tables,
    (SELECT sum(bytes_on_disk)                  FROM system.parts WHERE active AND database = {db:String}) AS bytes,
    (SELECT sum(rows)                           FROM system.parts WHERE active AND database = {db:String}) AS rows,
    (SELECT sum(data_compressed_bytes)          FROM system.parts WHERE active AND database = {db:String}) AS compressed,
    (SELECT sum(data_uncompressed_bytes)        FROM system.parts WHERE active AND database = {db:String}) AS uncompressed,
    (SELECT toString(min(modification_time))    FROM system.parts WHERE active AND database = {db:String}) AS first_modify,
    (SELECT toString(max(modification_time))    FROM system.parts WHERE active AND database = {db:String}) AS last_modify
` + suppressLogging

func FetchDatabaseDetail(ctx context.Context, q ParamQuerier, db string) (DatabaseDetail, error) {
	res, err := q.QueryAllWithParams(ctx, sqlDatabaseDetail, conn.StringParam("db", db))
	if err != nil {
		return DatabaseDetail{}, fmt.Errorf("database detail: %w", err)
	}
	if len(res.Rows) == 0 {
		return DatabaseDetail{Name: db}, nil
	}
	r := res.Rows[0]
	if len(r) < 10 {
		return DatabaseDetail{}, fmt.Errorf("database detail: expected 10 cols, got %d", len(r))
	}
	tables, _ := strconv.Atoi(r[3])
	bytes, _ := parseUint64Lenient(r[4])
	rows, _ := parseUint64Lenient(r[5])
	compressed, _ := parseUint64Lenient(r[6])
	uncompressed, _ := parseUint64Lenient(r[7])
	return DatabaseDetail{
		Name: db, Engine: r[0], UUID: r[1], Comment: r[2],
		Tables: tables, Bytes: bytes, Rows: rows,
		Compressed: compressed, Uncompressed: uncompressed,
		FirstModify: r[8], LastModify: r[9],
	}, nil
}

const sqlTableDetail = `
SELECT
    (SELECT engine                                     FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS engine,
    (SELECT toString(uuid)                             FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS uuid,
    (SELECT comment                                    FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS comment,
    (SELECT partition_key                              FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS partition_key,
    (SELECT sorting_key                                FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS sorting_key,
    (SELECT primary_key                                FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS primary_key,
    (SELECT sampling_key                               FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS sampling_key,
    (SELECT storage_policy                             FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS storage_policy,
    (SELECT metadata_path                              FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS metadata_path,
    (SELECT toString(metadata_modification_time)       FROM system.tables WHERE database = {db:String} AND name = {t:String}) AS created_at,
    (SELECT countIf(active)                            FROM system.parts  WHERE database = {db:String} AND table = {t:String}) AS active_parts,
    (SELECT countIf(NOT active)                        FROM system.parts  WHERE database = {db:String} AND table = {t:String}) AS inactive_parts,
    (SELECT sum(bytes_on_disk)                         FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS bytes,
    (SELECT sum(rows)                                  FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS rows,
    (SELECT sum(data_compressed_bytes)                 FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS compressed,
    (SELECT sum(data_uncompressed_bytes)               FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS uncompressed,
    (SELECT sum(marks)                                 FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS marks,
    (SELECT sum(primary_key_bytes_in_memory)           FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS pk_mem,
    (SELECT toString(min(min_time))                    FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS min_time,
    (SELECT toString(max(max_time))                    FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS max_time,
    (SELECT toString(min(modification_time))           FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS oldest_modify,
    (SELECT toString(max(modification_time))           FROM system.parts  WHERE database = {db:String} AND table = {t:String} AND active) AS newest_modify
` + suppressLogging

func FetchTableDetail(ctx context.Context, q ParamQuerier, db, table string) (TableDetail, error) {
	res, err := q.QueryAllWithParams(ctx, sqlTableDetail,
		conn.StringParam("db", db), conn.StringParam("t", table))
	if err != nil {
		return TableDetail{}, fmt.Errorf("table detail: %w", err)
	}
	if len(res.Rows) == 0 {
		return TableDetail{Database: db, Table: table}, nil
	}
	r := res.Rows[0]
	if len(r) < 22 {
		return TableDetail{}, fmt.Errorf("table detail: expected 22 cols, got %d", len(r))
	}
	active, _ := strconv.Atoi(r[10])
	inactive, _ := strconv.Atoi(r[11])
	bytes, _ := parseUint64Lenient(r[12])
	rows, _ := parseUint64Lenient(r[13])
	compressed, _ := parseUint64Lenient(r[14])
	uncompressed, _ := parseUint64Lenient(r[15])
	marks, _ := parseUint64Lenient(r[16])
	pkMem, _ := parseUint64Lenient(r[17])
	return TableDetail{
		Database: db, Table: table,
		Engine: r[0], UUID: r[1], Comment: r[2],
		PartitionKey: r[3], SortingKey: r[4], PrimaryKey: r[5], SamplingKey: r[6],
		StoragePolicy: r[7], MetadataPath: r[8], CreatedAt: r[9],
		ActiveParts: active, InactiveParts: inactive,
		Bytes: bytes, Rows: rows,
		Compressed: compressed, Uncompressed: uncompressed,
		Marks: marks, PrimaryKeyMem: pkMem,
		MinTime: r[18], MaxTime: r[19],
		OldestModified: r[20], NewestModified: r[21],
	}, nil
}

const sqlPartitionDetail = `
SELECT
    countIf(active)                                 AS active_parts,
    countIf(NOT active)                             AS inactive_parts,
    sumIf(bytes_on_disk, active)                    AS bytes,
    sumIf(rows, active)                             AS rows,
    sumIf(data_compressed_bytes, active)            AS compressed,
    sumIf(data_uncompressed_bytes, active)          AS uncompressed,
    sumIf(marks, active)                            AS marks,
    toInt64(ifNull(minIf(min_block_number, active), toInt64(0))) AS min_block,
    toInt64(ifNull(maxIf(max_block_number, active), toInt64(0))) AS max_block,
    toString(minIf(min_time, active))               AS min_time,
    toString(maxIf(max_time, active))               AS max_time,
    toString(minIf(modification_time, active))      AS oldest_modify,
    toString(maxIf(modification_time, active))      AS newest_modify,
    arrayStringConcat(
        arrayMap(x -> concat(toString(x.1), ':', toString(x.2)),
                 arraySort(groupArrayIf((level, 1), active))), ','
    ) AS lvl_pairs
FROM system.parts
WHERE database = {db:String} AND table = {t:String} AND partition = {p:String}
` + suppressLogging

func FetchPartitionDetail(ctx context.Context, q ParamQuerier, db, table, partition string) (PartitionDetail, error) {
	res, err := q.QueryAllWithParams(ctx, sqlPartitionDetail,
		conn.StringParam("db", db),
		conn.StringParam("t", table),
		conn.StringParam("p", partition))
	if err != nil {
		return PartitionDetail{}, fmt.Errorf("partition detail: %w", err)
	}
	if len(res.Rows) == 0 {
		return PartitionDetail{Database: db, Table: table, Partition: partition}, nil
	}
	r := res.Rows[0]
	if len(r) < 14 {
		return PartitionDetail{}, fmt.Errorf("partition detail: expected 14 cols, got %d", len(r))
	}
	active, _ := strconv.Atoi(r[0])
	inactive, _ := strconv.Atoi(r[1])
	bytes, _ := parseUint64Lenient(r[2])
	rows, _ := parseUint64Lenient(r[3])
	compressed, _ := parseUint64Lenient(r[4])
	uncompressed, _ := parseUint64Lenient(r[5])
	marks, _ := parseUint64Lenient(r[6])
	minBlock, _ := strconv.ParseInt(r[7], 10, 64)
	maxBlock, _ := strconv.ParseInt(r[8], 10, 64)

	return PartitionDetail{
		Database: db, Table: table, Partition: partition,
		ActiveParts: active, InactiveParts: inactive,
		Bytes: bytes, Rows: rows,
		Compressed: compressed, Uncompressed: uncompressed,
		Marks:          marks,
		MinBlockNumber: minBlock, MaxBlockNumber: maxBlock,
		MinTime: r[9], MaxTime: r[10],
		OldestModified: r[11], NewestModified: r[12],
		LevelCounts: parseLevelPairs(r[13]),
	}, nil
}

// parseLevelPairs turns "0:2,1:5,2:1" (the shape produced by the SQL's
// groupArray/arrayMap) into {0:2, 1:5, 2:1}. Malformed pairs are skipped
// so the detail pane just shows fewer buckets rather than erroring.
func parseLevelPairs(s string) map[int]int {
	out := map[int]int{}
	if s == "" {
		return out
	}
	// Split on comma, then colon.
	for _, pair := range splitComma(s) {
		colon := -1
		for i := range pair {
			if pair[i] == ':' {
				colon = i
				break
			}
		}
		if colon < 0 {
			continue
		}
		lvl, err1 := strconv.Atoi(pair[:colon])
		cnt, err2 := strconv.Atoi(pair[colon+1:])
		if err1 != nil || err2 != nil {
			continue
		}
		out[lvl] += cnt
	}
	return out
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := range s {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

const sqlPartDetail = `
SELECT
    part_type,
    level,
    toUInt8(active),
    rows,
    bytes_on_disk,
    data_compressed_bytes,
    data_uncompressed_bytes,
    marks,
    primary_key_bytes_in_memory,
    toInt64(min_block_number),
    toInt64(max_block_number),
    toString(min_time),
    toString(max_time),
    toString(modification_time),
    disk_name,
    path,
    refcount
FROM system.parts
WHERE database = {db:String} AND table = {t:String} AND name = {n:String}
LIMIT 1
` + suppressLogging

func FetchPartDetail(ctx context.Context, q ParamQuerier, db, table, partName string) (PartDetail, error) {
	res, err := q.QueryAllWithParams(ctx, sqlPartDetail,
		conn.StringParam("db", db),
		conn.StringParam("t", table),
		conn.StringParam("n", partName))
	if err != nil {
		return PartDetail{}, fmt.Errorf("part detail: %w", err)
	}
	if len(res.Rows) == 0 {
		return PartDetail{Database: db, Table: table, Name: partName}, nil
	}
	r := res.Rows[0]
	if len(r) < 17 {
		return PartDetail{}, fmt.Errorf("part detail: expected 17 cols, got %d", len(r))
	}
	level, _ := strconv.Atoi(r[1])
	activeByte, _ := strconv.Atoi(r[2])
	rows, _ := parseUint64Lenient(r[3])
	bytes, _ := parseUint64Lenient(r[4])
	compressed, _ := parseUint64Lenient(r[5])
	uncompressed, _ := parseUint64Lenient(r[6])
	marks, _ := parseUint64Lenient(r[7])
	pkMem, _ := parseUint64Lenient(r[8])
	minBlock, _ := strconv.ParseInt(r[9], 10, 64)
	maxBlock, _ := strconv.ParseInt(r[10], 10, 64)
	refcount, _ := strconv.Atoi(r[16])

	return PartDetail{
		Database: db, Table: table, Name: partName,
		PartType: r[0], Level: level, Active: activeByte != 0,
		Rows: rows, Bytes: bytes,
		Compressed: compressed, Uncompressed: uncompressed,
		Marks: marks, PrimaryKeyMem: pkMem,
		MinBlockNumber: minBlock, MaxBlockNumber: maxBlock,
		MinTime: r[11], MaxTime: r[12],
		ModificationTime: r[13],
		DiskName:         r[14], Path: r[15],
		Refcount: refcount,
	}, nil
}
