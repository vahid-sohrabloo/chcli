package tui

import (
	"github.com/vahid-sohrabloo/chcli/internal/uitheme"
)

// UITheme is re-exported from internal/uitheme so existing callers keep
// working. New code should import internal/uitheme directly.
type UITheme = uitheme.UITheme

// ActiveTheme is a mutable alias to uitheme.Active. Assigning to it updates
// the underlying uitheme state.
var ActiveTheme = &uitheme.Active

// SetUITheme changes the active UI theme by name. Returns false if not found.
func SetUITheme(name string) bool { return uitheme.Set(name) }

// TableStyles returns themed styles for bubbles/table.
var TableStyles = uitheme.TableStyles
