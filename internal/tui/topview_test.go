package tui

import (
	"testing"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

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
