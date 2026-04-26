package monitor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/uitheme"
)

const (
	chartsRefreshInterval = 30 * time.Second
	chartsFetchTimeout    = 10 * time.Second
)

var lookbackCycle = []time.Duration{15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
var bucketCycle = []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute}

type chartsBootstrapMsg struct {
	panels []chtop.Panel
	err    error
}
type chartsSeriesMsg struct {
	idx    int
	points []chtop.Point
	err    error
}
type chartsTickMsg time.Time

var _ Tab = (*chartsTab)(nil)

type chartsTab struct {
	q             chtop.ParamQuerier
	panels        []chtop.Panel
	series        [][]chtop.Point
	seriesErr     []error
	seriesFetched []bool // true once a fetch has completed for that panel
	bootErr       error

	lookback time.Duration
	bucket   time.Duration

	scrollRow     int
	width, height int
}

// NewChartsTab builds the Charts tab; q may be nil in tests.
func NewChartsTab(q chtop.ParamQuerier) Tab {
	return &chartsTab{q: q, lookback: time.Hour, bucket: time.Minute}
}

func (c *chartsTab) Title() string        { return "Charts" }
func (c *chartsTab) HasActiveModal() bool { return false }
func (c *chartsTab) HelpKeys() []keyHint {
	return []keyHint{
		{"[ / ]", "lookback shorter / longer"},
		{"{ / }", "bucket finer / coarser"},
		{"PgUp/PgDn", "scroll"},
		{"r", "refresh"},
	}
}

func (c *chartsTab) Init() tea.Cmd {
	return tea.Batch(c.bootstrapCmd(), c.tickCmd())
}

func (c *chartsTab) bootstrapCmd() tea.Cmd {
	if c.q == nil {
		return nil
	}
	q := c.q
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), chartsFetchTimeout)
		defer cancel()
		panels, err := chtop.LoadDashboard(ctx, q, "Overview")
		return chartsBootstrapMsg{panels: panels, err: err}
	}
}

func (c *chartsTab) refreshAllCmd() tea.Cmd {
	if c.q == nil || len(c.panels) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(c.panels))
	for i, p := range c.panels {
		// Skip panels whose SQL is broken on this server (e.g. the SQL
		// references columns/regexes that don't match anything). Without
		// this, the tab hammers the server with the same failing query
		// every chartsRefreshInterval. Manual refresh ('r') still retries.
		if c.seriesErr[i] != nil {
			continue
		}
		cmds = append(cmds, c.fetchSeriesCmd(i, p.SQL))
	}
	return tea.Batch(cmds...)
}

func (c *chartsTab) fetchSeriesCmd(idx int, sql string) tea.Cmd {
	q := c.q
	rounding := uint32(c.bucket.Seconds())
	lookback := uint32(c.lookback.Seconds())
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), chartsFetchTimeout)
		defer cancel()
		pts, err := chtop.FetchPanelSeries(ctx, q, sql, rounding, lookback)
		return chartsSeriesMsg{idx: idx, points: pts, err: err}
	}
}

func (c *chartsTab) tickCmd() tea.Cmd {
	return tea.Tick(chartsRefreshInterval, func(t time.Time) tea.Msg {
		return chartsTickMsg(t)
	})
}

func (c *chartsTab) onPanels(panels []chtop.Panel) {
	c.panels = panels
	c.series = make([][]chtop.Point, len(panels))
	c.seriesErr = make([]error, len(panels))
	c.seriesFetched = make([]bool, len(panels))
}

func (c *chartsTab) Update(msg tea.Msg) tea.Cmd {
	switch v := msg.(type) {
	case chartsTickMsg:
		// If bootstrap failed earlier, retry it on the next tick so the tab
		// recovers once the server is healthy again. Otherwise the tab would
		// stay stuck on the error until the user presses `r`.
		if c.bootErr != nil || len(c.panels) == 0 {
			return tea.Batch(c.bootstrapCmd(), c.tickCmd())
		}
		return tea.Batch(c.refreshAllCmd(), c.tickCmd())
	case chartsBootstrapMsg:
		c.bootErr = v.err
		if v.err == nil {
			c.onPanels(v.panels)
			return c.refreshAllCmd()
		}
		return nil
	case chartsSeriesMsg:
		if v.idx >= 0 && v.idx < len(c.series) {
			c.seriesFetched[v.idx] = true
			if v.err != nil {
				c.seriesErr[v.idx] = v.err
			} else {
				c.series[v.idx] = v.points
				c.seriesErr[v.idx] = nil
			}
		}
		return nil
	case tea.KeyPressMsg:
		return c.handleKey(v)
	case tea.WindowSizeMsg:
		c.width, c.height = v.Width, v.Height
		return nil
	}
	return nil
}

func (c *chartsTab) handleKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.Code {
	case '[':
		c.lookback = prevInCycle(lookbackCycle, c.lookback)
		return c.refreshAllCmd()
	case ']':
		c.lookback = nextInCycle(lookbackCycle, c.lookback)
		return c.refreshAllCmd()
	case '{':
		c.bucket = prevInCycle(bucketCycle, c.bucket)
		return c.refreshAllCmd()
	case '}':
		c.bucket = nextInCycle(bucketCycle, c.bucket)
		return c.refreshAllCmd()
	case 'r':
		// Manual refresh — clear previous errors so failed panels get
		// retried (refreshAllCmd skips panels with non-nil seriesErr).
		for i := range c.seriesErr {
			c.seriesErr[i] = nil
		}
		return tea.Batch(c.bootstrapCmd(), c.refreshAllCmd())
	case tea.KeyPgDown:
		maxRow := max((len(c.panels)+1)/2-1, 0)
		c.scrollRow = min(c.scrollRow+1, maxRow)
	case tea.KeyPgUp:
		c.scrollRow = max(c.scrollRow-1, 0)
	}
	return nil
}

func nextInCycle(cyc []time.Duration, cur time.Duration) time.Duration {
	for i, d := range cyc {
		if d == cur {
			return cyc[(i+1)%len(cyc)]
		}
	}
	return cyc[0]
}

func prevInCycle(cyc []time.Duration, cur time.Duration) time.Duration {
	for i, d := range cyc {
		if d == cur {
			return cyc[(i-1+len(cyc))%len(cyc)]
		}
	}
	return cyc[0]
}

func (c *chartsTab) View(w, h int) string {
	theme := uitheme.Active
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	muted := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted))
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentRed))

	if c.bootErr != nil {
		if errors.Is(c.bootErr, chtop.ErrNoDashboards) {
			return muted.Render("  system.dashboards has no rows — upgrade ClickHouse to ≥ 24.x for the built-in Overview.")
		}
		return errStyle.Render("  Error: " + c.bootErr.Error())
	}
	if len(c.panels) == 0 {
		return muted.Render("  loading panels…")
	}

	cellW := (w - 2) / 2 // 2 columns, 1-char margin between
	cellH := 10

	var sb strings.Builder
	startRow := c.scrollRow
	visibleRows := max((h-1)/cellH, 1)
	endPanel := min(2*(startRow+visibleRows), len(c.panels))

	for i := 2 * startRow; i < endPanel; i += 2 {
		leftIdx := i
		rightIdx := i + 1
		left := c.renderPanel(leftIdx, cellW, cellH, title, errStyle, muted)
		right := ""
		if rightIdx < len(c.panels) {
			right = c.renderPanel(rightIdx, cellW, cellH, title, errStyle, muted)
		}
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (c *chartsTab) renderPanel(idx, w, h int, title, errStyle, muted lipgloss.Style) string {
	p := c.panels[idx]
	if c.seriesErr[idx] != nil {
		header := title.Render("  " + p.Title)
		// Server-side panel errors (a missing column on an old version, an
		// arrayJoin of an empty Nothing-array on a fresh server, ...) are
		// per-panel and shouldn't dominate the grid. Strip wrapping prefixes
		// and trim to fit the panel cell.
		msg := shortenPanelErr(c.seriesErr[idx].Error(), max(w-4, 20))
		return header + "\n" + muted.Render("  (no data)") + "\n" + errStyle.Render("  "+msg)
	}
	pts := c.series[idx]
	if len(pts) == 0 {
		header := title.Render("  " + p.Title)
		// Distinguish "still in flight" from "fetched, empty result". Both
		// previously rendered as "(loading)" which made empty panels look
		// stuck.
		if !c.seriesFetched[idx] {
			return header + "\n" + muted.Render("  (loading)")
		}
		return header + "\n" + muted.Render("  (no data)")
	}

	lc := timeserieslinechart.New(max(w-2, 10), max(h-2, 3))
	for _, pt := range pts {
		lc.Push(timeserieslinechart.TimePoint{Time: pt.T, Value: pt.V})
	}
	lc.DrawBraille()
	last := pts[len(pts)-1].V
	headerWithVal := title.Render(fmt.Sprintf("  %-*s %10.2f", max(w-14, 1), p.Title, last))
	return headerWithVal + "\n" + lc.View()
}

// shortenPanelErr trims chconn's wrapping noise off a panel SQL error and
// truncates the remainder to width.
func shortenPanelErr(msg string, width int) string {
	// Drop wrapping prefixes added by chconn / chtop layers.
	for _, prefix := range []string{"panel fetch: ", "query: ", "DB::Exception "} {
		msg = strings.TrimPrefix(msg, prefix)
	}
	// Cut at first newline — only show the first line of multi-line errors.
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	runes := []rune(msg)
	if width >= 4 && len(runes) > width {
		return string(runes[:width-1]) + "…"
	}
	return msg
}
