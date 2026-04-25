package tui

import (
	"reflect"
	"testing"
)

func TestFuzzyMatch_Subsequence(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		text     string
		wantOK   bool
		wantIdxs []int // nil = don't check
	}{
		{"abbreviation across word", "slc25", "sellerCustomer2025", true, []int{0, 2, 6, 14, 17}},
		{"prefix match", "sel", "select * from t", true, []int{0, 1, 2}},
		{"case insensitive", "SEL", "select 1", true, []int{0, 1, 2}},
		{"middle subsequence", "sct", "select * from t", true, nil},
		{"exact substring beats scattered", "system", "system reload dictionary", true, nil},
		{"no match", "xyz", "select 1", false, nil},
		{"pattern longer than text", "selectfrom", "sel", false, nil},
		{"empty pattern matches everything", "", "anything", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, idxs, ok := fuzzyMatch(tt.pattern, tt.text)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want %v idxs=%v", ok, tt.wantOK, idxs)
			}
			if !ok {
				return
			}
			if tt.wantIdxs != nil && !reflect.DeepEqual(idxs, tt.wantIdxs) {
				t.Errorf("idxs=%v want %v", idxs, tt.wantIdxs)
			}
		})
	}
}

func TestFuzzyMatch_ScoreRanking(t *testing.T) {
	// Word-start matches outrank middle-of-word matches.
	prefixScore, _, _ := fuzzyMatch("sel", "select 1")
	middleScore, _, _ := fuzzyMatch("sel", "raselraz") // s,e,l appear but not at a boundary
	if !(prefixScore > middleScore) {
		t.Errorf("prefix score %d not greater than middle %d", prefixScore, middleScore)
	}

	// Consecutive matches outrank scattered ones of the same length.
	consec, _, _ := fuzzyMatch("abc", "abcdef")
	scatter, _, _ := fuzzyMatch("abc", "axbxcx")
	if !(consec > scatter) {
		t.Errorf("consecutive %d not greater than scattered %d", consec, scatter)
	}

	// Shorter haystacks rank higher than longer ones for the same pattern.
	short, _, _ := fuzzyMatch("sel", "select")
	long, _, _ := fuzzyMatch("sel", "select something quite long here from a big table somewhere")
	if !(short > long) {
		t.Errorf("short %d not greater than long %d", short, long)
	}
}

func TestSearchModel_FilterOrdersByScore(t *testing.T) {
	s := NewSearchModel()
	s.Activate([]string{
		"-- one off comment then nothing s e l ranks low",
		"select count(*) from users",
		"insert into users values (1)",
		"SELECT 1",
	})

	s.query = "sel"
	s.filter()

	if len(s.results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(s.results))
	}
	first := s.results[0].display
	// `SELECT 1` should rank first: word-start, consecutive, shortest text.
	if first != "SELECT 1" {
		t.Errorf("top result=%q want %q", first, "SELECT 1")
	}
}

func TestSearchModel_EmptyQueryReturnsAll(t *testing.T) {
	items := []string{"a", "b", "c"}
	s := NewSearchModel()
	s.Activate(items)
	if len(s.results) != len(items) {
		t.Fatalf("empty query should return all %d items, got %d", len(items), len(s.results))
	}
}

func TestSearchModel_SelectedReturnsOriginal(t *testing.T) {
	// Multi-line query: collapsed for matching, original returned by Selected().
	original := "SELECT *\n  FROM big_table\n  WHERE id = 1"
	s := NewSearchModel()
	s.Activate([]string{"other", original})

	s.query = "from"
	s.filter()
	s.cursor = 0

	if got := s.Selected(); got != original {
		t.Errorf("Selected()=%q want original %q", got, original)
	}
}

func TestSearchModel_NoMatchesYieldsEmpty(t *testing.T) {
	s := NewSearchModel()
	s.Activate([]string{"select 1", "insert into t"})
	s.query = "zzzzz"
	s.filter()
	if len(s.results) != 0 {
		t.Errorf("expected 0 results, got %d", len(s.results))
	}
	if got := s.Selected(); got != "" {
		t.Errorf("Selected()=%q want empty", got)
	}
}

func TestTruncateWithMatches(t *testing.T) {
	text := "abcdefghij"
	matches := []int{0, 3, 5, 8}
	got, gotMatches := truncateWithMatches(text, matches, 6)
	if got != "abcdef" {
		t.Errorf("text=%q want %q", got, "abcdef")
	}
	if !reflect.DeepEqual(gotMatches, []int{0, 3, 5}) {
		t.Errorf("matches=%v want [0 3 5]", gotMatches)
	}

	// No truncation when text fits.
	got2, gotMatches2 := truncateWithMatches("abc", []int{0, 2}, 10)
	if got2 != "abc" || !reflect.DeepEqual(gotMatches2, []int{0, 2}) {
		t.Errorf("no-op truncation broken: text=%q matches=%v", got2, gotMatches2)
	}
}
