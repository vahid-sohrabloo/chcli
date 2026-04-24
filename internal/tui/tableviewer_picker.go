package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type pickerEdit int

const (
	pickerEditX pickerEdit = iota
	pickerEditY
)

type pickerSubview struct {
	kinds  []colKind
	names  []string
	cursor int
	xIdx   int
	ySet   map[int]bool
	edit   pickerEdit
}

func newPickerSubview(kinds []colKind, names []string, xIdx int, yIdxs []int) *pickerSubview {
	ySet := make(map[int]bool, len(yIdxs))
	for _, i := range yIdxs {
		ySet[i] = true
	}
	return &pickerSubview{
		kinds:  kinds,
		names:  names,
		cursor: 0,
		xIdx:   xIdx,
		ySet:   ySet,
		edit:   pickerEditX,
	}
}

// toggleY flips the Y membership of column i.
// Rejects the current X column and any non-numeric column.
func (p *pickerSubview) toggleY(i int) {
	if i == p.xIdx {
		return
	}
	if i < 0 || i >= len(p.kinds) || p.kinds[i] != kindNumeric {
		return
	}
	if p.ySet[i] {
		delete(p.ySet, i)
	} else {
		p.ySet[i] = true
	}
}

// setX assigns X. Removes the new X from the Y set if present
// (a column can't be both X and Y).
func (p *pickerSubview) setX(i int) {
	if i < 0 || i >= len(p.kinds) {
		return
	}
	p.xIdx = i
	delete(p.ySet, i)
}

// sortedYIdxs returns the Y selection as a stable-sorted slice.
func (p *pickerSubview) sortedYIdxs() []int {
	out := make([]int, 0, len(p.ySet))
	for i := range p.ySet {
		out = append(out, i)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Update returns (committed, canceled). If neither, the picker stays open.
func (p *pickerSubview) Update(msg tea.Msg) (committed, canceled bool) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, false
	}
	switch kp.Code {
	case tea.KeyEscape:
		return false, true
	case tea.KeyEnter:
		if len(p.ySet) == 0 {
			return false, false
		}
		return true, false
	case tea.KeyTab:
		if p.edit == pickerEditX {
			p.edit = pickerEditY
		} else {
			p.edit = pickerEditX
		}
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
	case tea.KeyDown:
		if p.cursor < len(p.names)-1 {
			p.cursor++
		}
	case tea.KeySpace:
		if p.edit == pickerEditX {
			p.setX(p.cursor)
		} else {
			p.toggleY(p.cursor)
		}
	}
	return false, false
}

func (p *pickerSubview) View(width int, kindLabels func(colKind) string) string {
	t := ActiveTheme
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.TextSecondary)).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextPrimary))
	cursorStyle := rowStyle.Background(lipgloss.Color(t.BgOverlay))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted))

	rows := make([]string, 0, len(p.names))
	for i, name := range p.names {
		xMark := "○"
		if i == p.xIdx {
			xMark = "●"
		}
		yMark := "  "
		if p.kinds[i] == kindNumeric {
			if p.ySet[i] {
				yMark = "▣ "
			} else {
				yMark = "□ "
			}
		}
		line := xMark + " " + yMark + padRight(name, 18) + muted.Render(kindLabels(p.kinds[i]))
		if i == p.cursor {
			line = cursorStyle.Render(line)
		} else {
			line = rowStyle.Render(line)
		}
		rows = append(rows, line)
	}
	var editLabel string
	if p.edit == pickerEditX {
		editLabel = "editing: X  "
	} else {
		editLabel = "editing: Y  "
	}
	hint := muted.Render(editLabel + "Tab: X/Y  Space: toggle  Enter: apply  Esc: cancel")
	body := lipgloss.JoinVertical(lipgloss.Left, rows...) + "\n" + hint
	return box.Render(body)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	pad := make([]byte, n-len(s))
	for i := range pad {
		pad[i] = ' '
	}
	return s + string(pad)
}
