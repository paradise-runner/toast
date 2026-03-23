package quitdialog

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	dialogWidth = 50 // .Width() content width; outer = dialogWidth+2

	// Button row: "  [S] Save & Quit  [Q] Quit Anyway  [Esc] Cancel"
	saveBtnText   = "[S] Save & Quit"
	quitBtnText   = "[Q] Quit Anyway"
	cancelBtnText = "[Esc] Cancel"

	// Content-relative X positions (contentX = msg.X - 1).
	saveBtnStart   = 2
	saveBtnEnd     = saveBtnStart + len(saveBtnText)    // 17
	quitBtnStart   = saveBtnEnd + 2                     // 19
	quitBtnEnd     = quitBtnStart + len(quitBtnText)    // 34
	cancelBtnStart = quitBtnEnd + 2                     // 36
	cancelBtnEnd   = cancelBtnStart + len(cancelBtnText) // 48

	// Y=0 top border, Y=1 title, Y=2 separator, Y=3 button row, Y=4 bottom border.
	actionRowY = 3
)

// Model holds the state of the quit confirmation dialog.
type Model struct {
	theme *theme.Manager
	path  string // currently active file path
}

// New creates a quit dialog for the given file path.
func New(tm *theme.Manager, path string) Model {
	return Model{theme: tm, path: path}
}

// Dimensions returns the outer rendered width and height, used by the app
// layer to centre the overlay and hit-test mouse clicks.
func (m Model) Dimensions() (w, h int) {
	return dialogWidth + 2, 5
}

// Update handles keyboard and mouse input for the dialog.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "s", "enter":
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Save: true} }
		case "q":
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Save: false} }
		case "esc", "c":
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Cancelled: true} }
		}

	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if msg.Y != actionRowY {
			return m, nil
		}
		// Subtract 1 for the left border to get content-relative X.
		contentX := msg.X - 1
		switch {
		case contentX >= saveBtnStart && contentX < saveBtnEnd:
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Save: true} }
		case contentX >= quitBtnStart && contentX < quitBtnEnd:
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Save: false} }
		case contentX >= cancelBtnStart && contentX < cancelBtnEnd:
			return m, func() tea.Msg { return messages.QuitConfirmedMsg{Cancelled: true} }
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

	// innerW is the full content width (no Padding on the box; padding is manual).
	innerW := dialogWidth

	filename := filepath.Base(m.path)
	if filename == "" {
		filename = "untitled"
	}
	title := " Save \"" + filename + "\" before quitting?"
	if lipgloss.Width(title) > innerW {
		maxName := innerW - lipgloss.Width(" Save \"\" before quitting?")
		if maxName > 0 && len(filename) > maxName {
			filename = filename[:maxName-1] + "…"
		}
		title = " Save \"" + filename + "\" before quitting?"
	}
	if lipgloss.Width(title) < innerW {
		title += strings.Repeat(" ", innerW-lipgloss.Width(title))
	}
	titleLine := base.Render(title)

	sep := " " + strings.Repeat("─", innerW-4)
	if lipgloss.Width(sep) < innerW {
		sep += strings.Repeat(" ", innerW-lipgloss.Width(sep))
	}
	sepLine := base.Render(sep)

	// Button row: manually render segments and pad to innerW.
	btnRow := base.Render("  ") +
		highlight.Render(saveBtnText) +
		base.Render("  ") +
		base.Render(quitBtnText) +
		base.Render("  ") +
		base.Render(cancelBtnText)
	if remaining := innerW - lipgloss.Width(btnRow); remaining > 0 {
		btnRow += base.Render(strings.Repeat(" ", remaining))
	}

	body := strings.Join([]string{titleLine, sepLine, btnRow}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(dialogWidth).
		Render(body)
}
