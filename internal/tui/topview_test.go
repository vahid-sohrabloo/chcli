package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

// stripANSI returns s with ANSI escape sequences removed. Tiny test helper
// so we don't pull in a dependency.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func seedProcs(tv *topModel, n int) {
	procs := make([]chtop.Process, n)
	for i := 0; i < n; i++ {
		procs[i] = chtop.Process{
			QueryID: string(rune('a' + i)),
			Elapsed: float64(n - i),
		}
	}
	tv.snap = chtop.Snapshot{Processes: procs}
}

func pressKey(tv *topModel, code rune) (tea.Cmd, bool) {
	return tv.Update(tea.KeyPressMsg{Code: code})
}

func TestTopViewCursorClamps(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	seedProcs(tv, 3)

	pressKey(tv, tea.KeyDown)
	pressKey(tv, tea.KeyDown)
	pressKey(tv, tea.KeyDown)
	if tv.cursor != 2 {
		t.Errorf("cursor = %d after 3 downs, want 2", tv.cursor)
	}
	pressKey(tv, tea.KeyUp)
	pressKey(tv, tea.KeyUp)
	pressKey(tv, tea.KeyUp)
	pressKey(tv, tea.KeyUp)
	if tv.cursor != 0 {
		t.Errorf("cursor = %d after 4 ups, want 0", tv.cursor)
	}
}

func TestTopViewColumnOffsetClamps(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	for i := 0; i < 20; i++ {
		pressKey(tv, tea.KeyRight)
	}
	if tv.colOffset != topColumnCount-1 {
		t.Errorf("colOffset = %d after 20 rights, want %d", tv.colOffset, topColumnCount-1)
	}
	for i := 0; i < 20; i++ {
		pressKey(tv, tea.KeyLeft)
	}
	if tv.colOffset != 0 {
		t.Errorf("colOffset = %d after 20 lefts, want 0", tv.colOffset)
	}
}

func TestTopViewSortCycle(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	if tv.sortCol != sortElapsed {
		t.Fatalf("initial sortCol = %v", tv.sortCol)
	}
	pressKey(tv, 's')
	if tv.sortCol != sortMemory {
		t.Errorf("after 1 's': %v, want sortMemory", tv.sortCol)
	}
	pressKey(tv, 's')
	if tv.sortCol != sortReadRows {
		t.Errorf("after 2 's': %v, want sortReadRows", tv.sortCol)
	}
	pressKey(tv, 's')
	if tv.sortCol != sortElapsed {
		t.Errorf("after 3 's': %v, want sortElapsed (wrapped)", tv.sortCol)
	}
}

func TestTopViewIntervalCycle(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	want := []time.Duration{
		2 * time.Second,
		5 * time.Second,
		500 * time.Millisecond,
		time.Second,
	}
	for i, w := range want {
		pressKey(tv, 'd')
		if tv.interval != w {
			t.Errorf("press %d: interval = %v, want %v", i+1, tv.interval, w)
		}
	}
}

func TestTopViewQuitReturnsTrue(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	_, closed := pressKey(tv, 'q')
	if !closed {
		t.Fatal("q should close topview")
	}
}

func TestTopViewEscReturnsTrue(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	_, closed := pressKey(tv, tea.KeyEscape)
	if !closed {
		t.Fatal("Esc should close topview")
	}
}

func TestTopViewFilterMode(t *testing.T) {
	tv := newTopView(nil, 80, 24)

	pressKey(tv, '/')
	if tv.mode != modeFilter {
		t.Fatalf("mode = %v after '/', want modeFilter", tv.mode)
	}

	tv.Update(tea.KeyPressMsg{Code: 'f'})
	tv.Update(tea.KeyPressMsg{Code: 'o'})
	tv.Update(tea.KeyPressMsg{Code: 'o'})
	if tv.filterBuf != "foo" {
		t.Errorf("filterBuf = %q, want foo", tv.filterBuf)
	}

	tv.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if tv.filterBuf != "fo" {
		t.Errorf("filterBuf = %q after BS, want fo", tv.filterBuf)
	}

	tv.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if tv.mode != modeNormal {
		t.Errorf("mode = %v after Enter, want modeNormal", tv.mode)
	}
	if tv.filter != "fo" {
		t.Errorf("filter = %q after commit, want fo", tv.filter)
	}
}

func TestTopViewFilterEscRestoresPrior(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	tv.filter = "bar"
	pressKey(tv, '/')
	tv.Update(tea.KeyPressMsg{Code: 'x'})
	tv.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if tv.mode != modeNormal {
		t.Errorf("mode = %v after Esc, want modeNormal", tv.mode)
	}
	if tv.filter != "bar" {
		t.Errorf("filter = %q after Esc, want bar", tv.filter)
	}
}

type fakeKiller struct {
	called []string
	err    error
}

func (f *fakeKiller) KillQuery(queryID string) error {
	f.called = append(f.called, queryID)
	return f.err
}

func TestTopViewKillConfirmY(t *testing.T) {
	killer := &fakeKiller{}
	tv := newTopView(nil, 80, 24)
	tv.killer = killer
	seedProcs(tv, 2)
	tv.cursor = 1

	pressKey(tv, 'K')
	if tv.mode != modeConfirmKill {
		t.Fatalf("mode = %v after K, want modeConfirmKill", tv.mode)
	}
	want := string(rune('a' + 1))
	if tv.killTarget != want {
		t.Errorf("killTarget = %q, want %q", tv.killTarget, want)
	}

	cmd, _ := pressKey(tv, 'y')
	if tv.mode != modeNormal {
		t.Errorf("mode = %v after y, want modeNormal", tv.mode)
	}
	// Drain the command to trigger the KillQuery call.
	if cmd != nil {
		cmd()
	}
	if len(killer.called) != 1 || killer.called[0] != want {
		t.Errorf("KillQuery called with %v", killer.called)
	}
}

func TestTopViewKillConfirmCancel(t *testing.T) {
	killer := &fakeKiller{}
	tv := newTopView(nil, 80, 24)
	tv.killer = killer
	seedProcs(tv, 2)
	tv.cursor = 0

	pressKey(tv, 'K')
	pressKey(tv, 'n')
	if tv.mode != modeNormal {
		t.Errorf("mode = %v after n, want modeNormal", tv.mode)
	}
	if len(killer.called) != 0 {
		t.Errorf("KillQuery should not fire on cancel, called = %v", killer.called)
	}
}

func TestTopViewKillEmptyList(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	pressKey(tv, 'K')
	if tv.mode != modeNormal {
		t.Errorf("K on empty list should stay in modeNormal, got %v", tv.mode)
	}
}

func TestTopViewDetailEnterExit(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	seedProcs(tv, 3)
	tv.cursor = 1

	pressKey(tv, tea.KeyEnter)
	if tv.mode != modeDetail {
		t.Errorf("mode = %v after Enter, want modeDetail", tv.mode)
	}
	pressKey(tv, tea.KeyEscape)
	if tv.mode != modeNormal {
		t.Errorf("mode = %v after Esc, want modeNormal", tv.mode)
	}
}

func TestTopViewCursorByQueryIDAcrossTicks(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	// Tick 1: three rows a,b,c; cursor on b.
	tv.snap = chtop.Snapshot{Processes: []chtop.Process{
		{QueryID: "a", Elapsed: 3},
		{QueryID: "b", Elapsed: 2},
		{QueryID: "c", Elapsed: 1},
	}}
	tv.cursor = 1
	tv.rememberCursorID()
	if tv.cursorID != "b" {
		t.Fatalf("cursorID = %q, want b", tv.cursorID)
	}

	// Tick 2: a finished, rows c,b; cursor should re-lock on b.
	tv.onSnapshot(chtop.Snapshot{Processes: []chtop.Process{
		{QueryID: "c", Elapsed: 1.5},
		{QueryID: "b", Elapsed: 2.5},
	}}, chtop.Rates{})
	visible := tv.visibleProcesses()
	if tv.cursor >= len(visible) || visible[tv.cursor].QueryID != "b" {
		t.Errorf("cursor %d does not point to b after re-lock (visible: %v)",
			tv.cursor, visible)
	}

	// Tick 3: b also finishes. Cursor should clamp to nearest previous index.
	tv.onSnapshot(chtop.Snapshot{Processes: []chtop.Process{
		{QueryID: "c", Elapsed: 2.0},
	}}, chtop.Rates{})
	if tv.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", tv.cursor)
	}
}

func TestTopViewResize(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	tv.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if tv.width != 120 || tv.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", tv.width, tv.height)
	}
}

func TestTopViewHeaderContainsVersion(t *testing.T) {
	tv := newTopView(nil, 120, 24)
	tv.onSnapshot(chtop.Snapshot{Header: chtop.Header{
		Uptime: time.Hour, Version: "24.8.3.5", ActiveQueries: 3,
		MemUsed: 1 << 30, MemTotal: 1 << 33,
		MergesRunning: 1, ReplicaMaxDelay: -1,
	}}, chtop.Rates{QueriesPerSec: 12.5})

	view := stripANSI(tv.renderHeader())
	if !strings.Contains(view, "24.8.3.5") {
		t.Errorf("header missing version: %q", view)
	}
	if !strings.Contains(view, "Q 3 running") {
		t.Errorf("header missing active-queries line: %q", view)
	}
	if strings.Contains(view, "replica-lag") {
		t.Errorf("header should elide replica line, got: %q", view)
	}
}

func TestTopViewHeaderShowsReplicaLagWhenPositive(t *testing.T) {
	tv := newTopView(nil, 120, 24)
	tv.onSnapshot(chtop.Snapshot{Header: chtop.Header{
		Version: "24.8", ReplicaMaxDelay: 0.42,
	}}, chtop.Rates{})
	view := stripANSI(tv.renderHeader())
	if !strings.Contains(view, "replica-lag") {
		t.Errorf("header missing replica-lag line: %q", view)
	}
}

func TestTopViewTableIncludesQueryAndUser(t *testing.T) {
	tv := newTopView(nil, 160, 24)
	tv.onSnapshot(chtop.Snapshot{At: time.Now(), Processes: []chtop.Process{
		{QueryID: "abc", User: "alice", Elapsed: 1.2, Query: "SELECT * FROM t"},
	}}, chtop.Rates{})
	view := stripANSI(tv.renderTable())
	if !strings.Contains(view, "alice") {
		t.Errorf("table missing user: %q", view)
	}
	if !strings.Contains(view, "SELECT * FROM t") {
		t.Errorf("table missing query: %q", view)
	}
}

func TestTopViewTableEmptyConnecting(t *testing.T) {
	tv := newTopView(nil, 160, 24)
	view := stripANSI(tv.renderTable())
	if !strings.Contains(view, "connecting") {
		t.Errorf("expected 'connecting…' placeholder, got: %q", view)
	}
}

func TestTopViewTableEmptyNoQueries(t *testing.T) {
	tv := newTopView(nil, 160, 24)
	tv.onSnapshot(chtop.Snapshot{At: time.Now()}, chtop.Rates{})
	view := stripANSI(tv.renderTable())
	if !strings.Contains(view, "no active queries") {
		t.Errorf("expected 'no active queries' placeholder, got: %q", view)
	}
}

func TestTopViewFooterShowsCurrentSortAndInterval(t *testing.T) {
	tv := newTopView(nil, 120, 24)
	tv.sortCol = sortMemory
	tv.interval = 2 * time.Second
	view := stripANSI(tv.renderFooter())
	if !strings.Contains(view, "sort:memory") {
		t.Errorf("footer missing 'sort:memory': %q", view)
	}
	if !strings.Contains(view, "2s") {
		t.Errorf("footer missing interval '2s': %q", view)
	}
}

func TestTopViewModalFilter(t *testing.T) {
	tv := newTopView(nil, 120, 24)
	tv.mode = modeFilter
	tv.filterBuf = "foo"
	view := stripANSI(tv.renderModal())
	if !strings.Contains(view, "/filter: foo") {
		t.Errorf("modal missing filter prompt: %q", view)
	}
}

func TestTopViewModalKillConfirm(t *testing.T) {
	tv := newTopView(nil, 120, 24)
	tv.mode = modeConfirmKill
	tv.killTarget = "abc12345-xxx"
	tv.onSnapshot(chtop.Snapshot{Processes: []chtop.Process{
		{QueryID: "abc12345-xxx", User: "alice"},
	}}, chtop.Rates{})
	view := stripANSI(tv.renderModal())
	if !strings.Contains(view, "[y/N]") {
		t.Errorf("modal missing kill confirm: %q", view)
	}
}

func TestTopViewTopLevelViewContainsAllBands(t *testing.T) {
	tv := newTopView(nil, 160, 24)
	tv.onSnapshot(chtop.Snapshot{At: time.Now(), Header: chtop.Header{Version: "24.8"},
		Processes: []chtop.Process{{QueryID: "x", User: "alice", Query: "SELECT 1"}}},
		chtop.Rates{})
	view := stripANSI(tv.View())
	for _, want := range []string{"24.8", "alice", "SELECT 1", "q quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("View missing %q: %q", want, view)
		}
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestTopViewOnSnapshotUpdatesState(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	snap := chtop.Snapshot{
		At:     time.Now(),
		Header: chtop.Header{Version: "24.8"},
		Processes: []chtop.Process{
			{QueryID: "a", User: "alice", Elapsed: 2.0},
		},
	}
	rates := chtop.Rates{QueriesPerSec: 12.5}
	tv.onSnapshot(snap, rates)

	if tv.snap.Header.Version != "24.8" {
		t.Errorf("snap not stored: %+v", tv.snap)
	}
	if tv.rates != rates {
		t.Errorf("rates not stored: %+v", tv.rates)
	}
	if tv.err != nil {
		t.Errorf("err should be cleared on successful snapshot, got %v", tv.err)
	}
}

func TestTopViewOnFetchErrorSetsErr(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	tv.onFetchErr(errSentinel("boom"))
	if tv.err == nil || tv.err.Error() != "boom" {
		t.Errorf("err = %v, want boom", tv.err)
	}
}

func TestNewTopViewDefaults(t *testing.T) {
	tv := newTopView(nil, 80, 24)
	if tv.interval != time.Second {
		t.Errorf("interval = %v, want 1s", tv.interval)
	}
	if tv.sortCol != sortElapsed {
		t.Errorf("sortCol = %v, want sortElapsed", tv.sortCol)
	}
	if tv.mode != modeNormal {
		t.Errorf("mode = %v, want modeNormal", tv.mode)
	}
	if tv.width != 80 || tv.height != 24 {
		t.Errorf("size = %dx%d, want 80x24", tv.width, tv.height)
	}
}
