package themepicker

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	dialogWidth = 50

	// maxVisibleRows caps the theme list. With dozens of builtin themes the
	// dialog would otherwise grow taller than the terminal and clip the action
	// row, so the list scrolls instead.
	maxVisibleRows = 12

	// Action row button texts and their content-relative X positions.
	// Layout: "  [ confirm ]   [ cancel ]   [o] folder      "
	confirmBtnText  = "[ confirm ]"
	cancelBtnText   = "[ cancel ]"
	folderBtnText   = "[o] folder"
	confirmBtnStart = 2                                     // after leading "  "
	confirmBtnEnd   = confirmBtnStart + len(confirmBtnText) // 13
	cancelBtnStart  = confirmBtnEnd + 3                     // 16 (after "   " gap)
	cancelBtnEnd    = cancelBtnStart + len(cancelBtnText)   // 26
	folderBtnStart  = cancelBtnEnd + 3                      // 29 (after "   " gap)
	folderBtnEnd    = folderBtnStart + len(folderBtnText)   // 39

	// Scroll affordances, drawn in the last content column of the first/last
	// visible row. Both are clickable.
	scrollUpGlyph   = "▲"
	scrollDownGlyph = "▼"
)

// innerWidth is the width available inside the dialog borders. lipgloss Width
// includes the border, so the content area is two columns narrower.
func (m Model) innerWidth() int { return dialogWidth - 2 }

// Model holds the state of the theme picker dialog.
type Model struct {
	themes      []string
	selected    int
	offset      int    // index of the first visible theme row
	activeTheme string // the theme active when the picker was opened (for esc revert)
	themeDir    string
	theme       *theme.Manager
}

// New creates a new theme picker. activeTheme is the currently loaded theme name.
func New(tm *theme.Manager, themeDir, activeTheme string) Model {
	return Model{
		theme:       tm,
		themeDir:    themeDir,
		activeTheme: activeTheme,
	}
}

// Init (re)scans the theme list and positions the cursor on the active theme.
func (m Model) Init() (Model, tea.Cmd) {
	m.themes = theme.Discover(m.themeDir)
	m.selected = 0
	for i, n := range m.themes {
		if n == m.activeTheme {
			m.selected = i
			break
		}
	}
	m.clampOffset()
	return m, nil
}

// ActiveTheme returns the theme name that was active when the picker opened (used to revert on cancel).
func (m Model) ActiveTheme() string { return m.activeTheme }

// visibleRows returns how many theme rows the dialog shows at once.
func (m Model) visibleRows() int {
	return min(len(m.themes), maxVisibleRows)
}

// pageScroll moves the selection one window in dir (-1 up, +1 down) and keeps
// the window following it. The keyboard and the scroll arrows share it.
func (m Model) pageScroll(dir int) (Model, tea.Cmd) {
	next := m.selected + dir*m.visibleRows()
	m.selected = min(max(next, 0), len(m.themes)-1)
	m.clampOffset()
	name := m.themes[m.selected]
	return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }
}

// clampOffset scrolls the window so the selected theme is visible.
func (m *Model) clampOffset() {
	rows := m.visibleRows()
	if rows == 0 {
		m.offset = 0
		return
	}
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+rows {
		m.offset = m.selected - rows + 1
	}
	if maxOffset := len(m.themes) - rows; m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// Dimensions returns the visual outer width and height of the rendered picker.
// Used by app.go to compute the picker's screen position for hit-testing clicks.
func (m Model) Dimensions() (w, h int) {
	return dialogWidth + 2, m.visibleRows() + 4 // +2 borders (width); +4 borders+sep+btn (height)
}

// Update handles key and mouse events for the picker.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.themes) == 0 {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		// Coordinates are picker-local (app.go subtracts the picker's top-left corner).
		// Y=0 top border, Y=1..visibleRows theme rows, Y=rows+1 separator, Y=rows+2 action row.
		rows := m.visibleRows()
		actionRowY := rows + 2
		switch {
		case msg.Y >= 1 && msg.Y <= rows:
			// The last content column of the boundary rows holds the scroll
			// arrows; clicking them pages the list instead of selecting a theme.
			if msg.X-1 == m.innerWidth()-1 {
				if msg.Y == 1 && m.offset > 0 {
					return m.pageScroll(-1)
				}
				if msg.Y == rows && m.offset+rows < len(m.themes) {
					return m.pageScroll(1)
				}
			}
			// Theme row: move selection and preview, but don't confirm.
			row := m.offset + msg.Y - 1
			if row >= len(m.themes) {
				return m, nil
			}
			m.selected = row
			name := m.themes[row]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case msg.Y == actionRowY:
			// Action row: hit-test each button by content-relative X (subtract left border).
			contentX := msg.X - 1
			switch {
			case contentX >= confirmBtnStart && contentX < confirmBtnEnd:
				name := m.themes[m.selected]
				return m, func() tea.Msg { return messages.ThemePickerClosedMsg{ThemeName: name} }
			case contentX >= cancelBtnStart && contentX < cancelBtnEnd:
				orig := m.activeTheme
				return m, func() tea.Msg { return messages.ThemePickerClosedMsg{ThemeName: orig} }
			case contentX >= folderBtnStart && contentX < folderBtnEnd:
				dir := m.themeDir
				return m, func() tea.Msg { openDir(dir); return nil }
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			m.selected--
			if m.selected < 0 {
				m.selected = len(m.themes) - 1
			}
			m.clampOffset()
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case "down", "j":
			m.selected++
			if m.selected >= len(m.themes) {
				m.selected = 0
			}
			m.clampOffset()
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case "home":
			m.selected = 0
			m.clampOffset()
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case "end":
			m.selected = len(m.themes) - 1
			m.clampOffset()
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case "pgup":
			return m.pageScroll(-1)

		case "pgdown":
			return m.pageScroll(1)

		case "enter":
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemePickerClosedMsg{ThemeName: name} }

		case "esc":
			orig := m.activeTheme
			return m, func() tea.Msg { return messages.ThemePickerClosedMsg{ThemeName: orig} }

		case "o", "O":
			dir := m.themeDir
			return m, func() tea.Msg {
				openDir(dir)
				return nil
			}
		}
	}
	return m, nil
}

// Render returns the styled dialog box as a string.
func (m Model) Render() string {
	bg := lipgloss.Color(m.theme.UI("completion_bg"))
	fg := lipgloss.Color(m.theme.UI("completion_fg"))
	sel := lipgloss.Color(m.theme.UI("completion_selected"))
	border := lipgloss.Color(m.theme.UI("border"))

	baseStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	selectedStyle := lipgloss.NewStyle().Background(sel).Foreground(fg)

	// innerW fills the full content area (Width minus 2 border chars, no box padding).
	// Padding is included manually in each row so the selected highlight spans edge-to-edge
	// without lipgloss adding box-colored trailing spaces that break the selection background.
	innerW := m.innerWidth()

	var rows []string
	start := m.offset
	end := min(start+m.visibleRows(), len(m.themes))
	for i := start; i < end; i++ {
		name := m.themes[i]
		// ● marks the original (revert) theme; the highlight tracks the current selection.
		// These can differ after the user navigates away from the starting position.
		marker := "  "
		if name == m.activeTheme {
			marker = "● "
		}
		indicator := ""
		switch {
		case i == start && start > 0:
			indicator = scrollUpGlyph
		case i == end-1 && end < len(m.themes):
			indicator = scrollDownGlyph
		}
		// Leading space replaces the removed Padding(0,1) so the visual indent is preserved.
		label := " " + marker + name
		// Leave the last column to the scroll indicator, then pad to the content
		// width. A row wider than the box is wrapped by lipgloss, which pushes
		// the indicator onto a line of its own.
		target := innerW - lipgloss.Width(indicator)
		if w := lipgloss.Width(label); w > target {
			label = truncateToWidth(label, target)
		} else if w < target {
			label += strings.Repeat(" ", target-w)
		}
		label += indicator
		if i == m.selected {
			rows = append(rows, selectedStyle.Render(label))
		} else {
			rows = append(rows, baseStyle.Render(label))
		}
	}

	// Separator: 1-space indent then dashes, padded to full inner width.
	sep := " " + strings.Repeat("─", innerW-2)
	if w := lipgloss.Width(sep); w < innerW {
		sep += strings.Repeat(" ", innerW-w)
	}
	rows = append(rows, baseStyle.Render(sep))

	// Action row: [ confirm ] and [ cancel ] buttons plus [o] folder shortcut.
	// confirmBtnText is styled with selectedStyle to make it visually distinct.
	// Each segment is rendered separately then concatenated; lipgloss.Width measures
	// the total and any remaining space is filled with baseStyle to reach innerW.
	actionRow := baseStyle.Render("  ") +
		selectedStyle.Render(confirmBtnText) +
		baseStyle.Render("   ") +
		baseStyle.Render(cancelBtnText) +
		baseStyle.Render("   ") +
		baseStyle.Render(folderBtnText)
	if remaining := innerW - lipgloss.Width(actionRow); remaining > 0 {
		actionRow += baseStyle.Render(strings.Repeat(" ", remaining))
	}
	rows = append(rows, actionRow)

	body := strings.Join(rows, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(dialogWidth).
		Render(body)

	return box
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// truncateToWidth shortens label to width display columns, ending in an
// ellipsis when it had to cut. Unusually long (user-defined) theme names would
// otherwise wrap the row and break the dialog layout.
func truncateToWidth(label string, width int) string {
	if lipgloss.Width(label) <= width {
		return label
	}
	const ellipsis = "…"
	limit := width - lipgloss.Width(ellipsis)
	var b strings.Builder
	used := 0
	for _, r := range label {
		w := lipgloss.Width(string(r))
		if used+w > limit {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + ellipsis
}

// openDir opens the given directory in the system file manager.
func openDir(dir string) {
	if dir == "" {
		return
	}
	// Ensure dir exists
	_ = os.MkdirAll(dir, 0o755)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	_ = cmd.Start()
}
