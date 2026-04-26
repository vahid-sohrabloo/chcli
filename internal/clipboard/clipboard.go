// Package clipboard puts text on the user's system clipboard via the first
// available CLI tool (xclip, xsel, pbcopy, wl-copy). Used by both the
// metacmd \clip handler and the table viewer's row-yank.
package clipboard

import (
	"errors"
	"os/exec"
	"strings"
)

// ErrNoTool is returned when no clipboard helper is installed.
var ErrNoTool = errors.New("no clipboard tool found (install xclip, xsel, or wl-copy)")

// Copy writes text to the system clipboard using the first helper that succeeds.
// On success returns nil; on failure returns the underlying exec error or
// ErrNoTool if nothing on the system worked.
func Copy(text string) error {
	cmds := [][]string{
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
		{"wl-copy"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Stdin = strings.NewReader(text)
		if err := c.Run(); err == nil {
			return nil
		}
	}
	return ErrNoTool
}
