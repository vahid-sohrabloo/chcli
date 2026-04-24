// Package metacmd: \top and \monitor handlers.
package metacmd

import (
	"context"
	"fmt"
	"time"
)

// handleTop opens the multi-tab monitor focused on the Processes tab.
// Optional first argument is a refresh interval (e.g. "5s", "500ms").
func handleTop(_ context.Context, _ *HandlerContext, args []string) (*Result, error) {
	res, err := parseMonitorArgs(args)
	if err != nil {
		return nil, err
	}
	res.ActiveTab = "processes"
	return res, nil
}

// handleMonitor opens the multi-tab monitor on its default tab.
// Optional first argument is a refresh interval (e.g. "5s", "500ms").
func handleMonitor(_ context.Context, _ *HandlerContext, args []string) (*Result, error) {
	return parseMonitorArgs(args)
}

func parseMonitorArgs(args []string) (*Result, error) {
	res := &Result{OpenMonitor: true, OpenTop: true}
	if len(args) == 0 {
		return res, nil
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", args[0], err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("interval must be positive, got %s", d)
	}
	res.TopInterval = d.String()
	return res, nil
}
