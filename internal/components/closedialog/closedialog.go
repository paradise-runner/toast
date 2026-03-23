package closedialog

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const dialogWidth = 44

// Model holds the state of the close-tab confirmation dialog.
type Model struct {
	theme    *theme.Manager
	bufferID int
	path     string
}

// New creates a new close dialog for the given buffer.
func New(tm *theme.Manager, bufferID int, path string) Model {
	return Model{
		theme:    tm,
		bufferID: bufferID,
		path:     path,
	}
}

// Update handles keyboard input for the dialog.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch kp.String() {
	case "s", "enter":
		bufID, path := m.bufferID, m.path
		return m, func() tea.Msg {
			return messages.CloseTabConfirmedMsg{BufferID: bufID, Path: path, Save: true}
		}
	case "d":
		bufID, path := m.bufferID, m.path
		return m, func() tea.Msg {
			return messages.CloseTabConfirmedMsg{BufferID: bufID, Path: path, Save: false}
		}
	case "esc", "c":
		bufID, path := m.bufferID, m.path
		return m, func() tea.Msg {
			return messages.CloseTabConfirmedMsg{BufferID: bufID, Path: path, Cancelled: true}
		}
	}
	return m, nil
}

// Render returns the styled dialog box as a string for overlay composition.
func (m Model) Render() string {
	bg := lipgloss.Color(m.theme.UI("completion_bg"))
	fg := lipgloss.Color(m.theme.UI("completion_fg"))
	border := lipgloss.Color(m.theme.UI("border"))
	sel := lipgloss.Color(m.theme.UI("completion_selected"))

	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	highlight := lipgloss.NewStyle().Background(sel).Foreground(fg).Bold(true)

	innerW := dialogWidth - 4 // border (2) + padding (2)

	filename := filepath.Base(m.path)
	title := "Save \"" + filename + "\" before closing?"
	if len(title) > innerW {
		// Truncate filename if needed
		maxName := innerW - len("Save \"\" before closing?")
		if maxName > 0 && len(filename) > maxName {
			filename = filename[:maxName-1] + "…"
		}
		title = "Save \"" + filename + "\" before closing?"
	}

	pad := func(s string) string {
		w := lipgloss.Width(s)
		if w < innerW {
			return s + strings.Repeat(" ", innerW-w)
		}
		return s
	}

	titleLine := base.Render(pad(title))
	sep := base.Render(strings.Repeat("─", innerW))

	saveBtn := highlight.Render("[S] Save")
	discardBtn := base.Render("  [D] Discard")
	cancelBtn := base.Render("  [Esc] Cancel")

	btnLine := saveBtn + discardBtn + cancelBtn
	btnPad := innerW - lipgloss.Width(btnLine)
	if btnPad > 0 {
		btnLine += base.Render(strings.Repeat(" ", btnPad))
	}

	body := strings.Join([]string{titleLine, sep, btnLine}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(0, 1).
		Width(dialogWidth).
		Render(body)

	return box
}
