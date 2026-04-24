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

// detail fetch result messages — one per level.
type (
	storageDBDetailMsg struct {
		d   chtop.DatabaseDetail
		err error
	}
	storageTableDetailMsg struct {
		d   chtop.TableDetail
		err error
	}
	storagePartitionDetailMsg struct {
		d   chtop.PartitionDetail
		err error
	}
	storagePartDetailMsg struct {
		d   chtop.PartDetail
		err error
	}
)

type storageTab struct {
	q chtop.ParamQuerier

	dbs        []chtop.DBRow
	tables     []chtop.TableRow
	partitions []chtop.PartitionRow
	parts      []chtop.PartRow

	path     []string // e.g. ["events", "hits", "20260421"]
	loading  bool
	err      error
	sortBy   storageSort
	sortDesc bool // true = descending (default for size/rows)

	filter    string
	filterBuf string
	inFilter  bool

	// Disk-name filter (applies at every level — a table/partition
	// passes if any of its active parts lives on a matching disk).
	// Independent from the name filter so you can combine them. The
	// picker is modal UI for choosing from the disks actually in use at
	// the current level.
	diskFilter       string
	inDiskPicker     bool
	diskPickerOpts   []string // "(all disks)" + unique disks in use, lowercase
	diskPickerCursor int

	// Detail pane (opened with 'd' or Enter at leaf, or always-on in
	// split view). Exactly one of detailDB / detailTable / detailPartition
	// / detailPart is populated when inDetail is true. detailLoading is
	// true while the fetch is in flight; detailErr holds any fetch error.
	inDetail        bool
	detailLoading   bool
	detailErr       error
	detailLevel     storageLevel
	detailName      string
	detailDB        chtop.DatabaseDetail
	detailTable     chtop.TableDetail
	detailPartition chtop.PartitionDetail
	detailPart      chtop.PartDetail

	// Split view: list on the left, auto-refreshing detail on the right.
	// splitView is a toggle driven by `v`. When on, cursor moves fire a
	// detail fetch for the newly-highlighted row; stale results land but
	// are discarded by name comparison in the msg handlers.
	splitView bool

	// colOffset shifts the *data* columns left so levels with more cols
	// than fit the viewport (partitions: 9 cols, parts: 8) can be
	// scrolled sideways. The first (name) column stays pinned so the
	// user always knows which row is which.
	colOffset int

	table         table.Model
	visibleNames  []string // names of rows currently in the table, same order
	width, height int
}

// NewStorageTab builds the Storage tab; q may be nil in tests.
// Split view is on by default — the auto-refreshing detail pane is where
// most of the value lives, and narrow terminals can toggle it off with v.
func NewStorageTab(q chtop.ParamQuerier) Tab {
	return &storageTab{q: q, sortDesc: true, splitView: true}
}

func (s *storageTab) Title() string        { return "Storage" }
func (s *storageTab) HasActiveModal() bool { return s.inFilter || s.inDiskPicker || s.inDetail }
func (s *storageTab) HelpKeys() []keyHint {
	return []keyHint{
		{"↑↓", "navigate"},
		{"←/→", "scroll columns"},
		{"Enter", "drill in (leaf: details)"},
		{"Backspace", "up"},
		{"d", "details (full)"},
		{"D", "filter by disk (parts)"},
		{"v", "toggle split view"},
		{"/", "filter name"},
		{"s", "sort (size/rows/name)"},
		{"S", "reverse sort"},
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
		disk := s.diskFilter
		return func() tea.Msg {
			rows, err := chtop.FetchTables(context.Background(), q, path[0], disk)
			return storageTablesMsg{rows: rows, err: err}
		}
	case storageLevelPartitions:
		disk := s.diskFilter
		return func() tea.Msg {
			rows, err := chtop.FetchPartitions(context.Background(), q, path[0], path[1], disk)
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
		return s.refreshSplitDetail(true)
	case storageTablesMsg:
		s.loading, s.err, s.tables = false, v.err, v.rows
		s.rebuildTable()
		return s.refreshSplitDetail(true)
	case storagePartitionsMsg:
		s.loading, s.err, s.partitions = false, v.err, v.rows
		s.rebuildTable()
		return s.refreshSplitDetail(true)
	case storagePartsMsg:
		s.loading, s.err, s.parts = false, v.err, v.rows
		s.rebuildTable()
		return s.refreshSplitDetail(true)
	case storageDBDetailMsg:
		s.detailLoading, s.detailErr, s.detailDB = false, v.err, v.d
		return nil
	case storageTableDetailMsg:
		s.detailLoading, s.detailErr, s.detailTable = false, v.err, v.d
		return nil
	case storagePartitionDetailMsg:
		s.detailLoading, s.detailErr, s.detailPartition = false, v.err, v.d
		return nil
	case storagePartDetailMsg:
		s.detailLoading, s.detailErr, s.detailPart = false, v.err, v.d
		return nil
	case tea.WindowSizeMsg:
		s.width, s.height = v.Width, v.Height
		s.rebuildTable()
		return nil
	case tea.KeyPressMsg:
		cmd := s.handleKey(v)
		// When split view is on and we're not in a modal, the right pane
		// should follow the cursor. refreshSplitDetail is a no-op when the
		// cursor is on the same row, so it's safe to call after every key.
		if s.splitView && !s.inDetail && !s.inFilter {
			if rc := s.refreshSplitDetail(false); rc != nil {
				return tea.Batch(cmd, rc)
			}
		}
		return cmd
	}
	return nil
}

func (s *storageTab) handleKey(kp tea.KeyPressMsg) tea.Cmd {
	if s.inFilter {
		return s.handleFilterKey(kp)
	}
	if s.inDiskPicker {
		return s.handleDiskPickerKey(kp)
	}
	if s.inDetail {
		// Any of these exits the detail pane and releases the modal. The
		// tab's Esc/q stays captured here so the monitor container doesn't
		// close the whole monitor view.
		switch kp.Code {
		case tea.KeyEscape, 'q', tea.KeyEnter, tea.KeyBackspace, 'd':
			s.inDetail = false
			s.detailErr = nil
		}
		return nil
	}
	// Shifted-letter detection: under the Kitty Keyboard Protocol (used by
	// Warp, Kitty, Ghostty, and recent iTerm2 builds) Shift+s arrives as
	// Code='s' with ModShift set, not as Code='S'. Handle both so Shift-
	// bindings work on legacy and modern terminals alike.
	shift := kp.Mod&tea.ModShift != 0
	switch kp.Code {
	case tea.KeyEnter:
		// At the leaf (parts) Enter opens detail since there's nothing
		// left to drill into. At upper levels Enter drills as before.
		if s.level() == storageLevelParts {
			return s.openDetail()
		}
		return s.drillIn()
	case tea.KeyRight:
		s.scrollColumns(+1)
	case tea.KeyLeft:
		s.scrollColumns(-1)
	case 'd':
		// 'd' alone opens full-screen details; Shift+D (arrives as 'd'
		// with ModShift under Kitty-protocol terminals) opens the disk
		// picker.
		if shift {
			s.openDiskPicker()
			return nil
		}
		return s.openDetail()
	case 'D':
		s.openDiskPicker()
		return nil
	case 'v':
		s.splitView = !s.splitView
		s.rebuildTable()
		if s.splitView {
			return s.refreshSplitDetail(true)
		}
		return nil
	case tea.KeyBackspace:
		return s.drillOut()
	case 'g':
		if shift {
			s.table.GotoBottom()
		} else {
			s.table.GotoTop()
		}
	case 'G':
		s.table.GotoBottom()
	case '/':
		s.inFilter = true
		s.filterBuf = s.filter
	case 's':
		if shift {
			s.sortDesc = !s.sortDesc
		} else {
			s.sortBy = (s.sortBy + 1) % 3
			// Reset to the sort direction that matches user intuition for
			// the new column (size/rows descending, name ascending).
			s.sortDesc = s.sortBy != storageSortName
		}
		s.rebuildTable()
	case 'S':
		s.sortDesc = !s.sortDesc
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

// openDiskPicker collects the set of disks in use at the current level
// from the already-loaded rows, prepends "(all disks)" as the clear-
// filter option, and opens the picker modal. If no disk data is
// available (e.g. at the root, where DBRow doesn't carry disk info),
// the picker is not shown.
func (s *storageTab) openDiskPicker() {
	disks := s.disksInView()
	if len(disks) == 0 {
		return
	}
	s.diskPickerOpts = append([]string{""}, disks...) // "" slot = all disks
	s.diskPickerCursor = 0
	for i, d := range s.diskPickerOpts {
		if d == s.diskFilter {
			s.diskPickerCursor = i
			break
		}
	}
	s.inDiskPicker = true
}

// disksInView returns the unique disk names actually used by rows at
// the current level. Returns nil at the root level since DBRow has no
// disk aggregate (and filtering by disk at that level would need a
// separate server query).
func (s *storageTab) disksInView() []string {
	seen := map[string]struct{}{}
	add := func(ds []string) {
		for _, d := range ds {
			if d == "" {
				continue
			}
			seen[d] = struct{}{}
		}
	}
	switch s.level() {
	case storageLevelRoot:
		return nil
	case storageLevelTables:
		for _, r := range s.tables {
			add(r.Disks)
		}
	case storageLevelPartitions:
		for _, r := range s.partitions {
			add(r.Disks)
		}
	case storageLevelParts:
		for _, r := range s.parts {
			if r.DiskName != "" {
				seen[r.DiskName] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func (s *storageTab) handleDiskPickerKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.Code {
	case tea.KeyEscape, 'q':
		s.inDiskPicker = false
	case tea.KeyUp:
		if s.diskPickerCursor > 0 {
			s.diskPickerCursor--
		}
	case tea.KeyDown:
		if s.diskPickerCursor < len(s.diskPickerOpts)-1 {
			s.diskPickerCursor++
		}
	case tea.KeyEnter:
		if s.diskPickerCursor >= 0 && s.diskPickerCursor < len(s.diskPickerOpts) {
			s.diskFilter = s.diskPickerOpts[s.diskPickerCursor]
		}
		s.inDiskPicker = false
		// Tables/Partitions aggregate server-side, so the disk filter
		// requires a re-fetch for the sizes to be correct. Parts is
		// filtered client-side (each part has one disk) so rebuild is
		// enough there.
		if s.level() == storageLevelTables || s.level() == storageLevelPartitions {
			return s.fetchCurrentLevel()
		}
		s.rebuildTable()
		s.table.SetCursor(0)
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
	s.diskFilter = ""
	s.colOffset = 0
	return s.fetchCurrentLevel()
}

// scrollColumns shifts the column offset by delta, clamped to the range
// the current level supports. Drill in/out always resets offset to 0
// since the new level has a different column set.
func (s *storageTab) scrollColumns(delta int) {
	maxOffset := s.maxColOffset()
	next := min(max(s.colOffset+delta, 0), maxOffset)
	if next == s.colOffset {
		return
	}
	s.colOffset = next
	s.rebuildTable()
}

// maxColOffset returns the largest valid colOffset for the current
// level — always (data columns) − 1 so that at the max offset at least
// one data column remains visible alongside the pinned name column.
func (s *storageTab) maxColOffset() int {
	var total int
	switch s.level() {
	case storageLevelRoot:
		total = 6
	case storageLevelTables:
		total = 7
	case storageLevelPartitions:
		total = 9
	case storageLevelParts:
		total = 8
	}
	if total <= 2 {
		return 0
	}
	return total - 2
}

// refreshSplitDetail kicks off a detail fetch for the row under the cursor
// when the split view is on and the cursor has moved to a new row since the
// last fetch. force bypasses the "same row" short-circuit (used when the
// user first turns split view on). Returns nil when there's nothing to
// fetch — empty level, no querier, or cursor already matches.
func (s *storageTab) refreshSplitDetail(force bool) tea.Cmd {
	if s.q == nil || !s.splitView {
		return nil
	}
	i := s.table.Cursor()
	if i < 0 || i >= len(s.visibleNames) {
		return nil
	}
	name := s.visibleNames[i]
	if !force && name == s.detailName && s.detailLevel == s.level() {
		return nil
	}
	return s.fetchDetailFor(name)
}

// fetchDetailFor sets up the in-flight state for a detail fetch of `name`
// at the current level and returns the tea.Cmd that performs the query.
// Used by both split-view auto-refresh and the full-screen openDetail.
func (s *storageTab) fetchDetailFor(name string) tea.Cmd {
	s.detailLoading = true
	s.detailErr = nil
	s.detailLevel = s.level()
	s.detailName = name
	q := s.q
	path := append([]string(nil), s.path...)

	switch s.level() {
	case storageLevelRoot:
		return func() tea.Msg {
			d, err := chtop.FetchDatabaseDetail(context.Background(), q, name)
			return storageDBDetailMsg{d: d, err: err}
		}
	case storageLevelTables:
		return func() tea.Msg {
			d, err := chtop.FetchTableDetail(context.Background(), q, path[0], name)
			return storageTableDetailMsg{d: d, err: err}
		}
	case storageLevelPartitions:
		return func() tea.Msg {
			d, err := chtop.FetchPartitionDetail(context.Background(), q, path[0], path[1], name)
			return storagePartitionDetailMsg{d: d, err: err}
		}
	case storageLevelParts:
		return func() tea.Msg {
			d, err := chtop.FetchPartDetail(context.Background(), q, path[0], path[1], name)
			return storagePartDetailMsg{d: d, err: err}
		}
	}
	return nil
}

// openDetail opens the full-screen detail pane for the row under the
// cursor. The pane opens immediately in loading state so the user gets
// feedback before the network round-trip completes.
func (s *storageTab) openDetail() tea.Cmd {
	if s.q == nil {
		return nil
	}
	i := s.table.Cursor()
	if i < 0 || i >= len(s.visibleNames) {
		return nil
	}
	s.inDetail = true
	return s.fetchDetailFor(s.visibleNames[i])
}

func (s *storageTab) drillOut() tea.Cmd {
	if len(s.path) == 0 {
		return nil
	}
	s.path = s.path[:len(s.path)-1]
	s.filter = ""
	s.diskFilter = ""
	s.colOffset = 0
	return s.fetchCurrentLevel()
}

// rebuildTable applies the current filter + sort to the level's data and
// rebuilds the bubbles/table component's columns and rows. Called whenever
// data, filter, sort, or window size changes.
func (s *storageTab) rebuildTable() {
	cols, rows, names := s.buildColumnsAndRows()
	s.visibleNames = names

	// Apply horizontal scroll offset if the user has pressed ←/→. The
	// name column (index 0) stays pinned; offset slides the data cols.
	if s.colOffset > 0 && len(cols) > 2 {
		maxOff := s.maxColOffset()
		if s.colOffset > maxOff {
			s.colOffset = maxOff
		}
		slicedCols := make([]table.Column, 0, len(cols)-s.colOffset)
		slicedCols = append(slicedCols, cols[0])
		slicedCols = append(slicedCols, cols[1+s.colOffset:]...)
		cols = slicedCols
		for i := range rows {
			r := make(table.Row, 0, len(cols))
			r = append(r, rows[i][0])
			r = append(r, rows[i][1+s.colOffset:]...)
			rows[i] = r
		}
	}

	// Full width always — the detail block stacks below the table, not
	// beside it, so the column set never needs to shrink.
	width := max(s.width-2, 40)
	// Leave room for title + tab bar + breadcrumb + sort line + (optional
	// filter line) + blank. 7 rows is tight but fits the normal case; the
	// filter line only appears while it's open / active.
	height := max(s.height-7, 5)
	if s.splitView {
		// The split-bottom detail block is a fixed-height strip below the
		// table: one separator row + splitDetailLines summary rows.
		height = max(height-splitDetailLines-1, 5)
	}

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

// sortIndicators are the suffix appended to the active-sort column header.
// desc uses ▼, asc uses ▲ so direction is visible without opening help.
const (
	sortIndicatorDesc = " ▼"
	sortIndicatorAsc  = " ▲"
)

// buildColumnsAndRows produces the per-level table definition plus a
// parallel slice of item names (used by drillIn to look up the row under
// the cursor in the source data).
func (s *storageTab) buildColumnsAndRows() ([]table.Column, []table.Row, []string) {
	// Column widths chosen so the layout fits reasonably at 120 cols.
	// Widths include the bubbles/table default Padding(0, 1) on each cell.
	const (
		colSize    = 12
		colBar     = 14
		colPct     = 8
		colNumeric = 10
		colRatio   = 8
		colTime    = 22
	)

	// Figure out which column the sort indicator attaches to.
	sortBySize := s.sortBy == storageSortSize
	sortByRows := s.sortBy == storageSortRows
	sortByName := s.sortBy == storageSortName
	desc := s.sortDesc

	// fitName picks the smaller of "what the content actually needs"
	// (max name len + small padding) and "what the remaining layout
	// allows" — clamped to a sane min/max so the column doesn't vanish
	// on short names or balloon on very long ones.
	fitName := func(layoutMax int, names []string, label string) int {
		contentMax := lipgloss.Width(label) + 2 // header padding
		for _, n := range names {
			if w := lipgloss.Width(n); w+2 > contentMax {
				contentMax = w + 2
			}
		}
		const nameMin, nameCap = 16, 60
		return max(min(contentMax, layoutMax, nameCap), nameMin)
	}

	switch s.level() {
	case storageLevelRoot:
		names := s.dbNames()
		colName := fitName(max(s.width-(colSize+colBar+colPct+colNumeric+colNumeric+6), 20), names, "database")
		cols := []table.Column{
			{Title: sortTitle("database", sortByName, desc), Width: colName},
			{Title: sortTitle("size", sortBySize, desc), Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "tables", Width: colNumeric},
			{Title: sortTitle("rows", sortByRows, desc), Width: colNumeric},
		}
		return cols, s.buildDBRows(), names

	case storageLevelTables:
		names := s.tableNames()
		colName := fitName(max(s.width-(colSize+colBar+colPct+colNumeric+colNumeric+colRatio+6), 20), names, "table")
		cols := []table.Column{
			{Title: sortTitle("table", sortByName, desc), Width: colName},
			{Title: sortTitle("size", sortBySize, desc), Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "parts", Width: colNumeric},
			{Title: sortTitle("rows", sortByRows, desc), Width: colNumeric},
			{Title: "comp", Width: colRatio},
		}
		return cols, s.buildTableRows(), names

	case storageLevelPartitions:
		names := s.partitionNames()
		colName := fitName(max(s.width-(colSize+colBar+colPct+colNumeric+colNumeric+colRatio+colTime+colTime+6), 20), names, "partition")
		cols := []table.Column{
			{Title: sortTitle("partition", sortByName, desc), Width: colName},
			{Title: sortTitle("size", sortBySize, desc), Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "parts", Width: colNumeric},
			{Title: sortTitle("rows", sortByRows, desc), Width: colNumeric},
			{Title: "comp", Width: colRatio},
			{Title: "min_time", Width: colTime},
			{Title: "max_time", Width: colTime},
		}
		return cols, s.buildPartitionRows(), names

	case storageLevelParts:
		names := s.partNames()
		colName := fitName(max(s.width-(colSize+colBar+colPct+colNumeric+colNumeric+colRatio+colTime+6), 20), names, "part")
		cols := []table.Column{
			{Title: sortTitle("part", sortByName, desc), Width: colName},
			{Title: sortTitle("size", sortBySize, desc), Width: colSize},
			{Title: "", Width: colBar},
			{Title: "%", Width: colPct},
			{Title: "lvl", Width: colNumeric},
			{Title: sortTitle("rows", sortByRows, desc), Width: colNumeric},
			{Title: "comp", Width: colRatio},
			{Title: "disk·modified", Width: colTime},
		}
		return cols, s.buildPartRows(), names
	}
	return nil, nil, nil
}

func sortTitle(name string, active, desc bool) string {
	if !active {
		return name
	}
	if desc {
		return name + sortIndicatorDesc
	}
	return name + sortIndicatorAsc
}

// --- per-level row builders ---

func (s *storageTab) buildDBRows() []table.Row {
	items := s.sortDBs(s.filterDBs())
	totB := totalBytesDBs(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		share := ratio(r.Bytes, totB)
		rows[i] = table.Row{
			r.Name,
			humanBytesStorage(r.Bytes),
			storageBar(share),
			formatShare(r.Bytes, totB),
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
	totB := totalBytesTables(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		share := ratio(r.Bytes, totB)
		rows[i] = table.Row{
			r.Name,
			humanBytesStorage(r.Bytes),
			storageBar(share),
			formatShare(r.Bytes, totB),
			strconv.Itoa(r.Parts),
			humanCount(r.Rows),
			formatRatio(r.Ratio()),
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
	totB := totalBytesPartitions(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		share := ratio(r.Bytes, totB)
		rows[i] = table.Row{
			r.Name,
			humanBytesStorage(r.Bytes),
			storageBar(share),
			formatShare(r.Bytes, totB),
			strconv.Itoa(r.Parts),
			humanCount(r.Rows),
			formatRatio(r.Ratio()),
			shortTime(r.MinTime),
			shortTime(r.MaxTime),
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
	totB := totalBytesParts(items)
	rows := make([]table.Row, len(items))
	for i, r := range items {
		share := ratio(r.Bytes, totB)
		rows[i] = table.Row{
			r.Name,
			humanBytesStorage(r.Bytes),
			storageBar(share),
			formatShare(r.Bytes, totB),
			strconv.Itoa(r.Level),
			humanCount(r.Rows),
			formatRatio(r.Ratio()),
			r.DiskName + " · " + shortTime(r.ModificationTime),
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

// filterTables applies only the name filter; the disk filter is handled
// server-side by FetchTables so the aggregates (bytes/rows/…) reflect
// only the matching disk.
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

// filterPartitions applies only the name filter; server-side handles disk.
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
	if s.filter == "" && s.diskFilter == "" {
		return s.parts
	}
	needle := strings.ToLower(s.filter)
	disk := strings.ToLower(s.diskFilter)
	out := make([]chtop.PartRow, 0, len(s.parts))
	for _, r := range s.parts {
		if needle != "" && !strings.Contains(strings.ToLower(r.Name), needle) {
			continue
		}
		if disk != "" && !strings.Contains(strings.ToLower(r.DiskName), disk) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (s *storageTab) sortDBs(items []chtop.DBRow) []chtop.DBRow {
	sort.Slice(items, func(i, j int) bool {
		var less bool
		switch s.sortBy {
		case storageSortRows:
			less = items[i].Rows < items[j].Rows
		case storageSortName:
			less = items[i].Name < items[j].Name
		default:
			less = items[i].Bytes < items[j].Bytes
		}
		if s.sortDesc {
			return !less
		}
		return less
	})
	return items
}

func (s *storageTab) sortTables(items []chtop.TableRow) []chtop.TableRow {
	sort.Slice(items, func(i, j int) bool {
		var less bool
		switch s.sortBy {
		case storageSortRows:
			less = items[i].Rows < items[j].Rows
		case storageSortName:
			less = items[i].Name < items[j].Name
		default:
			less = items[i].Bytes < items[j].Bytes
		}
		if s.sortDesc {
			return !less
		}
		return less
	})
	return items
}

func (s *storageTab) sortPartitions(items []chtop.PartitionRow) []chtop.PartitionRow {
	sort.Slice(items, func(i, j int) bool {
		var less bool
		switch s.sortBy {
		case storageSortRows:
			less = items[i].Rows < items[j].Rows
		case storageSortName:
			less = items[i].Name < items[j].Name
		default:
			less = items[i].Bytes < items[j].Bytes
		}
		if s.sortDesc {
			return !less
		}
		return less
	})
	return items
}

func (s *storageTab) sortParts(items []chtop.PartRow) []chtop.PartRow {
	sort.Slice(items, func(i, j int) bool {
		var less bool
		switch s.sortBy {
		case storageSortRows:
			less = items[i].Rows < items[j].Rows
		case storageSortName:
			less = items[i].Name < items[j].Name
		default:
			less = items[i].Bytes < items[j].Bytes
		}
		if s.sortDesc {
			return !less
		}
		return less
	})
	return items
}

// levelTotals returns "N items · SIZE · ROWS rows · COMP×" for the header,
// computed over the filtered slice so it reflects what's on screen.
func (s *storageTab) levelTotals() string {
	var count int
	var size, rows, comp, uncomp uint64
	switch s.level() {
	case storageLevelRoot:
		items := s.filterDBs()
		count = len(items)
		for _, r := range items {
			size += r.Bytes
			rows += r.Rows
		}
		return fmt.Sprintf("%d databases · %s · %s rows",
			count, humanBytesStorage(size), humanCount(rows))

	case storageLevelTables:
		items := s.filterTables()
		count = len(items)
		for _, r := range items {
			size += r.Bytes
			rows += r.Rows
			comp += r.Compressed
			uncomp += r.Uncompressed
		}
		return fmt.Sprintf("%d tables · %s · %s rows · %s",
			count, humanBytesStorage(size), humanCount(rows),
			formatRatio(aggregateRatio(comp, uncomp)))

	case storageLevelPartitions:
		items := s.filterPartitions()
		count = len(items)
		for _, r := range items {
			size += r.Bytes
			rows += r.Rows
			comp += r.Compressed
			uncomp += r.Uncompressed
		}
		return fmt.Sprintf("%d partitions · %s · %s rows · %s",
			count, humanBytesStorage(size), humanCount(rows),
			formatRatio(aggregateRatio(comp, uncomp)))

	case storageLevelParts:
		items := s.filterParts()
		count = len(items)
		for _, r := range items {
			size += r.Bytes
			rows += r.Rows
			comp += r.DataCompressed
			uncomp += r.DataUncompressed
		}
		return fmt.Sprintf("%d parts · %s · %s rows · %s",
			count, humanBytesStorage(size), humanCount(rows),
			formatRatio(aggregateRatio(comp, uncomp)))
	}
	return ""
}

// aggregateRatio returns uncompressed/compressed weighted across all parts,
// which is more accurate than averaging individual ratios.
func aggregateRatio(compressed, uncompressed uint64) float64 {
	if compressed == 0 {
		return 0
	}
	return float64(uncompressed) / float64(compressed)
}

// totalBytes* sum the level's bytes for share-of-total sizing of both the
// bar and the "%" column.
func totalBytesDBs(items []chtop.DBRow) uint64 {
	var t uint64
	for _, r := range items {
		t += r.Bytes
	}
	return t
}

func totalBytesTables(items []chtop.TableRow) uint64 {
	var t uint64
	for _, r := range items {
		t += r.Bytes
	}
	return t
}

func totalBytesPartitions(items []chtop.PartitionRow) uint64 {
	var t uint64
	for _, r := range items {
		t += r.Bytes
	}
	return t
}

func totalBytesParts(items []chtop.PartRow) uint64 {
	var t uint64
	for _, r := range items {
		t += r.Bytes
	}
	return t
}

func ratio(b, m uint64) float64 {
	if m == 0 {
		return 0
	}
	return float64(b) / float64(m)
}

// formatShare renders a row's share of the level's total bytes, picking a
// precision that stays readable whether one row dominates (99.8%) or rows
// are spread evenly (0.4%).
func formatShare(b, total uint64) string {
	if total == 0 {
		return "—"
	}
	pct := float64(b) * 100 / float64(total)
	switch {
	case pct >= 10:
		return fmt.Sprintf("%.0f%%", pct)
	case pct >= 1:
		return fmt.Sprintf("%.1f%%", pct)
	default:
		return fmt.Sprintf("%.2f%%", pct)
	}
}

// Reset clears the drilldown + all cached levels so the next display
// re-fetches from L0. Called by the container when switching back to this
// tab (via the Resettable interface).
func (s *storageTab) Reset() tea.Cmd {
	s.path = nil
	s.filter = ""
	s.inFilter = false
	s.filterBuf = ""
	s.diskFilter = ""
	s.inDiskPicker = false
	s.diskPickerOpts = nil
	s.diskPickerCursor = 0
	s.colOffset = 0
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

	// Totals header + sort hint. Totals are computed over the currently
	// filtered slice so the header reflects what's actually on screen.
	dir := sortIndicatorDesc
	if !s.sortDesc {
		dir = sortIndicatorAsc
	}
	totals := s.levelTotals()
	header := "  " + totals + "    sort: " + s.sortBy.String() + dir +
		"  (s cycle · S reverse · / filter · D disk · r refresh)"
	sb.WriteString(muted.Render(header) + "\n")
	if s.inFilter {
		sb.WriteString(muted.Render("  /filter: "+s.filterBuf+"│") + "\n")
	} else if s.filter != "" {
		sb.WriteString(muted.Render("  filter: "+s.filter) + "\n")
	}
	if s.diskFilter != "" {
		sb.WriteString(muted.Render("  disk: "+s.diskFilter) + "\n")
	}

	if s.inDiskPicker {
		sb.WriteString(s.renderDiskPicker())
		return sb.String()
	}

	if s.inDetail {
		sb.WriteString(s.renderDetail())
		return sb.String()
	}

	if s.visibleCount() == 0 {
		sb.WriteString(muted.Render("  (empty)"))
		return sb.String()
	}

	if s.splitView {
		sb.WriteString(s.renderSplit())
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

// storageBar renders a width-10 unicode bar where pct is the row's share of
// the level's max (so the biggest row fills the whole bar). Empty cells are
// spaces, not shaded blocks, so the bar reads cleanly on a highlighted row
// where the table's Selected background would otherwise clash with ░.
func storageBar(pct float64) string {
	const width = 10
	filled := min(max(int(pct*float64(width)), 0), width)
	return strings.Repeat("█", filled) + strings.Repeat(" ", width-filled)
}

// formatRatio formats a compression ratio as "2.3×", "—" when unknown.
func formatRatio(r float64) string {
	if r <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f×", r)
}

// shortTime trims a ClickHouse-formatted DateTime ("2026-04-22 15:30:00") to
// just the date + HH:MM when width is tight. Returns "—" for empty or
// epoch-zero times.
func shortTime(t string) string {
	if t == "" || strings.HasPrefix(t, "1970-01-01") {
		return "—"
	}
	// "2026-04-22 15:30:00" → "2026-04-22 15:30"
	if len(t) >= 16 {
		return t[:16]
	}
	return t
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
