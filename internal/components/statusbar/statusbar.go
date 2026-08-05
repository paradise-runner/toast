package statusbar

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	themeButtonLabel = " theme "
	themeButtonWidth = len(themeButtonLabel) // 7

	// settingsButtonWidth is the rendered width of settingsButtonLabel.
	settingsButtonWidth = 3 // space + gear + space
)

// settingsButtonLabel sits at the far bottom-right of the status bar. The
// bundled desktop app (TOAST_GHOSTTY_BUNDLE=1) embeds JetBrains Mono, which
// has no U+2699 GEAR glyph (it would render as '?'); use the trigram-for-heaven
// glyph as a menu/settings substitute there.
func settingsButtonLabel() string {
	if os.Getenv("TOAST_GHOSTTY_BUNDLE") == "1" {
		return " ≡ "
	}
	return " ⚙ "
}

type Model struct {
	theme           *theme.Manager
	width           int
	filename        string
	language        string
	encoding        string
	line, col       int
	modified        bool
	branch          string
	errorCount      int
	warnCount       int
	lspStatus       map[string]messages.LSPServerStatus
	themeButtonX    int
	settingsButtonX int
}

func New(tm *theme.Manager) Model {
	return Model{theme: tm, encoding: "UTF-8", lspStatus: make(map[string]messages.LSPServerStatus), themeButtonX: -1, settingsButtonX: -1}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.settingsButtonX = m.width - settingsButtonWidth
		m.themeButtonX = m.width - settingsButtonWidth - themeButtonWidth
		if m.themeButtonX < 0 {
			m.themeButtonX = -1
			m.settingsButtonX = -1
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if msg.Y == 0 && m.settingsButtonX >= 0 && msg.X >= m.settingsButtonX && msg.X < m.settingsButtonX+settingsButtonWidth {
				return m, func() tea.Msg { return messages.SettingsOpenMsg{} }
			}
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
	right += base.Render(themeButtonLabel) + base.Render(settingsButtonLabel())

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	pad := m.width - leftW - rightW
	if pad < 0 {
		pad = 0
	}
	return tea.NewView(base.Width(m.width).Render(left + base.Render(strings.Repeat(" ", pad)) + right))
}
