package metacmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// handleCopy exports the last query result to a file or stdout.
// Usage:
//
//	\copy csv /path/to/file.csv   — export as CSV to file
//	\copy json /path/to/file.json — export as JSON to file
//	\copy csv                     — print CSV to output
//	\copy json                    — print JSON to output
func handleCopy(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("usage: \\copy csv|json [/path/to/file]")
	}

	format := strings.ToLower(args[0])
	if format != "csv" && format != "json" {
		return nil, fmt.Errorf("unknown format %q: must be csv or json", args[0])
	}

	if hctx.LastResult == nil {
		return nil, errors.New("no query result to export")
	}

	// Determine output destination.
	if len(args) >= 2 {
		path := args[1]
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("copy: create file %q: %w", path, err)
		}
		defer f.Close()

		if err := writeResult(format, hctx, f); err != nil {
			return nil, err
		}
		return &Result{Output: fmt.Sprintf("Exported %d row(s) to %s.", len(hctx.LastResult.Rows), path)}, nil
	}

	// No path — write to a string builder and return as output.
	var sb strings.Builder
	if err := writeResult(format, hctx, &sb); err != nil {
		return nil, err
	}
	return &Result{Output: sb.String()}, nil
}

// writeResult writes the last result in the requested format to w.
func writeResult(format string, hctx *HandlerContext, w io.Writer) error {
	result := hctx.LastResult
	switch format {
	case "csv":
		cw := csv.NewWriter(w)
		// Header row.
		headers := make([]string, len(result.Columns))
		for i, col := range result.Columns {
			headers[i] = col.Name
		}
		if err := cw.Write(headers); err != nil {
			return fmt.Errorf("copy csv: write header: %w", err)
		}
		// Data rows.
		for _, row := range result.Rows {
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("copy csv: write row: %w", err)
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return fmt.Errorf("copy csv: flush: %w", err)
		}
	case "json":
		// Build array of objects.
		records := make([]map[string]string, len(result.Rows))
		for i, row := range result.Rows {
			obj := make(map[string]string, len(result.Columns))
			for j, col := range result.Columns {
				if j < len(row) {
					obj[col.Name] = row[j]
				}
			}
			records[i] = obj
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			return fmt.Errorf("copy json: encode: %w", err)
		}
	}
	return nil
}
