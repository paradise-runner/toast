// Package lspinstall implements the managed language-server install prompt.
package lspinstall

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const promptWidth = 42

// Model holds the currently visible install prompt.
type Model struct {
	theme      *theme.Manager
	language   string
	name       string
	message    string
	visible    bool
	installing bool
	failed     bool
	queue      []installPrompt
}

type installPrompt struct{ language, name string }

// New creates an empty install prompt.
func New(tm *theme.Manager) Model { return Model{theme: tm} }

// Visible reports whether the prompt is on screen.
func (m Model) Visible() bool { return m.visible }

// Show opens a prompt for a missing language server.
func (m *Model) Show(language, name string) {
	if m.visible {
		if m.language == language {
			return
		}
		for _, queued := range m.queue {
			if queued.language == language {
				return
			}
		}
		m.queue = append(m.queue, installPrompt{language: language, name: name})
		return
	}
	m.show(language, name)
}

func (m *Model) show(language, name string) {
	m.language = language
	m.name = name
	m.message = ""
	m.visible = true
	m.installing = false
	m.failed = false
}

// Update handles prompt input and installation status.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.visible {
			return m, nil
		}
		switch msg.String() {
		case "enter", "i":
			if !m.installing {
				m.installing = true
				m.failed = false
				language := m.language
				return m, func() tea.Msg { return messages.LSPInstallRequestMsg{Language: language} }
			}
		case "esc", "n":
			m.advance()
		}

	case tea.MouseClickMsg:
		if m.visible && msg.Button == tea.MouseLeft && msg.Y == 4 {
			if msg.X >= 2 && msg.X < 13 && !m.installing {
				m.installing = true
				language := m.language
				return m, func() tea.Msg { return messages.LSPInstallRequestMsg{Language: language} }
			}
			if msg.X >= 15 && msg.X < 26 {
				m.advance()
			}
		}

	case messages.LSPInstallStatusMsg:
		if msg.Language != m.language {
			return m, nil
		}
		switch msg.Status {
		case messages.LSPInstallRunning:
			m.installing = true
			m.message = "Installing…"
		case messages.LSPInstallSucceeded:
			m.installing = false
			m.message = "Installed; starting server…"
		case messages.LSPInstallFailed:
			m.installing = false
			m.failed = true
			m.message = strings.Join(strings.Fields(msg.Message), " ")
		}

	case messages.LSPServerStatusMsg:
		if msg.Language == m.language && msg.Status == messages.LSPServerReady {
			m.advance()
		}
	}
	return m, nil
}

func (m *Model) advance() {
	if len(m.queue) == 0 {
		m.visible = false
		return
	}
	next := m.queue[0]
	m.queue = m.queue[1:]
	m.show(next.language, next.name)
}

// Dimensions returns the rendered outer dimensions.
func (m Model) Dimensions() (int, int) { return promptWidth + 2, 6 }

// Render returns the styled prompt.
func (m Model) Render() string {
	if !m.visible {
		return ""
	}
	bg := lipgloss.Color(m.theme.UI("completion_bg"))
	fg := lipgloss.Color(m.theme.UI("completion_fg"))
	border := lipgloss.Color(m.theme.UI("hover_border"))
	selected := lipgloss.Color(m.theme.UI("completion_selected"))
	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	action := lipgloss.NewStyle().Background(selected).Foreground(fg).Bold(true)

	title := fmt.Sprintf(" Language support for %s", m.language)
	detail := fmt.Sprintf(" Install %s for this language?", m.name)
	status := m.message
	if status == "" {
		status = "Ctrl-hover symbols, then Ctrl-click to jump."
	}
	buttons := action.Render(" [I] Install ") + base.Render("  [N] Not now")
	lines := []string{pad(title, promptWidth), pad(detail, promptWidth), pad(status, promptWidth), pad(buttons, promptWidth)}
	body := base.Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		BorderBackground(bg).Background(bg).Width(promptWidth).Render(body)
}

func pad(s string, width int) string {
	if lipgloss.Width(s) > width {
		runes := []rune(s)
		for len(runes) > 1 && lipgloss.Width(string(runes)+"…") > width {
			runes = runes[:len(runes)-1]
		}
		s = string(runes) + "…"
	}
	if n := width - lipgloss.Width(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}
