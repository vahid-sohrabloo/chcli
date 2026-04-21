package metacmd

import "context"

// handleTop asks the outer Model to open the ClickHouse top alt-screen view.
// The topview (which needs the Conn for polling and killing) is constructed
// in the TUI layer when it sees OpenTop in the Result.
func handleTop(_ context.Context, _ *HandlerContext, _ []string) (*Result, error) {
	return &Result{OpenTop: true}, nil
}
