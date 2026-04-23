package monitor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/chtop"
	"github.com/vahid-sohrabloo/chcli/internal/uitheme"
)

// mergesRefreshInterval is how often FetchMerges is re-run once the tab is
// at least once-activated.
const mergesRefreshInterval = 2 * time.Second

type mergesSnapshotMsg struct {
	merges []chtop.MergeRow
	muts   []chtop.MutationRow
}
type mergesErrMsg struct{ err error }
type mergesTickMsg time.Time

type mergesTab struct {
	q      chtop.ParamQuerier
	merges []chtop.MergeRow
	muts   []chtop.MutationRow
	err    error

	table         table.Model
	width, height int
}

// NewMergesTab builds the Merges tab; q may be nil in tests.
func NewMergesTab(q chtop.ParamQuerier) Tab {
	return &mergesTab{q: q}
}

func (m *mergesTab) Title() string        { return "Merges" }
func (m *mergesTab) HasActiveModal() bool { return false }
func (m *mergesTab) HelpKeys() []keyHint {
	return []keyHint{
		{"↑↓", "navigate"},
		{"PgUp/PgDn", "scroll"},
		{"r", "refresh"},
	}
}

func (m *mergesTab) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), m.tickCmd())
}

func (m *mergesTab) fetchCmd() tea.Cmd {
	if m.q == nil {
		return nil
	}
	q := m.q
	return func() tea.Msg {
		merges, muts, err := chtop.FetchMerges(context.Background(), q)
		if err != nil {
			return mergesErrMsg{err: err}
		}
		return mergesSnapshotMsg{merges: merges, muts: muts}
	}
}

func (m *mergesTab) tickCmd() tea.Cmd {
	return tea.Tick(mergesRefreshInterval, func(t time.Time) tea.Msg {
		return mergesTickMsg(t)
	})
}

func (m *mergesTab) Update(msg tea.Msg) tea.Cmd {
	switch v := msg.(type) {
	case mergesTickMsg:
		return tea.Batch(m.fetchCmd(), m.tickCmd())
	case mergesSnapshotMsg:
		m.onSnapshot(v.merges, v.muts)
		return nil
	case mergesErrMsg:
		m.err = v.err
		return nil
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		m.rebuildTable()
		return nil
	case tea.KeyPressMsg:
		return m.handleKey(v)
	}
	return nil
}

func (m *mergesTab) onSnapshot(merges []chtop.MergeRow, muts []chtop.MutationRow) {
	m.merges = merges
	m.muts = muts
	m.err = nil
	m.rebuildTable()
}

func (m *mergesTab) handleKey(kp tea.KeyPressMsg) tea.Cmd {
	switch kp.Code {
	case 'r':
		return m.fetchCmd()
	case 'g':
		m.table.GotoTop()
	case 'G':
		m.table.GotoBottom()
	default:
		// Forward ↑/↓/PgUp/PgDn etc. to the bubbles/table component.
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(kp)
		return cmd
	}
	return nil
}

// rebuildTable lays out both merges and mutations as rows in a single
// bubbles/table, with a "kind" column distinguishing the two.
func (m *mergesTab) rebuildTable() {
	// Column widths include bubbles/table default Padding(0, 1).
	const (
		colKind     = 12
		colProgress = 16
		colElapsed  = 10
		colParts    = 10
	)
	colDBTable := max((m.width-(colKind+colProgress+colElapsed+colParts+12))/2, 20)
	colDetail := max(m.width-(colKind+colDBTable+colProgress+colElapsed+colParts+14), 20)

	cols := []table.Column{
		{Title: "kind", Width: colKind},
		{Title: "db.table", Width: colDBTable},
		{Title: "progress", Width: colProgress},
		{Title: "elapsed", Width: colElapsed},
		{Title: "parts", Width: colParts},
		{Title: "detail", Width: colDetail},
	}

	rows := make([]table.Row, 0, len(m.merges)+len(m.muts))
	for _, mg := range m.merges {
		kind := "merge"
		if mg.IsMutation {
			kind = "merge·mut"
		}
		rows = append(rows, table.Row{
			kind,
			mg.Database + "." + mg.Table,
			mergeBar(mg.Progress) + fmt.Sprintf(" %3.0f%%", mg.Progress*100),
			fmt.Sprintf("%.1fs", mg.Elapsed),
			strconv.Itoa(mg.NumParts),
			mg.ResultPartName,
		})
	}
	for _, mu := range m.muts {
		detail := mu.Command
		if mu.LatestFailReason != "" {
			detail = "⚠ " + mu.LatestFailReason
		}
		rows = append(rows, table.Row{
			"mutation",
			mu.Database + "." + mu.Table,
			fmt.Sprintf("%d to do", mu.PartsToDo),
			mu.CreateTime,
			"—",
			detail,
		})
	}

	height := max(m.height-5, 5)
	width := max(m.width-2, 40)

	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(height),
		table.WithWidth(width),
		table.WithFocused(true),
		table.WithStyles(storageTableStyles()),
	)
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(max(len(rows)-1, 0))
	}
}

func (m *mergesTab) View(w, h int) string {
	if w != m.width || h != m.height {
		m.width, m.height = w, h
		m.rebuildTable()
	}
	theme := uitheme.Active
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))

	var sb strings.Builder

	if m.err != nil {
		sb.WriteString(errStyle.Render("  Error: "+m.err.Error()) + "\n\n")
	}

	summary := fmt.Sprintf("  %d merge(s), %d mutation(s)", len(m.merges), len(m.muts))
	sb.WriteString(muted.Render(summary))
	sb.WriteString("\n")

	if len(m.merges)+len(m.muts) == 0 {
		sb.WriteString(muted.Render("  no active merges or mutations"))
		return sb.String()
	}

	sb.WriteString(m.table.View())
	return sb.String()
}

func mergeBar(progress float64) string {
	const width = 10
	filled := min(max(int(progress*float64(width)), 0), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// humanCount formats a count as "1.2M" / "3.4k" / "42". Local copy so this
// package doesn't depend on internal/tui for it.
func humanCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return strconv.FormatUint(n, 10)
	}
}
