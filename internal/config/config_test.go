package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/config"
)

// TestLoadDefaults verifies that loading from a nonexistent path returns defaults.
func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load with nonexistent path returned error: %v", err)
	}
	d := cfg.Default
	if d.Host != "localhost" {
		t.Errorf("default Host = %q, want %q", d.Host, "localhost")
	}
	if d.Port != 9000 {
		t.Errorf("default Port = %d, want 9000", d.Port)
	}
	if d.User != "default" {
		t.Errorf("default User = %q, want %q", d.User, "default")
	}
	if d.Database != "default" {
		t.Errorf("default Database = %q, want %q", d.Database, "default")
	}
	if d.Keymap != "emacs" {
		t.Errorf("default Keymap = %q, want %q", d.Keymap, "emacs")
	}
	if d.Theme != "monokai" {
		t.Errorf("default Theme = %q, want %q", d.Theme, "monokai")
	}
	if d.Pager != "builtin" {
		t.Errorf("default Pager = %q, want %q", d.Pager, "builtin")
	}
}

// TestLoadFromFile verifies that a TOML file with [default], [profiles.staging], and [snippets] is parsed correctly.
func TestLoadFromFile(t *testing.T) {
	content := `
[default]
host     = "prod.example.com"
port     = 9440
user     = "admin"
password = "secret"
database = "analytics"
keymap   = "vi"
theme    = "dracula"
pager    = "less"
editor   = "nvim"
tls      = true

[profiles.staging]
host     = "staging.example.com"
port     = 9000
user     = "staging_user"
database = "staging_db"

[snippets]
top_tables = "SELECT database, name FROM system.tables ORDER BY total_bytes DESC LIMIT 10"
`

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Validate [default]
	d := cfg.Default
	if d.Host != "prod.example.com" {
		t.Errorf("Default.Host = %q, want %q", d.Host, "prod.example.com")
	}
	if d.Port != 9440 {
		t.Errorf("Default.Port = %d, want 9440", d.Port)
	}
	if d.User != "admin" {
		t.Errorf("Default.User = %q, want %q", d.User, "admin")
	}
	if d.Password != "secret" {
		t.Errorf("Default.Password = %q, want %q", d.Password, "secret")
	}
	if d.Database != "analytics" {
		t.Errorf("Default.Database = %q, want %q", d.Database, "analytics")
	}
	if d.Keymap != "vi" {
		t.Errorf("Default.Keymap = %q, want %q", d.Keymap, "vi")
	}
	if d.Theme != "dracula" {
		t.Errorf("Default.Theme = %q, want %q", d.Theme, "dracula")
	}
	if d.Pager != "less" {
		t.Errorf("Default.Pager = %q, want %q", d.Pager, "less")
	}
	if d.Editor != "nvim" {
		t.Errorf("Default.Editor = %q, want %q", d.Editor, "nvim")
	}
	if !d.TLS {
		t.Errorf("Default.TLS = false, want true")
	}

	// Validate [profiles.staging]
	staging, ok := cfg.Profiles["staging"]
	if !ok {
		t.Fatal("profiles[staging] not found")
	}
	if staging.Host != "staging.example.com" {
		t.Errorf("staging.Host = %q, want %q", staging.Host, "staging.example.com")
	}
	if staging.User != "staging_user" {
		t.Errorf("staging.User = %q, want %q", staging.User, "staging_user")
	}
	if staging.Database != "staging_db" {
		t.Errorf("staging.Database = %q, want %q", staging.Database, "staging_db")
	}

	// Validate [snippets]
	wantSnippet := "SELECT database, name FROM system.tables ORDER BY total_bytes DESC LIMIT 10"
	if cfg.Snippets["top_tables"] != wantSnippet {
		t.Errorf("snippets[top_tables] = %q, want %q", cfg.Snippets["top_tables"], wantSnippet)
	}
}

// TestResolve verifies the resolution order: defaults -> profile -> CLI flags.
func TestResolve(t *testing.T) {
	content := `
[default]
host     = "default.example.com"
port     = 9000
user     = "default_user"
database = "default_db"
keymap   = "emacs"
theme    = "monokai"
pager    = "builtin"

[profiles.staging]
host     = "staging.example.com"
user     = "staging_user"
`

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	t.Run("defaults only", func(t *testing.T) {
		resolved := cfg.Resolve("", config.ConnectionConfig{})
		if resolved.Host != "default.example.com" {
			t.Errorf("Host = %q, want %q", resolved.Host, "default.example.com")
		}
		if resolved.User != "default_user" {
			t.Errorf("User = %q, want %q", resolved.User, "default_user")
		}
	})

	t.Run("profile overrides default", func(t *testing.T) {
		resolved := cfg.Resolve("staging", config.ConnectionConfig{})
		if resolved.Host != "staging.example.com" {
			t.Errorf("Host = %q, want %q", resolved.Host, "staging.example.com")
		}
		if resolved.User != "staging_user" {
			t.Errorf("User = %q, want %q", resolved.User, "staging_user")
		}
		// database not set in staging, falls back to default
		if resolved.Database != "default_db" {
			t.Errorf("Database = %q, want %q", resolved.Database, "default_db")
		}
	})

	t.Run("CLI flags override profile and default", func(t *testing.T) {
		flags := config.ConnectionConfig{
			Host: "cli.example.com",
			User: "cli_user",
		}
		resolved := cfg.Resolve("staging", flags)
		if resolved.Host != "cli.example.com" {
			t.Errorf("Host = %q, want %q", resolved.Host, "cli.example.com")
		}
		if resolved.User != "cli_user" {
			t.Errorf("User = %q, want %q", resolved.User, "cli_user")
		}
		// database not in flags or staging, falls back to default
		if resolved.Database != "default_db" {
			t.Errorf("Database = %q, want %q", resolved.Database, "default_db")
		}
	})
}

// TestConnectionString verifies correct URL building.
func TestConnectionString(t *testing.T) {
	tests := []struct {
		name string
		cc   config.ConnectionConfig
		want string
	}{
		{
			name: "no password no TLS",
			cc: config.ConnectionConfig{
				Host:     "localhost",
				Port:     9000,
				User:     "default",
				Database: "default",
			},
			want: "clickhouse://default@localhost:9000/default",
		},
		{
			name: "with password no TLS",
			cc: config.ConnectionConfig{
				Host:     "prod.example.com",
				Port:     9440,
				User:     "admin",
				Password: "s3cr3t",
				Database: "analytics",
			},
			want: "clickhouse://admin:s3cr3t@prod.example.com:9440/analytics",
		},
		{
			name: "with TLS",
			cc: config.ConnectionConfig{
				Host:     "secure.example.com",
				Port:     9440,
				User:     "admin",
				Password: "pass",
				Database: "mydb",
				TLS:      true,
			},
			want: "clickhouse://admin:pass@secure.example.com:9440/mydb?sslmode=verify-ca",
		},
		{
			name: "no password with TLS",
			cc: config.ConnectionConfig{
				Host:     "secure.example.com",
				Port:     9440,
				User:     "reader",
				Database: "logs",
				TLS:      true,
			},
			want: "clickhouse://reader@secure.example.com:9440/logs?sslmode=verify-ca",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cc.ConnectionString()
			if got != tc.want {
				t.Errorf("ConnectionString() = %q, want %q", got, tc.want)
			}
		})
	}
}
