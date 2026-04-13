package metacmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// timingEnabled tracks whether query timing output is active.
var timingEnabled bool

// verticalEnabled tracks whether expanded (vertical) display is active.
var verticalEnabled bool

// IsVerticalEnabled returns whether vertical (expanded) display is currently enabled.
func IsVerticalEnabled() bool { return verticalEnabled }

// handleTiming toggles query timing on/off.
func handleTiming(_ context.Context, _ *HandlerContext, _ []string) (*Result, error) {
	timingEnabled = !timingEnabled
	if timingEnabled {
		return &Result{Output: "Timing is on."}, nil
	}
	return &Result{Output: "Timing is off."}, nil
}

// handleEditor opens the system editor, waits for it to close, then returns the
// file contents as a query to execute.
func handleEditor(_ context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	editor := hctx.Config.Default.Editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	// Create a temporary file for the user to edit.
	tmpFile, err := os.CreateTemp("", "chcli-*.sql")
	if err != nil {
		return nil, fmt.Errorf("editor: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Launch the editor and wait.
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("editor: read temp file: %w", err)
	}

	query := strings.TrimSpace(string(data))
	return &Result{Output: query, IsQuery: true}, nil
}

// handlePager toggles or sets the pager in the config.
func handlePager(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		// Toggle between builtin and off.
		if hctx.Config.Default.Pager == "builtin" {
			hctx.Config.Default.Pager = "off"
			return &Result{Output: "Pager is off."}, nil
		}
		hctx.Config.Default.Pager = "builtin"
		return &Result{Output: "Pager is on (builtin)."}, nil
	}
	hctx.Config.Default.Pager = args[0]
	return &Result{Output: fmt.Sprintf("Pager set to %q.", args[0])}, nil
}

// handleVerticalToggle toggles expanded (vertical) display on/off.
func handleVerticalToggle(_ context.Context, _ *HandlerContext, _ []string) (*Result, error) {
	verticalEnabled = !verticalEnabled
	if verticalEnabled {
		return &Result{Output: "Expanded display is on."}, nil
	}
	return &Result{Output: "Expanded display is off."}, nil
}

// handleExplain prepends EXPLAIN (or EXPLAIN AST / EXPLAIN PLAN) to the given
// query and returns it for execution.  If no query is supplied it falls back to
// the last executed query stored in HandlerContext.LastResult.
//
// Usage:
//
//	\explain [ast|plan] [<query>]
func handleExplain(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	prefix := "EXPLAIN"
	rest := args

	// Check for optional sub-command modifier.
	if len(rest) > 0 {
		switch strings.ToLower(rest[0]) {
		case "ast":
			prefix = "EXPLAIN AST"
			rest = rest[1:]
		case "plan":
			prefix = "EXPLAIN PLAN"
			rest = rest[1:]
		}
	}

	query := strings.TrimSpace(strings.Join(rest, " "))
	if query == "" {
		// Fall back to the last stored query text.
		if hctx.LastQuery == "" {
			return nil, errors.New("usage: \\explain [ast|plan] [query]  (or run a query first)")
		}
		query = hctx.LastQuery
	}

	return &Result{Output: prefix + " " + query, IsQuery: true}, nil
}
