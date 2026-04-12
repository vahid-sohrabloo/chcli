// Package render provides terminal table rendering utilities.
package render

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const minReadableColWidth = 8 // minimum chars per column to be readable

// RenderTable renders columns and rows as a pretty table that fits within maxWidth.
// If there are too many columns to display readably, auto-switches to vertical mode.
func RenderTable(columns []string, rows [][]string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 120
	}

	// If too many columns to fit readably, auto-switch to vertical mode.
	estOverhead := len(columns)*3 + 1
	estAvailable := maxWidth - estOverhead
	if len(columns) > 0 && estAvailable/len(columns) < minReadableColWidth {
		return RenderVertical(columns, rows)
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7aa2f7")).
		Padding(0, 1)

	evenRowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a9b1d6")).
		Padding(0, 1)

	oddRowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7"))

	// Auto-format numbers in cells.
	formattedRows := make([][]string, len(rows))
	for i, row := range rows {
		fr := make([]string, len(row))
		for j, cell := range row {
			fr[j] = autoFormatNumber(cell)
		}
		formattedRows[i] = fr
	}
	rows = formattedRows

	// Calculate natural column widths.
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Fit columns to maxWidth by shrinking wider ones.
	overhead := len(columns)*3 + 1 // padding + borders per column + left border
	availableWidth := max(maxWidth-overhead, len(columns)*3)
	colWidths = fitColumns(colWidths, availableWidth)

	// Truncate cell values to their assigned column width.
	truncatedRows := make([][]string, len(rows))
	for i, row := range rows {
		tr := make([]string, len(row))
		for j, cell := range row {
			maxW := 40 // fallback
			if j < len(colWidths) {
				maxW = colWidths[j]
			}
			if len(cell) > maxW {
				if maxW > 1 {
					tr[j] = cell[:maxW-1] + "…"
				} else {
					tr[j] = cell[:maxW]
				}
			} else {
				tr[j] = cell
			}
		}
		truncatedRows[i] = tr
	}

	styleFunc := func(row, col int) lipgloss.Style {
		switch {
		case row == table.HeaderRow:
			return headerStyle
		case row%2 == 0:
			return evenRowStyle
		default:
			return oddRowStyle
		}
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		StyleFunc(styleFunc).
		Headers(columns...).
		Width(maxWidth)

	for _, row := range truncatedRows {
		t.Row(row...)
	}

	return t.Render()
}

// autoFormatNumber adds thousand separators to numbers for readability.
// "1234567" → "1,234,567", "1234567.89" → "1,234,567.89"
// Non-numeric strings pass through unchanged.
func autoFormatNumber(s string) string {
	if s == "" || s == "NULL" || s == "true" || s == "false" {
		return s
	}

	// Check if it starts with optional minus, then digits.
	trimmed := s
	negative := false
	if len(trimmed) > 0 && trimmed[0] == '-' {
		negative = true
		trimmed = trimmed[1:]
	}

	// Split on decimal point.
	intPart := trimmed
	decPart := ""
	if dot := strings.IndexByte(trimmed, '.'); dot >= 0 {
		intPart = trimmed[:dot]
		decPart = trimmed[dot:] // includes the dot
	}

	// Validate integer part is all digits.
	if len(intPart) == 0 {
		return s
	}
	for _, c := range intPart {
		if c < '0' || c > '9' {
			return s // not a number
		}
	}

	// Validate decimal part if present.
	if decPart != "" {
		for i, c := range decPart {
			if i == 0 && c == '.' {
				continue
			}
			if c < '0' || c > '9' {
				return s // not a number
			}
		}
	}

	// Only format if >= 4 digits (1000+).
	if len(intPart) < 4 {
		return s
	}

	// Verify it's actually a valid number (not something like 0000123).
	if len(intPart) > 1 && intPart[0] == '0' {
		return s // leading zeros = not a plain number
	}

	// Add thousand separators.
	formatted := addThousandSeparators(intPart)

	result := formatted + decPart
	if negative {
		result = "-" + result
	}

	// Append human-readable suffix for large numbers (no decimal).
	if decPart == "" && len(intPart) >= 4 {
		n, err := strconv.ParseUint(intPart, 10, 64)
		if err == nil {
			dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
			result += dim.Render(" " + humanize(n))
		}
	}

	return result
}

// addThousandSeparators inserts commas every 3 digits from the right.
func addThousandSeparators(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	// Calculate number of commas.
	commas := (n - 1) / 3
	buf := make([]byte, n+commas)
	j := len(buf) - 1
	for i, count := n-1, 0; i >= 0; i-- {
		buf[j] = s[i]
		j--
		count++
		if count%3 == 0 && i > 0 {
			buf[j] = ','
			j--
		}
	}
	return string(buf)
}

// humanize converts a number to a short human-readable string.
func humanize(n uint64) string {
	switch {
	case n >= 1_000_000_000_000:
		return fmt.Sprintf("%.1fT", float64(n)/1e12)
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	default:
		return strconv.FormatUint(n, 10)
	}
}

// fitColumns distributes available width across columns.
// Columns that fit naturally keep their width.
// Excess is taken proportionally from the widest columns.
func fitColumns(widths []int, available int) []int {
	total := 0
	for _, w := range widths {
		total += w
	}

	if total <= available {
		return widths // everything fits
	}

	result := make([]int, len(widths))
	copy(result, widths)

	// Iteratively shrink the widest columns until we fit.
	for total > available {
		// Find the widest column.
		maxW := 0
		for _, w := range result {
			if w > maxW {
				maxW = w
			}
		}
		if maxW <= 3 {
			break // can't shrink further
		}

		// Shrink all columns at max width by 1.
		for i, w := range result {
			if w == maxW {
				result[i]--
				total--
				if total <= available {
					break
				}
			}
		}
	}

	// Ensure minimum width of 3 per column.
	for i, w := range result {
		if w < 3 {
			result[i] = 3
		}
	}

	return result
}
