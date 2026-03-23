// Package gotoline implements a small overlay component for jumping to a
// specific line number in the active buffer.
package gotoline

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const overlayWidth = 35

// Model holds the state of the go-to-line overlay.
type Model struct {
	input     string // digits only
	lineCount int    // total lines in active buffer (for display and clamping)
	open      bool
	theme     *theme.Manager
}

// New creates a new go-to-line Model (starts closed).
func New() Model {
	return Model{}
}

// NewWithTheme creates a new go-to-line Model with a theme manager.
func NewWithTheme(tm *theme.Manager) Model {
	return Model{theme: tm}
}

// Open activates the overlay, setting the total line count for clamping and display.
// Any previous input is cleared.
func (m Model) Open(lineCount int) Model {
	m.open = true
	m.lineCount = lineCount
	m.input = ""
	return m
}

// IsOpen reports whether the overlay is currently visible.
func (m Model) IsOpen() bool { return m.open }

// Update processes key events when the overlay is open.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.Code {
	case tea.KeyEnter:
		m.open = false
		if m.input == "" {
			return m, func() tea.Msg { return messages.GoToLineCancelMsg{} }
		}
		line, err := strconv.Atoi(m.input)
		if err != nil || line < 1 {
			return m, func() tea.Msg { return messages.GoToLineCancelMsg{} }
		}
		// Convert from 1-based to 0-based and clamp.
		line-- // 0-based
		if line < 0 {
			line = 0
		}
		if m.lineCount > 0 && line >= m.lineCount {
			line = m.lineCount - 1
		}
		target := line
		return m, func() tea.Msg { return messages.GoToLineMsg{Line: target} }

	case tea.KeyEscape:
		m.open = false
		return m, func() tea.Msg { return messages.GoToLineCancelMsg{} }

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	}

	// Accept digit characters only.
	r := kp.Code
	if r >= '0' && r <= '9' {
		m.input += string(r)
	}
	return m, nil
}

// View renders the overlay as a string (inner component, not top-level).
// Returns an empty string when the overlay is closed.
func (m Model) View() string {
	if !m.open {
		return ""
	}

	// Build content line: "Go to Line (1–{lineCount}): {input}_"
	cursor := "_"
	guidance := fmt.Sprintf("(1\u2013%d)", m.lineCount)
	content := fmt.Sprintf("Go to Line %s: %s%s", guidance, m.input, cursor)

	// Pad content to inner width.
	innerW := overlayWidth - 4 // border (2) + padding (2)
	contentRunes := []rune(content)
	if len(contentRunes) < innerW {
		content += strings.Repeat(" ", innerW-len(contentRunes))
	}

	// Apply theme colors if available.
	var box string
	if m.theme != nil {
		bgStr := m.theme.UI("completion_bg")
		if bgStr == "" {
			bgStr = "#313244"
		}
		fgStr := m.theme.UI("completion_fg")
		if fgStr == "" {
			fgStr = "#cdd6f4"
		}
		// Use hover_border for a visible contrast against completion_bg.
		// The generic "border" key often matches completion_bg exactly,
		// making the box outline invisible and creating apparent gaps.
		borderStr := m.theme.UI("hover_border")
		if borderStr == "" {
			borderStr = "#585b70"
		}

		bg := lipgloss.Color(bgStr)
		fg := lipgloss.Color(fgStr)
		border := lipgloss.Color(borderStr)

		// Pass raw content directly into the outer box style so that
		// lipgloss can paint the background and padding uniformly.
		// Pre-rendering with an inner style first embeds ANSI reset
		// sequences that break the outer style's right-side background fill.
		box = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			BorderBackground(bg).
			Background(bg).
			Foreground(fg).
			Padding(0, 1).
			Width(overlayWidth).
			Render(content)
	} else {
		// No theme: plain box.
		box = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(overlayWidth).
			Render(content)
	}

	return box
}

// Render is an alias for View, used by app.go for overlay composition.
func (m Model) Render() string { return m.View() }
