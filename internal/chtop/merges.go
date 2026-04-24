package chtop

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// MergeRow is one row from system.merges.
type MergeRow struct {
	Database                 string
	Table                    string
	Elapsed                  float64
	Progress                 float64
	NumParts                 int
	SourcePartNames          string
	ResultPartName           string
	TotalSizeBytesCompressed uint64
	MergedRows               uint64
	IsMutation               bool
}

// MutationRow is one row from system.mutations (active only).
type MutationRow struct {
	Database         string
	Table            string
	MutationID       string
	Command          string
	CreateTime       string // server-formatted timestamp; unparsed
	PartsToDo        int
	IsDone           bool
	LatestFailedPart string
	LatestFailReason string
}

const sqlMerges = `
SELECT
    database, table, elapsed, progress, num_parts,
    toString(source_part_names)                   AS source_part_names,
    result_part_name,
    total_size_bytes_compressed, rows_read,
    toUInt8(is_mutation)                          AS is_mutation
FROM system.merges
ORDER BY elapsed DESC
` + suppressLogging

const sqlMutations = `
SELECT
    database, table, mutation_id, command,
    toString(create_time)                         AS create_time,
    parts_to_do, toUInt8(is_done) AS is_done,
    latest_failed_part, latest_fail_reason
FROM system.mutations
WHERE is_done = 0
ORDER BY create_time DESC
` + suppressLogging

// FetchMerges runs the two merges-tab SQLs and returns both slices. When the
// first query succeeds but the second fails, the successful rows are still
// returned alongside the error so the UI can show something useful.
func FetchMerges(ctx context.Context, q Querier) ([]MergeRow, []MutationRow, error) {
	mRes, err := q.QueryAll(ctx, sqlMerges)
	if err != nil {
		return nil, nil, fmt.Errorf("merges: %w", err)
	}
	merges, err := parseMerges(mRes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse merges: %w", err)
	}
	muRes, err := q.QueryAll(ctx, sqlMutations)
	if err != nil {
		return merges, nil, fmt.Errorf("mutations: %w", err)
	}
	muts, err := parseMutations(muRes)
	if err != nil {
		return merges, nil, fmt.Errorf("parse mutations: %w", err)
	}
	return merges, muts, nil
}

func parseMerges(qr *conn.QueryResult) ([]MergeRow, error) {
	out := make([]MergeRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 10 {
			return nil, fmt.Errorf("row %d: expected 10 cols, got %d", i, len(row))
		}
		elapsed, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("row %d elapsed: %w", i, err)
		}
		prog, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, fmt.Errorf("row %d progress: %w", i, err)
		}
		nParts, err := strconv.Atoi(row[4])
		if err != nil {
			return nil, fmt.Errorf("row %d num_parts: %w", i, err)
		}
		totSz, err := parseUint64Lenient(row[7])
		if err != nil {
			return nil, fmt.Errorf("row %d total_size: %w", i, err)
		}
		mRows, err := parseUint64Lenient(row[8])
		if err != nil {
			return nil, fmt.Errorf("row %d merged_rows: %w", i, err)
		}
		isMut := row[9] == "1"
		out = append(out, MergeRow{
			Database: row[0], Table: row[1],
			Elapsed: elapsed, Progress: prog, NumParts: nParts,
			SourcePartNames:          row[5],
			ResultPartName:           row[6],
			TotalSizeBytesCompressed: totSz,
			MergedRows:               mRows,
			IsMutation:               isMut,
		})
	}
	return out, nil
}

func parseMutations(qr *conn.QueryResult) ([]MutationRow, error) {
	out := make([]MutationRow, 0, len(qr.Rows))
	for i, row := range qr.Rows {
		if len(row) < 9 {
			return nil, fmt.Errorf("row %d: expected 9 cols, got %d", i, len(row))
		}
		pto, err := strconv.Atoi(row[5])
		if err != nil {
			return nil, fmt.Errorf("row %d parts_to_do: %w", i, err)
		}
		out = append(out, MutationRow{
			Database: row[0], Table: row[1],
			MutationID: row[2], Command: row[3], CreateTime: row[4],
			PartsToDo: pto, IsDone: row[6] == "1",
			LatestFailedPart: row[7], LatestFailReason: row[8],
		})
	}
	return out, nil
}
