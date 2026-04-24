package monitor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/uitheme"
)

// splitDetailLines is the fixed height of the bottom detail strip
// (content rows only — the separator above is accounted for separately
// in rebuildTable). Every level renders exactly this many rows so the
// table's row count stays stable as the user drills.
const splitDetailLines = 5

// renderSplit stacks the full-width table on top with a grouped summary
// block below, separated by a dim horizontal rule. Used only when
// s.splitView is true; callers must check first.
func (s *storageTab) renderSplit() string {
	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))

	sep := muted.Render(strings.Repeat("─", max(s.width, 1)))

	var body string
	switch {
	case s.detailLoading && s.detailName == "":
		body = muted.Render("  (move cursor to load detail)")
	case s.detailLoading:
		body = muted.Render("  loading " + s.detailName + "…")
	case s.detailErr != nil:
		body = errStyle.Render("  Error: " + s.detailErr.Error())
	case s.detailLevel != s.level():
		body = muted.Render("  (cursor detail loading…)")
	default:
		body = s.renderSummaryLines()
	}
	return s.table.View() + "\n" + sep + "\n" + body
}

// renderSummaryLines dispatches to the per-level line builder and pads /
// clips the result to exactly splitDetailLines rows so the layout is
// stable as the cursor moves.
func (s *storageTab) renderSummaryLines() string {
	var lines []string
	switch s.detailLevel {
	case storageLevelRoot:
		lines = dbSummaryLines(s.detailDB)
	case storageLevelTables:
		lines = tableSummaryLines(s.detailTable)
	case storageLevelPartitions:
		lines = partitionSummaryLines(s.detailPartition)
	case storageLevelParts:
		lines = partSummaryLines(s.detailPart)
	}
	// Clip long lines to viewport width so a long path doesn't wrap and
	// push the table off-screen.
	maxW := max(s.width, 40)
	for i, ln := range lines {
		if lipgloss.Width(ln) > maxW {
			lines[i] = clipToWidth(ln, maxW)
		}
	}
	// Pad / truncate to splitDetailLines so table height stays constant.
	for len(lines) < splitDetailLines {
		lines = append(lines, "")
	}
	if len(lines) > splitDetailLines {
		lines = lines[:splitDetailLines]
	}
	return strings.Join(lines, "\n")
}

// clipToWidth truncates a rendered (ANSI-containing) string to w display
// columns. Lipgloss has no built-in equivalent, so we fall back to a
// rune-by-rune scan that respects display width.
func clipToWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	for len(s) > 0 && lipgloss.Width(s) > w {
		s = s[:len(s)-1]
	}
	return s
}

// --- summary-line builders ---
//
// Each builder returns up to splitDetailLines lines styled for the
// bottom pane: title line uses the accent-blue bold style, following
// lines prefix with a short dim label + dim separators between facts.
// Keeping the facts on one line per theme trades a bit of horizontal
// alignment for much higher information density than the one-KV-per-
// line layout used in the full-screen detail pane.

func summaryStyles() (title, label, dim lipgloss.Style) {
	theme := uitheme.Active
	title = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	label = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Bold(true)
	dim = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	return
}

func facts(parts ...string) string {
	_, _, dim := summaryStyles()
	sep := dim.Render(" · ")
	kept := parts[:0]
	for _, p := range parts {
		if p == "" {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, sep)
}

func summaryLine(lbl, body string) string {
	_, label, _ := summaryStyles()
	return "  " + label.Render(fmt.Sprintf("%-7s", lbl)) + body
}

func dbSummaryLines(d chtop.DatabaseDetail) []string {
	title, _, _ := summaryStyles()
	head := "  " + title.Render(d.Name)
	if d.Engine != "" {
		head += " " + dimify(fmt.Sprintf("(%s)", d.Engine))
	}
	if d.Comment != "" {
		head += "  " + dimify("\""+d.Comment+"\"")
	}
	return []string{
		head,
		summaryLine("Data", facts(
			intPlural(d.Tables, "table", "tables"),
			humanBytesStorage(d.Bytes)+" on disk",
			humanCount(d.Rows)+" rows",
			humanBytesStorage(d.Compressed)+" → "+humanBytesStorage(d.Uncompressed)+" ("+formatRatio(d.Ratio())+")",
		)),
		summaryLine("Time", facts(
			"first modify "+niceTime(d.FirstModify),
			"last "+niceTime(d.LastModify),
		)),
		summaryLine("Uuid", d.UUID),
	}
}

func tableSummaryLines(d chtop.TableDetail) []string {
	title, _, _ := summaryStyles()
	head := "  " + title.Render(d.Database+"."+d.Table)
	extras := []string{d.Engine, "created " + niceTime(d.CreatedAt)}
	if d.StoragePolicy != "" && d.StoragePolicy != "default" {
		extras = append(extras, "policy="+d.StoragePolicy)
	}
	head += " " + dimify("("+strings.Join(filterEmpty(extras), " · ")+")")

	keys := facts(
		ifVal("partition=", d.PartitionKey),
		ifVal("order=", d.SortingKey),
		ifVal("pk=", d.PrimaryKey),
		ifVal("sample=", d.SamplingKey),
	)
	if keys == "" {
		keys = dimify("—")
	}

	return []string{
		head,
		summaryLine("Keys", keys),
		summaryLine("Data", facts(
			humanCount(d.Rows)+" rows",
			humanBytesStorage(d.Bytes)+" on disk",
			humanBytesStorage(d.Compressed)+" → "+humanBytesStorage(d.Uncompressed)+" ("+formatRatio(d.Ratio())+")",
			"marks "+humanBytesStorage(d.Marks),
			"pk "+humanBytesStorage(d.PrimaryKeyMem),
		)),
		summaryLine("Parts", facts(
			fmt.Sprintf("%d active / %d inactive", d.ActiveParts, d.InactiveParts),
			"time "+niceTime(d.MinTime)+" → "+niceTime(d.MaxTime),
		)),
		summaryLine("Modify", facts(
			"oldest "+niceTime(d.OldestModified),
			"newest "+niceTime(d.NewestModified),
		)),
	}
}

func partitionSummaryLines(d chtop.PartitionDetail) []string {
	title, _, _ := summaryStyles()
	head := "  " + title.Render(d.Partition) +
		" " + dimify(fmt.Sprintf("(%d active / %d inactive)", d.ActiveParts, d.InactiveParts))
	return []string{
		head,
		summaryLine("Data", facts(
			humanCount(d.Rows)+" rows",
			humanBytesStorage(d.Bytes)+" on disk",
			humanBytesStorage(d.Compressed)+" → "+humanBytesStorage(d.Uncompressed)+" ("+formatRatio(d.Ratio())+")",
			"marks "+humanBytesStorage(d.Marks),
		)),
		summaryLine("Merge", facts(
			formatLevelCounts(d.LevelCounts),
			fmt.Sprintf("blocks %d..%d", d.MinBlockNumber, d.MaxBlockNumber),
		)),
		summaryLine("Time", niceTime(d.MinTime)+" → "+niceTime(d.MaxTime)),
		summaryLine("Modify", "oldest "+niceTime(d.OldestModified)+" · newest "+niceTime(d.NewestModified)),
	}
}

func partSummaryLines(d chtop.PartDetail) []string {
	title, _, _ := summaryStyles()
	active := "active"
	if !d.Active {
		active = "inactive"
	}
	head := "  " + title.Render(d.Name) + " " + dimify(fmt.Sprintf("(%s · L%d · %s · refcount %d)",
		d.PartType, d.Level, active, d.Refcount))
	return []string{
		head,
		summaryLine("Data", facts(
			humanCount(d.Rows)+" rows",
			humanBytesStorage(d.Bytes)+" on disk",
			humanBytesStorage(d.Compressed)+" → "+humanBytesStorage(d.Uncompressed)+" ("+formatRatio(d.Ratio())+")",
			"marks "+humanBytesStorage(d.Marks),
			"pk "+humanBytesStorage(d.PrimaryKeyMem),
		)),
		summaryLine("Time", niceTime(d.MinTime)+" → "+niceTime(d.MaxTime)+" · modified "+niceTime(d.ModificationTime)),
		summaryLine("Blocks", fmt.Sprintf("%d..%d", d.MinBlockNumber, d.MaxBlockNumber)),
		summaryLine("Store", d.DiskName+"  "+dimify(d.Path)),
	}
}

// --- small helpers ---

func dimify(s string) string {
	_, _, dim := summaryStyles()
	return dim.Render(s)
}

func ifVal(prefix, v string) string {
	if v == "" {
		return ""
	}
	return prefix + v
}

func filterEmpty(ss []string) []string {
	out := ss[:0]
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func intPlural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// renderDiskPicker draws the disk-selection modal in place of the
// table. "(all disks)" is the first entry and clears the filter; the
// rest are the disks the current level's rows live on, sorted.
func (s *storageTab) renderDiskPicker() string {
	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.BgDark)).
		Background(lipgloss.Color(theme.AccentBlue)).
		Bold(true)

	var b strings.Builder
	b.WriteString(title.Render("  Pick disk to filter by"))
	b.WriteString("   ")
	b.WriteString(muted.Render("↑↓ navigate · Enter select · Esc cancel"))
	b.WriteString("\n")
	b.WriteString(muted.Render(strings.Repeat("─", max(s.width, 1))))
	b.WriteString("\n")
	for i, opt := range s.diskPickerOpts {
		label := opt
		if label == "" {
			label = "(all disks — clear filter)"
		}
		line := "  " + label
		if i == s.diskPickerCursor {
			b.WriteString(selected.Render(fmt.Sprintf("%-*s", max(s.width-2, 4), line)))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderDetail draws the overlay for the row captured when the user
// pressed `d` / Enter-at-leaf. It is displayed *in place of* the table
// so all of the existing width/height budget is available.
func (s *storageTab) renderDetail() string {
	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))

	var sb strings.Builder

	// Title bar with right-aligned close hint.
	header := title.Render(fmt.Sprintf("  %s: %s", s.detailLevelLabel(), s.detailName))
	closeHint := muted.Render("d / Enter / Esc to close  ")
	width := max(s.width, 40)
	pad := max(width-lipgloss.Width(header)-lipgloss.Width(closeHint), 1)
	sb.WriteString(header)
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(closeHint)
	sb.WriteString("\n")
	sb.WriteString(muted.Render(strings.Repeat("─", width)))
	sb.WriteString("\n\n")

	if s.detailLoading {
		sb.WriteString(muted.Render("  loading…"))
		return sb.String()
	}
	if s.detailErr != nil {
		sb.WriteString(errStyle.Render("  Error: " + s.detailErr.Error()))
		return sb.String()
	}

	switch s.detailLevel {
	case storageLevelRoot:
		sb.WriteString(renderKV(dbDetailKV(s.detailDB)))
	case storageLevelTables:
		sb.WriteString(renderKV(tableDetailKV(s.detailTable)))
	case storageLevelPartitions:
		sb.WriteString(renderKV(partitionDetailKV(s.detailPartition)))
	case storageLevelParts:
		sb.WriteString(renderKV(partDetailKV(s.detailPart)))
	}
	return sb.String()
}

// detailLevelLabel returns the noun used in the detail pane's title bar.
func (s *storageTab) detailLevelLabel() string {
	switch s.detailLevel {
	case storageLevelRoot:
		return "Database"
	case storageLevelTables:
		return "Table"
	case storageLevelPartitions:
		return "Partition"
	case storageLevelParts:
		return "Part"
	}
	return "Detail"
}

// renderKV renders a right-aligned-label key/value block. Empty values
// are dropped so the pane doesn't show "partition_key : " for engines
// that don't have one.
func renderKV(pairs [][2]string) string {
	theme := uitheme.Active
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	value := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextPrimary))

	// Compute max label width across non-empty entries so keys align.
	maxLabel := 0
	filtered := make([][2]string, 0, len(pairs))
	for _, p := range pairs {
		if p[1] == "" {
			continue
		}
		if w := lipgloss.Width(p[0]); w > maxLabel {
			maxLabel = w
		}
		filtered = append(filtered, p)
	}

	var b strings.Builder
	for _, p := range filtered {
		b.WriteString("  ")
		b.WriteString(label.Render(fmt.Sprintf("%*s", maxLabel, p[0])))
		b.WriteString(" : ")
		b.WriteString(value.Render(p[1]))
		b.WriteString("\n")
	}
	return b.String()
}

func dbDetailKV(d chtop.DatabaseDetail) [][2]string {
	return [][2]string{
		{"database", d.Name},
		{"engine", d.Engine},
		{"uuid", d.UUID},
		{"comment", d.Comment},
		{"", ""},
		{"tables", intOrDash(d.Tables)},
		{"size", humanBytesStorage(d.Bytes)},
		{"rows", humanCount(d.Rows)},
		{"compressed", humanBytesStorage(d.Compressed)},
		{"uncompressed", humanBytesStorage(d.Uncompressed)},
		{"comp ratio", formatRatio(d.Ratio())},
		{"", ""},
		{"first modify", niceTime(d.FirstModify)},
		{"last modify", niceTime(d.LastModify)},
	}
}

func tableDetailKV(d chtop.TableDetail) [][2]string {
	return [][2]string{
		{"full name", d.Database + "." + d.Table},
		{"engine", d.Engine},
		{"uuid", d.UUID},
		{"comment", d.Comment},
		{"created at", niceTime(d.CreatedAt)},
		{"", ""},
		{"size", humanBytesStorage(d.Bytes)},
		{"rows", humanCount(d.Rows)},
		{"compressed", humanBytesStorage(d.Compressed)},
		{"uncompressed", humanBytesStorage(d.Uncompressed)},
		{"comp ratio", formatRatio(d.Ratio())},
		{"marks", humanBytesStorage(d.Marks)},
		{"primary key in mem", humanBytesStorage(d.PrimaryKeyMem)},
		{"", ""},
		{"partition key", d.PartitionKey},
		{"order by", d.SortingKey},
		{"primary key", d.PrimaryKey},
		{"sample key", d.SamplingKey},
		{"storage policy", d.StoragePolicy},
		{"metadata path", d.MetadataPath},
		{"", ""},
		{"parts (active/inactive)", fmt.Sprintf("%d / %d", d.ActiveParts, d.InactiveParts)},
		{"min time", niceTime(d.MinTime)},
		{"max time", niceTime(d.MaxTime)},
		{"oldest modify", niceTime(d.OldestModified)},
		{"newest modify", niceTime(d.NewestModified)},
	}
}

func partitionDetailKV(d chtop.PartitionDetail) [][2]string {
	return [][2]string{
		{"partition", d.Partition},
		{"parts (active/inactive)", fmt.Sprintf("%d / %d", d.ActiveParts, d.InactiveParts)},
		{"", ""},
		{"size", humanBytesStorage(d.Bytes)},
		{"rows", humanCount(d.Rows)},
		{"compressed", humanBytesStorage(d.Compressed)},
		{"uncompressed", humanBytesStorage(d.Uncompressed)},
		{"comp ratio", formatRatio(d.Ratio())},
		{"marks", humanBytesStorage(d.Marks)},
		{"", ""},
		{"level distribution", formatLevelCounts(d.LevelCounts)},
		{"block numbers", fmt.Sprintf("min %d · max %d", d.MinBlockNumber, d.MaxBlockNumber)},
		{"min time", niceTime(d.MinTime)},
		{"max time", niceTime(d.MaxTime)},
		{"oldest modify", niceTime(d.OldestModified)},
		{"newest modify", niceTime(d.NewestModified)},
	}
}

func partDetailKV(d chtop.PartDetail) [][2]string {
	active := "no"
	if d.Active {
		active = "yes"
	}
	return [][2]string{
		{"name", d.Name},
		{"type", d.PartType},
		{"level", strconv.Itoa(d.Level)},
		{"active", active},
		{"", ""},
		{"rows", humanCount(d.Rows)},
		{"size", humanBytesStorage(d.Bytes)},
		{"compressed", humanBytesStorage(d.Compressed)},
		{"uncompressed", humanBytesStorage(d.Uncompressed)},
		{"comp ratio", formatRatio(d.Ratio())},
		{"marks", humanBytesStorage(d.Marks)},
		{"primary key in mem", humanBytesStorage(d.PrimaryKeyMem)},
		{"", ""},
		{"block numbers", fmt.Sprintf("min %d · max %d", d.MinBlockNumber, d.MaxBlockNumber)},
		{"min time", niceTime(d.MinTime)},
		{"max time", niceTime(d.MaxTime)},
		{"modification time", niceTime(d.ModificationTime)},
		{"", ""},
		{"disk", d.DiskName},
		{"path", d.Path},
		{"refcount", strconv.Itoa(d.Refcount)},
	}
}

// formatLevelCounts turns {0:2, 1:5, 2:1} into "L0:2  L1:5  L2:1".
func formatLevelCounts(m map[int]int) string {
	if len(m) == 0 {
		return "—"
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("L%d:%d", k, m[k]))
	}
	return strings.Join(parts, "  ")
}

// niceTime trims a CH-formatted DateTime and replaces the epoch-zero
// sentinel with an em-dash.
func niceTime(t string) string {
	if t == "" || strings.HasPrefix(t, "1970-01-01") {
		return "—"
	}
	return t
}

// intOrDash renders 0 as "—" so empty-level detail panes don't show a
// misleading "0 tables".
func intOrDash(n int) string {
	if n == 0 {
		return "—"
	}
	return strconv.Itoa(n)
}
