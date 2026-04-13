package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const searchMaxVisible = 10

// SearchModel provides an interactive search overlay.
type SearchModel struct {
	active     bool
	query      string   // current search text
	results    []string // filtered entries
	allItems   []string // full list
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

// Selected returns the text of the currently highlighted result, or empty string.
func (s *SearchModel) Selected() string {
	if len(s.results) == 0 || s.cursor >= len(s.results) {
		return ""
	}
	return s.results[s.cursor]
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
			// Remove last rune (safe for multi-byte runes).
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
		// Append printable characters.
		if kp.Mod == 0 && kp.Code > 0 && !isControlCode(kp.Code) {
			s.query += string(kp.Code)
			s.cursor = 0
			s.filter()
		}
	}

	return "", false
}

// filter rebuilds s.results based on s.query (case-insensitive substring match).
func (s *SearchModel) filter() {
	if s.query == "" {
		// Show all items when query is empty.
		s.results = make([]string, len(s.allItems))
		copy(s.results, s.allItems)
		return
	}

	lower := strings.ToLower(s.query)
	s.results = s.results[:0]
	for _, item := range s.allItems {
		if strings.Contains(strings.ToLower(item), lower) {
			s.results = append(s.results, item)
		}
	}
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
)

// View renders the search overlay:
//
//	reverse-i-search: query_text
//	> selected result
//	  other result
func (s *SearchModel) View() string {
	if !s.active {
		return ""
	}

	var sb strings.Builder

	// Header line: prompt + query.
	prompt := searchPromptStyle.Render("reverse-i-search") + ": " + searchQueryStyle.Render(s.query) + "█"
	sb.WriteString(prompt)
	sb.WriteByte('\n')

	if len(s.results) == 0 {
		sb.WriteString(searchNormalStyle.Render("  (no matches)"))
		return sb.String()
	}

	// Determine visible window around cursor.
	start := max(s.cursor-s.maxVisible+1, 0)
	end := start + s.maxVisible
	if end > len(s.results) {
		end = len(s.results)
		start = max(end-s.maxVisible, 0)
	}

	for i := start; i < end; i++ {
		text := truncateStr(s.results[i], 120)
		if i == s.cursor {
			sb.WriteString(searchSelStyle.Render("> " + text))
		} else {
			sb.WriteString(searchNormalStyle.Render("  " + text))
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}
