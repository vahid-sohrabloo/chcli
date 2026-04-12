// Package metacmd implements the meta-command router and handlers for chcli.
// Meta-commands start with a backslash (e.g. \dt, \l, \d users).
package metacmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/config"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chcli/internal/history"
	"github.com/vahid-sohrabloo/chcli/internal/schema"
)

// Result is the output of a meta-command handler.
type Result struct {
	Output        string
	IsQuery       bool   // if true, Output contains a query to execute
	InsertToInput bool   // if true, insert Output into the input box instead of executing or printing
	SetTheme      string // if non-empty, switch the highlighter to this theme name
}

// HandlerContext holds the shared context passed to all handlers.
type HandlerContext struct {
	Conn       *conn.Conn
	Cache      *schema.Cache
	History    *history.Store
	Config     *config.Config
	CurrentDB  string
	LastResult *conn.QueryResult
	LastQuery  string // SQL text of the most-recently executed query
}

// Handler is a function that processes a meta-command.
type Handler func(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error)

// Router dispatches meta-commands to the appropriate handler.
type Router struct {
	handlers map[string]Handler
	hctx     *HandlerContext
}

// NewRouter creates a new Router, initialises the HandlerContext, and registers
// all built-in meta-command handlers.
func NewRouter(c *conn.Conn, cache *schema.Cache, hist *history.Store, cfg *config.Config) *Router {
	r := &Router{
		handlers: make(map[string]Handler),
		hctx: &HandlerContext{
			Conn:      c,
			Cache:     cache,
			History:   hist,
			Config:    cfg,
			CurrentDB: cfg.Default.Database,
		},
	}
	r.registerAll()
	return r
}

// SetCurrentDB updates the currently selected database stored in the context.
func (r *Router) SetCurrentDB(db string) {
	r.hctx.CurrentDB = db
}

// SetLastResult stores the most recent query result for use by export commands.
func (r *Router) SetLastResult(res *conn.QueryResult) {
	r.hctx.LastResult = res
}

// SetLastQuery stores the SQL text of the most recently executed query so that
// \explain (and similar commands) can reference it without arguments.
func (r *Router) SetLastQuery(query string) {
	r.hctx.LastQuery = query
}

// registerAll registers every known meta-command to its handler function.
func (r *Router) registerAll() {
	// Schema inspection
	r.handlers["l"] = handleListDatabases
	r.handlers["dt"] = handleListTables
	r.handlers["dt+"] = handleListTablesExtended
	r.handlers["d"] = handleDescribeTable
	r.handlers["d+"] = handleDescribeTableExtended
	r.handlers["di"] = handleListDictionaries
	r.handlers["dm"] = handleListMaterializedViews
	r.handlers["dv"] = handleListViews
	r.handlers["dp"] = handleListProcesses

	// Navigation
	r.handlers["use"] = handleUse
	r.handlers["c"] = handleConnect

	// Export
	r.handlers["copy"] = handleCopy
	r.handlers["clip"] = handleClip

	// Query tools
	r.handlers["timing"] = handleTiming
	r.handlers["e"] = handleEditor
	r.handlers["pager"] = handlePager
	r.handlers["x"] = handleVerticalToggle
	r.handlers["explain"] = handleExplain
	r.handlers["fmt"] = handleFormat
	r.handlers["format"] = handleFormat

	// Themes
	r.handlers["theme"] = handleTheme

	// Snippets
	r.handlers["f"] = handleSnippet
	r.handlers["fs"] = handleSnippetSave
	r.handlers["fd"] = handleSnippetDelete
	r.handlers["save"] = handleSave
	r.handlers["load"] = handleLoad
	r.handlers["saved"] = handleSaved

	// History
	r.handlers["h"] = handleHistory
	r.handlers["hb"] = handleHistoryBookmark
	r.handlers["hl"] = handleHistoryListBookmarks

	// System
	r.handlers["refresh"] = handleRefresh
	r.handlers["settings"] = handleSettings
	r.handlers["?"] = handleHelp
	r.handlers["help"] = handleHelp
	r.handlers["q"] = handleQuit
}

// IsMetaCommand reports whether the (trimmed) input is a meta-command, i.e.
// it starts with a backslash.
func IsMetaCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), `\`)
}

// Execute parses the meta-command in input, looks up the handler, and calls it.
func (r *Router) Execute(ctx context.Context, input string) (*Result, error) {
	cmd, args := parseMetaCommand(input)
	h, ok := r.handlers[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown meta-command: \\%s", cmd)
	}
	return h(ctx, r.hctx, args)
}

// parseMetaCommand strips the leading backslash and splits the input into the
// command name and its arguments.  For \fs and \hb the remainder is split only
// on the first space so that the trailing portion is treated as a single
// argument (name + query / tag + query).
func parseMetaCommand(input string) (string, []string) {
	trimmed := strings.TrimSpace(input)
	// Strip leading backslash.
	trimmed = strings.TrimPrefix(trimmed, `\`)

	// Split command from the rest on the first space.
	before, after, ok := strings.Cut(trimmed, " ")
	if !ok {
		// No arguments.
		return trimmed, nil
	}

	cmd := before
	rest := strings.TrimSpace(after)

	// For \fs and \hb, split rest on the first space to get (name, query) or
	// (tag, query).
	if cmd == "fs" || cmd == "hb" {
		before, after, ok := strings.Cut(rest, " ")
		if !ok {
			return cmd, []string{rest}
		}
		first := before
		second := strings.TrimSpace(after)
		return cmd, []string{first, second}
	}

	// All other commands: split on whitespace.
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return cmd, nil
	}
	return cmd, parts
}
