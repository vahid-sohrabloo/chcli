package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/vahid-sohrabloo/chcli/internal/highlight"
)

// InputModel wraps bubbles/textarea for multi-line SQL input with a custom
// prompt and optional syntax highlighting.
type InputModel struct {
	textarea    textarea.Model
	prompt      string
	highlighter *highlight.Highlighter
	submitted   bool
	width       int
}

// NewInputModel creates a new InputModel with the given prompt string and
// optional highlighter.
func NewInputModel(prompt string, highlighter *highlight.Highlighter) *InputModel {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)

	m := &InputModel{
		textarea:    ta,
		prompt:      prompt,
		highlighter: highlighter,
	}

	m.setPromptFunc()

	// Hide the border so the textarea blends into the surrounding layout.
	styles := ta.Styles()
	hiddenBorder := lipgloss.NewStyle().Border(lipgloss.HiddenBorder())
	styles.Focused.Base = hiddenBorder
	styles.Blurred.Base = hiddenBorder
	m.textarea.SetStyles(styles)

	// Focus immediately so the user can start typing.
	m.textarea.Focus()

	return m
}

// setPromptFunc configures the textarea prompt function based on the current
// prompt string. Line 0 gets the full prompt; subsequent lines get "...>"
// aligned to the same width.
func (m *InputModel) setPromptFunc() {
	promptWidth := len([]rune(m.prompt))
	pad := max(promptWidth-5, 0) // 5 = len("...> ")
	continuation := strings.Repeat(" ", pad) + "...> "

	m.textarea.SetPromptFunc(promptWidth, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return m.prompt
		}
		return continuation
	})
}

// InsertText sets the textarea content to the given text (cursor moves to end).
func (m *InputModel) InsertText(text string) {
	m.textarea.SetValue(text)
	m.resizeToContent()
}

// SetValue sets the textarea content to the given text (alias for InsertText).
func (m *InputModel) SetValue(text string) {
	m.textarea.SetValue(text)
	m.resizeToContent()
}

// ReplaceWordAtCursor replaces the word before the cursor with the given text,
// leaving the cursor right after the inserted text.
func (m *InputModel) ReplaceWordAtCursor(prefixLen int, replacement string) {
	// Move cursor left by prefixLen and delete those characters by
	// rebuilding the value around the cursor.
	toCursor := m.ValueToCursor()
	afterCursor := m.textarea.Value()[len(toCursor):]
	beforePrefix := toCursor[:len(toCursor)-prefixLen]

	newValue := beforePrefix + replacement + afterCursor
	m.textarea.SetValue(newValue)

	// Position cursor right after the inserted text.
	targetPos := len(beforePrefix) + len(replacement)
	lines := strings.Split(newValue[:targetPos], "\n")
	targetLine := len(lines) - 1
	targetCol := len([]rune(lines[targetLine]))

	// Navigate to the correct line and column.
	// SetValue puts cursor at end, so we need to move it.
	// First go to start, then to target line, then to target column.
	m.textarea.CursorStart()
	for range targetLine {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursorColumn(targetCol)
}

// Value returns the current textarea content.
func (m *InputModel) Value() string {
	return m.textarea.Value()
}

// Clear resets the textarea content and the submitted flag.
func (m *InputModel) Clear() {
	m.textarea.Reset()
	m.submitted = false
}

// MoveCursorLeft moves the cursor one position to the left.
func (m *InputModel) MoveCursorLeft() {
	col := m.textarea.Column()
	if col > 0 {
		m.textarea.SetCursorColumn(col - 1)
	}
}

// SetWidth sets the textarea and display width.
func (m *InputModel) SetWidth(w int) {
	m.width = w
	m.textarea.SetWidth(w)
}

// CursorScreenX returns the cursor's horizontal screen position
// including border (1) + padding (1) + prompt (2) + column.
func (m *InputModel) CursorScreenX() int {
	borderPadding := 3 // border char + padding on each side from lipgloss
	promptLen := 2     // "❯ "
	return borderPadding + promptLen + m.textarea.Column()
}

// ValueToCursor returns the input text from the start up to the cursor position.
func (m *InputModel) ValueToCursor() string {
	value := m.textarea.Value()
	lines := strings.Split(value, "\n")
	curLine := m.textarea.Line()
	curCol := m.textarea.Column()

	var sb strings.Builder
	for i := 0; i <= curLine && i < len(lines); i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i == curLine {
			runes := []rune(lines[i])
			if curCol > len(runes) {
				curCol = len(runes)
			}
			sb.WriteString(string(runes[:curCol]))
		} else {
			sb.WriteString(lines[i])
		}
	}
	return sb.String()
}

// SetPrompt changes the prompt string and updates the prompt function.
func (m *InputModel) SetPrompt(prompt string) {
	m.prompt = prompt
	m.setPromptFunc()
}

// SetHighlighter replaces the syntax highlighter used for rendering SQL input.
func (m *InputModel) SetHighlighter(hl *highlight.Highlighter) {
	m.highlighter = hl
}

// Focus gives keyboard focus to the input area.
func (m *InputModel) Focus() tea.Cmd {
	return m.textarea.Focus()
}

// Blur removes keyboard focus from the input area.
func (m *InputModel) Blur() {
	m.textarea.Blur()
}

// Submitted returns true when the user has submitted the current input.
func (m *InputModel) Submitted() bool {
	return m.submitted
}

// ResetSubmitted clears the submitted flag without clearing the content.
func (m *InputModel) ResetSubmitted() {
	m.submitted = false
}

// Update processes a bubbletea message.
//
// When the user presses Enter and the current value satisfies isSubmitInput,
// the submitted flag is set and the message is not forwarded to the textarea
// (preventing a newline from being inserted). All other messages are forwarded
// to the underlying textarea.
func (m *InputModel) Update(msg tea.Msg) (*InputModel, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		// Alt+Enter: insert a newline without submitting.
		if kp.Code == tea.KeyEnter && kp.Mod == tea.ModAlt {
			m.textarea.InsertString("\n")
			m.resizeToContent()
			return m, nil
		}

		// Ctrl+U: clear the entire input.
		if kp.Code == 'u' && kp.Mod == tea.ModCtrl {
			m.textarea.SetValue("")
			return m, nil
		}

		// Ctrl+K: kill from cursor to end of line.
		if kp.Code == 'k' && kp.Mod == tea.ModCtrl {
			toCursor := m.ValueToCursor()
			m.textarea.SetValue(toCursor)
			return m, nil
		}

		if kp.Code == tea.KeyEnter {
			if isSubmitInput(m.textarea.Value()) {
				m.submitted = true
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.resizeToContent()
	return m, cmd
}

// resizeToContent adjusts the textarea height to fit the current content.
func (m *InputModel) resizeToContent() {
	lines := max(strings.Count(m.textarea.Value(), "\n")+1, 1)
	m.textarea.SetHeight(lines)
}

var (
	inputBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")). // ANSI bright black (gray)
			Background(lipgloss.Color("0")).       // ANSI black
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7aa2f7")).
				Bold(true)

	inputCursorStyle = lipgloss.NewStyle().Reverse(true)
)

// View renders the input area with syntax-highlighted SQL inside a bordered box.
func (m *InputModel) View() string {
	if m.highlighter == nil {
		return m.textarea.View()
	}

	value := m.textarea.Value()
	lines := strings.Split(value, "\n")
	curLine := m.textarea.Line()
	curCol := m.textarea.Column()

	promptStr := "❯ "
	contStr := "  "

	var sb strings.Builder
	for i, line := range lines {
		// Prompt.
		if i == 0 {
			sb.WriteString(inputPromptStyle.Render(promptStr))
		} else {
			sb.WriteString(inputPromptStyle.Render(contStr))
		}

		// Insert block cursor on the active line.
		if i == curLine && m.textarea.Focused() {
			runes := []rune(line)
			col := min(curCol, len(runes))

			before := string(runes[:col])
			cursorChar := " "
			after := ""
			if col < len(runes) {
				cursorChar = string(runes[col])
				after = string(runes[col+1:])
			}

			if before != "" {
				sb.WriteString(m.highlighter.Highlight(before))
			}
			sb.WriteString(inputCursorStyle.Render(cursorChar))
			if after != "" {
				sb.WriteString(m.highlighter.Highlight(after))
			}
		} else {
			sb.WriteString(m.highlighter.Highlight(line))
		}

		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}

	// Wrap in bordered box.
	content := sb.String()
	lineCount := len(lines)
	box := inputBorder.Width(m.width - 4) // account for border + padding
	result := box.Render(content)

	// Show line/column indicator when the input spans multiple lines.
	if lineCount > 1 {
		countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
		result += "\n" + countStyle.Render(fmt.Sprintf("  line %d/%d", curLine+1, lineCount))
	}

	return result
}
