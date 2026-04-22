package metacmd

import (
	"context"
	"fmt"
	"time"
)

// handleTop asks the outer Model to open the ClickHouse top alt-screen view.
// Optional first argument is an initial refresh interval like "500ms", "1s",
// "2s", or "5s" (anything time.ParseDuration accepts). Invalid durations
// return an error; omitting the argument uses the default (1s).
func handleTop(_ context.Context, _ *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return &Result{OpenTop: true}, nil
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("\\top: invalid interval %q: %w", args[0], err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("\\top: interval must be positive, got %s", d)
	}
	return &Result{OpenTop: true, TopInterval: d.String()}, nil
}
