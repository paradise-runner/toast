package statusbar

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	themeButtonLabel = " theme "
	themeButtonWidth = len(themeButtonLabel) // 7
)

type Model struct {
	theme        *theme.Manager
	width        int
	filename     string
	language     string
	encoding     string
	line, col    int
	modified     bool
	branch       string
	errorCount   int
	warnCount    int
	lspStatus    map[string]messages.LSPServerStatus
	themeButtonX int
}

func New(tm *theme.Manager) Model {
	return Model{theme: tm, encoding: "UTF-8", lspStatus: make(map[string]messages.LSPServerStatus), themeButtonX: -1}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.themeButtonX = m.width - themeButtonWidth
		if m.themeButtonX < 0 {
			m.themeButtonX = -1
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if msg.Y == 0 && m.themeButtonX >= 0 && msg.X >= m.themeButtonX && msg.X < m.themeButtonX+themeButtonWidth {
				return m, func() tea.Msg { return messages.ThemePickerOpenMsg{} }
			}
		}
	case messages.ActiveBufferChangedMsg:
		m.filename = msg.Path
		m.line = 0
		m.col = 0
		m.modified = false
	case messages.BufferModifiedMsg:
		m.modified = msg.Modified
	case messages.DiagnosticsUpdatedMsg:
		m.errorCount = 0
		m.warnCount = 0
		for _, d := range msg.Diagnostics {
			switch d.Severity {
			case 1:
				m.errorCount++
			case 2:
				m.warnCount++
			}
		}
	case messages.GitStatusUpdatedMsg:
		m.branch = msg.Branch
	case messages.LSPServerStatusMsg:
		m.lspStatus[msg.Language] = msg.Status
	}
	return m, nil
}

func (m *Model) SetCursor(line, col int) { m.line = line; m.col = col }

func (m Model) View() tea.View {
	bg := lipgloss.Color(m.theme.UI("statusbar_bg"))
	fg := lipgloss.Color(m.theme.UI("statusbar_fg"))
	errColor := lipgloss.Color(m.theme.UI("diagnostic_error"))
	warnColor := lipgloss.Color(m.theme.UI("diagnostic_warning"))
	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	sep := base.Render("  ")

	filename := m.filename
	if filename == "" {
		filename = "untitled"
	}
	modified := ""
	if m.modified {
		modified = " ●"
	}
	left := base.Render(filename+modified) + sep
	if m.language != "" {
		left += base.Render(m.language) + sep
	}
	left += base.Render(m.encoding) + sep
	left += base.Render(fmt.Sprintf("Ln %d, Col %d", m.line+1, m.col+1))

	right := ""
	if m.branch != "" {
		right += base.Render(" "+m.branch) + sep
	}
	if m.errorCount > 0 {
		right += lipgloss.NewStyle().Background(bg).Foreground(errColor).Render(fmt.Sprintf("✕ %d", m.errorCount)) + sep
	}
	if m.warnCount > 0 {
		right += lipgloss.NewStyle().Background(bg).Foreground(warnColor).Render(fmt.Sprintf("⚠ %d", m.warnCount)) + sep
	}
	right += base.Render(themeButtonLabel)

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := m.width - leftW - rightW
	if pad < 0 {
		pad = 0
	}
	return tea.NewView(base.Width(m.width).Render(left + base.Render(strings.Repeat(" ", pad)) + right))
}
