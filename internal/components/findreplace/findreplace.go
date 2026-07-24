// Package findreplace implements the in-file find/replace overlay.
package findreplace

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	overlayMinWidth = 54
	overlayMaxWidth = 78
	overlayHeight   = 7

	contentOffsetX = 2
	contentOffsetY = 1

	findRow    = 1
	replaceRow = 2
	optionsRow = 3
)

type inputField int

const (
	findField inputField = iota
	replaceField
)

type textInput struct {
	value  string
	cursor int
}

func (t *textInput) Value() string { return t.value }

func (t *textInput) SetValue(value string) {
	t.value = value
	t.cursor = len([]rune(value))
}

// insert inserts text at the cursor, collapsing multi-line input to the first
// line since the field is single-line.
func (t *textInput) insert(text string) {
	if text == "" {
		return
	}
	if idx := strings.IndexAny(text, "\n\r"); idx >= 0 {
		text = text[:idx]
	}
	if text == "" {
		return
	}
	runes := []rune(t.value)
	in := []rune(text)
	merged := make([]rune, 0, len(runes)+len(in))
	merged = append(merged, runes[:t.cursor]...)
	merged = append(merged, in...)
	merged = append(merged, runes[t.cursor:]...)
	t.value = string(merged)
	t.cursor += len(in)
}

// deleteWordBackward deletes from the cursor back to the start of the
// previous word. Word boundaries follow isWordChar: letters, digits,
// underscore. Returns true when text was deleted.
func (t *textInput) deleteWordBackward() bool {
	if t.cursor == 0 {
		return false
	}
	runes := []rune(t.value)
	pos := t.cursor
	// skip trailing non-word chars
	for pos > 0 && !isWordChar(runes[pos-1]) {
		pos--
	}
	// skip word chars
	for pos > 0 && isWordChar(runes[pos-1]) {
		pos--
	}
	if pos == t.cursor {
		return false
	}
	t.value = string(runes[:pos]) + string(runes[t.cursor:])
	t.cursor = pos
	return true
}

// deleteWordForward deletes from the cursor forward to the end of the
// current word (readline / alt+d style). If the cursor sits on non-word
// characters those are skipped first, then the following word is deleted.
// Returns true when text was deleted.
func (t *textInput) deleteWordForward() bool {
	runes := []rune(t.value)
	if t.cursor >= len(runes) {
		return false
	}
	pos := t.cursor
	// If cursor is on a non-word char, skip non-word chars first.
	if !isWordChar(runes[pos]) {
		for pos < len(runes) && !isWordChar(runes[pos]) {
			pos++
		}
	}
	// Skip word chars.
	for pos < len(runes) && isWordChar(runes[pos]) {
		pos++
	}
	if pos == t.cursor {
		return false
	}
	t.value = string(runes[:t.cursor]) + string(runes[pos:])
	return true
}

func (t *textInput) handleKey(msg tea.KeyPressMsg) bool {
	if msg.Mod.Contains(tea.ModSuper) {
		return false
	}

	runes := []rune(t.value)
	switch msg.String() {
	case "backspace":
		if t.cursor > 0 {
			runes = append(runes[:t.cursor-1], runes[t.cursor:]...)
			t.value = string(runes)
			t.cursor--
			return true
		}
	case "delete":
		if t.cursor < len(runes) {
			runes = append(runes[:t.cursor], runes[t.cursor+1:]...)
			t.value = string(runes)
			return true
		}
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		if t.cursor < len(runes) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len(runes)
	case "ctrl+u":
		if t.cursor > 0 {
			t.value = string(runes[t.cursor:])
			t.cursor = 0
			return true
		}
	case "ctrl+k":
		if t.cursor < len(runes) {
			t.value = string(runes[:t.cursor])
			return true
		}
	case "ctrl+w", "ctrl+backspace", "alt+backspace":
		return t.deleteWordBackward()
	case "alt+d":
		return t.deleteWordForward()
	default:
		if msg.Text == "" {
			return false
		}
		text := []rune(msg.Text)
		runes = append(runes[:t.cursor], append(text, runes[t.cursor:]...)...)
		t.value = string(runes)
		t.cursor += len(text)
		return true
	}
	return false
}

func (t textInput) View(width int, placeholder string, active bool) string {
	if width < 1 {
		width = 1
	}
	runes := []rune(t.value)
	var rendered string
	if t.value == "" {
		rendered = placeholder
		if active {
			rendered += "█"
		}
	} else if active {
		if t.cursor >= len(runes) {
			rendered = t.value + "█"
		} else {
			rendered = string(runes[:t.cursor]) + "█" + string(runes[t.cursor+1:])
		}
	} else {
		rendered = t.value
	}
	renderedRunes := []rune(rendered)
	if len(renderedRunes) > width {
		rendered = string(renderedRunes[len(renderedRunes)-width:])
	}
	if lipgloss.Width(rendered) < width {
		rendered += strings.Repeat(" ", width-lipgloss.Width(rendered))
	}
	return rendered
}

// Model is the in-file find/replace overlay state.
type Model struct {
	theme     *theme.Manager
	width     int
	height    int
	active    bool
	focus     inputField
	find      textInput
	replace   textInput
	matchCase bool
	wholeWord bool
	current   int
	total     int
}

// New creates a closed find/replace overlay.
func New(tm *theme.Manager) Model {
	return Model{theme: tm}
}

// Open activates the overlay and optionally seeds the find query.
func (m Model) Open(seed string) Model {
	m.active = true
	m.focus = findField
	if seed != "" && !strings.Contains(seed, "\n") {
		m.find.SetValue(seed)
	}
	return m
}

// IsOpen reports whether the overlay is active.
func (m Model) IsOpen() bool { return m.active }

// Query returns the current find query.
func (m Model) Query() string { return m.find.Value() }

// Replacement returns the current replacement text.
func (m Model) Replacement() string { return m.replace.Value() }

// MatchCase reports whether match-case is enabled.
func (m Model) MatchCase() bool { return m.matchCase }

// WholeWord reports whether whole-word matching is enabled.
func (m Model) WholeWord() bool { return m.wholeWord }

// SetMatchStatus updates the rendered current/total match count.
func (m *Model) SetMatchStatus(current, total int) {
	m.current = current
	m.total = total
}

// Dimensions returns the rendered overlay size.
func (m Model) Dimensions() (w, h int) {
	return m.overlayWidth(), overlayHeight
}

// Update handles overlay input and emits find/replace messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case messages.FindReplaceOpenMsg:
		m.active = true
		m.focus = findField

	case messages.FindReplaceCloseMsg:
		m.active = false

	case tea.PasteMsg:
		if !m.active {
			break
		}
		text := msg.Content
		if idx := strings.IndexAny(text, "\n\r"); idx >= 0 {
			text = text[:idx]
		}
		if text == "" {
			break
		}
		if m.focus == findField {
			prev := m.find.Value()
			m.find.insert(text)
			if m.find.Value() != prev {
				return m, m.queryChangedCmd()
			}
		} else {
			m.replace.insert(text)
		}

	case tea.KeyPressMsg:
		if !m.active {
			break
		}
		switch {
		case msg.String() == "escape" || msg.Code == tea.KeyEscape:
			m.active = false
			return m, func() tea.Msg { return messages.FindReplaceCloseMsg{} }
		case msg.String() == "tab":
			if m.focus == findField {
				m.focus = replaceField
			} else {
				m.focus = findField
			}
		case msg.String() == "shift+tab":
			if m.focus == replaceField {
				m.focus = findField
			} else {
				m.focus = replaceField
			}
		case isPrevMatchKey(msg):
			return m, func() tea.Msg { return messages.FindReplaceNavigateMsg{Forward: false} }
		case isNextMatchKey(msg):
			return m, func() tea.Msg { return messages.FindReplaceNavigateMsg{Forward: true} }
		case isReplaceAllKey(msg):
			return m, m.replaceAllCmd()
		case isReplaceOneKey(msg):
			return m, m.replaceCurrentCmd()
		case isMatchCaseKey(msg):
			m.matchCase = !m.matchCase
			return m, m.queryChangedCmd()
		case isWholeWordKey(msg):
			m.wholeWord = !m.wholeWord
			return m, m.queryChangedCmd()
		default:
			prevQuery := m.find.Value()
			if m.focus == findField {
				if m.find.handleKey(msg) && m.find.Value() != prevQuery {
					return m, m.queryChangedCmd()
				}
			} else {
				m.replace.handleKey(msg)
			}
		}

	case tea.MouseClickMsg:
		if !m.active || msg.Button != tea.MouseLeft {
			break
		}
		return m.handleMouseClick(msg)
	}
	return m, nil
}

func (m Model) handleMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	contentX := msg.X - contentOffsetX
	contentY := msg.Y - contentOffsetY
	switch contentY {
	case findRow:
		m.focus = findField
	case replaceRow:
		m.focus = replaceField
	case optionsRow:
		return m.handleOptionsClick(contentX)
	}
	return m, nil
}

func (m Model) handleOptionsClick(contentX int) (Model, tea.Cmd) {
	line := optionsLine(false, false)
	caseStart := 0
	caseEnd := strings.Index(line, "   ")
	wordStart := strings.Index(line, "Whole word")
	if wordStart >= 4 {
		wordStart -= 4
	}
	wordEnd := len(line)

	switch {
	case contentX >= caseStart && contentX < caseEnd:
		m.matchCase = !m.matchCase
		return m, m.queryChangedCmd()
	case contentX >= wordStart && contentX < wordEnd:
		m.wholeWord = !m.wholeWord
		return m, m.queryChangedCmd()
	default:
		return m, nil
	}
}

func (m Model) queryChangedCmd() tea.Cmd {
	query := m.find.Value()
	matchCase := m.matchCase
	wholeWord := m.wholeWord
	return func() tea.Msg {
		return messages.FindReplaceQueryChangedMsg{
			Query:     query,
			MatchCase: matchCase,
			WholeWord: wholeWord,
		}
	}
}

func (m Model) replaceCurrentCmd() tea.Cmd {
	query := m.find.Value()
	replacement := m.replace.Value()
	matchCase := m.matchCase
	wholeWord := m.wholeWord
	return func() tea.Msg {
		return messages.FindReplaceReplaceCurrentMsg{
			Query:       query,
			Replacement: replacement,
			MatchCase:   matchCase,
			WholeWord:   wholeWord,
		}
	}
}

// isWordChar returns true for letters, digits, and underscore.
func isWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (m Model) replaceAllCmd() tea.Cmd {
	query := m.find.Value()
	replacement := m.replace.Value()
	matchCase := m.matchCase
	wholeWord := m.wholeWord
	return func() tea.Msg {
		return messages.FindReplaceReplaceAllMsg{
			Query:       query,
			Replacement: replacement,
			MatchCase:   matchCase,
			WholeWord:   wholeWord,
		}
	}
}

func isNextMatchKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "enter" || msg.String() == "down" ||
		msg.String() == "f3" || msg.String() == "ctrl+n"
}

func isPrevMatchKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "shift+enter" || msg.String() == "up" ||
		msg.String() == "shift+f3" || msg.String() == "ctrl+p"
}

func isReplaceOneKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+r" ||
		(msg.Mod.Contains(tea.ModSuper) && msg.Code == 'r' && !msg.Mod.Contains(tea.ModShift))
}

func isReplaceAllKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+shift+r" ||
		(msg.Mod.Contains(tea.ModSuper) && msg.Code == 'r' && msg.Mod.Contains(tea.ModShift))
}

func isMatchCaseKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "alt+c" || (msg.Mod.Contains(tea.ModAlt) && msg.Code == 'c')
}

func isWholeWordKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "alt+w" || (msg.Mod.Contains(tea.ModAlt) && msg.Code == 'w')
}

// View renders the overlay.
func (m Model) View() string {
	if !m.active {
		return ""
	}

	width := m.overlayWidth()
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}

	bg := ""
	fg := ""
	border := ""
	muted := ""
	selectedBG := ""
	selectedFG := ""
	if m.theme != nil {
		bg = m.theme.UI("completion_bg")
		if bg == "" {
			bg = m.theme.UI("background")
		}
		fg = m.theme.UI("completion_fg")
		if fg == "" {
			fg = m.theme.UI("foreground")
		}
		border = m.theme.UI("find_replace_border")
		if border == "" {
			border = m.theme.UI("hover_border")
		}
		muted = m.theme.UI("statusbar_fg")
		selectedBG = m.theme.UI("sidebar_selected_bg")
		selectedFG = m.theme.UI("sidebar_selected_fg")
	}

	base := lipgloss.NewStyle()
	if bg != "" {
		base = base.Background(lipgloss.Color(bg))
	}
	if fg != "" {
		base = base.Foreground(lipgloss.Color(fg))
	}
	mutedStyle := base.Copy()
	if muted != "" {
		mutedStyle = mutedStyle.Foreground(lipgloss.Color(muted))
	}

	findStyle := base.Copy()
	replaceStyle := base.Copy()
	if selectedBG != "" {
		if m.focus == findField {
			findStyle = findStyle.Background(lipgloss.Color(selectedBG))
		} else {
			replaceStyle = replaceStyle.Background(lipgloss.Color(selectedBG))
		}
	}
	if selectedFG != "" {
		if m.focus == findField {
			findStyle = findStyle.Foreground(lipgloss.Color(selectedFG))
		} else {
			replaceStyle = replaceStyle.Foreground(lipgloss.Color(selectedFG))
		}
	}

	count := "0/0"
	if m.total > 0 {
		count = fmt.Sprintf("%d/%d", m.current, m.total)
	}
	title := "Find / Replace in File"
	headerPad := innerWidth - lipgloss.Width(title) - lipgloss.Width(count)
	if headerPad < 1 {
		headerPad = 1
	}

	inputWidth := innerWidth - len("Replace ")
	if inputWidth < 1 {
		inputWidth = 1
	}

	lines := []string{
		base.Bold(true).Render(title) + base.Render(strings.Repeat(" ", headerPad)) + mutedStyle.Render(count),
		base.Render("Find    ") + findStyle.Render(m.find.View(inputWidth, "", m.focus == findField)),
		base.Render("Replace ") + replaceStyle.Render(m.replace.View(inputWidth, "", m.focus == replaceField)),
		mutedStyle.Render(optionsLine(m.matchCase, m.wholeWord)),
		mutedStyle.Render("Enter/down next  Up previous  Ctrl+R replace  Ctrl+Shift+R all  Esc close"),
	}

	for i, line := range lines {
		lines[i] = base.Render(" ") + fitLine(line, innerWidth, base) + base.Render(" ")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width)
	if bg != "" {
		box = box.Background(lipgloss.Color(bg)).BorderBackground(lipgloss.Color(bg))
	}
	if fg != "" {
		box = box.Foreground(lipgloss.Color(fg))
	}
	if border != "" {
		box = box.BorderForeground(lipgloss.Color(border))
	}
	return box.Render(strings.Join(lines, "\n"))
}

// Render is an alias for View, used by the app overlay layer.
func (m Model) Render() string { return m.View() }

func (m Model) overlayWidth() int {
	width := m.width - 8
	if width < overlayMinWidth {
		width = overlayMinWidth
	}
	if width > overlayMaxWidth {
		width = overlayMaxWidth
	}
	if m.width > 0 && width > m.width {
		width = m.width
	}
	if width < 1 {
		width = 1
	}
	return width
}

func fitLine(line string, width int, padStyle lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	if lipgloss.Width(line) > width {
		return ansi.Truncate(line, width, "")
	}
	if lipgloss.Width(line) < width {
		return line + padStyle.Render(strings.Repeat(" ", width-lipgloss.Width(line)))
	}
	return line
}

func optionsLine(matchCase, wholeWord bool) string {
	caseState := "[ ]"
	if matchCase {
		caseState = "[x]"
	}
	wordState := "[ ]"
	if wholeWord {
		wordState = "[x]"
	}
	return fmt.Sprintf("%s Match case (alt+c)   %s Whole word (alt+w)", caseState, wordState)
}
