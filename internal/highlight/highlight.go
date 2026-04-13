package highlight

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter applies syntax highlighting to ClickHouse SQL strings.
type Highlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
}

// NewHighlighter creates a Highlighter using the named chroma style.
// Falls back to monokai if the name is not found.
func NewHighlighter(themeName string) *Highlighter {
	style := styles.Get(themeName)
	if style == nil {
		style = styles.Get("monokai")
	}
	if style == nil {
		style = styles.Fallback
	}
	return &Highlighter{
		style:     style,
		formatter: formatters.TTY16m,
	}
}

// Highlight tokenises input with the ClickHouse lexer and returns an
// ANSI-colored string. On error it returns the original input unchanged.
func (h *Highlighter) Highlight(input string) string {
	it, err := chroma.Coalesce(ClickHouseLexer).Tokenise(nil, input)
	if err != nil {
		return input
	}

	var buf bytes.Buffer
	if err := h.formatter.Format(&buf, h.style, it); err != nil {
		return input
	}
	return buf.String()
}

// SetTheme changes the active color theme by name.
// Falls back to monokai if the name is not found.
func (h *Highlighter) SetTheme(name string) {
	style := styles.Get(name)
	if style == nil {
		style = styles.Get("monokai")
	}
	if style == nil {
		style = styles.Fallback
	}
	h.style = style
}

// ThemeName returns the name of the active theme.
func (h *Highlighter) ThemeName() string {
	return h.style.Name
}
