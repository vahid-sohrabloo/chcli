package monitor

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/uitheme"
)

type storageLevel int

const (
	storageLevelRoot storageLevel = iota
	storageLevelTables
	storageLevelPartitions
	storageLevelParts
)

type storageSort int

const (
	storageSortSize storageSort = iota
	storageSortRows
	storageSortName
)

func (s storageSort) String() string {
	switch s {
	case storageSortRows:
		return "rows"
	case storageSortName:
		return "name"
	default:
		return "size"
	}
}

type storageDBsMsg struct {
	rows []chtop.DBRow
	err  error
}
type storageTablesMsg struct {
	rows []chtop.TableRow
	err  error
}
type storagePartitionsMsg struct {
	rows []chtop.PartitionRow
	err  error
}
type storagePartsMsg struct {
	rows []chtop.PartRow
	err  error
}

type storageTab struct {
	q chtop.ParamQuerier

	dbs        []chtop.DBRow
	tables     []chtop.TableRow
	partitions []chtop.PartitionRow
	parts      []chtop.PartRow

	path    []string // e.g. ["events", "hits", "20260421"]
	loading bool
	err     error
	sortBy  storageSort

	filter    string
	filterBuf string
	inFilter  bool

	table         table.Model
	visibleNames  []string // names of rows currently in the table, same order
	width, height int
}

// NewStorageTab builds the Storage tab; q may be nil in tests.
func NewStorageTab(q chtop.ParamQuerier) Tab {
	return &storageTab{q: q}
}

func (s *storageTab) Title() string        { return "Storage" }
func (s *storageTab) HasActiveModal() bool { return s.inFilter }
func (s *storageTab) HelpKeys() []keyHint {
	return []keyHint{
		{"↑↓", "navigate"},
		{"Enter/→", "drill in"},
		{"←/Backspace", "up"},
		{"/", "filter"},
		{"s", "sort (size/rows/name)"},
		{"r", "refresh level"},
	}
}

func (s *storageTab) level() storageLevel {
	return storageLevel(len(s.path))
}

func (s *storageTab) Init() tea.Cmd { return s.fetchCurrentLevel() }

func (s *storageTab) fetchCurrentLevel() tea.Cmd {
	if s.q == nil {
		return nil
	}
	q := s.q
	path := append([]string(nil), s.path...)
	s.loading = true
	switch s.level() {
	case storageLevelRoot:
		return func() tea.Msg {
			rows, err := chtop.FetchDatabases(context.Background(), q)
			return storageDBsMsg{rows: rows, err: err}
		}
	case storageLevelTables:
		return func() tea.Msg {
			rows, err := chtop.FetchTables(context.Background(), q, path[0])
			return storageTablesMsg{rows: rows, err: err}
		}
	case storageLevelPartitions:
		return func() tea.Msg {
			rows, err := chtop.FetchPartitions(context.Background(), q, path[0], path[1])
			return storagePartitionsMsg{rows: rows, err: err}
		}
	case storageLevelParts:
		return func() tea.Msg {
			rows, err := chtop.FetchParts(context.Background(), q, path[0], path[1], path[2])
			return storagePartsMsg{rows: rows, err: err}
		}
	}
	return nil
}

func (s *storageTab) Update(msg tea.Msg) tea.Cmd {
	switch v := msg.(type) {
	case storageDBsMsg:
		s.loading, s.err, s.dbs = false, v.err, v.rows
		s.rebuildTable()
		return nil
	case storageTablesMsg:
		s.loading, s.err, s.tables = false, v.err, v.rows
		s.rebuildTable()
		return nil
	case storagePartitionsMsg:
		s.loading, s.err, s.partitions = false, v.err, v.rows
		s.rebuildTable()
		return nil
	case storagePartsMsg:
		s.loading, s.err, s.parts = false, v.err, v.rows
		s.rebuildTable()
		return nil
	case tea.WindowSizeMsg:
		s.width, s.height = v.Width, v.Height
		s.rebuildTable()
		return nil
	case tea.KeyPressMsg:
		return s.handleKey(v)
	}
	return nil
}

func (s *storageTab) handleKey(kp tea.KeyPressMsg) tea.Cmd {
	if s.inFilter {
		return s.handleFilterKey(kp)
	}
	switch kp.Code {
	case tea.KeyEnter, tea.KeyRight:
		return s.drillIn()
	case tea.KeyBackspace, tea.KeyLeft:
		return s.drillOut()
	case 'g':
		s.table.GotoTop()
	case 'G':
		s.table.GotoBottom()
	case '/':
		s.inFilter = true
		s.filterBuf = s.filter
	case 's':
		s.sortBy = (s.sortBy + 1) % 3
		s.rebuildTable()
	case 'r':
		return s.fetchCurrentLevel()
	default:
		// Forward ↑/↓/PgUp/PgDn etc. to the bubbles/table component.
		var cmd tea.Cmd
		s.table, cmd = s.table.Update(kp)
		return cmd
	}
	return nil
}

func (s *storageTab) handleFilterKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.Code {
	case tea.KeyEscape:
		s.inFilter = false
		s.filterBuf = ""
	case tea.KeyEnter:
		s.filter = s.filterBuf
		s.inFilter = false
		s.filterBuf = ""
		s.rebuildTable()
		s.table.SetCursor(0)
	case tea.KeyBackspace, tea.KeyDelete:
		if len(s.filterBuf) > 0 {
			r := []rune(s.filterBuf)
			s.filterBuf = string(r[:len(r)-1])
		}
	default:
		if kp.Mod == 0 && kp.Code >= 0x20 && kp.Code != 0x7f {
			s.filterBuf += string(kp.Code)
		}
	}
	return nil
}

func (s *storageTab) drillIn() tea.Cmd {
	if s.level() == storageLevelParts {
		return nil // leaf
	}
	i := s.table.Cursor()
	if i < 0 || i >= len(s.visibleNames) {
		return nil
	}
	s.path = append(s.path, s.visibleNames[i])
	s.filter = ""
	return s.fetchCurrentLevel()
}

func (s *storageTab) drillOut() tea.Cmd {
	if len(s.path) == 0 {
		return nil
	}
	s.path = s.path[:len(s.path)-1]
	s.filter = ""
	return s.fetchCurrentLevel()
}

// rebuildTable applies the current filter + sort to the level's data and
// rebuilds the bubbles/table component's columns and rows. Called whenever
// data, filter, sort, or window size changes.
func (s *storageTab) rebuildTable() {
	cols, rows, names := s.buildColumnsAndRows()
	s.visibleNames = names

	width := max(s.width-2, 40)
	// Leave room for title row + tab bar + breadcrumb + filter + blank.
	height := max(s.height-6, 5)

	s.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(height),
		table.WithWidth(width),
		table.WithFocused(true),
		table.WithStyles(storageTableStyles()),
	)
	// Clamp cursor if the row count shrank.
	if s.table.Cursor() >= len(rows) {
		s.table.SetCursor(max(len(rows)-1, 0))
	}
}

// buildColumnsAndRows produces the per-level table definition plus a
// parallel slice of item names (used by drillIn to look up the row under
// the cursor in the source data).
func (s *storageTab) buildColumnsAndRows() ([]table.Column, []table.Row, []string) {
	// Column widths chosen so the layout fits reasonably at 100 cols.
	// Widths include the bubbles/table default Padding(0, 1) on each cell.
	const (
		colSize    = 12
		colBar     = 24
		colPct     = 8
		colNumeric = 10
	)
	colName := max(s.width-(colSize+colBar+colPct+colNumeric+colNumeric+6), 20)

	switch s.level() {
	case storageLevelRoot:
		cols := []table.Column{
			{Title: "size", Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "database", Width: colName},
			{Title: "tables", Width: colNumeric},
			{Title: "rows", Width: colNumeric},
		}
		return cols, s.buildDBRows(), s.dbNames()

	case storageLevelTables:
		cols := []table.Column{
			{Title: "size", Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "table", Width: colName},
			{Title: "parts", Width: colNumeric},
			{Title: "rows", Width: colNumeric},
		}
		return cols, s.buildTableRows(), s.tableNames()

	case storageLevelPartitions:
		cols := []table.Column{
			{Title: "size", Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "partition", Width: colName},
			{Title: "parts", Width: colNumeric},
			{Title: "rows", Width: colNumeric},
		}
		return cols, s.buildPartitionRows(), s.partitionNames()

	case storageLevelParts:
		cols := []table.Column{
			{Title: "size", Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "part", Width: colName},
			{Title: "lvl", Width: colNumeric},
			{Title: "rows", Width: colNumeric},
		}
		return cols, s.buildPartRows(), s.partNames()
	}
	return nil, nil, nil
}

// --- per-level row builders ---

func (s *storageTab) buildDBRows() []table.Row {
	items := s.sortDBs(s.filterDBs())
	maxB := maxBytesDBs(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		rows[i] = table.Row{
			humanBytesStorage(r.Bytes),
			storageBar(ratio(r.Bytes, maxB)),
			fmt.Sprintf("%.1f%%", ratio(r.Bytes, maxB)*100),
			r.Name,
			strconv.Itoa(r.Tables),
			humanCount(r.Rows),
		}
	}
	return rows
}

func (s *storageTab) dbNames() []string {
	items := s.sortDBs(s.filterDBs())
	names := make([]string, len(items))
	for i, r := range items {
		names[i] = r.Name
	}
	return names
}

func (s *storageTab) buildTableRows() []table.Row {
	items := s.sortTables(s.filterTables())
	maxB := maxBytesTables(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		rows[i] = table.Row{
			humanBytesStorage(r.Bytes),
			storageBar(ratio(r.Bytes, maxB)),
			fmt.Sprintf("%.1f%%", ratio(r.Bytes, maxB)*100),
			r.Name,
			strconv.Itoa(r.Parts),
			humanCount(r.Rows),
		}
	}
	return rows
}

func (s *storageTab) tableNames() []string {
	items := s.sortTables(s.filterTables())
	names := make([]string, len(items))
	for i, r := range items {
		names[i] = r.Name
	}
	return names
}

func (s *storageTab) buildPartitionRows() []table.Row {
	items := s.sortPartitions(s.filterPartitions())
	maxB := maxBytesPartitions(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		rows[i] = table.Row{
			humanBytesStorage(r.Bytes),
			storageBar(ratio(r.Bytes, maxB)),
			fmt.Sprintf("%.1f%%", ratio(r.Bytes, maxB)*100),
			r.Name,
			strconv.Itoa(r.Parts),
			humanCount(r.Rows),
		}
	}
	return rows
}

func (s *storageTab) partitionNames() []string {
	items := s.sortPartitions(s.filterPartitions())
	names := make([]string, len(items))
	for i, r := range items {
		names[i] = r.Name
	}
	return names
}

func (s *storageTab) buildPartRows() []table.Row {
	items := s.sortParts(s.filterParts())
	maxB := maxBytesParts(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		rows[i] = table.Row{
			humanBytesStorage(r.Bytes),
			storageBar(ratio(r.Bytes, maxB)),
			fmt.Sprintf("%.1f%%", ratio(r.Bytes, maxB)*100),
			r.Name,
			strconv.Itoa(r.Level),
			humanCount(r.Rows),
		}
	}
	return rows
}

func (s *storageTab) partNames() []string {
	items := s.sortParts(s.filterParts())
	names := make([]string, len(items))
	for i, r := range items {
		names[i] = r.Name
	}
	return names
}

// --- filter and sort helpers ---

func (s *storageTab) filterDBs() []chtop.DBRow {
	if s.filter == "" {
		return s.dbs
	}
	needle := strings.ToLower(s.filter)
	out := make([]chtop.DBRow, 0, len(s.dbs))
	for _, r := range s.dbs {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	return out
}

func (s *storageTab) filterTables() []chtop.TableRow {
	if s.filter == "" {
		return s.tables
	}
	needle := strings.ToLower(s.filter)
	out := make([]chtop.TableRow, 0, len(s.tables))
	for _, r := range s.tables {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	return out
}

func (s *storageTab) filterPartitions() []chtop.PartitionRow {
	if s.filter == "" {
		return s.partitions
	}
	needle := strings.ToLower(s.filter)
	out := make([]chtop.PartitionRow, 0, len(s.partitions))
	for _, r := range s.partitions {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	return out
}

func (s *storageTab) filterParts() []chtop.PartRow {
	if s.filter == "" {
		return s.parts
	}
	needle := strings.ToLower(s.filter)
	out := make([]chtop.PartRow, 0, len(s.parts))
	for _, r := range s.parts {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	return out
}

func (s *storageTab) sortDBs(items []chtop.DBRow) []chtop.DBRow {
	sort.Slice(items, func(i, j int) bool {
		switch s.sortBy {
		case storageSortRows:
			return items[i].Rows > items[j].Rows
		case storageSortName:
			return items[i].Name < items[j].Name
		default:
			return items[i].Bytes > items[j].Bytes
		}
	})
	return items
}

func (s *storageTab) sortTables(items []chtop.TableRow) []chtop.TableRow {
	sort.Slice(items, func(i, j int) bool {
		switch s.sortBy {
		case storageSortRows:
			return items[i].Rows > items[j].Rows
		case storageSortName:
			return items[i].Name < items[j].Name
		default:
			return items[i].Bytes > items[j].Bytes
		}
	})
	return items
}

func (s *storageTab) sortPartitions(items []chtop.PartitionRow) []chtop.PartitionRow {
	sort.Slice(items, func(i, j int) bool {
		switch s.sortBy {
		case storageSortRows:
			return items[i].Rows > items[j].Rows
		case storageSortName:
			return items[i].Name < items[j].Name
		default:
			return items[i].Bytes > items[j].Bytes
		}
	})
	return items
}

func (s *storageTab) sortParts(items []chtop.PartRow) []chtop.PartRow {
	sort.Slice(items, func(i, j int) bool {
		switch s.sortBy {
		case storageSortRows:
			return items[i].Rows > items[j].Rows
		case storageSortName:
			return items[i].Name < items[j].Name
		default:
			return items[i].Bytes > items[j].Bytes
		}
	})
	return items
}

func maxBytesDBs(items []chtop.DBRow) uint64 {
	var m uint64
	for _, r := range items {
		if r.Bytes > m {
			m = r.Bytes
		}
	}
	return m
}

func maxBytesTables(items []chtop.TableRow) uint64 {
	var m uint64
	for _, r := range items {
		if r.Bytes > m {
			m = r.Bytes
		}
	}
	return m
}

func maxBytesPartitions(items []chtop.PartitionRow) uint64 {
	var m uint64
	for _, r := range items {
		if r.Bytes > m {
			m = r.Bytes
		}
	}
	return m
}

func maxBytesParts(items []chtop.PartRow) uint64 {
	var m uint64
	for _, r := range items {
		if r.Bytes > m {
			m = r.Bytes
		}
	}
	return m
}

func ratio(b, m uint64) float64 {
	if m == 0 {
		return 0
	}
	return float64(b) / float64(m)
}

// Reset clears the drilldown + all cached levels so the next display
// re-fetches from L0. Called by the container when switching back to this
// tab (via the Resettable interface).
func (s *storageTab) Reset() tea.Cmd {
	s.path = nil
	s.filter = ""
	s.inFilter = false
	s.filterBuf = ""
	s.dbs = nil
	s.tables = nil
	s.partitions = nil
	s.parts = nil
	s.err = nil
	s.rebuildTable()
	return s.fetchCurrentLevel()
}

func (s *storageTab) View(w, h int) string {
	// Remember the latest size in case the container resizes us without a
	// WindowSizeMsg reaching here first (the monitor Resize is broadcast).
	if w != s.width || h != s.height {
		s.width, s.height = w, h
		s.rebuildTable()
	}

	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))

	var sb strings.Builder

	// Breadcrumb.
	var crumb strings.Builder
	crumb.WriteString("  Storage")
	for _, p := range s.path {
		crumb.WriteString(" ▸ ")
		crumb.WriteString(p)
	}
	sb.WriteString(muted.Render(crumb.String()))
	sb.WriteString("\n")

	if s.err != nil {
		sb.WriteString(errStyle.Render("  Error: " + s.err.Error()))
		return sb.String()
	}
	if s.loading && s.visibleCount() == 0 {
		sb.WriteString(muted.Render("  loading…"))
		return sb.String()
	}

	// Filter status line.
	if s.inFilter {
		sb.WriteString(muted.Render("  /filter: "+s.filterBuf+"│") + "\n")
	} else if s.filter != "" {
		sb.WriteString(muted.Render("  filter: "+s.filter) + "\n")
	}

	if s.visibleCount() == 0 {
		sb.WriteString(muted.Render("  (empty)"))
		return sb.String()
	}

	sb.WriteString(s.table.View())
	return sb.String()
}

func (s *storageTab) visibleCount() int {
	switch s.level() {
	case storageLevelRoot:
		return len(s.dbs)
	case storageLevelTables:
		return len(s.tables)
	case storageLevelPartitions:
		return len(s.partitions)
	case storageLevelParts:
		return len(s.parts)
	}
	return 0
}

// storageTableStyles returns bubbles/table styles that match the monitor's
// AccentBlue / TextMuted / BgDark palette so the Storage tab blends with the
// rest of the UI.
func storageTableStyles() table.Styles {
	t := uitheme.Active
	return table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.AccentBlue)).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Border)).
			BorderBottom(true),
		Cell: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextPrimary)).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.BgDark)).
			Background(lipgloss.Color(t.AccentBlue)).
			Bold(true).
			Padding(0, 1),
	}
}

func storageBar(pct float64) string {
	const width = 20
	filled := min(max(int(pct*float64(width)), 0), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// humanBytesStorage formats bytes as "8.3 GB" / "512 MB" / "48 kB".
func humanBytesStorage(n uint64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	v := float64(n) / k
	i := 0
	for v >= k && i < len(units)-1 {
		v /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
