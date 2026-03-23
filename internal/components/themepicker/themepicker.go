package themepicker

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	dialogWidth = 50

	// Action row button texts and their content-relative X positions.
	// Layout: "  [ confirm ]   [ cancel ]   [o] folder      "
	confirmBtnText  = "[ confirm ]"
	cancelBtnText   = "[ cancel ]"
	folderBtnText   = "[o] folder"
	confirmBtnStart = 2                                    // after leading "  "
	confirmBtnEnd   = confirmBtnStart + len(confirmBtnText) // 13
	cancelBtnStart  = confirmBtnEnd + 3                    // 16 (after "   " gap)
	cancelBtnEnd    = cancelBtnStart + len(cancelBtnText)  // 26
	folderBtnStart  = cancelBtnEnd + 3                     // 29 (after "   " gap)
	folderBtnEnd    = folderBtnStart + len(folderBtnText)  // 39
)

// Model holds the state of the theme picker dialog.
type Model struct {
	themes      []string
	selected    int
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
	m.themes = discoverThemes(m.themeDir)
	m.selected = 0
	for i, n := range m.themes {
		if n == m.activeTheme {
			m.selected = i
			break
		}
	}
	return m, nil
}

// ActiveTheme returns the theme name that was active when the picker opened (used to revert on cancel).
func (m Model) ActiveTheme() string { return m.activeTheme }

// Dimensions returns the visual outer width and height of the rendered picker.
// Used by app.go to compute the picker's screen position for hit-testing clicks.
func (m Model) Dimensions() (w, h int) {
	return dialogWidth + 2, len(m.themes) + 4 // +2 borders (width); +4 borders+sep+btn (height)
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
		// Y=0 top border, Y=1..N theme rows, Y=N+1 separator, Y=N+2 action row.
		actionRowY := len(m.themes) + 2
		switch {
		case msg.Y >= 1 && msg.Y <= len(m.themes):
			// Theme row: move selection and preview, but don't confirm.
			row := msg.Y - 1
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
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

		case "down", "j":
			m.selected++
			if m.selected >= len(m.themes) {
				m.selected = 0
			}
			name := m.themes[m.selected]
			return m, func() tea.Msg { return messages.ThemeChangedMsg{ThemeName: name} }

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
	innerW := dialogWidth

	var rows []string
	for i, name := range m.themes {
		// ● marks the original (revert) theme; the highlight tracks the current selection.
		// These can differ after the user navigates away from the starting position.
		marker := "  "
		if name == m.activeTheme {
			marker = "● "
		}
		// Leading space replaces the removed Padding(0,1) so the visual indent is preserved.
		label := " " + marker + name
		// Pad to inner width (rune-aware: ● is 3 bytes but 1 display column)
		if utf8.RuneCountInString(label) < innerW {
			label += strings.Repeat(" ", innerW-utf8.RuneCountInString(label))
		}
		if i == m.selected {
			rows = append(rows, selectedStyle.Render(label))
		} else {
			rows = append(rows, baseStyle.Render(label))
		}
	}

	// Separator: 1-space indent then dashes, padded to full inner width.
	sep := " " + strings.Repeat("─", dialogWidth-4)
	if utf8.RuneCountInString(sep) < innerW {
		sep += strings.Repeat(" ", innerW-utf8.RuneCountInString(sep))
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

// discoverThemes returns sorted list of theme names: builtins first, then user themes.
func discoverThemes(themeDir string) []string {
	seen := make(map[string]bool)
	var names []string

	for _, n := range theme.ListBuiltin() {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	if themeDir != "" {
		entries, err := os.ReadDir(themeDir)
		if err == nil {
			for _, e := range entries {
				n := e.Name()
				if filepath.Ext(n) == ".json" {
					base := strings.TrimSuffix(n, ".json")
					if !seen[base] {
						seen[base] = true
						names = append(names, base)
					}
				}
			}
		}
	}

	return names
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
