package metacmd

import (
	"context"
	"fmt"
	"strings"
)

// handleRefresh refreshes the schema cache.
func handleRefresh(ctx context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	result := hctx.Cache.Refresh(ctx)
	output := result.Summary()
	if result.HasErrors() {
		var outputSb13 strings.Builder
		for _, e := range result.Errors {
			outputSb13.WriteString("\n  ⚠ " + e)
		}
		output += outputSb13.String()
	}
	return &Result{Output: output}, nil
}

// handleSettings prints the current connection and UI settings.
func handleSettings(_ context.Context, hctx *HandlerContext, _ []string) (*Result, error) {
	cfg := hctx.Config.Default
	out := fmt.Sprintf(
		"Host:     %s\nPort:     %d\nUser:     %s\nDatabase: %s\nKeymap:   %s\nTheme:    %s\nPager:    %s\nTiming:   %v\nVertical: %v\n",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Database,
		cfg.Keymap,
		cfg.Theme,
		cfg.Pager,
		timingEnabled,
		verticalEnabled,
	)
	return &Result{Output: out}, nil
}

// handleHelp returns the full list of meta-commands and their descriptions.
func handleHelp(_ context.Context, _ *HandlerContext, _ []string) (*Result, error) {
	help := `Meta-commands:
  \l                  List databases
  \dt  [db]           List tables in current/given database
  \dt+ [db]           List tables with row count and size
  \d   <table>        Describe table columns
  \d+  <table>        Describe table columns (extended)
  \di  [db]           List dictionaries
  \dm  [db]           List materialized views
  \dv  [db]           List views (including materialized)
  \dp                 List running processes

  \use <db>           Switch database
  \c   <profile>      Connect using a named profile

  \timing             Toggle query timing
  \e                  Open editor; run result as query
  \pager [name]       Toggle/set pager
  \x                  Toggle expanded (vertical) display
  \explain [ast|plan] [query]
                      Run EXPLAIN on query (or last query)
  \fmt / \format [query]
                      Format SQL query and insert into input

  \f   [name]         List or execute a snippet
  \fs  <name> <query> Save a snippet
  \fd  <name>         Delete a snippet
  \save <name>        Save last query as a named snippet
  \load <name>        Load snippet into input (for editing)
  \saved              List all saved snippets

  \theme [name]       List themes or switch to a named theme

  \h   [term]         Show recent history (or search)
  \hb  <tag> [query]  Bookmark last (or given) query with tag
  \hl  [tag]          List bookmarks (optionally filtered by tag)

  \refresh            Refresh schema cache
  \settings           Show current settings
  \? / \help          Show this help
  \q                  Quit
`
	return &Result{Output: help}, nil
}

// handleQuit signals the REPL to exit.
func handleQuit(_ context.Context, _ *HandlerContext, _ []string) (*Result, error) {
	return &Result{Output: "quit"}, nil
}
