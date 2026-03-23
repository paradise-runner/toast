package breadcrumbs

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

type Model struct {
	theme   *theme.Manager
	width   int
	path    string
	rootDir string
}

func New(tm *theme.Manager, rootDir string) Model { return Model{theme: tm, rootDir: rootDir} }
func (m Model) Init() tea.Cmd                     { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case messages.ActiveBufferChangedMsg:
		m.path = msg.Path
	}
	return m, nil
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
	return tea.NewView(base.Width(m.width).Render(strings.Join(rendered, sep)))
}
