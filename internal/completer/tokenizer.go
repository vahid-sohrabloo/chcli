package completer

import (
	"strings"
	"unicode"
)

// Clause represents the SQL clause context at the cursor position.
type Clause int

const (
	ClauseUnknown Clause = iota
	ClauseSelect
	ClauseFrom
	ClauseWhere
	ClauseJoin
	ClauseOrderBy
	ClauseGroupBy
	ClauseHaving
	ClauseEngine
	ClauseUse
	ClauseSet
	ClauseInsert
)

// tokenizeWords splits input on whitespace, parentheses, commas, and semicolons,
// treating "=" as a separate token.
func tokenizeWords(input string) []string {
	var tokens []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	for _, ch := range input {
		switch {
		case unicode.IsSpace(ch), ch == '(', ch == ')', ch == ',', ch == ';':
			flush()
		case ch == '=':
			flush()
			tokens = append(tokens, "=")
		default:
			current.WriteRune(ch)
		}
	}
	flush()

	return tokens
}

// DetectClause examines the input and returns the SQL clause context
// that the cursor is currently in.
func DetectClause(input string) Clause {
	upper := strings.ToUpper(input)
	tokens := tokenizeWords(upper)

	n := len(tokens)
	if n == 0 {
		return ClauseUnknown
	}

	// Walk backwards through tokens to find the most recent clause keyword.
	for i := n - 1; i >= 0; i-- {
		tok := tokens[i]
		switch tok {
		case "SELECT":
			return ClauseSelect
		case "FROM":
			return ClauseFrom
		case "WHERE", "PREWHERE":
			return ClauseWhere
		case "JOIN":
			return ClauseJoin
		case "HAVING":
			return ClauseHaving
		case "INTO":
			return ClauseInsert
		case "USE":
			return ClauseUse
		case "SET":
			// SET is only a clause keyword when it appears at position 0.
			if i == 0 {
				return ClauseSet
			}
		case "BY":
			// Look at the word before BY.
			if i > 0 {
				prev := tokens[i-1]
				switch prev {
				case "ORDER":
					return ClauseOrderBy
				case "GROUP":
					return ClauseGroupBy
				}
			}
		case "ENGINE":
			return ClauseEngine
		case "=":
			// Check if "=" follows ENGINE token.
			if i > 0 && tokens[i-1] == "ENGINE" {
				return ClauseEngine
			}
		}
	}

	return ClauseUnknown
}

// TableRef represents a table reference with optional alias and database.
type TableRef struct {
	Database string // "predict" or "" for current db
	Table    string // "floor_samples"
	Alias    string // "a" or "" if no alias
	FullName string // "predict.floor_samples" or "floor_samples"
}

// ExtractTables finds all table names referenced after FROM and JOIN tokens.
// It strips any database prefix (everything up to and including the last ".").
func ExtractTables(input string) []string {
	refs := ExtractTableRefs(input)
	var tables []string
	seen := make(map[string]struct{})
	for _, ref := range refs {
		name := ref.Table
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			tables = append(tables, name)
		}
	}
	return tables
}

// ExtractTableRefs finds all table references with aliases from FROM/JOIN clauses.
// Handles: FROM db.table AS alias, FROM table alias, FROM table AS alias
func ExtractTableRefs(input string) []TableRef {
	upper := strings.ToUpper(input)
	tokens := tokenizeWords(upper)
	origTokens := tokenizeWords(input)

	var refs []TableRef

	for i, tok := range tokens {
		if (tok == "FROM" || tok == "JOIN") && i+1 < len(origTokens) {
			name := origTokens[i+1]
			if isKeyword(strings.ToUpper(name)) {
				continue
			}

			ref := TableRef{FullName: name}

			// Split database.table.
			if idx := strings.LastIndex(name, "."); idx >= 0 {
				ref.Database = name[:idx]
				ref.Table = name[idx+1:]
			} else {
				ref.Table = name
			}

			if ref.Table == "" {
				continue
			}

			// Check for alias: "table AS alias" or "table alias"
			next := i + 2
			if next < len(tokens) {
				if tokens[next] == "AS" && next+1 < len(origTokens) {
					ref.Alias = origTokens[next+1]
				} else if !isKeyword(tokens[next]) && tokens[next] != "ON" &&
					tokens[next] != "WHERE" && tokens[next] != "LEFT" &&
					tokens[next] != "RIGHT" && tokens[next] != "INNER" &&
					tokens[next] != "CROSS" && tokens[next] != "JOIN" &&
					tokens[next] != "FULL" && tokens[next] != "SEMI" &&
					tokens[next] != "ANTI" && tokens[next] != "GROUP" &&
					tokens[next] != "ORDER" && tokens[next] != "LIMIT" &&
					tokens[next] != "HAVING" && tokens[next] != "UNION" {
					ref.Alias = origTokens[next]
				}
			}

			refs = append(refs, ref)
		}
	}

	return refs
}

// sqlKeywordSet is a set of SQL keywords for O(1) lookup. Initialized in init().
var sqlKeywordSet map[string]struct{}

func init() {
	sqlKeywordSet = make(map[string]struct{}, len(sqlKeywords))
	for _, kw := range sqlKeywords {
		sqlKeywordSet[kw] = struct{}{}
	}
}

// isKeyword returns true if the given uppercase token is a SQL keyword.
func isKeyword(s string) bool {
	_, ok := sqlKeywordSet[strings.ToUpper(s)]
	return ok
}

// LastWord returns the last token in the input, or "" if the input ends
// with whitespace or a separator like ( ) , ; = (meaning the user is starting
// a new token, e.g. inside function arguments).
func LastWord(input string) string {
	if len(input) == 0 {
		return ""
	}
	last := rune(input[len(input)-1])
	// If input ends with whitespace or a separator, there's no partial word.
	if unicode.IsSpace(last) || isSeparator(last) {
		return ""
	}
	tokens := tokenizeWords(input)
	if len(tokens) == 0 {
		return ""
	}
	return tokens[len(tokens)-1]
}

// EnclosingFunction returns the function name if the cursor is inside
// function parentheses, and the argument index (0-based, counted by commas).
// Returns ("", 0) if not inside a function call.
func EnclosingFunction(toCursor string) (string, int) {
	// Walk backwards from end to find matching '('.
	depth := 0
	argIndex := 0
	for i := len(toCursor) - 1; i >= 0; i-- {
		switch toCursor[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				// Found the opening paren. The function name is the word before it.
				before := strings.TrimRight(toCursor[:i], " \t")
				tokens := tokenizeWords(before)
				if len(tokens) > 0 {
					return tokens[len(tokens)-1], argIndex
				}
				return "", 0
			}
			depth--
		case ',':
			if depth == 0 {
				argIndex++
			}
		}
	}
	return "", 0
}

func isSeparator(r rune) bool {
	switch r {
	case '(', ')', ',', ';', '=':
		return true
	}
	return false
}
