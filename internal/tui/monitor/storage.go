package monitor

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

	path      []string // e.g. ["events", "hits", "20260421"]
	cursor    int
	loading   bool
	err       error
	sortBy    storageSort
	filter    string
	filterBuf string
	inFilter  bool
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
		s.resortCurrent()
		return nil
	case storageTablesMsg:
		s.loading, s.err, s.tables = false, v.err, v.rows
		s.resortCurrent()
		return nil
	case storagePartitionsMsg:
		s.loading, s.err, s.partitions = false, v.err, v.rows
		s.resortCurrent()
		return nil
	case storagePartsMsg:
		s.loading, s.err, s.parts = false, v.err, v.rows
		s.resortCurrent()
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
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
	case tea.KeyDown:
		if s.cursor < s.visibleCount()-1 {
			s.cursor++
		}
	case 'g':
		s.cursor = 0
	case 'G':
		s.cursor = s.visibleCount() - 1
	case tea.KeyEnter, tea.KeyRight:
		return s.drillIn()
	case tea.KeyBackspace, tea.KeyLeft:
		return s.drillOut()
	case '/':
		s.inFilter = true
		s.filterBuf = s.filter
	case 's':
		s.sortBy = (s.sortBy + 1) % 3
		s.resortCurrent()
	case 'r':
		return s.fetchCurrentLevel()
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
		s.cursor = 0
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
	switch s.level() {
	case storageLevelRoot:
		if s.cursor < len(s.dbs) {
			s.path = append(s.path, s.dbs[s.cursor].Name)
			s.cursor, s.filter = 0, ""
			return s.fetchCurrentLevel()
		}
	case storageLevelTables:
		if s.cursor < len(s.tables) {
			s.path = append(s.path, s.tables[s.cursor].Name)
			s.cursor, s.filter = 0, ""
			return s.fetchCurrentLevel()
		}
	case storageLevelPartitions:
		if s.cursor < len(s.partitions) {
			s.path = append(s.path, s.partitions[s.cursor].Name)
			s.cursor, s.filter = 0, ""
			return s.fetchCurrentLevel()
		}
	}
	return nil
}

func (s *storageTab) drillOut() tea.Cmd {
	if len(s.path) == 0 {
		return nil
	}
	s.path = s.path[:len(s.path)-1]
	s.cursor, s.filter = 0, ""
	return s.fetchCurrentLevel()
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

func (s *storageTab) resortCurrent() {
	less := func(iName, jName string, iBytes, jBytes, iRows, jRows uint64) bool {
		switch s.sortBy {
		case storageSortRows:
			return iRows > jRows
		case storageSortName:
			return iName < jName
		default:
			return iBytes > jBytes
		}
	}
	switch s.level() {
	case storageLevelRoot:
		sort.Slice(s.dbs, func(i, j int) bool {
			return less(s.dbs[i].Name, s.dbs[j].Name, s.dbs[i].Bytes, s.dbs[j].Bytes, s.dbs[i].Rows, s.dbs[j].Rows)
		})
	case storageLevelTables:
		sort.Slice(s.tables, func(i, j int) bool {
			return less(s.tables[i].Name, s.tables[j].Name, s.tables[i].Bytes, s.tables[j].Bytes, s.tables[i].Rows, s.tables[j].Rows)
		})
	case storageLevelPartitions:
		sort.Slice(s.partitions, func(i, j int) bool {
			return less(s.partitions[i].Name, s.partitions[j].Name, s.partitions[i].Bytes, s.partitions[j].Bytes, s.partitions[i].Rows, s.partitions[j].Rows)
		})
	case storageLevelParts:
		sort.Slice(s.parts, func(i, j int) bool {
			return less(s.parts[i].Name, s.parts[j].Name, s.parts[i].Bytes, s.parts[j].Bytes, s.parts[i].Rows, s.parts[j].Rows)
		})
	}
}

// Reset clears the drilldown + all cached levels so the next display
// re-fetches from L0. Called by the container when switching back to this
// tab (via the Resettable interface).
func (s *storageTab) Reset() tea.Cmd {
	s.path = nil
	s.cursor = 0
	s.filter = ""
	s.inFilter = false
	s.filterBuf = ""
	s.dbs = nil
	s.tables = nil
	s.partitions = nil
	s.parts = nil
	s.err = nil
	return s.fetchCurrentLevel()
}

func (s *storageTab) View(w, h int) string {
	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextPrimary))
	sel := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.BgDark)).
		Background(lipgloss.Color(theme.AccentBlue)).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))

	var sb strings.Builder

	// Breadcrumb
	crumb := "  Storage"
	for _, p := range s.path {
		crumb += " ▸ " + p
	}
	sb.WriteString(muted.Render(crumb))
	sb.WriteString("\n")

	if s.err != nil {
		sb.WriteString(errStyle.Render("  Error: " + s.err.Error()))
		return sb.String()
	}
	if s.loading && s.visibleCount() == 0 {
		sb.WriteString(muted.Render("  loading…"))
		return sb.String()
	}

	if s.inFilter {
		sb.WriteString(muted.Render("  /filter: "+s.filterBuf+"│") + "\n")
	} else if s.filter != "" {
		sb.WriteString(muted.Render("  filter: "+s.filter) + "\n")
	}

	rows := s.renderRows(w)
	for i, line := range rows {
		if i == s.cursor {
			sb.WriteString(sel.Render(line))
		} else {
			sb.WriteString(normal.Render(line))
		}
		sb.WriteString("\n")
	}
	_ = h
	return sb.String()
}

func (s *storageTab) renderRows(w int) []string {
	switch s.level() {
	case storageLevelRoot:
		return renderStorageRows(s.dbs, s.filter, func(r chtop.DBRow) (string, uint64, uint64) {
			return r.Name, r.Bytes, r.Rows
		}, func(r chtop.DBRow, bar string, pct float64) string {
			return fmt.Sprintf("  %9s  [%s]  %5.1f%%  %s   %d tables",
				humanBytesStorage(r.Bytes), bar, pct*100, r.Name, r.Tables)
		})
	case storageLevelTables:
		return renderStorageRows(s.tables, s.filter, func(r chtop.TableRow) (string, uint64, uint64) {
			return r.Name, r.Bytes, r.Rows
		}, func(r chtop.TableRow, bar string, pct float64) string {
			return fmt.Sprintf("  %9s  [%s]  %5.1f%%  %s   parts %d   rows %s",
				humanBytesStorage(r.Bytes), bar, pct*100, r.Name, r.Parts, humanCount(r.Rows))
		})
	case storageLevelPartitions:
		return renderStorageRows(s.partitions, s.filter, func(r chtop.PartitionRow) (string, uint64, uint64) {
			return r.Name, r.Bytes, r.Rows
		}, func(r chtop.PartitionRow, bar string, pct float64) string {
			return fmt.Sprintf("  %9s  [%s]  %5.1f%%  %s   parts %d",
				humanBytesStorage(r.Bytes), bar, pct*100, r.Name, r.Parts)
		})
	case storageLevelParts:
		return renderStorageRows(s.parts, s.filter, func(r chtop.PartRow) (string, uint64, uint64) {
			return r.Name, r.Bytes, r.Rows
		}, func(r chtop.PartRow, bar string, pct float64) string {
			return fmt.Sprintf("  %9s  [%s]  %5.1f%%  %s   rows %s   lvl %d",
				humanBytesStorage(r.Bytes), bar, pct*100, r.Name, humanCount(r.Rows), r.Level)
		})
	}
	_ = w
	return nil
}

// renderStorageRows is a generic renderer: filter + bar scaling + row
// assembly, delegating naming and formatting to callbacks.
func renderStorageRows[T any](
	items []T,
	filter string,
	key func(T) (name string, bytes, rows uint64),
	format func(T, string, float64) string,
) []string {
	if len(items) == 0 {
		return []string{"  (empty)"}
	}
	needle := strings.ToLower(filter)
	var kept []T
	for _, it := range items {
		name, _, _ := key(it)
		if filter == "" || strings.Contains(strings.ToLower(name), needle) {
			kept = append(kept, it)
		}
	}
	if len(kept) == 0 {
		return []string{"  (no matches)"}
	}
	var maxBytes uint64
	for _, it := range kept {
		_, b, _ := key(it)
		if b > maxBytes {
			maxBytes = b
		}
	}
	rows := make([]string, 0, len(kept))
	for _, it := range kept {
		_, b, _ := key(it)
		var pct float64
		if maxBytes > 0 {
			pct = float64(b) / float64(maxBytes)
		}
		bar := storageBar(pct)
		rows = append(rows, format(it, bar, pct))
	}
	return rows
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
