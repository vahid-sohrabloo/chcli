package monitor

import (
	tea "charm.land/bubbletea/v2"

	"github.com/vahid-sohrabloo/chcli/internal/tui"
)

// processesTab wraps internal/tui.topModel so it can live as a tab inside
// the monitor. The inner model is unmodified; the wrapper just satisfies
// the `tab` interface and routes calls through.
type processesTab struct {
	m *tui.TopModel
}

// NewProcessesTab builds a Processes tab around an already-constructed
// topModel. The caller owns the inner model's lifetime (so things like
// highlighter / Killer can be set before wrapping).
func NewProcessesTab(m *tui.TopModel) tab {
	return &processesTab{m: m}
}

func (p *processesTab) Init() tea.Cmd {
	if p.m == nil {
		return nil
	}
	return tea.Batch(p.m.FetchCmd(), p.m.TickCmd())
}

func (p *processesTab) Update(msg tea.Msg) tea.Cmd {
	if p.m == nil {
		return nil
	}
	cmd, _ := p.m.Update(msg)
	return cmd
}

func (p *processesTab) View(w, h int) string {
	if p.m == nil {
		return ""
	}
	p.m.SetSize(w, h)
	return p.m.View()
}

func (p *processesTab) Title() string { return "Processes" }

func (p *processesTab) HelpKeys() []keyHint {
	return []keyHint{
		{"↑↓", "navigate"},
		{"←→", "scroll columns"},
		{"Enter", "detail"},
		{"k", "kill"},
		{"/", "filter"},
		{"s", "sort"},
		{"d", "refresh"},
	}
}

func (p *processesTab) HasActiveModal() bool {
	return p.m != nil && p.m.Mode() != tui.ModeNormal
}
