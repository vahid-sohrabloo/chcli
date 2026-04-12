package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chcli/internal/render"
)

// Styles for printed results.
var (
	printQueryPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true)
	printQueryText   = lipgloss.NewStyle().Foreground(lipgloss.Color("#a9b1d6"))
	printFooter      = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	printError       = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	printSep         = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
	printRowsV       = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	printReadV       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	printMemV        = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	printCPUV        = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9e64"))
)

// FormatQueryResult renders a query result as a styled string for terminal output.
func FormatQueryResult(result *conn.QueryResult, query string, vertical bool, width int, progress *conn.Progress) string {
	var sb strings.Builder

	// Query echo.
	if query != "" {
		sb.WriteString(printQueryPrompt.Render("❯ ") + printQueryText.Render(query) + "\n")
	}

	// Table or vertical output.
	columns := make([]string, len(result.Columns))
	for i, c := range result.Columns {
		columns[i] = c.Name
	}

	if vertical {
		sb.WriteString(render.RenderVertical(columns, result.Rows))
	} else {
		sb.WriteString(render.RenderTable(columns, result.Rows, width))
	}
	sb.WriteString("\n")

	// Rich footer with metrics.
	footer := formatResultFooter(result.TotalRows, result.Elapsed, result.Truncated)

	sep := printSep.Render(" │ ")
	line := printFooter.Render(footer)

	if progress != nil && progress.ReadRows > 0 {
		line += sep + printRowsV.Render(formatNumber(progress.ReadRows)+" read")
		line += sep + printReadV.Render(formatBytes(progress.ReadBytes))
		if progress.MemoryUsage > 0 || progress.PeakMemory > 0 {
			mem := max(progress.PeakMemory, progress.MemoryUsage)
			line += sep + printMemV.Render(formatBytes(uint64(mem))+" mem")
		}
		cpuTotal := progress.CPUUser + progress.CPUSystem
		if cpuTotal > 0 {
			cpu := fmt.Sprintf("%.2fs CPU", float64(cpuTotal)/1e6)
			if progress.Threads > 0 {
				cpu += fmt.Sprintf(" (%dt)", progress.Threads)
			}
			line += sep + printCPUV.Render(cpu)
		}
	}

	sb.WriteString(line + "\n")
	return sb.String()
}

// FormatError renders an error as a styled string for terminal output.
func FormatError(err error) string {
	return printError.Render("Error: "+err.Error()) + "\n"
}

// FormatText renders text output (from meta-commands) for terminal output.
func FormatText(text string) string {
	return text + "\n"
}

// formatResultFooter returns a brief summary line: "N row(s) | elapsed[truncation note]".
func formatResultFooter(totalRows int, elapsed fmt.Stringer, truncated bool) string {
	rowWord := "rows"
	if totalRows == 1 {
		rowWord = "row"
	}
	s := fmt.Sprintf("%d %s | %s", totalRows, rowWord, elapsed)
	if truncated {
		s += fmt.Sprintf(" (showing first %d)", conn.MaxRows)
	}
	return s
}

// formatNumber and formatBytes are in model.go (same package).
