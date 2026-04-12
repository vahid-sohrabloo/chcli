package highlight

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
)

// filterTokens returns only the tokens whose type matches tt.
func filterTokens(tokens []chroma.Token, tt chroma.TokenType) []chroma.Token {
	var out []chroma.Token
	for _, t := range tokens {
		if t.Type == tt {
			out = append(out, t)
		}
	}
	return out
}

func tokenize(t *testing.T, input string) []chroma.Token {
	t.Helper()
	tokens, err := chroma.Tokenise(ClickHouseLexer, nil, input)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	return tokens
}

func TestLexerTokenizesKeywords(t *testing.T) {
	tokens := tokenize(t, "SELECT count() FROM users WHERE id = 1")
	keywords := filterTokens(tokens, chroma.Keyword)
	if len(keywords) < 3 {
		t.Errorf("expected ≥3 Keyword tokens, got %d: %v", len(keywords), keywords)
	}
}

func TestLexerTokenizesStrings(t *testing.T) {
	tokens := tokenize(t, "SELECT 'hello world'")
	strs := filterTokens(tokens, chroma.LiteralStringSingle)
	if len(strs) < 1 {
		t.Errorf("expected ≥1 LiteralStringSingle token, got %d", len(strs))
	}
}

func TestLexerTokenizesComments(t *testing.T) {
	tokens := tokenize(t, "-- this is a comment\nSELECT 1")
	comments := filterTokens(tokens, chroma.CommentSingle)
	if len(comments) < 1 {
		t.Errorf("expected ≥1 CommentSingle token, got %d", len(comments))
	}
}

func TestLexerTokenizesClickHouseTypes(t *testing.T) {
	tokens := tokenize(t, "CREATE TABLE t (id UInt64, name String, tags Array(String))")
	types := filterTokens(tokens, chroma.KeywordType)
	if len(types) < 3 {
		t.Errorf("expected ≥3 KeywordType tokens, got %d: %v", len(types), types)
	}
}

func TestLexerTokenizesEngines(t *testing.T) {
	tokens := tokenize(t, "ENGINE = MergeTree()")
	builtins := filterTokens(tokens, chroma.NameBuiltin)
	if len(builtins) < 1 {
		t.Errorf("expected ≥1 NameBuiltin token, got %d", len(builtins))
	}
}
