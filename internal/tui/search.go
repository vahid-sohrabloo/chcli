package tui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const searchMaxVisible = 10

// searchResult is one filtered entry. matches holds rune indices into display
// for highlighting; allItems[origIdx] is the canonical text returned on Enter.
type searchResult struct {
	origIdx int
	display string
	matches []int
	score   int
}

// SearchModel provides an interactive fuzzy-search overlay.
//
// Matching is subsequence-based: the pattern's runes must appear in order in
// the haystack (case-insensitive). So `slc25` matches `sellerCustomer2025`.
// Results are sorted by score; matches at word starts and consecutive runs
// rank higher, longer haystacks rank lower.
type SearchModel struct {
	active     bool
	query      string
	results    []searchResult
	allItems   []string
	cursor     int
	maxVisible int
}

// NewSearchModel creates a SearchModel ready for use.
func NewSearchModel() *SearchModel {
	return &SearchModel{maxVisible: searchMaxVisible}
}

// Active reports whether the search overlay is currently shown.
func (s *SearchModel) Active() bool { return s.active }

// Activate shows the search overlay pre-populated with the given items.
func (s *SearchModel) Activate(items []string) {
	s.allItems = items
	s.query = ""
	s.cursor = 0
	s.active = true
	s.filter()
}

// Deactivate hides the search overlay.
func (s *SearchModel) Deactivate() {
	s.active = false
}

// Selected returns the original (un-collapsed) text of the highlighted result.
func (s *SearchModel) Selected() string {
	if len(s.results) == 0 || s.cursor >= len(s.results) {
		return ""
	}
	return s.allItems[s.results[s.cursor].origIdx]
}

// Update handles key events while the search overlay is active.
// Returns a (selected, accepted) pair — accepted is true when the user pressed
// Enter and accepted the current selection, false when the overlay was just
// updated or was canceled.
func (s *SearchModel) Update(msg tea.Msg) (selected string, accepted bool) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return "", false
	}

	switch kp.Code {
	case tea.KeyEnter:
		sel := s.Selected()
		s.Deactivate()
		return sel, true

	case tea.KeyEscape:
		s.Deactivate()
		return "", false

	case tea.KeyBackspace, tea.KeyDelete:
		if len(s.query) > 0 {
			runes := []rune(s.query)
			s.query = string(runes[:len(runes)-1])
			s.cursor = 0
			s.filter()
		}

	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}

	case tea.KeyDown:
		if s.cursor < len(s.results)-1 {
			s.cursor++
		}

	default:
		if kp.Mod == 0 && kp.Code > 0 && !isControlCode(kp.Code) {
			s.query += string(kp.Code)
			s.cursor = 0
			s.filter()
		}
	}

	return "", false
}

// filter rebuilds s.results from s.query using fuzzy subsequence match,
// ranked by score descending.
func (s *SearchModel) filter() {
	s.results = s.results[:0]
	if s.query == "" {
		for i, item := range s.allItems {
			s.results = append(s.results, searchResult{origIdx: i, display: collapseWhitespace(item)})
		}
		return
	}
	for i, item := range s.allItems {
		display := collapseWhitespace(item)
		if score, matches, ok := fuzzyMatch(s.query, display); ok {
			s.results = append(s.results, searchResult{
				origIdx: i,
				display: display,
				matches: matches,
				score:   score,
			})
		}
	}
	sort.SliceStable(s.results, func(i, j int) bool {
		return s.results[i].score > s.results[j].score
	})
}

// fuzzyMatch reports whether pattern's runes appear (case-insensitively) in
// text in order. Returns the matched rune-offsets in text and a relevance
// score where higher is better.
//
// Scoring favors:
//   - matches at word starts (after a separator or at index 0): +8 each
//   - consecutive matches (no gap since previous match): +12 each
//   - tight matches (penalty per gap rune since previous match): -2 each
//   - shorter haystacks (length penalty)
//
// The consecutive bonus is intentionally larger than the boundary bonus so
// that compact matches (e.g. "sel" in "SELECT") outrank scattered ones
// (e.g. "s e l" surrounded by spaces in a comment).
func fuzzyMatch(pattern, text string) (score int, matchIdxs []int, ok bool) {
	if pattern == "" {
		return 0, nil, true
	}
	pRunes := []rune(strings.ToLower(pattern))
	tRunes := []rune(strings.ToLower(text))
	if len(pRunes) > len(tRunes) {
		return 0, nil, false
	}
	matchIdxs = make([]int, 0, len(pRunes))
	pi := 0
	prev := -2
	for ti := 0; ti < len(tRunes) && pi < len(pRunes); ti++ {
		if tRunes[ti] != pRunes[pi] {
			continue
		}
		matchIdxs = append(matchIdxs, ti)
		if ti == 0 || isWordBoundary(tRunes[ti-1]) {
			score += 8
		}
		switch {
		case prev == -2:
			// first matched rune — no consecutive/gap accounting
		case ti == prev+1:
			score += 12
		default:
			score -= 2 * (ti - prev - 1)
		}
		prev = ti
		pi++
	}
	if pi < len(pRunes) {
		return 0, nil, false
	}
	score += len(pRunes) * 2
	score -= len(tRunes) / 8
	return score, matchIdxs, true
}

func isWordBoundary(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '_', '-', '.', '/', ',', '(', ')', '[', ']', '{', '}', '"', '\'', '`', ';', ':', '=':
		return true
	}
	return false
}

// isControlCode reports whether the rune is a non-printable control code that
// should not be appended to the query string.
func isControlCode(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// --- Styles ---

var (
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e0af68")).
				Bold(true)

	searchQueryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c0caf5"))

	searchSelStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7aa2f7")).
			Foreground(lipgloss.Color("#1a1b26")).
			Bold(true)

	searchNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89"))

	searchMatchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e0af68")).
				Bold(true)
)

// View renders the search overlay:
//
//	fuzzy: query_text
//	> selected result
//	  other result
func (s *SearchModel) View() string {
	if !s.active {
		return ""
	}

	var sb strings.Builder

	prompt := searchPromptStyle.Render("fuzzy") + ": " + searchQueryStyle.Render(s.query) + "█"
	sb.WriteString(prompt)
	sb.WriteByte('\n')

	if len(s.results) == 0 {
		sb.WriteString(searchNormalStyle.Render("  (no matches)"))
		return sb.String()
	}

	start := max(s.cursor-s.maxVisible+1, 0)
	end := start + s.maxVisible
	if end > len(s.results) {
		end = len(s.results)
		start = max(end-s.maxVisible, 0)
	}

	for i := start; i < end; i++ {
		r := s.results[i]
		text := r.display
		matches := r.matches
		if utf8RuneLen(text) > 120 {
			text, matches = truncateWithMatches(text, matches, 119)
			text += "…"
		}
		if i == s.cursor {
			sb.WriteString(searchSelStyle.Render("> " + text))
		} else {
			sb.WriteString("  ")
			sb.WriteString(renderWithMatches(text, matches))
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

// renderWithMatches paints text with searchNormalStyle, except runes whose
// rune-index is in matches, which use searchMatchStyle. Consecutive runs are
// rendered together to keep ANSI output compact.
func renderWithMatches(text string, matches []int) string {
	if len(matches) == 0 {
		return searchNormalStyle.Render(text)
	}
	hit := make(map[int]bool, len(matches))
	for _, m := range matches {
		hit[m] = true
	}
	runes := []rune(text)
	var sb strings.Builder
	i := 0
	for i < len(runes) {
		j := i
		on := hit[i]
		for j < len(runes) && hit[j] == on {
			j++
		}
		chunk := string(runes[i:j])
		if on {
			sb.WriteString(searchMatchStyle.Render(chunk))
		} else {
			sb.WriteString(searchNormalStyle.Render(chunk))
		}
		i = j
	}
	return sb.String()
}

// truncateWithMatches returns the rune-prefix of text up to maxRunes runes,
// dropping any match indices that fall outside that prefix.
func truncateWithMatches(text string, matches []int, maxRunes int) (string, []int) {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text, matches
	}
	cut := matches
	for len(cut) > 0 && cut[len(cut)-1] >= maxRunes {
		cut = cut[:len(cut)-1]
	}
	return string(runes[:maxRunes]), cut
}

func utf8RuneLen(s string) int {
	return len([]rune(s))
}

// collapseWhitespace flattens a multi-line query to a single line so it fits
// on one search-result row. Newlines, tabs, and runs of spaces collapse to a
// single space — Enter still picks the original multi-line entry from results.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
