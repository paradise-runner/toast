package breadcrumbs

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	previewButtonLabel  = " Preview "
	previewButtonWidth  = len(previewButtonLabel) // 9
)

type Model struct {
	theme       *theme.Manager
	width       int
	path        string
	rootDir     string
	previewOpen bool
	buttonX     int // screen X of the preview button, -1 when not shown
}

func New(tm *theme.Manager, rootDir string) Model {
	return Model{theme: tm, rootDir: rootDir, buttonX: -1}
}

func (m Model) Init() tea.Cmd { return nil }

// SetPreviewOpen updates the preview state so the button renders correctly.
func (m *Model) SetPreviewOpen(open bool) { m.previewOpen = open }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.updateButtonX()
	case messages.ActiveBufferChangedMsg:
		m.path = msg.Path
		// Close preview when switching to a non-markdown file.
		if !isMarkdownPath(m.path) {
			m.previewOpen = false
		}
		m.updateButtonX()
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && m.buttonX >= 0 &&
			msg.X >= m.buttonX && msg.X < m.buttonX+previewButtonWidth {
			return m, func() tea.Msg { return messages.MarkdownPreviewToggleMsg{} }
		}
	}
	return m, nil
}

// updateButtonX recalculates the screen X of the preview button.
// The button is always flush-right, so its position is simply width - buttonWidth.
// Set to -1 when the button should not be shown.
func (m *Model) updateButtonX() {
	if isMarkdownPath(m.path) && m.width >= previewButtonWidth {
		m.buttonX = m.width - previewButtonWidth
	} else {
		m.buttonX = -1
	}
}

func (m Model) View() tea.View {
	fg := lipgloss.Color(m.theme.UI("breadcrumbs_fg"))
	activeFG := lipgloss.Color(m.theme.UI("breadcrumbs_active_fg"))
	bg := lipgloss.Color(m.theme.UI("background"))
	base := lipgloss.NewStyle().Background(bg)
	inactive := base.Foreground(fg)
	active := base.Foreground(activeFG)
	sep := inactive.Render(" / ")

	if m.path == "" {
		return tea.NewView(base.Width(m.width).Render(""))
	}

	rel, err := filepath.Rel(m.rootDir, m.path)
	if err != nil {
		rel = m.path
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	var rendered []string
	for i, part := range parts {
		if i == len(parts)-1 {
			rendered = append(rendered, active.Render(part))
		} else {
			rendered = append(rendered, inactive.Render(part))
		}
	}
	crumbs := strings.Join(rendered, sep)

	// Only show the preview button for markdown files.
	if m.buttonX < 0 {
		return tea.NewView(base.Width(m.width).Render(crumbs))
	}

	// Build the preview button, highlighted when preview is active.
	var btnStyle lipgloss.Style
	if m.previewOpen {
		btnStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(m.theme.UI("breadcrumbs_active_fg"))).
			Foreground(lipgloss.Color(m.theme.UI("background")))
	} else {
		btnStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(m.theme.UI("background"))).
			Foreground(lipgloss.Color(m.theme.UI("breadcrumbs_fg")))
	}
	btn := btnStyle.Render(previewButtonLabel)

	crumbsW := lipgloss.Width(crumbs)
	pad := m.width - crumbsW - previewButtonWidth
	if pad < 0 {
		pad = 0
	}

	return tea.NewView(base.Width(m.width).Render(
		crumbs + base.Render(strings.Repeat(" ", pad)) + btn,
	))
}

// isMarkdownPath returns true for .md / .markdown / .mdx files.
func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".mdx")
}
