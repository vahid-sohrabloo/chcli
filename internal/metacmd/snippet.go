package metacmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// truncate shortens s to at most maxLen characters, appending "..." if trimmed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// handleSnippet retrieves a snippet by name (IsQuery=true) or lists all snippets.
func handleSnippet(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		// List all snippets.
		if len(hctx.Config.Snippets) == 0 {
			return &Result{Output: "No snippets saved."}, nil
		}
		var sb strings.Builder
		sb.WriteString("Saved snippets:\n")
		for name, query := range hctx.Config.Snippets {
			fmt.Fprintf(&sb, "  %-20s  %s\n", name, truncate(query, 60))
		}
		return &Result{Output: sb.String()}, nil
	}

	name := args[0]
	query, ok := hctx.Config.Snippets[name]
	if !ok {
		return nil, fmt.Errorf("snippet %q not found", name)
	}
	return &Result{Output: query, IsQuery: true}, nil
}

// handleSnippetSave saves a new snippet (requires name + query).
func handleSnippetSave(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) < 2 {
		return nil, errors.New("\\fs requires a name and a query")
	}
	name, query := args[0], args[1]
	if err := hctx.Config.SaveSnippet(name, query); err != nil {
		return nil, err
	}
	return &Result{Output: fmt.Sprintf("Snippet %q saved.", name)}, nil
}

// handleSnippetDelete removes a snippet by name.
func handleSnippetDelete(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\fd requires a snippet name")
	}
	name := args[0]
	hctx.Config.DeleteSnippet(name)
	return &Result{Output: fmt.Sprintf("Snippet %q deleted.", name)}, nil
}

// handleSave saves the last executed query as a named snippet.
// It is an alias for \fs but uses LastQuery instead of requiring the SQL text
// to be typed inline.
//
// Usage: \save <name>
func handleSave(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\save requires a name")
	}
	name := args[0]
	if hctx.LastQuery == "" {
		return nil, errors.New("\\save: no query has been run yet")
	}
	if err := hctx.Config.SaveSnippet(name, hctx.LastQuery); err != nil {
		return nil, err
	}
	return &Result{Output: fmt.Sprintf("Query saved as snippet %q.", name)}, nil
}

// handleLoad retrieves a named snippet and inserts it into the input box.
// Unlike \f (which executes immediately), \load places the text back in the
// editor so the user can review or modify it before running.
//
// Usage: \load <name>
func handleLoad(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\load requires a snippet name")
	}
	name := args[0]
	query, ok := hctx.Config.Snippets[name]
	if !ok {
		return nil, fmt.Errorf("snippet %q not found", name)
	}
	return &Result{Output: query, InsertToInput: true}, nil
}

// handleSaved lists all saved snippets — alias for \f with no arguments.
func handleSaved(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	return handleSnippet(ctx, hctx, nil)
}
