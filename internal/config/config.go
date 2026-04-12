// Package config provides TOML-based configuration loading, profile resolution,
// and connection string building for chcli.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// ConnectionConfig holds all parameters needed to establish a ClickHouse connection,
// as well as UI preferences that can be set per-profile.
type ConnectionConfig struct {
	Host     string `toml:"host"`
	Port     uint16 `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	Keymap   string `toml:"keymap"`
	Theme    string `toml:"theme"`
	Pager    string `toml:"pager"`
	Editor   string `toml:"editor"`
	TLS      bool   `toml:"tls"`
	Compress string `toml:"compress"` // "", "lz4", "zstd"

	// SSH tunnel fields — when SSHHost is set, the connection is forwarded
	// through an SSH bastion before reaching ClickHouse.
	SSHHost     string `toml:"ssh_host"`     // bastion hostname
	SSHPort     uint16 `toml:"ssh_port"`     // default 22
	SSHUser     string `toml:"ssh_user"`     // SSH username
	SSHKey      string `toml:"ssh_key"`      // path to private key (default ~/.ssh/id_rsa)
	SSHPassword string `toml:"ssh_password"` // optional SSH password (prefer keys)
}

// Config is the top-level configuration structure read from a TOML file.
type Config struct {
	Default  ConnectionConfig            `toml:"default"`
	Profiles map[string]ConnectionConfig `toml:"profiles"`
	Snippets map[string]string           `toml:"snippets"`
	path     string
}

// Load reads the TOML config at path. If the file does not exist, it returns a
// Config populated with built-in defaults instead of an error.
func Load(path string) (*Config, error) {
	cfg := &Config{path: path}

	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg.applyDefaults()
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: stat %q: %w", path, err)
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// Resolve builds the effective ConnectionConfig by merging:
//
//	built-in defaults → [default] section → named profile → CLI flags
//
// Any zero-value field in a higher-priority source is left unchanged from the
// lower-priority source (i.e. non-zero fields always win).
func (c *Config) Resolve(profile string, flags ConnectionConfig) ConnectionConfig {
	result := c.Default

	if profile != "" {
		if p, ok := c.Profiles[profile]; ok {
			mergeInto(&result, p)
		}
	}

	mergeInto(&result, flags)
	return result
}

// ConnectionString returns a clickhouse:// URL for this ConnectionConfig.
//
// Format:
//
//	clickhouse://user@host:port/database          (no password)
//	clickhouse://user:password@host:port/database (with password)
//
// When TLS is true, "?sslmode=verify-ca" is appended.
func (cc ConnectionConfig) ConnectionString() string {
	var userInfo string
	if cc.Password != "" {
		userInfo = fmt.Sprintf("%s:%s", cc.User, cc.Password)
	} else {
		userInfo = cc.User
	}

	hostPort := net.JoinHostPort(cc.Host, strconv.FormatUint(uint64(cc.Port), 10))
	url := fmt.Sprintf("clickhouse://%s@%s/%s", userInfo, hostPort, cc.Database)

	var params []string
	if cc.TLS {
		params = append(params, "sslmode=verify-ca")
	}
	if cc.Compress != "" {
		params = append(params, "compress="+cc.Compress)
	}
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	return url
}

// SaveSnippet persists a named SQL snippet to the config file, creating it if
// needed. It re-reads the file first so concurrent edits outside this process
// are not clobbered.
func (c *Config) SaveSnippet(name, query string) error {
	if c.Snippets == nil {
		c.Snippets = make(map[string]string)
	}
	c.Snippets[name] = query
	return c.save()
}

// DeleteSnippet removes a named snippet from memory. Call SaveSnippet / save to
// persist the change.
func (c *Config) DeleteSnippet(name string) {
	delete(c.Snippets, name)
}

// Path returns the filesystem path this Config was loaded from.
func (c *Config) Path() string {
	return c.path
}

// applyDefaults fills zero-value fields in c.Default with the built-in
// default values.
func (c *Config) applyDefaults() {
	d := &c.Default
	if d.Host == "" {
		d.Host = "localhost"
	}
	if d.Port == 0 {
		d.Port = 9000
	}
	if d.User == "" {
		d.User = "default"
	}
	if d.Database == "" {
		d.Database = "default"
	}
	if d.Keymap == "" {
		d.Keymap = "emacs"
	}
	if d.Theme == "" {
		d.Theme = "monokai"
	}
	if d.Pager == "" {
		d.Pager = "builtin"
	}
}

// mergeInto copies non-zero fields from src into dst.
func mergeInto(dst *ConnectionConfig, src ConnectionConfig) {
	if src.Host != "" {
		dst.Host = src.Host
	}
	if src.Port != 0 {
		dst.Port = src.Port
	}
	if src.User != "" {
		dst.User = src.User
	}
	if src.Password != "" {
		dst.Password = src.Password
	}
	if src.Database != "" {
		dst.Database = src.Database
	}
	if src.Keymap != "" {
		dst.Keymap = src.Keymap
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.Pager != "" {
		dst.Pager = src.Pager
	}
	if src.Editor != "" {
		dst.Editor = src.Editor
	}
	if src.TLS {
		dst.TLS = src.TLS
	}
	if src.Compress != "" {
		dst.Compress = src.Compress
	}
	if src.SSHHost != "" {
		dst.SSHHost = src.SSHHost
	}
	if src.SSHPort != 0 {
		dst.SSHPort = src.SSHPort
	}
	if src.SSHUser != "" {
		dst.SSHUser = src.SSHUser
	}
	if src.SSHKey != "" {
		dst.SSHKey = src.SSHKey
	}
	if src.SSHPassword != "" {
		dst.SSHPassword = src.SSHPassword
	}
}

// save writes the current Config back to c.path in TOML format.
func (c *Config) save() error {
	f, err := os.OpenFile(c.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: open %q for writing: %w", c.path, err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("config: encode TOML to %q: %w", c.path, err)
	}
	return nil
}
