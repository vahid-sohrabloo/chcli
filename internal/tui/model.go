package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/spinner"
	"github.com/vahid-sohrabloo/chcli/internal/completer"
	"github.com/vahid-sohrabloo/chcli/internal/config"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chcli/internal/functions"
	"github.com/vahid-sohrabloo/chcli/internal/highlight"
	"github.com/vahid-sohrabloo/chcli/internal/history"
	"github.com/vahid-sohrabloo/chcli/internal/metacmd"
	"github.com/vahid-sohrabloo/chcli/internal/schema"
)

// queryResultMsg carries the result (or error) of an async SQL query.
type queryResultMsg struct {
	result   *conn.QueryResult
	err      error
	vertical bool
	query    string // the SQL text for echo
}

// queryTickMsg is sent every 500ms while a query is running to update elapsed time.
type queryTickMsg time.Time

// watchTickMsg is sent periodically to re-run the watch query.
type watchTickMsg time.Time

// metaCmdResultMsg carries the result (or error) of an async meta-command.
type metaCmdResultMsg struct {
	result *metacmd.Result
	err    error
}

// schemaCacheMsg is sent when the schema cache finishes loading.
type schemaCacheMsg struct {
	result *schema.RefreshResult
}

// Model is the root bubbletea model for the chcli TUI.
type Model struct {
	conn        *conn.Conn
	config      *config.Config
	cache       *schema.Cache
	history     *history.Store
	completer   *completer.Completer
	highlighter *highlight.Highlighter
	router      *metacmd.Router

	input      *InputModel
	completion *CompletionModel
	statusBar  *StatusBarModel
	search     *SearchModel

	program       *tea.Program // for Println (output above TUI)
	currentDB     string
	width, height int
	quitting      bool
	spinner       spinner.Model      // loading spinner
	running       bool               // query is executing
	loading       bool               // schema cache is loading
	cancelQuery   context.CancelFunc // cancel running query
	activeQueryID string             // query ID for KILL QUERY
	progress      *conn.Progress     // latest progress update
	lastMetrics   *conn.Progress     // metrics from last completed query
	lastResult    *conn.QueryResult  // last query result for table viewer
	lastQuery     string
	queryStart    time.Time // when current query started

	// Table viewer mode.
	tableViewer *tableViewerModel

	historyEntries []string // cached history queries
	historyIdx     int      // -1 = not browsing, 0..N = browsing
	historySaved   string   // what user was typing before browsing history

	isWarp        bool // running in Warp terminal
	watching      bool
	watchQuery    string
	watchInterval time.Duration
	watchCount    int
}

// NewModel creates the root TUI model, wiring together all sub-models.
func NewModel(
	c *conn.Conn,
	cfg *config.Config,
	cache *schema.Cache,
	hist *history.Store,
	resolved config.ConnectionConfig,
) *Model {
	km := KeymapFromString(resolved.Keymap)
	hl := highlight.NewHighlighter(resolved.Theme)
	// Use embedded builtins for instant completions before schema loads.
	comp := completer.NewWithBuiltins(c.ServerVersion(), cfg.Snippets)
	router := metacmd.NewRouter(c, cache, hist, cfg)

	prompt := fmt.Sprintf("%s@%s:%d/%s> ", resolved.User, resolved.Host, resolved.Port, resolved.Database)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))

	m := &Model{
		spinner:     sp,
		conn:        c,
		config:      cfg,
		cache:       cache,
		history:     hist,
		completer:   comp,
		highlighter: hl,
		router:      router,

		input:      NewInputModel(prompt, hl),
		completion: NewCompletionModel(),
		statusBar:  NewStatusBarModel(resolved.Host, resolved.Port, resolved.User, resolved.Database, km),
		search:     NewSearchModel(),

		currentDB:  resolved.Database,
		historyIdx: -1,
	}
	m.statusBar.SetServerVersion(c.ServerVersion())
	// Detect Warp terminal for layout adjustments.
	m.isWarp = os.Getenv("TERM_PROGRAM") == "WarpTerminal"
	return m
}

// SetProgram stores the tea.Program reference for Println output.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init returns the initial command (focus the textarea and start schema loading).
func (m *Model) Init() tea.Cmd {
	m.loading = true
	m.statusBar.SetLoading(true)
	cache := m.cache
	return tea.Batch(m.input.Focus(), m.spinner.Tick, func() tea.Msg {
		result := cache.Refresh(context.Background())
		return schemaCacheMsg{result: result}
	})
}

// Update is the main message dispatch loop.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Table viewer mode: full-screen interactive table.
	if m.tableViewer != nil {
		if kp, ok := msg.(tea.KeyPressMsg); ok {
			closed, cmd := m.tableViewer.Update(kp)
			if closed {
				m.tableViewer = nil
				return m, m.input.Focus()
			}
			return m, cmd
		}
		// Forward mouse/resize to table viewer.
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = wsm.Width
			m.height = wsm.Height
			m.tableViewer.width = wsm.Width
			m.tableViewer.height = wsm.Height
			m.tableViewer.rebuildTable()
		}
		if wm, ok := msg.(tea.MouseWheelMsg); ok {
			_, cmd := m.tableViewer.Update(wm)
			return m, cmd
		}
		return m, nil
	}

	// When the search overlay is active, forward ONLY key presses to it.
	if m.search.Active() {
		if _, isKey := msg.(tea.KeyPressMsg); isKey {
			selected, accepted := m.search.Update(msg)
			if !m.search.Active() && accepted && selected != "" {
				m.input.InsertText(selected)
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.statusBar.SetWidth(m.width)
		m.input.SetWidth(m.width)
		m.completion.SetMaxWidth(m.width)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case queryResultMsg:
		m.lastMetrics = m.progress
		m.running = false
		m.progress = nil
		m.cancelQuery = nil
		return m.handleQueryResult(msg)

	case metaCmdResultMsg:
		return m.handleMetaCmdResult(msg)

	case schemaCacheMsg:
		m.loading = false
		m.statusBar.SetLoading(false)
		m.completer = completer.New(m.cache, m.config.Snippets)
		// Show errors if any, silently succeed otherwise.
		var printCmd tea.Cmd
		if msg.result.HasErrors() {
			var sb strings.Builder
			sb.WriteString(msg.result.Summary() + "\n")
			for _, e := range msg.result.Errors {
				sb.WriteString("  ⚠ " + e + "\n")
			}
			printCmd = m.printAbove(sb.String())
		}
		return m, tea.Batch(printCmd, m.input.Focus())

	case spinner.TickMsg:
		if m.loading || m.running {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case queryTickMsg:
		if m.running {
			return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return queryTickMsg(t)
			})
		}
		return m, nil

	case watchTickMsg:
		if !m.watching {
			return m, nil
		}
		return m, m.runWatchQuery()

	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	}

	// Default: forward to input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Submitted() {
		m.input.ResetSubmitted()
		return m, m.executeInput()
	}
	return m, cmd
}

// handleKey processes key presses with pgcli-like completion interaction:
//   - Completions auto-show while typing
//   - Up/Down navigate the popup
//   - Tab accepts the selected completion
//   - Escape dismisses the popup
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	// Tab: accept completion if visible, otherwise trigger completions.
	case msg.Code == tea.KeyTab && msg.Mod == 0:
		if m.completion.Visible() {
			m.acceptCompletion()
			m.completion.Hide()
		} else {
			m.updateCompletions()
			m.completion.Show()
		}
		return m, nil

	// Down arrow: completion → history.
	case msg.Code == tea.KeyDown:
		if m.completion.Visible() {
			m.completion.Next()
		} else if !strings.Contains(m.input.Value(), "\n") {
			m.browseHistory(1)
		}
		return m, nil

	// Up arrow: completion → history.
	case msg.Code == tea.KeyUp:
		if m.completion.Visible() {
			m.completion.Prev()
		} else if !strings.Contains(m.input.Value(), "\n") {
			m.browseHistory(-1)
		}
		return m, nil

	// Escape: hide completion.
	case msg.Code == tea.KeyEscape:
		if m.completion.Visible() {
			m.completion.Hide()
			return m, nil
		}
		return m, nil

	// Ctrl+D on empty input: quit.
	case msg.Code == 'd' && msg.Mod == tea.ModCtrl:
		if strings.TrimSpace(m.input.Value()) == "" {
			m.quitting = true
			return m, tea.Quit
		}

	// F2: open interactive table viewer for last result.
	case msg.Code == tea.KeyF2 && m.lastResult != nil:
		tv := newTableViewer(m.lastResult, m.lastQuery, m.width, m.height)
		tv.isWarp = m.isWarp
		m.tableViewer = tv
		return m, nil

	// Ctrl+C: stop watch, cancel running query, or clear input.
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		if m.watching {
			m.watching = false
			m.watchQuery = ""
			return m, tea.Batch(m.printAbove("Watch stopped."), m.input.Focus())
		}
		if m.running && m.cancelQuery != nil {
			// Send KILL QUERY to ClickHouse server on a separate connection.
			if m.activeQueryID != "" {
				go m.conn.KillQuery(m.activeQueryID)
			}
			m.cancelQuery()
			m.cancelQuery = nil
			m.activeQueryID = ""
			m.running = false
			printCmd := m.printAbove(FormatError(errors.New("query canceled")))
			m.tryReconnect()
			return m, tea.Batch(printCmd, m.input.Focus())
		}
		m.input.Clear()
		m.completion.Hide()
		return m, nil

	// Ctrl+R: interactive fuzzy history search.
	case msg.Code == 'r' && msg.Mod == tea.ModCtrl:
		queries, _ := m.history.Queries(200)
		m.search.Activate(queries)
		return m, nil

	// Ctrl+Right → word forward (textarea uses Alt+F).
	case msg.Code == tea.KeyRight && msg.Mod == tea.ModCtrl:
		m.completion.Hide()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModAlt})
		return m, cmd

	// Ctrl+Left → word backward (textarea uses Alt+B).
	case msg.Code == tea.KeyLeft && msg.Mod == tea.ModCtrl:
		m.completion.Hide()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModAlt})
		return m, cmd
	}

	// Forward to input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Submitted() {
		m.input.ResetSubmitted()
		m.completion.Hide()
		m.resetHistoryBrowsing()
		return m, m.executeInput()
	}

	// Only auto-show completions on typing keys (characters, backspace),
	// not on navigation keys (arrows, Home, End, Ctrl+arrows).
	if isTypingKey(msg) {
		m.updateCompletions()
		if m.completion.Len() > 0 {
			m.completion.Show()
		} else {
			m.completion.Hide()
		}
	}

	return m, cmd
}

// isTypingKey returns true for keys that modify text content (characters,
// backspace, delete), false for navigation keys (arrows, Home, End, etc.).
func isTypingKey(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight,
		tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown:
		return false
	}
	// Ctrl+arrow and other Ctrl combos used for navigation.
	if msg.Mod == tea.ModCtrl {
		switch msg.Code {
		case 'a', 'e', 'f', 'b', 'p', 'n': // emacs nav keys
			return false
		case 'u', 'k': // kill commands — modify text but completions should hide
			return false
		}
	}
	if msg.Mod == (tea.ModCtrl | tea.ModShift) {
		return false
	}
	if msg.Mod == tea.ModAlt {
		switch msg.Code {
		case 'f', 'b': // alt+f/b word movement
			return false
		case tea.KeyEnter: // alt+enter inserts newline, don't re-trigger completions
			return false
		}
	}
	return true
}

// executeInput processes the submitted input: meta-commands go to the router,
// SQL queries are executed asynchronously.
func (m *Model) executeInput() tea.Cmd {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		return nil
	}

	// Block new queries while watching — only \watch and Ctrl+C are allowed.
	if m.watching && !strings.HasPrefix(raw, `\watch`) {
		m.input.Clear()
		return tea.Batch(m.input.Focus(), m.printAbove(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Render(
				"⚠ Watch mode active — press Ctrl+C to stop before running new queries")))
	}

	// Save to history (best effort).
	_ = m.history.Add(raw, 0, m.currentDB, "")

	m.input.Clear()
	focusCmd := m.input.Focus()

	// Handle \doc <function> — show function documentation.
	if strings.HasPrefix(raw, `\doc`) {
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) == 2 {
			f := functions.Lookup(strings.TrimSpace(parts[1]))
			if f != nil {
				return tea.Batch(focusCmd, m.printAbove(functions.FormatFunctionDoc(f)))
			}
			return tea.Batch(focusCmd, m.printAbove(FormatError(fmt.Errorf("function %q not found", parts[1]))))
		}
		return tea.Batch(focusCmd, m.printAbove("Usage: \\doc <function_name>"))
	}

	// Handle \metrics locally (needs access to model state).
	if raw == `\metrics` {
		return tea.Batch(focusCmd, m.printAbove(m.formatLastMetrics()))
	}

	// Handle \watch N <query> — re-run query every N seconds.
	if strings.HasPrefix(raw, `\watch`) {
		parts := strings.SplitN(raw, " ", 3)
		if len(parts) < 3 {
			return tea.Batch(focusCmd, m.printAbove("Usage: \\watch <seconds> <query>"))
		}
		interval, err := strconv.Atoi(parts[1])
		if err != nil || interval <= 0 {
			return tea.Batch(focusCmd, m.printAbove(FormatError(fmt.Errorf("invalid interval %q: must be a positive integer", parts[1]))))
		}
		m.watching = true
		m.watchQuery = parts[2]
		m.watchInterval = time.Duration(interval) * time.Second
		m.watchCount = 0
		return tea.Batch(focusCmd, m.runWatchQuery())
	}

	// Handle \refresh locally — needs to rebuild completer after.
	if raw == `\refresh` {
		m.loading = true
		m.statusBar.SetLoading(true)
		cache := m.cache
		return tea.Batch(focusCmd, m.spinner.Tick, func() tea.Msg {
			result := cache.Refresh(context.Background())
			return schemaCacheMsg{result: result}
		})
	}

	if metacmd.IsMetaCommand(raw) {
		router := m.router
		return tea.Batch(focusCmd, func() tea.Msg {
			result, err := router.Execute(context.Background(), raw)
			return metaCmdResultMsg{result: result, err: err}
		})
	}

	// Determine display mode and strip terminators.
	vertical := metacmd.IsVerticalEnabled()
	query := raw
	if before, ok := strings.CutSuffix(query, `\G`); ok {
		query = before
		vertical = true
	}
	query = strings.TrimRight(query, ";")
	query = strings.TrimSpace(query)

	m.running = true
	m.progress = &conn.Progress{}
	m.queryStart = time.Now()
	m.activeQueryID = conn.GenerateQueryID()

	c := m.conn
	qid := m.activeQueryID
	prog := m.progress

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelQuery = cancel

	tickCmd := tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return queryTickMsg(t)
	})
	queryFunc := func() tea.Msg {
		result, err := c.QueryWithID(ctx, query, qid, prog)
		return queryResultMsg{result: result, err: err, vertical: vertical, query: query}
	}
	return tea.Batch(focusCmd, m.spinner.Tick, tickCmd, queryFunc)
}

// runWatchQuery executes the watch query immediately and schedules the next tick.
func (m *Model) runWatchQuery() tea.Cmd {
	m.watchCount++
	m.running = true
	m.queryStart = time.Now()

	c := m.conn
	query := strings.TrimRight(strings.TrimSpace(m.watchQuery), ";")
	vertical := metacmd.IsVerticalEnabled()
	interval := m.watchInterval
	count := m.watchCount

	execCmd := func() tea.Msg {
		result, err := c.Query(context.Background(), query)
		return queryResultMsg{result: result, err: err, vertical: vertical, query: fmt.Sprintf("[watch #%d] %s", count, query)}
	}
	tickCmd := tea.Tick(interval, func(t time.Time) tea.Msg {
		return watchTickMsg(t)
	})
	return tea.Batch(execCmd, tickCmd)
}

// printAbove prints text above the TUI via tea.Println (real terminal scrollback).
func (m *Model) printAbove(text string) tea.Cmd { //nolint:unparam // nil return is intentional; used in tea.Batch
	p := m.program
	if p == nil {
		return nil
	}
	go p.Println(text)
	return nil
}

// handleQueryResult processes an async query result.
func (m *Model) handleQueryResult(msg queryResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.tryReconnect()
		return m, tea.Batch(m.printAbove(FormatError(msg.err)), m.input.Focus())
	}

	output := FormatQueryResult(msg.result, msg.query, msg.vertical, m.width, m.progress)
	m.lastMetrics = m.progress
	m.lastResult = msg.result
	m.lastQuery = msg.query
	m.router.SetLastResult(msg.result)
	m.router.SetLastQuery(msg.query)

	return m, tea.Batch(m.printAbove(output), m.input.Focus())
}

// handleMetaCmdResult processes an async meta-command result.
func (m *Model) handleMetaCmdResult(msg metaCmdResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Batch(m.printAbove(FormatError(msg.err)), m.input.Focus())
	}

	// Quit command.
	if msg.result.Output == "quit" {
		m.quitting = true
		return m, tea.Quit
	}

	// If the meta-command wants to insert text into the input box, do so.
	if msg.result.InsertToInput {
		m.input.SetValue(msg.result.Output)
		return m, m.input.Focus()
	}

	// If the meta-command requests a theme change, update UI + syntax.
	if msg.result.SetTheme != "" {
		// Try UI theme first, fall back to syntax-only.
		if SetUITheme(msg.result.SetTheme) {
			m.highlighter.SetTheme(ActiveTheme.SyntaxTheme)
		} else {
			m.highlighter.SetTheme(msg.result.SetTheme)
		}
		m.input.SetHighlighter(m.highlighter)
	}

	// If the meta-command produced a query, execute it.
	if msg.result.IsQuery {
		query := strings.TrimSpace(msg.result.Output)
		c := m.conn
		vertical := metacmd.IsVerticalEnabled()
		return m, func() tea.Msg {
			result, err := c.Query(context.Background(), query)
			return queryResultMsg{result: result, err: err, vertical: vertical, query: query}
		}
	}

	// Print text output above TUI.
	return m, tea.Batch(m.printAbove(FormatText(msg.result.Output)), m.input.Focus())
}

// updateCompletions fetches completions from the completer and sets them on
// the completion model. Uses text up to cursor for clause detection, and full
// text for table extraction (so columns from FROM clause are available in SELECT).
func (m *Model) updateCompletions() {
	toCursor := m.input.ValueToCursor()
	fullText := m.input.Value()
	items := m.completer.CompleteAt(toCursor, fullText, m.currentDB)
	m.completion.SetItems(items)
}

// acceptCompletion replaces the word at the cursor with the selected completion.
// Functions get "()" appended with the cursor placed between the parens.
func (m *Model) acceptCompletion() {
	selected := m.completion.Selected()
	kind := m.completion.SelectedKind()
	if selected == "" {
		return
	}

	toCursor := m.input.ValueToCursor()
	prefix := completer.LastWord(toCursor)

	// For functions, append () and place cursor inside.
	if kind == completer.KindFunction || kind == completer.KindAggFunction {
		m.input.ReplaceWordAtCursor(len(prefix), selected+"()")
		// Move cursor back one position to be inside the parens.
		m.input.MoveCursorLeft()
	} else {
		m.input.ReplaceWordAtCursor(len(prefix), selected)
	}
}

// View composes the full TUI from sub-model views.
func (m *Model) View() tea.View {
	if m.quitting {
		return tea.NewView("Goodbye.\n")
	}

	// Table viewer takes over the entire screen.
	if m.tableViewer != nil {
		v := tea.NewView(m.tableViewer.View())
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	var sections []string

	// Update spinner in top bar.
	if m.loading {
		m.statusBar.SetSpinnerView(m.spinner.View())
	}

	// Status bar (top).
	sections = append(sections, m.statusBar.TopBarView())

	// Results are printed via tea.Println (terminal scrollback).

	// Watch mode indicator.
	if m.watching {
		watchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Bold(true)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
		watchInfo := watchStyle.Render(fmt.Sprintf("  ⟳ WATCH #%d", m.watchCount)) +
			dimStyle.Render(fmt.Sprintf("  every %s  ", m.watchInterval)) +
			dimStyle.Render("Ctrl+C to stop")
		sections = append(sections, watchInfo)
	}

	// Show progress during query execution.
	if m.running {
		sections = append(sections, m.renderQueryProgress())
	}

	// When search overlay is active, show it instead of input + completion.
	if m.search.Active() {
		sections = append(sections, m.search.View())
		m.statusBar.SetHintMode(HintDefault)
		sections = append(sections, m.statusBar.HintsBarView())
		return tea.NewView(strings.Join(sections, "\n"))
	}

	// Input area (bordered box).
	sections = append(sections, m.input.View())

	// Signature help — show function syntax + argument details below input.
	toCursor := m.input.ValueToCursor()
	funcName, argIdx := completer.EnclosingFunction(toCursor)
	if funcName != "" {
		sig, retVal, args := m.completer.FunctionSignatureDetail(funcName, argIdx)
		if sig != "" {
			hintBg := lipgloss.NewStyle().
				Background(lipgloss.Color("#2a2e3f")).
				Foreground(lipgloss.Color("#a9b1d6"))
			retStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("#2a2e3f")).
				Foreground(lipgloss.Color("#7aa2f7"))
			argStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("#2a2e3f")).
				Foreground(lipgloss.Color("#565f89"))

			// Line 1: signature with highlighted arg + return type.
			argHint := highlightArg(sig, argIdx)
			line1 := "  " + argHint
			if retVal != "" {
				line1 += " → " + retStyle.Render(retVal)
			}

			// Line 2: argument description (if available).
			hint := hintBg.Render(line1 + "  ")
			if args != "" {
				hint += "\n" + argStyle.Render("  "+args)
			}

			sections = append(sections, hint)
		}
	}

	// Completion popup.
	if m.completion.Visible() {
		prefix := completer.LastWord(toCursor)
		popupX := max(m.input.CursorScreenX()-len(prefix), 0)
		sections = append(sections, m.completion.ViewAt(popupX))
	}

	// Update hint mode based on state.
	if m.completion.Visible() {
		m.statusBar.SetHintMode(HintCompletion)
	} else {
		m.statusBar.SetHintMode(HintDefault)
	}

	// Bottom hints bar.
	sections = append(sections, m.statusBar.HintsBarView())

	return tea.NewView(strings.Join(sections, "\n"))
}

// handleMouseClick processes mouse click events.
func (m *Model) handleMouseClick(_ tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	// Click on ticker area (bottom) — check if a ticker item was clicked.
	// Ticker is near the bottom of the screen. Simple approach: just focus input.
	return m, m.input.Focus()
}

// handleMouseWheel handles mouse wheel events.
func (m *Model) handleMouseWheel(_ tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	return m, nil
}

// tryReconnect attempts to reconnect if the connection is broken.
func (m *Model) tryReconnect() {
	ctx := context.Background()
	if err := m.conn.Reconnect(ctx); err != nil {
		// reconnect error logged
		m.statusBar.SetConnected(false)
		return
	}
	m.statusBar.SetConnected(true)
}

// browseHistory navigates through query history.
// direction: -1 = older, +1 = newer.
func (m *Model) browseHistory(direction int) {
	// Lazy-load history entries.
	if m.historyEntries == nil {
		entries, err := m.history.Queries(500)
		if err != nil || len(entries) == 0 {
			return
		}
		m.historyEntries = entries
	}

	if len(m.historyEntries) == 0 {
		return
	}

	// Save current input when starting to browse.
	if m.historyIdx == -1 {
		m.historySaved = m.input.Value()
	}

	newIdx := max(m.historyIdx+direction, -1)
	if newIdx >= len(m.historyEntries) {
		newIdx = len(m.historyEntries) - 1
	}

	m.historyIdx = newIdx

	if m.historyIdx == -1 {
		// Back to the user's original input.
		m.input.InsertText(m.historySaved)
	} else {
		m.input.InsertText(m.historyEntries[m.historyIdx])
	}
}

// resetHistoryBrowsing resets history navigation state.
func (m *Model) resetHistoryBrowsing() {
	m.historyIdx = -1
	m.historyEntries = nil
	m.historySaved = ""
}

// formatLastMetrics formats all captured metrics from the last query.
// Package-level styles for renderQueryProgress — allocated once, not on every render.
var (
	progressSep   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261")).Render(" │ ")
	progressLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	progressRows  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
	progressRead  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Bold(true)
	progressMem   = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68")).Bold(true)
	progressCPU   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9e64")).Bold(true)
	progressTime  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5")).Bold(true)
	progressDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
	progressBar   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	progressEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("#2a2e3f"))
	progressPct   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
)

// renderQueryProgress renders two-line live progress during query execution.
// Line 1: key metrics with separators, color-coded
// Line 2: throughput/detail in dim
func (m *Model) renderQueryProgress() string {
	elapsed := time.Since(m.queryStart)
	secs := elapsed.Seconds()
	p := m.progress

	// Line 1: spinner + key metrics
	line1 := fmt.Sprintf("  %s ", m.spinner.View())

	if p != nil && p.ReadRows > 0 {
		line1 += progressLabel.Render("Rows ") + progressRows.Render(formatNumber(p.ReadRows))
		line1 += progressSep + progressLabel.Render("Read ") + progressRead.Render(formatBytes(p.ReadBytes))
		if p.MemoryUsage > 0 {
			line1 += progressSep + progressLabel.Render("Mem ") + progressMem.Render(formatBytes(uint64(p.MemoryUsage)))
		}
		cpuTotal := p.CPUUser + p.CPUSystem
		if cpuTotal > 0 {
			line1 += progressSep + progressLabel.Render("CPU ") + progressCPU.Render(fmt.Sprintf("%.2fs", float64(cpuTotal)/1e6))
		}
		line1 += progressSep + progressTime.Render(fmt.Sprintf("%.1fs", secs))
	} else {
		line1 += progressLabel.Render("Running... ") + progressTime.Render(fmt.Sprintf("%.1fs", secs))
	}

	// Line 2: throughput + progress bar
	line2 := ""
	if p != nil && p.ReadRows > 0 {
		var parts []string

		if secs > 0.1 {
			parts = append(parts, progressDim.Render(formatNumber(uint64(float64(p.ReadRows)/secs))+"/s"))
			parts = append(parts, progressDim.Render(formatBytes(uint64(float64(p.ReadBytes)/secs))+"/s"))
		}
		if p.Threads > 0 {
			parts = append(parts, progressDim.Render(fmt.Sprintf("%d threads", p.Threads)))
		}
		if p.DiskRead > 0 {
			parts = append(parts, progressDim.Render("disk "+formatBytes(uint64(p.DiskRead))))
		}

		// Progress bar when total rows known.
		if p.TotalRows > 0 {
			pct := float64(p.ReadRows) / float64(p.TotalRows) * 100
			if pct > 100 {
				pct = 100
			}
			barWidth := 20
			filled := int(pct / 100 * float64(barWidth))
			bar := progressBar.Render(strings.Repeat("█", filled)) + progressEmpty.Render(strings.Repeat("░", barWidth-filled))
			parts = append(parts, bar+" "+progressPct.Render(fmt.Sprintf("%.0f%%", pct)))

			if pct > 0 && pct < 100 {
				remaining := secs * (100 - pct) / pct
				parts = append(parts, progressDim.Render(fmt.Sprintf("ETA %.0fs", remaining)))
			}
		}

		if len(parts) > 0 {
			line2 = "\n    " + strings.Join(parts, "  ")
		}
	}

	return line1 + line2
}

func (m *Model) formatLastMetrics() string {
	if m.lastMetrics == nil || len(m.lastMetrics.Metrics) == 0 {
		return "No metrics from last query."
	}
	p := m.lastMetrics
	var sb strings.Builder
	sb.WriteString("Last query metrics:\n\n")

	// Sort metric names.
	names := make([]string, 0, len(p.Metrics))
	for name := range p.Metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		val := p.Metrics[name]
		if val == 0 {
			continue
		}
		// Format value based on name suffix.
		var fmtVal string
		switch {
		case strings.HasSuffix(name, "Microseconds"):
			fmtVal = fmt.Sprintf("%.3fs", float64(val)/1e6)
		case strings.HasSuffix(name, "Bytes") || name == "MemoryTrackerUsage":
			fmtVal = formatBytes(uint64(val))
		default:
			fmtVal = formatNumber(uint64(val))
		}
		fmt.Fprintf(&sb, "  %-45s %s\n", name, fmtVal)
	}
	return sb.String()
}

func formatNumber(n uint64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return strconv.FormatUint(n, 10)
}

func formatBytes(b uint64) string {
	if b >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	}
	if b >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	if b >= 1<<10 {
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	}
	return fmt.Sprintf("%d B", b)
}

// highlightArg highlights the Nth argument in a function signature string.
// E.g., highlightArg("toDate(expr, timezone)", 1) bolds "timezone".
func highlightArg(sig string, argIdx int) string {
	openParen := strings.IndexByte(sig, '(')
	closeParen := strings.LastIndexByte(sig, ')')
	if openParen < 0 || closeParen < 0 || closeParen <= openParen {
		return sig
	}

	prefix := sig[:openParen+1]
	args := sig[openParen+1 : closeParen]
	suffix := sig[closeParen:]

	parts := strings.Split(args, ",")
	if argIdx >= len(parts) {
		argIdx = len(parts) - 1
	}

	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9e2af"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))

	var highlighted []string
	for i, part := range parts {
		if i == argIdx {
			highlighted = append(highlighted, boldStyle.Render(strings.TrimSpace(part)))
		} else {
			highlighted = append(highlighted, dimStyle.Render(strings.TrimSpace(part)))
		}
	}

	return prefix + strings.Join(highlighted, ", ") + suffix
}
