package tui

import "strings"

// KeymapMode represents the keybinding mode for the input area.
type KeymapMode int

const (
	KeymapEmacs KeymapMode = iota
	KeymapVi
)

// KeymapFromString converts a string to a KeymapMode.
// Accepts "vi" or "vim" for Vi mode; defaults to Emacs mode.
func KeymapFromString(s string) KeymapMode {
	if s == "vi" || s == "vim" {
		return KeymapVi
	}
	return KeymapEmacs
}

// isSubmitInput returns true when the input is ready to be submitted.
// An input is ready when the trimmed text ends with ";" or "\G",
// or when it starts with "\" (meta-commands submit on Enter directly).
func isSubmitInput(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	// Meta-commands start with backslash — submit immediately.
	if strings.HasPrefix(trimmed, `\`) {
		return true
	}
	// SQL terminators.
	if strings.HasSuffix(trimmed, ";") || strings.HasSuffix(trimmed, `\G`) {
		return true
	}
	return false
}
