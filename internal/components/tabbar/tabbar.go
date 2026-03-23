package tabbar

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	exitButtonIcon  = " ✕ "
	exitButtonWidth = 3 // display width: space + icon + space
)

// Tab represents a single open tab in the tab bar.
type Tab struct {
	BufferID int
	Path     string
	Modified bool
}

// Model is the tab bar component model.
type Model struct {
	theme       *theme.Manager
	width       int
	tabs        []Tab
	active      int
	exitButtonX int
}

// New creates a new tab bar model with the given theme manager.
func New(tm *theme.Manager) Model {
	return Model{
		theme:       tm,
		active:      -1,
		exitButtonX: -1,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.BufferOpenedMsg:
		// Add a new tab if one with this BufferID doesn't already exist.
		for _, t := range m.tabs {
			if t.BufferID == msg.BufferID {
				return m, nil
			}
		}
		m.tabs = append(m.tabs, Tab{
			BufferID: msg.BufferID,
			Path:     msg.Path,
		})
		// Activate the newly opened tab.
		m.active = len(m.tabs) - 1

	case messages.BufferClosedMsg:
		idx := m.indexByID(msg.BufferID)
		if idx < 0 {
			return m, nil
		}
		m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
		// Adjust active index after removal.
		if len(m.tabs) == 0 {
			m.active = -1
		} else if m.active >= len(m.tabs) {
			m.active = len(m.tabs) - 1
		} else if m.active > idx {
			m.active--
		}

	case messages.BufferModifiedMsg:
		idx := m.indexByID(msg.BufferID)
		if idx >= 0 {
			m.tabs[idx].Modified = msg.Modified
		}

	case messages.ActiveBufferChangedMsg:
		idx := m.indexByID(msg.BufferID)
		if idx >= 0 {
			m.active = idx
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.width >= exitButtonWidth {
			m.exitButtonX = m.width - exitButtonWidth
		} else {
			m.exitButtonX = -1
		}

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(msg)
	}

	return m, nil
}

// handleMouseRelease handles mouse release events on the tab bar.
func (m Model) handleMouseRelease(msg tea.MouseReleaseMsg) (Model, tea.Cmd) {
	// Only handle events on row 0 (the tab bar row).
	if msg.Y != 0 {
		return m, nil
	}

	// Exit button at the far right.
	if msg.Button == tea.MouseLeft && m.exitButtonX >= 0 && msg.X >= m.exitButtonX && msg.X < m.exitButtonX+exitButtonWidth {
		return m, func() tea.Msg { return messages.QuitRequestMsg{} }
	}

	tabIdx := m.tabIndexAtX(msg.X)
	if tabIdx < 0 || tabIdx >= len(m.tabs) {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseLeft:
		// Check if click lands on a close button first.
		if closeIdx := m.closeButtonAtX(msg.X); closeIdx >= 0 {
			tab := m.tabs[closeIdx]
			return m, func() tea.Msg {
				return messages.CloseTabRequestMsg{BufferID: tab.BufferID, Path: tab.Path}
			}
		}
		// Otherwise activate the tab.
		m.active = tabIdx
		tab := m.tabs[tabIdx]
		return m, func() tea.Msg {
			return messages.ActiveBufferChangedMsg{
				BufferID: tab.BufferID,
				Path:     tab.Path,
			}
		}

	case tea.MouseMiddle:
		tab := m.tabs[tabIdx]
		return m, func() tea.Msg {
			return messages.CloseTabRequestMsg{BufferID: tab.BufferID, Path: tab.Path}
		}
	}

	return m, nil
}

// tabIndexAtX returns the tab index for a given horizontal pixel/cell offset,
// based on the cumulative rendered widths of tab labels.
func (m Model) tabIndexAtX(x int) int {
	pos := 0
	for i, t := range m.tabs {
		label := m.tabLabel(t)
		// Strip ANSI from label to get display width.
		width := lipgloss.Width(label)
		if x >= pos && x < pos+width {
			return i
		}
		pos += width
	}
	return -1
}

// tabLabel builds the display label for a tab, including a close button.
func (m Model) tabLabel(t Tab) string {
	name := filepath.Base(t.Path)
	if t.Modified {
		return " ● " + name + " × "
	}
	return " " + name + " × "
}

// closeButtonAtX returns the tab index whose close button (×) sits at column x,
// or -1 if x does not land on any close button.
func (m Model) closeButtonAtX(x int) int {
	pos := 0
	for i, t := range m.tabs {
		label := m.tabLabel(t)
		width := lipgloss.Width(label)
		// The label ends with " × " so × is at pos+width-3 and the trailing
		// space is at pos+width-2.  Accept clicks on either cell so the hit
		// target is 2 columns wide instead of 1 — much easier to hit.
		closeCol := pos + width - 3
		if x >= closeCol && x <= pos+width-2 {
			return i
		}
		pos += width
	}
	return -1
}

// View implements tea.Model and renders the tab bar.
func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	activeBG := m.theme.UI("tab_active_bg")
	activeFG := m.theme.UI("tab_active_fg")
	inactiveBG := m.theme.UI("tab_inactive_bg")
	inactiveFG := m.theme.UI("tab_inactive_fg")

	activeStyle := lipgloss.NewStyle()
	if activeBG != "" {
		activeStyle = activeStyle.Background(lipgloss.Color(activeBG))
	}
	if activeFG != "" {
		activeStyle = activeStyle.Foreground(lipgloss.Color(activeFG))
	}

	inactiveStyle := lipgloss.NewStyle()
	if inactiveBG != "" {
		inactiveStyle = inactiveStyle.Background(lipgloss.Color(inactiveBG))
	}
	if inactiveFG != "" {
		inactiveStyle = inactiveStyle.Foreground(lipgloss.Color(inactiveFG))
	}

	var sb strings.Builder
	usedWidth := 0

	for i, t := range m.tabs {
		label := m.tabLabel(t)
		var rendered string
		if i == m.active {
			rendered = activeStyle.Render(label)
		} else {
			rendered = inactiveStyle.Render(label)
		}
		sb.WriteString(rendered)
		usedWidth += lipgloss.Width(rendered)
	}

	// Fill remaining width with inactive background, then exit button at far right.
	remaining := m.width - usedWidth
	if remaining > 0 {
		buttonSpace := exitButtonWidth
		if remaining < buttonSpace {
			buttonSpace = 0
		}
		padWidth := remaining - buttonSpace
		if padWidth > 0 {
			sb.WriteString(inactiveStyle.Render(strings.Repeat(" ", padWidth)))
		}
		if buttonSpace > 0 {
			exitStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(m.theme.UI("tab_active_bg"))).
				Foreground(lipgloss.Color(m.theme.UI("tab_active_fg")))
			sb.WriteString(exitStyle.Render(exitButtonIcon))
		}
	}

	return tea.NewView(sb.String())
}

// NextTab advances to the next tab, wrapping around.
func (m *Model) NextTab() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	m.active = (m.active + 1) % len(m.tabs)
	tab := m.tabs[m.active]
	return func() tea.Msg {
		return messages.ActiveBufferChangedMsg{
			BufferID: tab.BufferID,
			Path:     tab.Path,
		}
	}
}

// PrevTab moves to the previous tab, wrapping around.
func (m *Model) PrevTab() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
	tab := m.tabs[m.active]
	return func() tea.Msg {
		return messages.ActiveBufferChangedMsg{
			BufferID: tab.BufferID,
			Path:     tab.Path,
		}
	}
}

// ActiveTab returns a pointer to the currently active tab, or nil if none.
func (m Model) ActiveTab() *Tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	t := m.tabs[m.active]
	return &t
}

// indexByID finds the slice index of a tab with the given BufferID, or -1.
func (m Model) indexByID(bufferID int) int {
	for i, t := range m.tabs {
		if t.BufferID == bufferID {
			return i
		}
	}
	return -1
}
