package chtop

import (
	"context"
	"errors"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chconn/v3"
)

type fakeParamQuerier struct {
	*fakeQuerier
	dashboards *conn.QueryResult
	panel      *conn.QueryResult
	lastSQL    string
	lastParams []chconn.Parameter
	err        error
}

func (f *fakeParamQuerier) QueryAllWithParams(_ context.Context, sql string, params ...chconn.Parameter) (*conn.QueryResult, error) {
	f.lastSQL = sql
	f.lastParams = params
	if f.err != nil {
		return nil, f.err
	}
	if sql == sqlListDashboards {
		return f.dashboards, nil
	}
	return f.panel, nil
}

func TestLoadDashboardParsesPanels(t *testing.T) {
	q := &fakeParamQuerier{dashboards: &conn.QueryResult{
		Columns: make([]conn.ResultColumn, 3),
		Rows: [][]string{
			{"Overview", "Queries/sec", "SELECT toStartOfInterval(event_time, INTERVAL {rounding:UInt32} SECOND) AS t, avg(ProfileEvent_Query) AS v FROM system.metric_log GROUP BY t ORDER BY t"},
			{"Overview", "CPU", "SELECT 1"},
		},
	}}
	panels, err := LoadDashboard(context.Background(), q, "Overview")
	if err != nil {
		t.Fatalf("LoadDashboard: %v", err)
	}
	if len(panels) != 2 {
		t.Fatalf("panels = %d, want 2", len(panels))
	}
	if panels[0].Title != "Queries/sec" || panels[1].Title != "CPU" {
		t.Errorf("titles = %+v", panels)
	}
}

func TestLoadDashboardEmptyReturnsError(t *testing.T) {
	q := &fakeParamQuerier{dashboards: &conn.QueryResult{Columns: make([]conn.ResultColumn, 3)}}
	_, err := LoadDashboard(context.Background(), q, "Overview")
	if !errors.Is(err, ErrNoDashboards) {
		t.Errorf("err = %v, want ErrNoDashboards", err)
	}
}

func TestFetchPanelSeriesParsesPoints(t *testing.T) {
	q := &fakeParamQuerier{panel: &conn.QueryResult{
		Columns: []conn.ResultColumn{{Name: "t", Type: "DateTime"}, {Name: "v", Type: "Float64"}},
		Rows: [][]string{
			{"2026-04-22 12:00:00", "42"},
			{"2026-04-22 12:01:00", "44.5"},
		},
	}}
	pts, err := FetchPanelSeries(context.Background(), q, "SELECT 1", 60, 3600)
	if err != nil {
		t.Fatalf("FetchPanelSeries: %v", err)
	}
	if len(pts) != 2 || pts[1].V != 44.5 {
		t.Errorf("points = %+v", pts)
	}
}
