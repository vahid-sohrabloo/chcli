package metacmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/clipboard"
)

// handleClip copies the last query result to the system clipboard in TSV format.
func handleClip(_ context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	if hctx.LastResult == nil {
		return nil, errors.New("no result to copy")
	}

	var sb strings.Builder
	// Header row.
	for i, col := range hctx.LastResult.Columns {
		if i > 0 {
			sb.WriteByte('\t')
		}
		sb.WriteString(col.Name)
	}
	sb.WriteByte('\n')
	// Data rows.
	for _, row := range hctx.LastResult.Rows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteByte('\n')
	}

	return copyToClipboard(sb.String())
}

// copyToClipboard writes text to the system clipboard using the first available tool.
func copyToClipboard(text string) (*Result, error) {
	if err := clipboard.Copy(text); err != nil {
		return nil, err
	}
	return &Result{Output: fmt.Sprintf("Copied %d bytes to clipboard.", len(text))}, nil
}
