package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	chlipgloss "github.com/charmbracelet/lipgloss"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

type chartSubview struct {
	result *conn.QueryResult
	kinds  []colKind

	xIdx         int
	yIdxs        []int
	chartType    chartType
	nonChartable bool
	parsed       parsedData

	picker        *pickerSubview
	width, height int
	isWarp        bool
}

func newChartSubview(res *conn.QueryResult, width, height int, isWarp bool) *chartSubview {
	c := &chartSubview{
		result: res,
		width:  width,
		height: height,
		isWarp: isWarp,
	}
	c.kinds = make([]colKind, len(res.Columns))
	for i, col := range res.Columns {
		c.kinds[i] = classifyColumn(col.Type)
	}
	c.xIdx, c.yIdxs, c.chartType = autoDetect(res)
	c.nonChartable = !isChartable(res)
	c.reparse()
	return c
}

// chartDims returns (w, h) for the chart area inside the viewer body.
// Returns (0, 0) if the area is too small to render.
// Reserves 1 line for the header, 2 lines for the footer, and 1 line
// for the legend that renders below the chart.
func (c *chartSubview) chartDims() (int, int) {
	w := c.width - 2
	h := c.height - 4
	if c.isWarp {
		h -= 4
	}
	if w < 30 || h < 5 {
		return 0, 0
	}
	return w, h
}

// seriesColors returns the fixed 6-color palette, sourced from ActiveTheme.
func seriesColors() []string {
	t := ActiveTheme
	return []string{
		t.AccentBlue, t.AccentGreen, t.AccentYellow,
		t.AccentMagenta, t.AccentCyan, t.AccentOrange,
	}
}

func paletteColor(idx int) string {
	colors := seriesColors()
	return colors[idx%len(colors)]
}

// reparse rebuilds parsedData from the current xIdx/yIdxs selection.
// Skips when the result is non-chartable or no Ys selected.
func (c *chartSubview) reparse() {
	c.parsed = parsedData{}
	if c.nonChartable || len(c.yIdxs) == 0 {
		return
	}
	c.parsed = parseRows(c.result, c.xIdx, c.yIdxs)
}

func (c *chartSubview) Update(msg tea.Msg, width, height int, isWarp bool) (bool, tea.Cmd) {
	c.width, c.height, c.isWarp = width, height, isWarp
	if c.picker != nil {
		committed, canceled := c.picker.Update(msg)
		switch {
		case committed:
			c.commitPicker()
		case canceled:
			c.cancelPicker()
		}
		return true, nil
	}
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch kp.Code {
		case 'x':
			c.openPicker()
			return true, nil
		case 'r':
			c.xIdx, c.yIdxs, c.chartType = autoDetect(c.result)
			c.reparse()
			return true, nil
		}
	}
	return true, nil
}

func (c *chartSubview) View() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.TextMuted))
	w, h := c.chartDims()
	if w == 0 {
		return muted.Render("  Terminal too narrow")
	}
	body := c.chartBody(w, h)
	if c.picker != nil {
		return overlayPicker(body, c.picker.View(w/2, kindLabel), w, h)
	}
	return body
}

func (c *chartSubview) chartBody(w, h int) string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.TextMuted))
	if c.nonChartable {
		return muted.Render("  No numeric columns to chart")
	}
	if len(c.yIdxs) == 0 {
		return muted.Render("  No Y columns selected — press x to pick columns")
	}
	switch c.chartType {
	case chartLine:
		return c.renderLine(w, h)
	default:
		return c.renderBar(w, h)
	}
}

func overlayPicker(body, picker string, w, h int) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, picker,
		lipgloss.WithWhitespaceChars(" "))
}

func kindLabel(k colKind) string {
	switch k {
	case kindNumeric:
		return "numeric"
	case kindTime:
		return "time"
	case kindCategory:
		return "category"
	default:
		return "other"
	}
}

func (c *chartSubview) renderLine(w, h int) string {
	if len(c.parsed.xTimes) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ActiveTheme.TextMuted)).
			Render("  0 plottable rows")
	}
	lc := timeserieslinechart.New(w, h)
	minX, maxX := c.parsed.xTimes[0], c.parsed.xTimes[0]
	minY, maxY := c.parsed.ys[0][0], c.parsed.ys[0][0]
	for _, t := range c.parsed.xTimes {
		if t.Before(minX) {
			minX = t
		}
		if t.After(maxX) {
			maxX = t
		}
	}
	for _, series := range c.parsed.ys {
		for _, v := range series {
			if v < minY {
				minY = v
			}
			if v > maxY {
				maxY = v
			}
		}
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}
	lc.SetTimeRange(minX, maxX)
	lc.SetYRange(minY, maxY)
	lc.SetViewTimeAndYRange(minX, maxX, minY, maxY)

	for si, series := range c.parsed.ys {
		name := c.result.Columns[c.yIdxs[si]].Name
		style := chlipgloss.NewStyle().Foreground(chlipgloss.Color(paletteColor(si)))
		lc.SetDataSetStyle(name, style)
		for i, t := range c.parsed.xTimes {
			lc.PushDataSet(name, timeserieslinechart.TimePoint{Time: t, Value: series[i]})
		}
	}
	names := make([]string, len(c.parsed.ys))
	for si := range c.parsed.ys {
		names[si] = c.result.Columns[c.yIdxs[si]].Name
	}
	lc.DrawBrailleDataSets(names)
	return lc.View() + "\n" + c.legend(names)
}

func (c *chartSubview) footer(tv *tableViewerModel) string {
	t := ActiveTheme
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Render("  │  ")
	secondary := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextSecondary))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))

	line1 := secondary.Render("  " + tv.footer)
	if !c.nonChartable && len(c.yIdxs) > 0 {
		xName := "(row index)"
		if c.xIdx >= 0 {
			xName = c.result.Columns[c.xIdx].Name
		}
		yNames := make([]string, len(c.yIdxs))
		for i, idx := range c.yIdxs {
			yNames[i] = c.result.Columns[idx].Name
		}
		line1 += sep + secondary.Render(
			"X: "+xName+"  Y: "+strings.Join(yNames, ", "))
	}
	if c.parsed.dropped > 0 {
		line1 += sep + muted.Render(
			fmt.Sprintf("%d null rows skipped", c.parsed.dropped))
	}

	var hintText string
	if c.pickerOpen() {
		hintText = "Tab X/Y   Space toggle   Enter apply   Esc cancel"
	} else {
		hintText = "x pick cols   r reset   c table   q/Esc exit"
	}
	line2 := muted.Render("  " + hintText)
	return line1 + "\n" + line2
}

func (c *chartSubview) pickerOpen() bool { return c.picker != nil }

func (c *chartSubview) openPicker() {
	names := make([]string, len(c.result.Columns))
	for i, col := range c.result.Columns {
		names[i] = col.Name
	}
	c.picker = newPickerSubview(c.kinds, names, c.xIdx, c.yIdxs)
}

func (c *chartSubview) closePicker()  { c.picker = nil }
func (c *chartSubview) cancelPicker() { c.picker = nil }

func (c *chartSubview) commitPicker() {
	if c.picker == nil {
		return
	}
	c.xIdx = c.picker.xIdx
	c.yIdxs = c.picker.sortedYIdxs()
	c.picker = nil
	c.reparse()
}

func (c *chartSubview) renderBar(w, h int) string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(ActiveTheme.TextMuted))

	labels := c.parsed.xCats
	if len(labels) == 0 {
		labels = make([]string, len(c.parsed.xNums))
		for i, v := range c.parsed.xNums {
			labels[i] = fmt.Sprintf("%g", v)
		}
	}
	if len(labels) == 0 {
		return muted.Render("  0 plottable rows")
	}

	bc := barchart.New(w, h)
	bc.SetHorizontal(true)

	bars := make([]barchart.BarData, 0, len(labels))
	for i, label := range labels {
		values := make([]barchart.BarValue, 0, len(c.yIdxs))
		for si, seriesIdx := range c.yIdxs {
			series := c.parsed.ys[si]
			values = append(values, barchart.BarValue{
				Name:  c.result.Columns[seriesIdx].Name,
				Value: series[i],
				Style: chlipgloss.NewStyle().Foreground(chlipgloss.Color(paletteColor(si))),
			})
		}
		bars = append(bars, barchart.BarData{
			Label:  truncate(label, 16),
			Values: values,
		})
	}
	bc.PushAll(bars)
	bc.Draw()
	names := make([]string, len(c.yIdxs))
	for si, idx := range c.yIdxs {
		names[si] = c.result.Columns[idx].Name
	}
	return bc.View() + "\n" + c.legend(names)
}

func (c *chartSubview) legend(names []string) string {
	var parts []string
	for i, name := range names {
		marker := lipgloss.NewStyle().Foreground(paletteColor2(i)).Render("●")
		parts = append(parts, marker+" "+name)
	}
	line := strings.Join(parts, "  ")
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ActiveTheme.TextSecondary)).
		Render("  " + line)
}

// paletteColor2 returns the palette color as a color.Color for use in
// non-ntcharts rendering (legend, footer). paletteColor itself returns a
// string usable by both v1 (ntcharts) and v2 style APIs via their Color ctors.
func paletteColor2(idx int) color.Color {
	colors := seriesColors()
	return lipgloss.Color(colors[idx%len(colors)])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return s[:n-1] + "…"
}
