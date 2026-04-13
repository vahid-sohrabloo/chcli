package completer

import (
	"maps"
	"strings"

	"github.com/vahid-sohrabloo/chcli/internal/functions"
	"github.com/vahid-sohrabloo/chcli/internal/schema"
)

// CompletionKind classifies the type of a completion suggestion.
type CompletionKind int

const (
	KindKeyword CompletionKind = iota
	KindTable
	KindColumn
	KindFunction
	KindAggFunction
	KindDatabase
	KindSnippet
	KindEngine
	KindSetting
)

// Completion is a single autocompletion candidate.
type Completion struct {
	Text   string
	Detail string // optional extra info (e.g., "aggregate")
	Kind   CompletionKind
}

// Completer generates context-aware SQL autocompletion suggestions.
type Completer struct {
	databases  []string
	tables     map[string][]string            // db → table names
	columns    map[string][]schema.ColumnInfo // "db.table" → column info
	functions  []schema.FunctionInfo
	funcByName map[string]int // lowercase name → index into functions for O(1) lookup
	settings   []string
	snippets   map[string]string
}

// buildFuncIndex builds a lowercase-name → index map for O(1) function lookups.
func buildFuncIndex(funcs []schema.FunctionInfo) map[string]int {
	idx := make(map[string]int, len(funcs))
	for i, fn := range funcs {
		idx[strings.ToLower(fn.Name)] = i
	}
	return idx
}

// NewWithBuiltins creates a Completer using embedded function metadata.
// Used for instant completions before schema loads.
func NewWithBuiltins(serverVersion string, snippets map[string]string) *Completer {
	funcs := functions.ForVersion(serverVersion)
	schemaFuncs := make([]schema.FunctionInfo, len(funcs))
	for i, f := range funcs {
		schemaFuncs[i] = schema.FunctionInfo{
			Name:          f.Name,
			IsAggregate:   f.IsAggregate,
			Description:   f.Description,
			Syntax:        f.Syntax,
			Arguments:     f.Arguments,
			ReturnedValue: f.ReturnedVal,
		}
	}
	return &Completer{
		tables:     make(map[string][]string),
		columns:    make(map[string][]schema.ColumnInfo),
		functions:  schemaFuncs,
		funcByName: buildFuncIndex(schemaFuncs),
		snippets:   snippets,
	}
}

// New creates a Completer populated from the given schema.Cache and snippet map.
// Merges live schema data with embedded function metadata for best coverage.
func New(cache *schema.Cache, snippets map[string]string) *Completer {
	// Always use embedded functions — better metadata than live data.
	allBuiltins := functions.ForVersion("")
	funcs := make([]schema.FunctionInfo, len(allBuiltins))
	for i, f := range allBuiltins {
		funcs[i] = schema.FunctionInfo{
			Name:          f.Name,
			IsAggregate:   f.IsAggregate,
			Description:   f.Description,
			Syntax:        f.Syntax,
			Arguments:     f.Arguments,
			ReturnedValue: f.ReturnedVal,
		}
	}

	c := &Completer{
		databases:  make([]string, len(cache.Databases)),
		tables:     make(map[string][]string),
		columns:    make(map[string][]schema.ColumnInfo),
		functions:  funcs,
		funcByName: buildFuncIndex(funcs),
		settings:   make([]string, len(cache.Settings)),
		snippets:   snippets,
	}

	copy(c.databases, cache.Databases)
	copy(c.settings, cache.Settings)

	for db, tableInfos := range cache.Tables {
		names := make([]string, len(tableInfos))
		for i, t := range tableInfos {
			names[i] = t.Name
		}
		c.tables[db] = names
	}

	maps.Copy(c.columns, cache.Columns)

	return c
}

// UpdateSnippets replaces the completer's snippet map.
func (c *Completer) UpdateSnippets(snippets map[string]string) {
	c.snippets = snippets
}

// Complete returns completion candidates for the given SQL input and current database.
func (c *Completer) Complete(input string, currentDB string) []Completion {
	return c.CompleteAt(input, input, currentDB)
}

// CompleteAt returns completion candidates using toCursor for clause detection
// (which clause the cursor is in) and fullText for table extraction (so that
// tables in FROM are available for column completions in SELECT).
func (c *Completer) CompleteAt(toCursor, fullText, currentDB string) []Completion {
	// Meta-command completion: when input starts with \, complete from meta-commands.
	// Preserve trailing space — it signals "complete arguments, not command name".
	trimLeft := strings.TrimLeft(toCursor, " \t")
	if strings.HasPrefix(trimLeft, `\`) {
		return c.metaCommandCompletions(trimLeft)
	}

	prefix := LastWord(toCursor)
	clause := DetectClause(toCursor)

	var candidates []Completion

	switch clause {
	case ClauseSelect:
		candidates = append(candidates, c.columnCompletions(fullText, currentDB)...)
		candidates = append(candidates, contextKeywords(clause)...)
		candidates = append(candidates, c.functionCompletions()...)
	case ClauseFrom, ClauseJoin:
		candidates = append(candidates, contextKeywords(clause)...)
		candidates = append(candidates, c.tableCompletions(currentDB)...)
	case ClauseWhere, ClauseOrderBy, ClauseGroupBy, ClauseHaving:
		candidates = append(candidates, c.columnCompletions(fullText, currentDB)...)
		candidates = append(candidates, contextKeywords(clause)...)
		candidates = append(candidates, c.functionCompletions()...)
	case ClauseEngine:
		candidates = c.engineCompletions()
	case ClauseUse:
		candidates = c.databaseCompletions()
	case ClauseSet:
		candidates = c.settingCompletions()
	default:
		candidates = append(candidates, c.keywordCompletions()...)
		candidates = append(candidates, c.tableCompletions(currentDB)...)
	}

	// Handle alias.column completion: if prefix contains ".", extract alias.
	if strings.Contains(prefix, ".") {
		parts := strings.SplitN(prefix, ".", 2)
		alias := parts[0]
		colPrefix := parts[1]
		aliasCols := c.columnsForAlias(alias, fullText, currentDB)
		if len(aliasCols) > 0 {
			// Return alias-qualified completions.
			var result []Completion
			for _, col := range aliasCols {
				if colPrefix == "" || strings.HasPrefix(strings.ToLower(col.Text), strings.ToLower(colPrefix)) {
					result = append(result, Completion{
						Text:   alias + "." + col.Text,
						Detail: col.Detail,
						Kind:   col.Kind,
					})
				}
			}
			return result
		}
	}

	return filterByPrefix(candidates, prefix)
}

// columnsForAlias resolves a table alias to its columns.
func (c *Completer) columnsForAlias(alias, fullText, currentDB string) []Completion {
	refs := ExtractTableRefs(fullText)
	for _, ref := range refs {
		if strings.EqualFold(ref.Alias, alias) || strings.EqualFold(ref.Table, alias) {
			// Found the table for this alias. Look up columns.
			var cols []schema.ColumnInfo

			// Try with database prefix first.
			if ref.Database != "" {
				cols = c.columns[ref.Database+"."+ref.Table]
			}
			if len(cols) == 0 {
				cols = c.columns[currentDB+"."+ref.Table]
			}
			// Cross-database fallback.
			if len(cols) == 0 {
				for k, v := range c.columns {
					kParts := strings.SplitN(k, ".", 2)
					if len(kParts) == 2 && strings.EqualFold(kParts[1], ref.Table) {
						cols = v
						break
					}
				}
			}

			completions := make([]Completion, 0, len(cols))
			for _, col := range cols {
				completions = append(completions, Completion{
					Text:   col.Name,
					Detail: col.Type,
					Kind:   KindColumn,
				})
			}
			return completions
		}
	}
	return nil
}

// functionCompletions returns all function candidates with syntax signature.
func (c *Completer) functionCompletions() []Completion {
	completions := make([]Completion, len(c.functions))
	for i, fn := range c.functions {
		kind := KindFunction
		if fn.IsAggregate {
			kind = KindAggFunction
		}
		// Prefer syntax (shows signature), fall back to cleaned description.
		detail := fn.Syntax
		if detail == "" {
			detail = cleanMarkdown(fn.Description)
		}
		completions[i] = Completion{Text: fn.Name, Detail: detail, Kind: kind}
	}
	return completions
}

// tableCompletions returns table candidates for the current and other databases.
func (c *Completer) tableCompletions(currentDB string) []Completion {
	var completions []Completion

	// Tables in the current database (unqualified names).
	for _, tbl := range c.tables[currentDB] {
		completions = append(completions, Completion{Text: tbl, Kind: KindTable})
	}

	// Tables in other databases (qualified as db.table).
	for db, tables := range c.tables {
		if db == currentDB {
			continue
		}
		for _, tbl := range tables {
			completions = append(completions, Completion{Text: db + "." + tbl, Kind: KindTable})
		}
	}

	return completions
}

// columnCompletions returns column candidates from tables referenced in the input.
func (c *Completer) columnCompletions(input string, currentDB string) []Completion {
	tables := ExtractTables(input)
	seen := make(map[string]struct{})
	var completions []Completion

	for _, tbl := range tables {
		// Try current database first.
		key := currentDB + "." + tbl
		cols := c.columns[key]

		// If not found, check if table name already has db prefix (e.g., "db.table").
		if len(cols) == 0 {
			cols = c.columns[tbl]
		}

		// Fallback: search all databases for this table name.
		if len(cols) == 0 {
			for k, v := range c.columns {
				parts := strings.SplitN(k, ".", 2)
				if len(parts) == 2 && strings.EqualFold(parts[1], tbl) {
					cols = v
					break
				}
			}
		}

		for _, col := range cols {
			if _, ok := seen[col.Name]; !ok {
				seen[col.Name] = struct{}{}
				completions = append(completions, Completion{Text: col.Name, Detail: col.Type, Kind: KindColumn})
			}
		}
	}

	return completions
}

// contextKeywords returns only keywords that are grammatically valid after the given clause.
func contextKeywords(clause Clause) []Completion {
	var words []string
	switch clause {
	case ClauseSelect:
		words = []string{
			"FROM", "AS", "DISTINCT", "TOP",
			"CASE", "WHEN", "THEN", "ELSE", "END",
			"IF", "NULL", "NOT", "IN", "BETWEEN",
			"AND", "OR",
		}
	case ClauseFrom:
		words = []string{
			"WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
			"CROSS", "FULL", "SEMI", "ANTI", "ANY", "ALL",
			"GLOBAL", "AS", "ON",
			"ORDER", "GROUP", "HAVING", "LIMIT", "UNION",
			"PREWHERE", "FINAL", "SAMPLE",
			"FORMAT", "SETTINGS", "INTO",
		}
	case ClauseJoin:
		words = []string{
			"ON", "AS", "USING",
			"WHERE", "ORDER", "GROUP", "HAVING", "LIMIT",
		}
	case ClauseWhere:
		words = []string{
			"AND", "OR", "NOT", "IN", "BETWEEN", "LIKE", "ILIKE",
			"IS", "NULL", "EXISTS",
			"ORDER", "GROUP", "HAVING", "LIMIT",
			"JOIN", "LEFT", "RIGHT",
			"UNION", "EXCEPT", "INTERSECT",
			"FORMAT", "SETTINGS",
		}
	case ClauseOrderBy:
		words = []string{
			"ASC", "DESC", "NULLS", "FIRST", "LAST",
			"LIMIT", "OFFSET", "WITH", "TIES",
			"FORMAT", "SETTINGS",
		}
	case ClauseGroupBy:
		words = []string{
			"HAVING", "ORDER", "LIMIT",
			"WITH", "ROLLUP", "CUBE", "TOTALS",
			"FORMAT", "SETTINGS",
		}
	case ClauseHaving:
		words = []string{
			"AND", "OR", "NOT",
			"ORDER", "LIMIT",
			"FORMAT", "SETTINGS",
		}
	default:
		words = sqlKeywords
	}

	completions := make([]Completion, len(words))
	for i, kw := range words {
		completions[i] = Completion{Text: kw, Kind: KindKeyword}
	}
	return completions
}

// keywordCompletions returns all SQL keyword candidates.
func (c *Completer) keywordCompletions() []Completion {
	completions := make([]Completion, len(sqlKeywords))
	for i, kw := range sqlKeywords {
		completions[i] = Completion{Text: kw, Kind: KindKeyword}
	}
	return completions
}

// databaseCompletions returns all database name candidates.
func (c *Completer) databaseCompletions() []Completion {
	completions := make([]Completion, len(c.databases))
	for i, db := range c.databases {
		completions[i] = Completion{Text: db, Kind: KindDatabase}
	}
	return completions
}

// engineCompletions returns all ClickHouse engine name candidates.
func (c *Completer) engineCompletions() []Completion {
	completions := make([]Completion, len(engineNames))
	for i, eng := range engineNames {
		completions[i] = Completion{Text: eng, Kind: KindEngine}
	}
	return completions
}

// settingCompletions returns all setting name candidates.
func (c *Completer) settingCompletions() []Completion {
	completions := make([]Completion, len(c.settings))
	for i, s := range c.settings {
		completions[i] = Completion{Text: s, Kind: KindSetting}
	}
	return completions
}

// FunctionSignatureDetail returns the syntax, return value, and the specific
// argument description for the given argument index.
func (c *Completer) FunctionSignatureDetail(name string, argIdx int) (syntax, returnedValue, argument string) {
	idx, ok := c.funcByName[strings.ToLower(name)]
	if !ok {
		return "", "", ""
	}
	fn := c.functions[idx]
	syntax = fn.Syntax
	if syntax == "" {
		syntax = fn.Name + "(...)"
	}
	returnedValue = cleanMarkdown(fn.ReturnedValue)

	// Split arguments by "- " prefix lines and pick the one at argIdx.
	if fn.Arguments != "" {
		args := splitArguments(fn.Arguments)
		if argIdx >= 0 && argIdx < len(args) {
			argument = cleanMarkdown(args[argIdx])
		} else if len(args) > 0 {
			// Show last arg if beyond range (e.g., variadic).
			argument = cleanMarkdown(args[len(args)-1])
		}
	}
	return syntax, returnedValue, argument
}

// splitArguments splits the arguments string into individual argument descriptions.
// Arguments are separated by "\n- " in the Markdown source.
func splitArguments(s string) []string {
	// Split on lines starting with "- "
	lines := strings.Split(s, "\n")
	var args []string
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if text, ok := strings.CutPrefix(trimmed, "- "); ok {
			if current.Len() > 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			}
			current.WriteString(text)
		} else if current.Len() > 0 && trimmed != "" {
			current.WriteString(" " + trimmed)
		}
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}
	return args
}

// cleanMarkdown strips common Markdown syntax for inline display.
func cleanMarkdown(s string) string {
	// [text](url) → text
	for {
		start := strings.Index(s, "[")
		if start == -1 {
			break
		}
		mid := strings.Index(s[start:], "](")
		if mid == -1 {
			break
		}
		end := strings.Index(s[start+mid:], ")")
		if end == -1 {
			break
		}
		text := s[start+1 : start+mid]
		s = s[:start] + text + s[start+mid+end+1:]
	}
	// Strip backticks.
	s = strings.ReplaceAll(s, "`", "")
	// First line only.
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// metaCommands is the list of all meta-commands available for completion.
// Defined as a package-level var so it is built only once.
var metaCommands = []struct{ name, desc string }{
	{`\l`, "List databases"},
	{`\dt`, "List tables"},
	{`\dt+`, "List tables with details"},
	{`\d`, "Describe table"},
	{`\d+`, "Describe table with details"},
	{`\di`, "List dictionaries"},
	{`\dm`, "List materialized views"},
	{`\dv`, "List views"},
	{`\dp`, "List running processes"},
	{`\use`, "Switch database"},
	{`\c`, "Connect using a named profile"},
	{`\timing`, "Toggle timing"},
	{`\e`, "Open editor"},
	{`\pager`, "Toggle/set pager"},
	{`\x`, "Toggle vertical mode"},
	{`\f`, "List or execute a snippet"},
	{`\fs`, "Save a snippet"},
	{`\fd`, "Delete a snippet"},
	{`\h`, "Show recent history"},
	{`\hb`, "Bookmark last query"},
	{`\hl`, "List bookmarks"},
	{`\refresh`, "Refresh schema cache"},
	{`\settings`, "Show current settings"},
	{`\help`, "Show help"},
	{`\?`, "Show help"},
	{`\q`, "Quit"},
	{`\copy`, "Export results (csv/json)"},
	{`\clip`, "Copy to clipboard"},
	{`\doc`, "Function documentation"},
	{`\metrics`, "Show query metrics"},
	{`\watch`, "Re-run query periodically"},
	{`\explain`, "EXPLAIN query plan"},
	{`\fmt`, "Format SQL query"},
	{`\format`, "Format SQL query"},
	{`\theme`, "List/switch color themes"},
	{`\save`, "Save last query as snippet"},
	{`\load`, "Load snippet into input"},
	{`\saved`, "List saved snippets"},
}

// metaCommandCompletions returns completion candidates for meta-commands (inputs starting with \).
func (c *Completer) metaCommandCompletions(input string) []Completion {
	commands := metaCommands

	// If input contains a space, user wants argument completions.
	if before, after, ok := strings.Cut(input, " "); ok {
		cmd := before
		argPrefix := strings.TrimSpace(after)
		args := c.metaCommandArgCompletions(cmd, argPrefix)
		if len(args) > 0 {
			return args
		}
		return nil // command doesn't have arg completions
	}

	// If input exactly matches a complete command (no space), no suggestions needed.
	for _, cmd := range commands {
		if cmd.name == input {
			return nil
		}
	}

	var result []Completion
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.name, input) {
			result = append(result, Completion{Text: cmd.name, Detail: cmd.desc, Kind: KindKeyword})
		}
	}
	return result
}

// metaCommandArgCompletions returns completions for arguments of specific meta-commands.
func (c *Completer) metaCommandArgCompletions(cmd, prefix string) []Completion {
	switch cmd {
	case `\theme`:
		// NOTE: cannot use tui.UIThemeNames() here — tui imports completer (circular).
		// Keep in sync with uiThemes in internal/tui/theme.go.
		themes := []string{
			"tokyo-night", "dracula", "nord", "gruvbox", "catppuccin", "solarized",
		}
		var result []Completion
		for _, t := range themes {
			if prefix == "" || strings.HasPrefix(t, strings.ToLower(prefix)) {
				result = append(result, Completion{Text: t, Detail: "color theme", Kind: KindKeyword})
			}
		}
		return result

	case `\d`, `\d+`:
		var result []Completion
		for db, tables := range c.tables {
			for _, tbl := range tables {
				if prefix == "" || strings.HasPrefix(strings.ToLower(tbl), strings.ToLower(prefix)) {
					result = append(result, Completion{Text: tbl, Detail: db, Kind: KindTable})
				}
			}
		}
		return result

	case `\use`:
		var result []Completion
		for _, db := range c.databases {
			if prefix == "" || strings.HasPrefix(strings.ToLower(db), strings.ToLower(prefix)) {
				result = append(result, Completion{Text: db, Kind: KindDatabase})
			}
		}
		return result

	case `\doc`:
		var result []Completion
		lower := strings.ToLower(prefix)
		for _, fn := range c.functions {
			if prefix == "" || strings.HasPrefix(strings.ToLower(fn.Name), lower) {
				result = append(result, Completion{Text: fn.Name, Detail: fn.Syntax, Kind: KindFunction})
			}
		}
		return result

	case `\load`, `\fd`:
		var result []Completion
		for name := range c.snippets {
			if prefix == "" || strings.HasPrefix(name, prefix) {
				result = append(result, Completion{Text: name, Detail: "snippet", Kind: KindSnippet})
			}
		}
		return result
	}

	return nil
}

// filterByPrefix returns only completions whose Text has the given prefix
// (case-insensitive).
func filterByPrefix(completions []Completion, prefix string) []Completion {
	if prefix == "" {
		return completions
	}
	lower := strings.ToLower(prefix)
	var result []Completion
	for _, c := range completions {
		if strings.HasPrefix(strings.ToLower(c.Text), lower) {
			result = append(result, c)
		}
	}
	return result
}
