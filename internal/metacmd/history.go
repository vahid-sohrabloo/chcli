package metacmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// handleHistory shows recent history or searches for a term.
func handleHistory(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	var entries []histEntry
	if len(args) == 0 {
		raw, err := hctx.History.Recent(20)
		if err != nil {
			return nil, err
		}
		for _, e := range raw {
			entries = append(entries, histEntry{
				ts:    e.ExecutedAt,
				durMs: e.DurationMs,
				query: e.Query,
			})
		}
	} else {
		raw, err := hctx.History.Search(args[0], 20)
		if err != nil {
			return nil, err
		}
		for _, e := range raw {
			entries = append(entries, histEntry{
				ts:    e.ExecutedAt,
				durMs: e.DurationMs,
				query: e.Query,
			})
		}
	}

	if len(entries) == 0 {
		return &Result{Output: "No history entries found."}, nil
	}

	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "[%s] (%dms) %s\n",
			e.ts.Format(time.RFC3339),
			e.durMs,
			truncate(e.query, 80))
	}
	return &Result{Output: sb.String()}, nil
}

// histEntry is a small helper to hold the fields we render for a history line.
type histEntry struct {
	ts    time.Time
	durMs int64
	query string
}

// handleHistoryBookmark bookmarks the last query or a supplied query.
// If 2 args: args[0]=tag, args[1]=query
// If 1 arg:  args[0]=tag, bookmark last query from Recent(1)
func handleHistoryBookmark(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\hb requires a tag")
	}

	var tag, query string

	if len(args) >= 2 {
		tag = args[0]
		query = args[1]
	} else {
		tag = args[0]
		recent, err := hctx.History.Recent(1)
		if err != nil {
			return nil, err
		}
		if len(recent) == 0 {
			return nil, errors.New("no history to bookmark")
		}
		query = recent[0].Query
	}

	if err := hctx.History.AddBookmark(query, tag, ""); err != nil {
		return nil, err
	}
	return &Result{Output: fmt.Sprintf("Bookmarked under tag %q.", tag)}, nil
}

// handleHistoryListBookmarks lists bookmarks, optionally filtered by tag.
func handleHistoryListBookmarks(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	var tag string
	if len(args) > 0 {
		tag = args[0]
	}

	bookmarks, err := hctx.History.ListBookmarks(tag)
	if err != nil {
		return nil, err
	}
	if len(bookmarks) == 0 {
		return &Result{Output: "No bookmarks found."}, nil
	}

	var sb strings.Builder
	for _, b := range bookmarks {
		fmt.Fprintf(&sb, "[%s] tag=%s  %s\n",
			b.CreatedAt.Format(time.RFC3339),
			b.Tag,
			truncate(b.Query, 80))
	}
	return &Result{Output: sb.String()}, nil
}
