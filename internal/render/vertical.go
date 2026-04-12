package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderVertical renders query results in vertical format, one field per line.
// Each row is preceded by a "*** Row N ***" header. Labels are right-aligned
// and padded to the width of the longest column name.
func RenderVertical(columns []string, rows [][]string) string {
	rowHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#e0af68"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7aa2f7"))

	maxLabelWidth := 0
	for _, col := range columns {
		if len(col) > maxLabelWidth {
			maxLabelWidth = len(col)
		}
	}

	var sb strings.Builder

	for rowIdx, row := range rows {
		header := fmt.Sprintf("*************************** Row %d ***************************", rowIdx+1)
		sb.WriteString(rowHeaderStyle.Render(header))
		sb.WriteString("\n")

		for colIdx, col := range columns {
			value := ""
			if colIdx < len(row) {
				value = row[colIdx]
			}

			paddedLabel := fmt.Sprintf("%*s", maxLabelWidth, col)
			label := labelStyle.Render(paddedLabel + ":")
			fmt.Fprintf(&sb, "%s %s\n", label, value)
		}
	}

	return sb.String()
}
