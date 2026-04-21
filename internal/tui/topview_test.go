package tui

import (
	"testing"
	"time"
)

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
