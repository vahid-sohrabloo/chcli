package monitor

import (
	tea "charm.land/bubbletea/v2"
)

// TopModelAPI is the minimal interface the Processes tab needs from the
// underlying \top model. Defined as an interface here so this package
// doesn't have to import internal/tui (which would create a cycle).
type TopModelAPI interface {
	FetchCmd() tea.Cmd
	TickCmd() tea.Cmd
	Update(msg tea.Msg) (tea.Cmd, bool)
	View() string
	SetSize(w, h int)
	// InModal reports whether the \top model is currently inside a
	// filter / kill-confirm / detail modal. Used for modal-aware q/Esc
	// routing at the container level.
	InModal() bool
}

// processesTab wraps a \top model so it can live as a tab inside the monitor.
type processesTab struct {
	m TopModelAPI
}

// NewProcessesTab builds a Processes tab around an already-constructed
// \top model. The caller owns the inner model's lifetime.
func NewProcessesTab(m TopModelAPI) Tab {
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
	return p.m != nil && p.m.InModal()
}
