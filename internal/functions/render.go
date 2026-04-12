package functions

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdCodeRe = regexp.MustCompile("`([^`]+)`")
	mdBoldRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e0af68"))
	codeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	boldStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

// RenderMarkdown converts basic Markdown to styled terminal output.
func RenderMarkdown(md string) string {
	if strings.TrimSpace(md) == "" {
		return md
	}

	// Process line by line.
	lines := strings.Split(md, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		// Code blocks.
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			result = append(result, dimStyle.Render("────────────────────────────"))
			continue
		}
		if inCodeBlock {
			result = append(result, codeStyle.Render("  "+line))
			continue
		}

		// Headers.
		if text, ok := strings.CutPrefix(line, "# "); ok {
			result = append(result, "", titleStyle.Render(text), "")
			continue
		}
		if text, ok := strings.CutPrefix(line, "## "); ok {
			result = append(result, "", headerStyle.Render(text), "")
			continue
		}
		if text, ok := strings.CutPrefix(line, "### "); ok {
			result = append(result, headerStyle.Render(text))
			continue
		}

		// Inline formatting.
		line = renderInline(line)
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// renderInline applies inline Markdown formatting.
// Two passes: first strip to plain text, then apply terminal styles.
func renderInline(s string) string {
	// Pass 1: Strip Markdown to plain text.
	// [`Date`](/url) → Date
	s = mdLinkRe.ReplaceAllString(s, "$1")
	// **bold** → bold (mark for styling)
	s = mdBoldRe.ReplaceAllString(s, "\x01$1\x02")
	// `code` → code (mark for styling)
	s = mdCodeRe.ReplaceAllString(s, "\x03$1\x04")

	// Pass 2: Apply terminal styles to marked regions.
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\x01': // bold start
			end := strings.IndexByte(s[i+1:], '\x02')
			if end >= 0 {
				result.WriteString(boldStyle.Render(s[i+1 : i+1+end]))
				i += end + 1
			}
		case '\x03': // code start
			end := strings.IndexByte(s[i+1:], '\x04')
			if end >= 0 {
				result.WriteString(codeStyle.Render(s[i+1 : i+1+end]))
				i += end + 1
			}
		default:
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// FormatFunctionDoc renders a full function documentation page.
func FormatFunctionDoc(f *FunctionDef) string {
	if f == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("# " + f.Name + "\n\n")

	if f.Syntax != "" {
		sb.WriteString("```sql\n" + f.Syntax + "\n```\n\n")
	}

	if f.Description != "" {
		sb.WriteString(f.Description + "\n\n")
	}

	if f.Arguments != "" {
		sb.WriteString("## Arguments\n\n" + f.Arguments + "\n\n")
	}

	if f.ReturnedVal != "" {
		sb.WriteString("## Returns\n\n" + f.ReturnedVal + "\n\n")
	}

	if f.Categories != "" {
		sb.WriteString("**Categories:** " + f.Categories + "\n")
	}

	if f.IntroducedIn != "" {
		sb.WriteString("**Since:** ClickHouse " + f.IntroducedIn + "\n")
	}

	return RenderMarkdown(sb.String())
}
