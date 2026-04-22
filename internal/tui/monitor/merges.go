package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	q      chtop.Querier
	merges []chtop.MergeRow
	muts   []chtop.MutationRow
	err    error
	cursor int
}

// NewMergesTab builds the Merges tab; q may be nil in tests.
func NewMergesTab(q chtop.Querier) Tab {
	return &mergesTab{q: q}
}

func (m *mergesTab) Title() string        { return "Merges" }
func (m *mergesTab) HasActiveModal() bool { return false }
func (m *mergesTab) HelpKeys() []keyHint {
	return []keyHint{
		{"↑↓", "navigate"},
		{"Enter", "detail"},
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
	case tea.KeyPressMsg:
		return m.handleKey(v)
	}
	return nil
}

func (m *mergesTab) onSnapshot(merges []chtop.MergeRow, muts []chtop.MutationRow) {
	m.merges = merges
	m.muts = muts
	m.err = nil
	total := len(merges) + len(muts)
	if m.cursor >= total {
		m.cursor = total - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *mergesTab) handleKey(kp tea.KeyPressMsg) tea.Cmd {
	total := len(m.merges) + len(m.muts)
	switch kp.Code {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < total-1 {
			m.cursor++
		}
	case 'r':
		return m.fetchCmd()
	}
	return nil
}

func (m *mergesTab) View(w, h int) string {
	theme := uitheme.Active
	section := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentBlue)).Bold(true)
	normal := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextPrimary))
	muted := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.TextMuted))
	selStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.BgDark)).
		Background(lipgloss.Color(theme.AccentBlue)).Bold(true)
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.AccentRed))

	var sb strings.Builder

	if m.err != nil {
		sb.WriteString(errStyle.Render("  Error: "+m.err.Error()) + "\n\n")
	}

	sb.WriteString(section.Render(fmt.Sprintf("  Merges (%d active)", len(m.merges))) + "\n")
	if len(m.merges) == 0 {
		sb.WriteString(muted.Render("    No active merges") + "\n")
	}
	for i, mg := range m.merges {
		line := fmt.Sprintf("  %s.%s   %5.1fs   %s %3.0f%%   parts %d   rows %s",
			mg.Database, mg.Table, mg.Elapsed,
			mergeBar(mg.Progress), mg.Progress*100,
			mg.NumParts, humanCount(mg.MergedRows))
		if i == m.cursor {
			sb.WriteString(selStyle.Render(line))
		} else {
			sb.WriteString(normal.Render(line))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(section.Render(fmt.Sprintf("  Mutations (%d active)", len(m.muts))) + "\n")
	if len(m.muts) == 0 {
		sb.WriteString(muted.Render("    No active mutations") + "\n")
	}
	offset := len(m.merges)
	for i, mu := range m.muts {
		line := fmt.Sprintf("  %s.%s   %s   parts %d   %s",
			mu.Database, mu.Table, mu.MutationID, mu.PartsToDo, mu.Command)
		if offset+i == m.cursor {
			sb.WriteString(selStyle.Render(line))
		} else {
			sb.WriteString(normal.Render(line))
		}
		sb.WriteString("\n")
		if mu.LatestFailReason != "" {
			sb.WriteString(errStyle.Render("        ↳ "+mu.LatestFailReason) + "\n")
		}
	}

	_ = w
	_ = h
	return sb.String()
}

func mergeBar(progress float64) string {
	const width = 10
	filled := min(max(int(progress*float64(width)), 0), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// humanCount formats a count as "1.2M" / "3.4k" / "42". Local copy so this
// package doesn't depend on internal/tui (avoids a cycle).
func humanCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}
