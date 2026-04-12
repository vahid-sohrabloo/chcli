package metacmd

import (
	"context"
	"errors"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/format"
)

// handleFormat reformats the last query (or provided SQL text) using the
// built-in SQL formatter and inserts the result back into the input box.
//
// Usage:
//
//	\fmt           — format the last executed query
//	\fmt <query>   — format the supplied query text
func handleFormat(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	var sql string
	switch {
	case len(args) > 0:
		sql = joinArgs(args)
	case hctx.LastQuery != "":
		sql = hctx.LastQuery
	default:
		return nil, errors.New("usage: \\fmt [query]  (or run a query first)")
	}

	formatted := format.FormatSQL(sql)
	return &Result{Output: formatted, InsertToInput: true}, nil
}

// joinArgs joins a slice of strings with single spaces, reconstructing the
// original whitespace-separated token sequence.
func joinArgs(args []string) string {
	return strings.Join(args, " ")
}
