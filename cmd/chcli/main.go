package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/vahid-sohrabloo/chcli/internal/config"
	"github.com/vahid-sohrabloo/chcli/internal/conn"
	"github.com/vahid-sohrabloo/chcli/internal/history"
	"github.com/vahid-sohrabloo/chcli/internal/schema"
	"github.com/vahid-sohrabloo/chcli/internal/tui"
	"github.com/vahid-sohrabloo/chcli/internal/tunnel"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	root := &cobra.Command{
		Use:     "chcli",
		Short:   "Modern interactive ClickHouse client for the terminal",
		Version: version + " (" + commit + ")",
		RunE:    run,
	}

	root.PersistentFlags().Bool("help", false, "help for chcli")
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		help, _ := cmd.Flags().GetBool("help")
		if help {
			cmd.Usage()
		}
	})

	root.Flags().StringP("host", "h", "localhost", "ClickHouse host")
	root.Flags().Uint16P("port", "p", 9000, "ClickHouse port")
	root.Flags().StringP("user", "u", "default", "ClickHouse user")
	root.Flags().StringP("database", "d", "default", "Database name")
	root.Flags().String("password", "", "Password (empty = prompt if --password flag present)")
	root.Flags().String("profile", "", "Named connection profile from config")
	root.Flags().String("compress", "", "Compression: lz4, zstd, or empty for none")

	root.Flags().String("ssh-host", "", "SSH bastion host for tunneling")
	root.Flags().String("ssh-user", "", "SSH username for bastion")
	root.Flags().String("ssh-key", "", "Path to SSH private key (default: ~/.ssh/id_rsa)")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 1. Load config.
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".chcli", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Build CLI flag overrides.
	var flags config.ConnectionConfig

	if cmd.Flags().Changed("host") {
		flags.Host, _ = cmd.Flags().GetString("host")
	}
	if cmd.Flags().Changed("port") {
		flags.Port, _ = cmd.Flags().GetUint16("port")
	}
	if cmd.Flags().Changed("user") {
		flags.User, _ = cmd.Flags().GetString("user")
	}
	if cmd.Flags().Changed("database") {
		flags.Database, _ = cmd.Flags().GetString("database")
	}
	if password, _ := cmd.Flags().GetString("password"); password != "" {
		flags.Password = password
	}
	if cmd.Flags().Changed("compress") {
		flags.Compress, _ = cmd.Flags().GetString("compress")
	}
	if cmd.Flags().Changed("ssh-host") {
		flags.SSHHost, _ = cmd.Flags().GetString("ssh-host")
	}
	if cmd.Flags().Changed("ssh-user") {
		flags.SSHUser, _ = cmd.Flags().GetString("ssh-user")
	}
	if cmd.Flags().Changed("ssh-key") {
		flags.SSHKey, _ = cmd.Flags().GetString("ssh-key")
	}

	profile, _ := cmd.Flags().GetString("profile")

	// 3. Resolve config.
	resolved := cfg.Resolve(profile, flags)

	// 3b. If SSH tunnel is configured, open it and redirect the connection.
	if resolved.SSHHost != "" {
		sshCfg := tunnel.SSHConfig{
			Host:     resolved.SSHHost,
			Port:     resolved.SSHPort,
			User:     resolved.SSHUser,
			KeyFile:  resolved.SSHKey,
			Password: resolved.SSHPassword,
		}
		tun, tunnelErr := tunnel.Open(sshCfg, resolved.Host, resolved.Port)
		if tunnelErr != nil {
			return fmt.Errorf("SSH tunnel: %w", tunnelErr)
		}
		defer tun.Close()

		fmt.Fprintf(os.Stderr, "SSH tunnel: %s → localhost:%d\n",
			tun.RemoteAddr(), tun.LocalPort())

		resolved.Host = "127.0.0.1"
		resolved.Port = uint16(tun.LocalPort())
	}

	// 4. Connect to ClickHouse.
	connStr := resolved.ConnectionString()
	c, err := conn.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer c.Close()

	// 5. Open history store.
	histPath := filepath.Join(home, ".chcli", "history.db")
	os.MkdirAll(filepath.Dir(histPath), 0755)
	hist, err := history.Open(histPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	// 6. Build schema cache (refresh happens async in TUI on separate connection).
	cache := schema.New(connStr)

	// 7. Launch TUI.
	model := tui.NewModel(c, cfg, cache, hist, resolved)
	p := tea.NewProgram(model)
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
