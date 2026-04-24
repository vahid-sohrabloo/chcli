package chtop

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chconn/v3"
)

// ParamQuerier is the subset of *conn.Conn the Charts and Storage tabs need:
// QueryAll plus the parameterized variant for SQL using {name:Type}.
type ParamQuerier interface {
	Querier
	QueryAllWithParams(ctx context.Context, sql string, params ...chconn.Parameter) (*conn.QueryResult, error)
}

// Panel is one row from system.dashboards: a named chart and its SQL.
type Panel struct {
	Dashboard string
	Title     string
	SQL       string
}

// Point is a single (time, value) sample in a panel series.
type Point struct {
	T time.Time
	V float64
}

// ErrNoDashboards is returned when system.dashboards has no rows for the
// requested dashboard (e.g. pre-24.x ClickHouse).
var ErrNoDashboards = errors.New("system.dashboards has no matching rows")

const sqlListDashboards = `
SELECT dashboard, title, query
FROM   system.dashboards
WHERE  dashboard = {dashboard:String}
ORDER  BY title
` + suppressLogging

// LoadDashboard fetches the panel definitions for a named dashboard. Returns
// ErrNoDashboards when the result set is empty.
func LoadDashboard(ctx context.Context, q ParamQuerier, name string) ([]Panel, error) {
	res, err := q.QueryAllWithParams(ctx, sqlListDashboards, conn.StringParam("dashboard", name))
	if err != nil {
		return nil, fmt.Errorf("list dashboards: %w", err)
	}
	if len(res.Rows) == 0 {
		return nil, ErrNoDashboards
	}
	out := make([]Panel, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 3 {
			continue
		}
		out = append(out, Panel{Dashboard: row[0], Title: row[1], SQL: row[2]})
	}
	return out, nil
}

// FetchPanelSeries runs a panel's SQL with the parameters that ClickHouse
// dashboards accept across versions:
//   - {rounding:UInt32}  — older dashboards
//   - {seconds:UInt32}   — older dashboards
//   - {from:DateTime}    — newer (≥ 26.x) dashboards
//   - {to:DateTime}      — newer (≥ 26.x) dashboards
//
// Supplying extra parameters a panel doesn't reference is harmless on the
// server side, so we always send all four and let each panel pick what it
// needs.
func FetchPanelSeries(ctx context.Context, q ParamQuerier, sql string, roundingSeconds, lookbackSeconds uint32) ([]Point, error) {
	to := time.Now().UTC()
	from := to.Add(-time.Duration(lookbackSeconds) * time.Second)
	const dtLayout = "2006-01-02 15:04:05"

	res, err := q.QueryAllWithParams(ctx, sql,
		conn.UintParam("rounding", roundingSeconds),
		conn.UintParam("seconds", lookbackSeconds),
		conn.StringParam("from", from.Format(dtLayout)),
		conn.StringParam("to", to.Format(dtLayout)))
	if err != nil {
		return nil, fmt.Errorf("panel fetch: %w", err)
	}
	out := make([]Point, 0, len(res.Rows))
	for i, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		// ClickHouse formats DateTime as "YYYY-MM-DD HH:MM:SS".
		tVal, err := time.Parse("2006-01-02 15:04:05", row[0])
		if err != nil {
			// Fall back to Unix timestamp if the column is numeric.
			if u, err2 := strconv.ParseInt(row[0], 10, 64); err2 == nil {
				tVal = time.Unix(u, 0)
			} else {
				return nil, fmt.Errorf("row %d time: %w", i, err)
			}
		}
		v, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			return nil, fmt.Errorf("row %d value: %w", i, err)
		}
		out = append(out, Point{T: tVal, V: v})
	}
	return out, nil
}
