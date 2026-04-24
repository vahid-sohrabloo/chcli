package monitor

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
)

func TestChartsTabDefaults(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	if c.lookback != time.Hour {
		t.Errorf("lookback = %v, want 1h", c.lookback)
	}
	if c.bucket != time.Minute {
		t.Errorf("bucket = %v, want 1m", c.bucket)
	}
}

func TestChartsTabLookbackCycle(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	want := []time.Duration{6 * time.Hour, 24 * time.Hour, 15 * time.Minute, time.Hour}
	for i, w := range want {
		c.Update(tea.KeyPressMsg{Code: ']'})
		if c.lookback != w {
			t.Errorf("press %d: lookback = %v, want %v", i+1, c.lookback, w)
		}
	}
}

func TestChartsTabBucketCycle(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	want := []time.Duration{5 * time.Minute, 15 * time.Minute, 10 * time.Second, time.Minute}
	for i, w := range want {
		c.Update(tea.KeyPressMsg{Code: '}'})
		if c.bucket != w {
			t.Errorf("press %d: bucket = %v, want %v", i+1, c.bucket, w)
		}
	}
}

func TestChartsTabOnPanelsStoresPanels(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	c.onPanels([]chtop.Panel{{Title: "A"}, {Title: "B"}})
	if len(c.panels) != 2 {
		t.Errorf("panels = %d, want 2", len(c.panels))
	}
}

func TestChartsTabScrollClamps(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	c.onPanels([]chtop.Panel{{}, {}, {}, {}, {}, {}, {}, {}})
	for range 20 {
		c.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if c.scrollRow > 3 {
		t.Errorf("scrollRow = %d, want <= 3", c.scrollRow)
	}
	for range 20 {
		c.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	}
	if c.scrollRow != 0 {
		t.Errorf("scrollRow = %d, want 0", c.scrollRow)
	}
}

func TestChartsTabViewEmptyShowsConnecting(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	out := stripANSI(c.View(120, 24))
	if !contains(out, "loading") && !contains(out, "connecting") {
		t.Errorf("expected a loading placeholder, got:\n%s", out)
	}
}

func TestChartsTabViewBootErrorShown(t *testing.T) {
	c := NewChartsTab(nil).(*chartsTab)
	c.bootErr = chtop.ErrNoDashboards
	out := stripANSI(c.View(120, 24))
	if !contains(out, "system.dashboards") {
		t.Errorf("expected ErrNoDashboards message, got:\n%s", out)
	}
}
