package highlight

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

func init() {
	styles.Register(chroma.MustNewStyle("chcli-dark", chroma.StyleEntries{
		chroma.Background:          "bg:#1a1b26 #a9b1d6",
		chroma.Keyword:             "bold #7aa2f7",
		chroma.KeywordType:         "#e0af68",
		chroma.NameBuiltin:         "#ff9e64",
		chroma.NameFunction:        "#7dcfff",
		chroma.LiteralStringSingle: "#9ece6a",
		chroma.LiteralNumber:       "#ff9e64",
		chroma.Comment:             "italic #565f89",
		chroma.CommentSingle:       "italic #565f89",
		chroma.CommentMultiline:    "italic #565f89",
		chroma.Operator:            "#89ddff",
		chroma.Punctuation:         "#a9b1d6",
		chroma.Name:                "#c0caf5",
	}))
}

// AvailableThemes returns all registered chroma style names.
func AvailableThemes() []string {
	return styles.Names()
}
