package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

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
