package metacmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/highlight"
)

// handleTheme lists available themes or switches the active theme.
//
// Usage:
//
//	\theme           — list available themes
//	\theme <name>    — switch to the named theme
func handleTheme(_ context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		// List all available themes.
		names := highlight.AvailableThemes()
		sort.Strings(names)
		var sb strings.Builder
		sb.WriteString("Available themes:\n")
		for _, n := range names {
			sb.WriteString("  " + n + "\n")
		}
		return &Result{Output: sb.String()}, nil
	}

	name := args[0]
	// Persist the chosen theme into the config so \settings reflects it.
	hctx.Config.Default.Theme = name
	return &Result{
		Output:   fmt.Sprintf("Theme set to %q.", name),
		SetTheme: name,
	}, nil
}
