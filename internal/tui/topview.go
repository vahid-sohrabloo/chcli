package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

// Message types exchanged between the tick loop and Update.
type (
	topTickMsg         time.Time
	topSnapshotMsg     struct {
		snap  chtop.Snapshot
		rates chtop.Rates
	}
	topFetchErrMsg     struct{ err error }
	topBannerExpireMsg time.Time
)

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

// rememberCursorID and relockCursor are stubs until Task 12 when they're
// needed by the cursor-by-query-id stability tests. Kept as no-ops here so
// onSnapshot compiles and doesn't change behavior for earlier tasks.
func (t *topModel) rememberCursorID() {}
func (t *topModel) relockCursor()     {}
