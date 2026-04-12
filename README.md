# chcli

Modern interactive ClickHouse client for the terminal. Built with Go.

Features smart autocompletion, syntax highlighting, query progress tracking, SSH tunneling, multiple themes, and 30+ meta-commands — everything you need to work with ClickHouse without leaving the terminal.

## Install

### Go install

```bash
go install github.com/vahid-sohrabloo/chcli/cmd/chcli@latest
```

### Download binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/vahid-sohrabloo/chcli/releases) page.

### Build from source

```bash
git clone https://github.com/vahid-sohrabloo/chcli.git
cd chcli
make install
```

## Quick start

```bash
# Connect to localhost:9000
chcli

# Connect to a specific host
chcli -h clickhouse.example.com -u admin --password secret -d analytics

# Connect through SSH bastion
chcli --ssh-host bastion.example.com -h db-internal.example.com

# Use a saved profile
chcli --profile production
```

## Features

### Context-aware autocompletion

Completions adapt to your SQL context — column names after SELECT, table names after FROM, join keywords after JOIN, engine names after ENGINE, and more. Alias-aware: type `t.` and get columns for the aliased table.

Includes 1000+ built-in ClickHouse function signatures with syntax, arguments, and return types — available instantly without querying the server.

### Syntax highlighting

Full SQL syntax highlighting using a custom ClickHouse lexer that recognizes ClickHouse-specific keywords, types, functions, and engine names.

### Query progress

Live progress display during query execution showing rows read, bytes processed, elapsed time, memory usage, CPU time, and thread count.

Cancel any running query with `Ctrl+C` (sends `KILL QUERY` to the server).

### Interactive table viewer

Press `F2` after a query to open a full-screen scrollable table viewer with column panning (`Left`/`Right`), row scrolling (`Up`/`Down`/`PgUp`/`PgDn`), and `q`/`Esc` to exit.

### SSH tunnel

Connect to ClickHouse through an SSH bastion host. Reads `~/.ssh/config` for host aliases, so `--ssh-host myserver` just works if it's defined in your SSH config.

Authentication (in priority order): SSH agent, key file (auto-discovers `id_ed25519`, `id_rsa`, `id_ecdsa`), password.

### Themes

Six built-in color themes: `tokyo-night`, `dracula`, `nord`, `gruvbox`, `catppuccin`, `solarized`. Switch with `\theme <name>`.

### History & bookmarks

SQLite-backed query history with arrow-key browsing and fuzzy search (`Ctrl+R`). Bookmark important queries with tags using `\hb`.

### Watch mode

Re-run a query on an interval: `\watch 5 SELECT count() FROM events` runs every 5 seconds. `Ctrl+C` to stop.

### Snippets

Save and recall frequently used queries:

```
\fs popular   SELECT name, engine FROM system.tables LIMIT 10
\f popular    -- execute it
\save mytag   -- save last query
\load mytag   -- load into editor
```

## Connection flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--host` | `-h` | `localhost` | ClickHouse host |
| `--port` | `-p` | `9000` | ClickHouse native port |
| `--user` | `-u` | `default` | Username |
| `--password` | | | Password |
| `--database` | `-d` | `default` | Database |
| `--profile` | | | Named profile from config |
| `--compress` | | | `lz4` or `zstd` |
| `--ssh-host` | | | SSH bastion host |
| `--ssh-user` | | | SSH username |
| `--ssh-key` | | auto-detect | Path to SSH private key |

## Meta-commands

| Command | Description |
|---------|-------------|
| `\l` | List databases |
| `\dt [db]` | List tables |
| `\dt+ [db]` | List tables with row count and size |
| `\d <table>` | Describe table columns |
| `\d+ <table>` | Describe columns (extended) |
| `\di` | List dictionaries |
| `\dm` | List materialized views |
| `\dv` | List views |
| `\dp` | List running processes |
| `\use <db>` | Switch database |
| `\c <profile>` | Switch connection profile |
| `\e` | Open editor (`$EDITOR`) |
| `\fmt [query]` | Format SQL |
| `\x` | Toggle vertical display |
| `\explain [query]` | Run EXPLAIN |
| `\timing` | Toggle execution timing |
| `\pager [cmd]` | Set pager |
| `\copy <fmt> [path]` | Export to CSV or JSON |
| `\clip` | Copy result to clipboard |
| `\doc <func>` | Show function documentation |
| `\metrics` | Show last query metrics |
| `\watch <sec> <query>` | Re-run query on interval |
| `\theme [name]` | List or switch theme |
| `\f [name]` | List or run snippet |
| `\fs <name> <query>` | Save snippet |
| `\fd <name>` | Delete snippet |
| `\save <name>` | Save last query as snippet |
| `\load <name>` | Load snippet into input |
| `\h [term]` | Search history |
| `\hb <tag> [query]` | Bookmark query |
| `\hl [tag]` | List bookmarks |
| `\refresh` | Refresh schema cache |
| `\settings` | Show settings |
| `\?` | Help |
| `\q` | Quit |

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Tab` | Accept / trigger completion |
| `Up` / `Down` | Browse history or completions |
| `Ctrl+R` | Fuzzy history search |
| `Enter` | Submit (when query ends with `;` or `\G`) |
| `Alt+Enter` | Insert newline |
| `F2` | Open table viewer |
| `Ctrl+C` | Cancel query / stop watch / clear input |
| `Ctrl+D` | Quit (on empty input) |
| `Ctrl+U` | Clear line |
| `Ctrl+K` | Kill to end of line |
| `Ctrl+A` / `Ctrl+E` | Start / end of line |

## Configuration

Config file: `~/.chcli/config.toml`

```toml
[default]
host = "localhost"
port = 9000
user = "default"
database = "default"
keymap = "emacs"           # or "vi"
theme = "tokyo-night"
pager = "builtin"
editor = ""                # uses $EDITOR, then vi

# SSH tunnel (optional)
ssh_host = "bastion.example.com"
ssh_user = "vahid"
ssh_key = "~/.ssh/id_ed25519"

[profiles.production]
host = "prod-ch.internal"
user = "analytics"
database = "events"
ssh_host = "jump.prod.example.com"
ssh_user = "deploy"

[profiles.staging]
host = "staging-ch.internal"
database = "events"

[snippets]
slow_queries = "SELECT query, elapsed FROM system.query_log WHERE elapsed > 10 ORDER BY elapsed DESC LIMIT 20"
table_sizes = "SELECT database, name, formatReadableSize(total_bytes) FROM system.tables ORDER BY total_bytes DESC LIMIT 20"
```

Config priority: defaults < `[default]` section < named profile < CLI flags.

## Architecture

chcli uses the [chconn](https://github.com/vahid-sohrabloo/chconn) native ClickHouse protocol driver (no HTTP, no CGO) and [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI.

```
cmd/chcli/          CLI entry point, flag parsing, tunnel setup
cmd/chcli-gen/      Code generator for embedded function metadata
internal/
  config/           TOML config, profile resolution, connection strings
  conn/             chconn wrapper, query execution, progress, KILL QUERY
  completer/        Context-aware SQL completion engine
  functions/        1000+ embedded ClickHouse function signatures
  highlight/        Custom ClickHouse chroma lexer + themes
  history/          SQLite history store + bookmarks
  metacmd/          30+ meta-command handlers
  render/           Table + vertical result formatting
  schema/           Async schema cache (databases, tables, columns, types)
  tunnel/           SSH tunnel with agent/key/password auth
  tui/              Bubble Tea model, input, completion popup, themes
```

## License

MIT
