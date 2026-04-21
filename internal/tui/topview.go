package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/render"
)

// topColumns is the full column list in display order. The column-offset
// window slides across [0, len-1) and the last column (query) is pinned.
var topColumns = []string{
	"user", "database", "client", "initial_address", "query_id",
	"elapsed", "rows_read", "bytes_read", "memory_usage", "query",
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
	modeNormal topMode = iota
	modeFilter
	modeConfirmKill
	modeDetail
)

// topColumnCount is how many columns the process table supports. Keeping this
// as a compile-time constant lets the column-offset clamp stay honest. The
// actual column labels live next to the render code (Task 15).
const topColumnCount = 10

// topIntervals is the cycle applied by the 'd' key.
var topIntervals = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
}

// topModel is the alt-screen bubbletea model for \top.
type topModel struct {
	fetcher *chtop.Fetcher
	killer  Killer
	snap    chtop.Snapshot
	rates   chtop.Rates
	err     error

	interval  time.Duration
	sortCol   sortKey
	colOffset int
	cursor    int    // index into the filtered+sorted slice
	cursorID  string // query_id under the cursor, used to re-lock across ticks
	filter    string

	mode        topMode
	filterBuf   string
	killTarget  string
	banner      string
	bannerUntil time.Time

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
		interval: time.Second,
		sortCol:  sortElapsed,
		mode:     modeNormal,
		width:    width,
		height:   height,
	}
}

// WithKiller attaches a Killer. Called from the TUI code that already has
// the Conn; tests leave it nil and set it manually.
func (t *topModel) WithKiller(k Killer) *topModel { t.killer = k; return t }

// onSnapshot is called from Update when a topSnapshotMsg arrives.
func (t *topModel) onSnapshot(snap chtop.Snapshot, rates chtop.Rates) {
	t.rememberCursorID()
	t.snap = snap
	t.rates = rates
	t.err = nil
	t.relockCursor()
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
			t.mode = modeDetail
		}
	case '/':
		t.mode = modeFilter
		t.filterBuf = ""
	case 'K':
		if len(t.snap.Processes) == 0 {
			return nil, false
		}
		visible := t.visibleProcesses()
		if t.cursor >= len(visible) {
			return nil, false
		}
		t.killTarget = visible[t.cursor].QueryID
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
	switch kp.Code {
	case 'y', tea.KeyEnter:
		target := t.killTarget
		t.mode = modeNormal
		t.killTarget = ""
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
	// Any other key cancels.
	t.mode = modeNormal
	t.killTarget = ""
	return nil, false
}

func (t *topModel) handleKeyDetail(kp tea.KeyPressMsg) (tea.Cmd, bool) {
	switch kp.Code {
	case tea.KeyEscape, tea.KeyEnter, 'q':
		t.mode = modeNormal
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

	// Line 2: Q N running · q/s · rows/s · mem used/total · merges
	b.WriteString(val(fmt.Sprintf("Q %d running", h.ActiveQueries)))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("%.1f q/s", t.rates.QueriesPerSec)))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("%s rows/s inserts", humanCount(uint64(t.rates.InsertRowsPerSec)))))
	b.WriteString(dim(" · "))
	b.WriteString(val(fmt.Sprintf("mem %s/%s", humanBytes(h.MemUsed), humanBytes(h.MemTotal))))
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
	return render.RenderTable(cols, rows, t.width)
}

// visibleColumns returns the column labels and their indices in topColumns
// for the current colOffset, pinning "query" as the last column.
func (t *topModel) visibleColumns() ([]string, []int) {
	const pinned = 9 // index of "query"
	start := t.colOffset
	if start >= pinned {
		start = pinned - 1
	}
	// Take up to 4 scrollable columns plus the pinned query column.
	end := start + 4
	if end > pinned {
		end = pinned
	}
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

func processCell(p chtop.Process, col int) string {
	switch col {
	case 0:
		return p.User
	case 1:
		return p.Database
	case 2:
		return p.Client
	case 3:
		return p.InitialAddress
	case 4:
		return p.QueryID
	case 5:
		return fmt.Sprintf("%.2fs", p.Elapsed)
	case 6:
		return humanCount(p.ReadRows)
	case 7:
		return humanBytes(p.ReadBytes)
	case 8:
		return humanBytes(uint64(p.MemoryUsage))
	case 9:
		return p.Query
	}
	return ""
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
		return fmt.Sprintf("%d", n)
	}
}
