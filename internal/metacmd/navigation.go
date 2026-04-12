package metacmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/vahid-sohrabloo/chcli/internal/config"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

// handleUse switches the active database.
func handleUse(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\use requires a database name")
	}
	db := args[0]

	if err := hctx.Conn.Exec(ctx, "USE "+db); err != nil {
		return nil, err
	}

	hctx.CurrentDB = db

	// Refresh schema cache in the background; ignore errors here — the caller
	// can always run \refresh explicitly.
	_ = hctx.Cache.Refresh(ctx)

	return &Result{Output: fmt.Sprintf("Database changed to %q.", db)}, nil
}

// handleConnect switches to a named connection profile.
func handleConnect(ctx context.Context, hctx *HandlerContext, args []string) (*Result, error) {
	if len(args) == 0 {
		return nil, errors.New("\\c requires a profile name")
	}
	profile := args[0]

	cc := hctx.Config.Resolve(profile, config.ConnectionConfig{})
	connStr := cc.ConnectionString()

	newConn, err := conn.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect to profile %q: %w", profile, err)
	}

	// Close old connection (best effort).
	_ = hctx.Conn.Close()

	hctx.Conn = newConn
	hctx.CurrentDB = cc.Database

	// Refresh schema cache; ignore errors.
	_ = hctx.Cache.Refresh(ctx)

	return &Result{Output: fmt.Sprintf("Connected using profile %q (database: %s).", profile, cc.Database)}, nil
}
