// Package format provides a lightweight SQL formatter for ClickHouse queries.
// It does not require a full parser — it works at the token level and is
// intentionally simple: capitalise keywords and add newlines before major
// clause boundaries.
package format

import (
	"strings"
	"unicode"
)

// SQL keywords that should always be rendered in upper-case.
var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true,
	"NOT": true, "IN": true, "IS": true, "NULL": true, "AS": true,
	"ON": true, "JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"FULL": true, "OUTER": true, "CROSS": true, "UNION": true, "ALL": true,
	"DISTINCT": true, "LIMIT": true, "OFFSET": true, "ORDER": true,
	"BY": true, "GROUP": true, "HAVING": true, "BETWEEN": true,
	"LIKE": true, "ILIKE": true, "CASE": true, "WHEN": true, "THEN": true,
	"ELSE": true, "END": true, "EXISTS": true, "WITH": true, "INSERT": true,
	"INTO": true, "VALUES": true, "UPDATE": true, "SET": true, "DELETE": true,
	"CREATE": true, "DROP": true, "ALTER": true, "TABLE": true, "VIEW": true,
	"IF": true, "ASC": true, "DESC": true, "NULLS": true, "FIRST": true,
	"LAST": true, "EXPLAIN": true, "AST": true, "PLAN": true,
	"ARRAY": true, "TUPLE": true, "MAP": true, "PREWHERE": true,
	"SAMPLE": true, "FINAL": true, "FORMAT": true, "SETTINGS": true,
}

// clauseKeywords are the tokens (or two-word phrases) that trigger a newline +
// indent before them.  Each entry is the upper-case first word; the second word
// (when set) must also match to apply the clause break.
type clause struct {
	first  string
	second string // empty means single-word clause
}

var clauses = []clause{
	{"FROM", ""},
	{"WHERE", ""},
	{"PREWHERE", ""},
	{"JOIN", ""},
	{"INNER", "JOIN"},
	{"LEFT", "JOIN"},
	{"RIGHT", "JOIN"},
	{"FULL", "JOIN"},
	{"CROSS", "JOIN"},
	{"GROUP", "BY"},
	{"ORDER", "BY"},
	{"HAVING", ""},
	{"LIMIT", ""},
	{"UNION", ""},
	{"SETTINGS", ""},
	{"FORMAT", ""},
}

// token is a raw lexical unit from the SQL string.
type token struct {
	text    string
	isQuote bool // true for string literals and backtick-quoted identifiers
}

// tokenise splits sql into tokens, preserving quoted strings as single units.
func tokenise(sql string) []token {
	var tokens []token
	i := 0
	for i < len(sql) {
		ch := sql[i]

		// Skip whitespace between tokens.
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}

		// Single-quoted string literal.
		if ch == '\'' {
			j := i + 1
			for j < len(sql) {
				if sql[j] == '\'' {
					if j+1 < len(sql) && sql[j+1] == '\'' { // escaped ''
						j += 2
						continue
					}
					j++
					break
				}
				if sql[j] == '\\' {
					j++ // skip escaped character
				}
				j++
			}
			tokens = append(tokens, token{text: sql[i:j], isQuote: true})
			i = j
			continue
		}

		// Double-quoted string or backtick-quoted identifier.
		if ch == '"' || ch == '`' {
			quote := ch
			j := i + 1
			for j < len(sql) && sql[j] != quote {
				if sql[j] == '\\' {
					j++
				}
				j++
			}
			if j < len(sql) {
				j++
			}
			tokens = append(tokens, token{text: sql[i:j], isQuote: true})
			i = j
			continue
		}

		// Single-line comment.
		if ch == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			j := i + 2
			for j < len(sql) && sql[j] != '\n' {
				j++
			}
			tokens = append(tokens, token{text: sql[i:j], isQuote: true}) // preserve as-is
			i = j
			continue
		}

		// Block comment.
		if ch == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			j := i + 2
			for j+1 < len(sql) && (sql[j] != '*' || sql[j+1] != '/') {
				j++
			}
			if j+1 < len(sql) {
				j += 2
			}
			tokens = append(tokens, token{text: sql[i:j], isQuote: true}) // preserve as-is
			i = j
			continue
		}

		// Regular token: read until whitespace or a quote character.
		j := i
		for j < len(sql) && !unicode.IsSpace(rune(sql[j])) && sql[j] != '\'' && sql[j] != '"' && sql[j] != '`' {
			j++
		}
		tokens = append(tokens, token{text: sql[i:j]})
		i = j
	}
	return tokens
}

// isClauseBreak checks whether tokens[i] (and optionally tokens[i+1]) mark the
// start of a major clause that should be preceded by a newline.
func isClauseBreak(tokens []token, i int) bool {
	if tokens[i].isQuote {
		return false
	}
	upper := strings.ToUpper(tokens[i].text)
	for _, c := range clauses {
		if upper != c.first {
			continue
		}
		if c.second == "" {
			return true
		}
		// Two-word clause: peek at next non-quote token.
		if i+1 < len(tokens) && !tokens[i+1].isQuote {
			next := strings.ToUpper(tokens[i+1].text)
			if next == c.second {
				return true
			}
		}
	}
	return false
}

// FormatSQL formats a SQL string by capitalising keywords and inserting newlines
// before major clause keywords.  String literals and quoted identifiers are
// preserved verbatim.
func FormatSQL(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return sql
	}

	tokens := tokenise(sql)
	if len(tokens) == 0 {
		return sql
	}

	var sb strings.Builder
	for i, tok := range tokens {
		text := tok.text
		if !tok.isQuote {
			upper := strings.ToUpper(text)
			if keywords[upper] {
				text = upper
			}
		}

		if i == 0 {
			sb.WriteString(text)
			continue
		}

		if isClauseBreak(tokens, i) {
			sb.WriteString("\n    ")
		} else {
			sb.WriteString(" ")
		}
		sb.WriteString(text)
	}

	return sb.String()
}
