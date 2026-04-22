package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/format"
	"github.com/vahid-sohrabloo/chcli/internal/highlight"
	"github.com/vahid-sohrabloo/chcli/internal/render"
)

// topColumns is the full column list in display order. Most-useful columns
// come first so the default column-offset of 0 shows user/elapsed/memory/rows
// without needing to scroll. The last column (query) is pinned as the right-
// most visible column and always shows.
var topColumns = []string{
	"user", "elapsed", "memory", "rows_read", "progress", "cpu",
	"database", "client", "initial_address", "query_id", "bytes_read",
	"query",
}

func init() {
	if len(topColumns) != topColumnCount {
		panic("topColumns and topColumnCount are out of sync")
	}
}

// Message types exchanged between the tick loop and Update.
type (
	topTickMsg     time.Time
	topSnapshotMsg struct {
		snap  chtop.Snapshot
		rates chtop.Rates
	}
	topFetchErrMsg     struct{ err error }
	topBannerExpireMsg time.Time
	topKillResultMsg   struct {
		queryID string
		err     error
	}
)

const topBannerTTL = 3 * time.Second

// sortKey selects which column the process list is sorted by.
type sortKey int

const (
	sortElapsed sortKey = iota
	sortMemory
	sortReadRows
)

// String returns the footer-friendly name for the sort key.
func (s sortKey) String() string {
	switch s {
	case sortMemory:
		return "memory"
	case sortReadRows:
		return "rows"
	default:
		return "elapsed"
	}
}

// topMode is the current modal state: only one of filter/kill/detail is
// active at a time.
type topMode int

const (
	ModeNormal topMode = iota
	ModeFilter
	ModeConfirmKill
	ModeDetail
)

// Internal aliases so the existing package-private switches stay readable.
const (
	modeNormal      = ModeNormal
	modeFilter      = ModeFilter
	modeConfirmKill = ModeConfirmKill
	modeDetail      = ModeDetail
)

// topColumnCount is how many columns the process table supports. Keeping this
// as a compile-time constant lets the column-offset clamp stay honest. The
// actual column labels live next to the render code (see topColumns).
const topColumnCount = 12

// sparkSize is the number of samples kept in the rolling sparkline buffers.
const sparkSize = 30

// topIntervals is the cycle applied by the 'd' key.
var topIntervals = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
}

// topModel is the alt-screen bubbletea model for \top.
type topModel struct {
	fetcher     *chtop.Fetcher
	killer      Killer
	highlighter *highlight.Highlighter
	snap        chtop.Snapshot
	rates       chtop.Rates
	err         error

	interval  time.Duration
	sortCol   sortKey
	colOffset int
	cursor    int    // index into the filtered+sorted slice
	cursorID  string // query_id under the cursor, used to re-lock across ticks
	filter    string

	mode        topMode
	filterBuf   string
	killTarget  string        // query_id captured at Enter to modeConfirmKill
	killUser    string        // user captured at the same moment
	killQuery   string        // SQL text preview for the confirm line
	detailID    string        // query_id pinned by entering modeDetail
	detailLast  chtop.Process // last-known snapshot, used when the query finishes
	banner      string
	bannerUntil time.Time

	// Rolling history for header sparklines. Each ring buffer holds the
	// last sparkSize samples; oldest value at [0] after rolling.
	qpsHist    []float64
	insertHist []float64
	memHist    []float64

	width, height int
}

// Killer is the subset of *conn.Conn that topview uses to cancel queries.
// Taking an interface here keeps topview unit-testable.
type Killer interface {
	KillQuery(queryID string) error
}

// newTopView constructs a topview bound to the given Fetcher. A nil Fetcher
// is only useful for tests that exercise state transitions without ticking.
func newTopView(f *chtop.Fetcher, width, height int) *topModel {
	return &topModel{
		fetcher:  f,
		interval: 2 * time.Second,
		sortCol:  sortElapsed,
		mode:     modeNormal,
		width:    width,
		height:   height,
	}
}

// WithKiller attaches a Killer. Called from the TUI code that already has
// the Conn; tests leave it nil and set it manually.
func (t *topModel) WithKiller(k Killer) *topModel { t.killer = k; return t }

// WithHighlighter attaches a SQL syntax highlighter used by the detail pane.
// A nil highlighter is safe — the detail pane falls back to plain text.
func (t *topModel) WithHighlighter(h *highlight.Highlighter) *topModel {
	t.highlighter = h
	return t
}

// onSnapshot is called from Update when a topSnapshotMsg arrives.
func (t *topModel) onSnapshot(snap chtop.Snapshot, rates chtop.Rates) {
	t.rememberCursorID()
	t.snap = snap
	t.rates = rates
	t.err = nil
	t.relockCursor()
	t.qpsHist = pushSample(t.qpsHist, rates.QueriesPerSec)
	t.insertHist = pushSample(t.insertHist, rates.InsertRowsPerSec)
	t.memHist = pushSample(t.memHist, float64(snap.Header.MemUsed))
}

// pushSample appends v to buf, dropping the oldest entry when buf is full.
func pushSample(buf []float64, v float64) []float64 {
	if len(buf) < sparkSize {
		return append(buf, v)
	}
	return append(buf[1:], v)
}

// sparkline renders a series as a one-line sparkline using the 9-level block
// characters. Empty series returns an empty string.
func sparkline(series []float64) string {
	if len(series) == 0 {
		return ""
	}
	blocks := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	minV, maxV := series[0], series[0]
	for _, v := range series[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	rangeV := maxV - minV
	var b strings.Builder
	for _, v := range series {
		var idx int
		if rangeV > 0 {
			idx = int((v - minV) / rangeV * float64(len(blocks)-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// onFetchErr is called from Update when a topFetchErrMsg arrives.
func (t *topModel) onFetchErr(err error) {
	t.err = err
}

// fetchCmd runs Fetcher.Fetch in a tea.Cmd and converts the result into one
// of topSnapshotMsg / topFetchErrMsg. Returns nil when the fetcher is nil
// (useful in tests).
func (t *topModel) fetchCmd() tea.Cmd {
	if t.fetcher == nil {
		return nil
	}
	f := t.fetcher
	return func() tea.Msg {
		snap, rates, err := f.Fetch(context.Background())
		if err != nil {
			return topFetchErrMsg{err: err}
		}
		return topSnapshotMsg{snap: snap, rates: rates}
	}
}

// tickCmd schedules the next topTickMsg using the current interval. Reading
// t.interval at schedule time means the 'd' key always takes effect on the
// next cycle, not the current one.
func (t *topModel) tickCmd() tea.Cmd {
	d := t.interval
	return tea.Tick(d, func(now time.Time) tea.Msg { return topTickMsg(now) })
}

// rememberCursorID snapshots the query_id under the cursor before a new tick
// lands. Called from onSnapshot against the OLD visible slice.
func (t *topModel) rememberCursorID() {
	visible := t.visibleProcesses()
	if t.cursor >= 0 && t.cursor < len(visible) {
		t.cursorID = visible[t.cursor].QueryID
	} else {
		t.cursorID = ""
	}
}

// relockCursor finds t.cursorID in the new visible slice and moves the cursor
// there. If the id is gone, it clamps to the nearest previous valid index.
func (t *topModel) relockCursor() {
	visible := t.visibleProcesses()
	if t.cursorID != "" {
		for i, p := range visible {
			if p.QueryID == t.cursorID {
				t.cursor = i
				return
			}
		}
	}
	if t.cursor >= len(visible) {
		t.cursor = len(visible) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

// SetSize updates width/height without requiring a WindowSizeMsg. Used by
// the monitor container that resizes tabs directly.
func (t *topModel) SetSize(w, h int) {
	t.width = w
	t.height = h
}

// Mode returns the current modal state. Used by the monitor's Processes tab
// adapter to implement HasActiveModal().
func (t *topModel) Mode() topMode { return t.mode }

// FetchCmd / TickCmd are the exported forms of fetchCmd / tickCmd so the
// monitor's Processes adapter can fire them on tab activation.
func (t *topModel) FetchCmd() tea.Cmd { return t.fetchCmd() }
func (t *topModel) TickCmd() tea.Cmd  { return t.tickCmd() }

// Update is the bubbletea update for the topview. Returns (cmd, closed); when
// closed is true the outer Model should nil out its topView field and return
// focus to the REPL input.
func (t *topModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case topTickMsg:
		return tea.Batch(t.fetchCmd(), t.tickCmd()), false
	case topSnapshotMsg:
		t.onSnapshot(msg.snap, msg.rates)
		return nil, false
	case topFetchErrMsg:
		t.onFetchErr(msg.err)
		return nil, false
	case topBannerExpireMsg:
		if !t.bannerUntil.IsZero() && !time.Now().Before(t.bannerUntil) {
			t.banner = ""
			t.bannerUntil = time.Time{}
		}
		return nil, false
	case topKillResultMsg:
		if msg.err != nil {
			t.setBanner("KILL error: " + msg.err.Error())
		} else {
			t.setBanner("KILL sent for " + shortID(msg.queryID))
		}
		return t.bannerExpireCmd(), false
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return nil, false
	case tea.KeyPressMsg:
		return t.handleKey(msg)
	}
	return nil, false
}

func (t *topModel) handleKey(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	// Modal states are handled by their own sub-handlers (filled in by the
	// filter / kill / detail tasks).
	switch t.mode {
	case modeFilter:
		return t.handleKeyFilter(kp)
	case modeConfirmKill:
		return t.handleKeyConfirmKill(kp)
	case modeDetail:
		return t.handleKeyDetail(kp)
	}

	switch kp.Code {
	case 'q', tea.KeyEscape:
		return nil, true
	case tea.KeyUp:
		if t.cursor > 0 {
			t.cursor--
		}
	case tea.KeyDown:
		if t.cursor < len(t.snap.Processes)-1 {
			t.cursor++
		}
	case tea.KeyLeft:
		if t.colOffset > 0 {
			t.colOffset--
		}
	case tea.KeyRight:
		if t.colOffset < topColumnCount-1 {
			t.colOffset++
		}
	case tea.KeyEnter:
		visible := t.visibleProcesses()
		if len(visible) > 0 && t.cursor < len(visible) {
			t.detailID = visible[t.cursor].QueryID
			t.detailLast = visible[t.cursor]
			t.mode = modeDetail
		}
	case '/':
		t.mode = modeFilter
		t.filterBuf = ""
	case 'k', 'K':
		if len(t.snap.Processes) == 0 {
			return nil, false
		}
		visible := t.visibleProcesses()
		if t.cursor >= len(visible) {
			return nil, false
		}
		t.killTarget = visible[t.cursor].QueryID
		t.killUser = visible[t.cursor].User
		t.killQuery = visible[t.cursor].Query
		t.mode = modeConfirmKill
	case 's':
		t.sortCol = (t.sortCol + 1) % 3
	case 'd':
		t.interval = nextInterval(t.interval)
	}
	return nil, false
}

func (t *topModel) handleKeyFilter(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	switch kp.Code {
	case tea.KeyEscape:
		t.mode = modeNormal
		t.filterBuf = ""
		return nil, false
	case tea.KeyEnter:
		t.filter = t.filterBuf
		t.mode = modeNormal
		t.filterBuf = ""
		return nil, false
	case tea.KeyBackspace, tea.KeyDelete:
		if len(t.filterBuf) > 0 {
			runes := []rune(t.filterBuf)
			t.filterBuf = string(runes[:len(runes)-1])
		}
		return nil, false
	}
	// Treat any printable char as input.
	if kp.Mod == 0 && kp.Code >= 0x20 && kp.Code != 0x7f {
		t.filterBuf += string(kp.Code)
	}
	return nil, false
}

func (t *topModel) handleKeyConfirmKill(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	// Only an explicit lowercase or uppercase 'y' confirms. Enter / Esc /
	// anything else cancels — matches the [y/N] convention where N is the
	// default.
	if kp.Code == 'y' || kp.Code == 'Y' {
		target := t.killTarget
		t.mode = modeNormal
		t.killTarget = ""
		t.killUser = ""
		t.killQuery = ""
		if t.killer == nil || target == "" {
			return nil, false
		}
		k := t.killer
		return func() tea.Msg {
			if err := k.KillQuery(target); err != nil {
				return topKillResultMsg{err: err}
			}
			return topKillResultMsg{queryID: target}
		}, false
	}
	t.mode = modeNormal
	t.killTarget = ""
	t.killUser = ""
	t.killQuery = ""
	return nil, false
}

func (t *topModel) handleKeyDetail(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	switch kp.Code {
	case tea.KeyEscape, tea.KeyEnter, 'q':
		t.mode = modeNormal
		t.detailID = ""
	}
	return nil, false
}

// setBanner stores a transient status message shown in the modal slot for
// topBannerTTL before topBannerExpireMsg clears it.
func (t *topModel) setBanner(s string) {
	t.banner = s
	t.bannerUntil = time.Now().Add(topBannerTTL)
}

// bannerExpireCmd returns a tea.Cmd that fires topBannerExpireMsg once the
// banner's TTL has elapsed.
func (t *topModel) bannerExpireCmd() tea.Cmd {
	return tea.Tick(topBannerTTL, func(now time.Time) tea.Msg { return topBannerExpireMsg(now) })
}

// shortID returns the first 8 chars of a query_id for compact banners.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

// visibleProcesses returns the sorted+filtered slice the UI currently shows.
// Returns a fresh slice; does not mutate t.snap.Processes.
func (t *topModel) visibleProcesses() []chtop.Process {
	return applyFilterAndSort(t.snap.Processes, t.filter, t.sortCol)
}

func applyFilterAndSort(procs []chtop.Process, filter string, key sortKey) []chtop.Process {
	var out []chtop.Process
	if filter != "" {
		needle := strings.ToLower(filter)
		out = make([]chtop.Process, 0, len(procs))
		for _, p := range procs {
			if strings.Contains(strings.ToLower(p.User), needle) ||
				strings.Contains(strings.ToLower(p.Database), needle) ||
				strings.Contains(strings.ToLower(p.InitialAddress), needle) ||
				strings.Contains(strings.ToLower(p.Query), needle) {
				out = append(out, p)
			}
		}
	} else {
		out = append([]chtop.Process(nil), procs...)
	}
	sort.Slice(out, func(i, j int) bool {
		switch key {
		case sortMemory:
			return out[i].MemoryUsage > out[j].MemoryUsage
		case sortReadRows:
			return out[i].ReadRows > out[j].ReadRows
		default:
			return out[i].Elapsed > out[j].Elapsed
		}
	})
	return out
}

// nextInterval cycles through topIntervals, wrapping around. Any unknown
// current value resets to 1s.
func nextInterval(cur time.Duration) time.Duration {
	for i, d := range topIntervals {
		if d == cur {
			return topIntervals[(i+1)%len(topIntervals)]
		}
	}
	return time.Second
}

// renderHeader draws the 2-3 line summary band at the top of the alt-screen.
// Foreground-only styling — no background fills — matching the REPL look.
func (t *topModel) renderHeader() string {
	h := t.snap.Header
	label := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.AccentBlue)).Render(s)
	}
	val := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.TextPrimary)).Render(s)
	}

	var b strings.Builder
	// Line 1: chtop  up UPTIME  vVERSION
	b.WriteString(label("chtop"))
	b.WriteString("  ")
	b.WriteString(label("up "))
	b.WriteString(val(formatUptime(h.Uptime)))
	if h.Version != "" {
		b.WriteString("  ")
		b.WriteString(val("v" + h.Version))
	}
	b.WriteString("\n")

	// Line 2: Q N running · q/s [spark] · rows/s inserts [spark] · mem used/total [spark] · merges
	b.WriteString(val(fmt.Sprintf("Q %d running", h.ActiveQueries)))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("%.1f q/s ", t.rates.QueriesPerSec)))
	b.WriteString(dim(sparkline(t.qpsHist)))
	b.WriteString(dim(" · "))
	b.WriteString(val(humanCount(uint64(t.rates.InsertRowsPerSec)) + " rows/s inserts "))
	b.WriteString(dim(sparkline(t.insertHist)))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("mem %s/%s ", humanBytes(h.MemUsed), humanBytes(h.MemTotal))))
	b.WriteString(dim(sparkline(t.memHist)))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("merges %d", h.MergesRunning)))
	b.WriteString("\n")

	// Line 3: mutations N · replica-lag (elided when no replicas)
	b.WriteString(val(fmt.Sprintf("mutations %d", h.MutationsRunning)))
	if h.ReplicaMaxDelay >= 0 {
		b.WriteString(dim(" · "))
		b.WriteString(val(fmt.Sprintf("replica-lag max %.2fs", h.ReplicaMaxDelay)))
	}

	return b.String()
}

func dim(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.TextMuted)).Render(s)
}

// formatUptime renders a time.Duration as "3d 12:04:57" / "12:04:57" / "4:57".
func formatUptime(d time.Duration) string {
	total := int64(d.Seconds())
	days := total / 86400
	h := (total % 86400) / 3600
	m := (total % 3600) / 60
	s := total % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %02d:%02d:%02d", days, h, m, s)
	case h > 0:
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	default:
		return fmt.Sprintf("%d:%02d", m, s)
	}
}

// humanBytes formats bytes as "8.3 GB" / "512 MB" / "48 kB".
func humanBytes(n uint64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	v := float64(n) / k
	i := 0
	for v >= k && i < len(units)-1 {
		v /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// renderTable draws the process list via the shared RenderTable helper.
// Shows a placeholder when the first snapshot has not yet arrived or when the
// filtered list is empty.
func (t *topModel) renderTable() string {
	if t.snap.At.IsZero() {
		if t.err != nil {
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color(ActiveTheme.AccentRed)).
				Render("  Error: " + t.err.Error())
		}
		return dim("  connecting…")
	}
	visible := t.visibleProcesses()
	if len(visible) == 0 {
		return dim("  no active queries")
	}

	cols, colIdx := t.visibleColumns()
	rows := make([][]string, len(visible))
	for i, p := range visible {
		row := make([]string, len(colIdx))
		for j, idx := range colIdx {
			row[j] = processCell(p, idx)
		}
		rows[i] = row
	}
	return render.RenderTableSelected(cols, rows, t.width, t.cursor)
}

// visibleColumns returns the column labels and their indices in topColumns
// for the current colOffset, pinning "query" as the last column.
func (t *topModel) visibleColumns() ([]string, []int) {
	const pinned = 11 // index of "query" in topColumns
	start := t.colOffset
	if start >= pinned {
		start = pinned - 1
	}
	// Take up to 5 scrollable columns plus the pinned query column.
	end := min(start+5, pinned)
	idx := make([]int, 0, end-start+1)
	for i := start; i < end; i++ {
		idx = append(idx, i)
	}
	idx = append(idx, pinned)

	labels := make([]string, len(idx))
	for i, k := range idx {
		labels[i] = topColumns[k]
	}
	return labels, idx
}

// processCell formats the value for column index `col` (matching the
// topColumns order).
func processCell(p chtop.Process, col int) string {
	switch col {
	case 0:
		return p.User
	case 1:
		return fmt.Sprintf("%.2fs", p.Elapsed)
	case 2:
		return humanBytes(uint64(p.MemoryUsage))
	case 3:
		return humanCount(p.ReadRows)
	case 4:
		return renderProgressBar(p.ReadRows, p.TotalRowsApprox)
	case 5:
		return fmt.Sprintf("%.2fs", p.CPUSeconds)
	case 6:
		return p.Database
	case 7:
		return p.Client
	case 8:
		return p.InitialAddress
	case 9:
		return p.QueryID
	case 10:
		return humanBytes(p.ReadBytes)
	case 11:
		return p.Query
	}
	return ""
}

// renderProgressBar returns a compact 10-cell bar + percentage for a read/total
// pair. When total is 0 (ClickHouse doesn't know) it returns a dash.
func renderProgressBar(read, total uint64) string {
	if total == 0 || read > total {
		return "—"
	}
	const width = 10
	pct := float64(read) / float64(total)
	filled := min(int(pct*float64(width)), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled) +
		fmt.Sprintf(" %3.0f%%", pct*100)
}

// renderFooter draws the bottom hint line.
func (t *topModel) renderFooter() string {
	hint := func(k, d string) string {
		return dim(k) + " " + lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.TextMuted)).Render(d)
	}
	parts := []string{
		hint("↑↓", "nav"),
		hint("←→", "cols"),
		hint("s", "sort:"+t.sortCol.String()),
		hint("d", formatIntervalShort(t.interval)),
		hint("/", "filter"),
		hint("k", "kill"),
		hint("Enter", "detail"),
		hint("q", "quit"),
	}
	return "  " + strings.Join(parts, "  ")
}

func formatIntervalShort(d time.Duration) string {
	if d == 500*time.Millisecond {
		return "0.5s"
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// renderModal draws the optional one-line band above the footer. Empty string
// when no modal/banner is active.
func (t *topModel) renderModal() string {
	now := time.Now()
	if t.banner != "" && now.Before(t.bannerUntil) {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.AccentYellow)).Render("  " + t.banner)
	}
	switch t.mode {
	case modeFilter:
		cursor := lipgloss.NewStyle().Reverse(true).Render(" ")
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.TextPrimary)).
			Render("  /filter: " + t.filterBuf + cursor)
	case modeConfirmKill:
		red := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.AccentRed)).Bold(true)
		muted := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.TextMuted))
		// Preview the SQL text — truncated to whatever fits after the prefix.
		prefix := "  Kill " + t.killTarget
		if t.killUser != "" {
			prefix += " by " + t.killUser
		}
		suffix := "  [y/N]"
		// Allow at least 20 chars for the preview on very narrow terminals.
		budget := max(t.width-lipgloss.Width(prefix)-lipgloss.Width(suffix)-4, 20)
		preview := t.killQuery
		if len(preview) > budget {
			preview = preview[:budget-1] + "…"
		}
		return red.Render(prefix) + muted.Render("  "+preview) + red.Render(suffix)
	}
	return ""
}

// View composes the full alt-screen: header · sep · table-or-detail · (modal)
// · sep · footer.
func (t *topModel) View() string {
	sep := dim(strings.Repeat("─", t.width))

	body := t.renderTable()
	if t.mode == modeDetail {
		body = t.renderDetail()
	}

	parts := []string{
		t.renderHeader(),
		sep,
		body,
	}
	if modal := t.renderModal(); modal != "" {
		parts = append(parts, sep, modal)
	}
	parts = append(parts, sep, t.renderFooter())
	return strings.Join(parts, "\n")
}

// renderDetail draws the full query metadata + formatted/highlighted SQL for
// the currently-selected row. Structure:
//
//	Title row ............................... q/Esc close
//	────────────────────────────────────────────────────
//	  label  : value
//	  label  : value
//	  ...
//	────────────────────────────────────────────────────
//	  <formatted, highlighted SQL>
func (t *topModel) renderDetail() string {
	// Pin to the query_id captured at Enter. If it's still running, show live
	// stats from the current snap; if it finished, show the last-known snapshot
	// with a "(finished)" note so the pane doesn't drift to another query.
	p := t.detailLast
	finished := true
	for i := range t.snap.Processes {
		if t.snap.Processes[i].QueryID == t.detailID {
			p = t.snap.Processes[i]
			t.detailLast = p
			finished = false
			break
		}
	}

	sep := dim(strings.Repeat("─", t.width))

	// Title row with right-aligned close hint.
	titleText := "  Query detail"
	if finished {
		titleText += " (finished)"
	}
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.AccentBlue)).Bold(true).
		Render(titleText)
	closeHint := dim("q/Esc to close  ")
	pad := max(t.width-lipgloss.Width(title)-lipgloss.Width(closeHint), 1)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString(strings.Repeat(" ", pad))
	b.WriteString(closeHint)
	b.WriteString("\n")
	b.WriteString(sep)
	b.WriteString("\n")

	// Labeled metadata block with right-aligned labels.
	labels := []string{
		"query_id", "user", "database", "client", "initial_address",
		"elapsed", "cpu", "memory_usage", "read_rows", "read_bytes",
	}
	values := []string{
		p.QueryID, p.User, p.Database, p.Client, p.InitialAddress,
		fmt.Sprintf("%.2fs", p.Elapsed),
		fmt.Sprintf("%.2fs", p.CPUSeconds),
		humanBytes(uint64(p.MemoryUsage)),
		humanCount(p.ReadRows),
		humanBytes(p.ReadBytes),
	}
	b.WriteString(renderKVBlock(labels, values))
	b.WriteString(sep)
	b.WriteString("\n\n")

	// Formatted + highlighted SQL, indented.
	sql := format.FormatSQL(p.Query)
	if t.highlighter != nil {
		sql = t.highlighter.Highlight(sql)
	}
	b.WriteString("  ")
	b.WriteString(strings.ReplaceAll(sql, "\n", "\n  "))

	return b.String()
}

// renderKVBlock renders a labeled key : value list with right-aligned labels
// matching the existing vertical-query mode's look.
func renderKVBlock(labels, values []string) string {
	maxLabel := 0
	for _, l := range labels {
		if w := lipgloss.Width(l); w > maxLabel {
			maxLabel = w
		}
	}
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.AccentBlue)).Bold(true)
	valStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.TextPrimary))

	var b strings.Builder
	for i, l := range labels {
		b.WriteString("  ")
		b.WriteString(labelStyle.Render(fmt.Sprintf("%*s", maxLabel, l)))
		b.WriteString(" : ")
		b.WriteString(valStyle.Render(values[i]))
		b.WriteString("\n")
	}
	return b.String()
}

// humanCount formats a count as "1.2M" / "3.4k" / "42".
func humanCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return strconv.FormatUint(n, 10)
	}
}
