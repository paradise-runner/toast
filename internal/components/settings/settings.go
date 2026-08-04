// Package settings implements a two-pane settings dialog: a list of setting
// groups on the left and the selected group's settings on the right. Panes
// are switched with tab or left/right arrows; up/down navigate within a
// pane, enter/space activate, h/l adjust values. Mouse is fully supported —
// hovering changes the selection, clicking activates. Changes apply live:
// every mutation emits a SettingsChangedMsg carrying the updated config for
// the app to apply and persist.
package settings

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	// dialogWidth is the inner content width, excluding the 2 border columns.
	dialogWidth = 66
	// leftWidth is the group-list column width within the content area.
	leftWidth = 16
	// controlWidth is the right-aligned control column width in the right pane.
	controlWidth = 24
	// contentRows is the number of setting rows (max group size).
	contentRows = 7
	// Row layout (dialog-local coordinates, Y=0 is the top border):
	headerY  = 1
	firstRow = 2
	sepY     = firstRow + contentRows // 7
	footerY  = sepY + 1               // 8

	headerText = "Settings"
	footerText = "tab/←→: pane  ↑↓: move  h/l: adjust  enter: toggle  esc: close"
)

// kind identifies how a setting's value is displayed and edited.
type kind int

const (
	kindToggle kind = iota
	kindStepper
	kindCycle
)

// setting describes one editable row in the right pane. Getters/setters
// operate on the live config so every control reads and writes the same
// source of truth.
type setting struct {
	label    string
	kind     kind
	getBool  func(*config.Config) bool
	setBool  func(*config.Config, bool)
	getInt   func(*config.Config) int
	setInt   func(*config.Config, int)
	min, max int
	options  []string
	getCycle func(*config.Config) string
	setCycle func(*config.Config, string)
}

type group struct {
	name     string
	settings []setting
}

// Model holds the state of the settings dialog.
type Model struct {
	theme       *theme.Manager
	cfg         config.Config
	groups      []group
	activeGroup int
	cursor      int // selected setting row within the active group
	focusLeft   bool
}

// New creates a settings dialog for cfg. themeDir is scanned for user themes
// so the Appearance group can offer the full theme list.
func New(tm *theme.Manager, themeDir string, cfg config.Config) Model {
	return Model{
		theme:     tm,
		cfg:       cfg,
		groups:    buildGroups(theme.Discover(themeDir)),
		focusLeft: true,
	}
}

// Dimensions returns the visual outer width and height of the rendered
// dialog. Used by app.go to compute the dialog's screen position for
// hit-testing mouse events.
func (m Model) Dimensions() (w, h int) {
	return dialogWidth + 2, footerY + 2
}

// Update handles key and mouse events for the dialog.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		return m.handleHover(msg.X, msg.Y)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		return m.handleClick(msg.X, msg.Y)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// ── Keyboard ─────────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg { return messages.SettingsClosedMsg{} }

	case "tab", "shift+tab":
		m.focusLeft = !m.focusLeft
		m.clampCursor()
		return m, nil

	case "up", "k":
		if m.focusLeft {
			m.activeGroup--
			if m.activeGroup < 0 {
				m.activeGroup = len(m.groups) - 1
			}
			m.clampCursor()
		} else {
			n := len(m.activeSettings())
			if n > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = n - 1
				}
			}
		}
		return m, nil

	case "down", "j":
		if m.focusLeft {
			m.activeGroup = (m.activeGroup + 1) % len(m.groups)
			m.clampCursor()
		} else {
			n := len(m.activeSettings())
			if n > 0 {
				m.cursor = (m.cursor + 1) % n
			}
		}
		return m, nil

	case "left":
		// Left/right arrows are reserved for switching panes.
		if !m.focusLeft {
			m.focusLeft = true
		}
		return m, nil

	case "right":
		if m.focusLeft {
			m.focusLeft = false
			m.clampCursor()
		}
		return m, nil

	case "h":
		if m.focusLeft {
			return m, nil
		}
		return m.adjust(-1)

	case "l":
		if m.focusLeft {
			return m, nil
		}
		return m.adjust(+1)

	case "enter", "space":
		if m.focusLeft {
			// Enter on a group jumps into its settings.
			m.focusLeft = false
			m.cursor = 0
			return m, nil
		}
		return m.adjust(+1)
	}
	return m, nil
}

// adjust changes the value of the selected setting by delta steps (±1).
// Toggles flip regardless of direction. Returns the model plus a command that
// announces the change when one occurred.
func (m Model) adjust(delta int) (Model, tea.Cmd) {
	settings := m.activeSettings()
	if len(settings) == 0 {
		return m, nil
	}
	s := settings[m.cursor]
	switch s.kind {
	case kindToggle:
		s.setBool(&m.cfg, !s.getBool(&m.cfg))
	case kindStepper:
		v := s.getInt(&m.cfg) + delta
		if v < s.min {
			v = s.min
		}
		if v > s.max {
			v = s.max
		}
		if v == s.getInt(&m.cfg) {
			return m, nil
		}
		s.setInt(&m.cfg, v)
	case kindCycle:
		if len(s.options) == 0 {
			return m, nil
		}
		idx := 0
		cur := s.getCycle(&m.cfg)
		for i, o := range s.options {
			if o == cur {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(s.options)) % len(s.options)
		s.setCycle(&m.cfg, s.options[idx])
	}
	return m, m.changeCmd()
}

func (m Model) changeCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg { return messages.SettingsChangedMsg{Config: cfg} }
}

// ── Mouse ────────────────────────────────────────────────────────────────────

// handleHover moves the selection to whatever is under the pointer, matching
// the file tree's hover behavior. Hovering the left pane changes the active
// group; hovering the right pane moves the setting cursor.
func (m Model) handleHover(x, y int) (Model, tea.Cmd) {
	contentX := x - 1 // subtract left border
	row := y - firstRow
	if row < 0 || row >= contentRows {
		return m, nil
	}
	if contentX < leftWidth {
		if row < len(m.groups) {
			m.activeGroup = row
			m.focusLeft = true
			m.clampCursor()
		}
		return m, nil
	}
	if row < len(m.activeSettings()) {
		m.cursor = row
		m.focusLeft = false
	}
	return m, nil
}

// handleClick selects the item under the pointer and activates it: toggles
// flip, and stepper/cycle controls respond to their ◄/► arrows.
func (m Model) handleClick(x, y int) (Model, tea.Cmd) {
	contentX := x - 1 // subtract left border
	row := y - firstRow
	if row < 0 || row >= contentRows {
		return m, nil
	}

	if contentX < leftWidth {
		if row < len(m.groups) {
			m.activeGroup = row
			m.focusLeft = true
			m.clampCursor()
		}
		return m, nil
	}

	settings := m.activeSettings()
	if row >= len(settings) {
		return m, nil
	}
	m.cursor = row
	m.focusLeft = false

	s := settings[row]
	switch s.kind {
	case kindToggle:
		s.setBool(&m.cfg, !s.getBool(&m.cfg))
		return m, m.changeCmd()
	case kindStepper, kindCycle:
		// Hit-test the ◄ / ► arrows within the right-aligned control column.
		ctrl := controlString(s, &m.cfg)
		leftPad := controlWidth - utf8.RuneCountInString(ctrl)
		rightStart := leftWidth + (dialogWidth - leftWidth - controlWidth) + leftPad
		arrowLeftX := rightStart
		arrowRightX := rightStart + utf8.RuneCountInString(ctrl) - 1
		switch contentX {
		case arrowLeftX:
			return m.adjust(-1)
		case arrowRightX:
			return m.adjust(+1)
		}
	}
	return m, nil
}

// ── Rendering ────────────────────────────────────────────────────────────────

// Render returns the styled dialog box as a string.
func (m Model) Render() string {
	bg := lipgloss.Color(m.theme.Settings("bg"))
	fg := lipgloss.Color(m.theme.Settings("fg"))
	sel := lipgloss.Color(m.theme.Settings("selected"))
	sepColor := lipgloss.Color(m.theme.Settings("separator"))
	border := lipgloss.Color(m.theme.Settings("border"))

	baseStyle := lipgloss.NewStyle().Background(bg).Foreground(fg)
	selectedStyle := lipgloss.NewStyle().Background(sel).Foreground(fg)
	sepStyle := lipgloss.NewStyle().Background(bg).Foreground(sepColor)

	rightWidth := dialogWidth - leftWidth
	rows := make([]string, 0, contentRows+3)

	// Header row.
	rows = append(rows, baseStyle.Render(padRune(" "+headerText, dialogWidth)))

	// Content rows: left group list + right settings pane side by side.
	settings := m.activeSettings()
	for i := 0; i < contentRows; i++ {
		var left string
		if i < len(m.groups) {
			marker := "  "
			if i == m.activeGroup {
				marker = "▸ "
			}
			label := " " + marker + m.groups[i].name
			left = padRune(label, leftWidth)
			if i == m.activeGroup {
				left = selectedStyle.Render(left)
			} else {
				left = baseStyle.Render(left)
			}
		} else {
			left = baseStyle.Render(strings.Repeat(" ", leftWidth))
		}

		var right string
		if i < len(settings) {
			text := " " + settings[i].label
			labelCol := padRune(text, rightWidth-controlWidth)
			ctrl := padL(controlString(settings[i], &m.cfg), controlWidth)
			right = padRune(labelCol+ctrl, rightWidth)
			if !m.focusLeft && i == m.cursor {
				right = selectedStyle.Render(right)
			} else {
				right = baseStyle.Render(right)
			}
		} else {
			right = baseStyle.Render(strings.Repeat(" ", rightWidth))
		}

		rows = append(rows, left+right)
	}

	// Separator row.
	sep := strings.Repeat("─", dialogWidth)
	rows = append(rows, sepStyle.Render(sep))

	// Footer hint row.
	rows = append(rows, baseStyle.Render(padRune(" "+footerText, dialogWidth)))

	body := strings.Join(rows, "\n")

	// lipgloss v2's Width includes the border columns, so add 2 to keep the
	// inner content area at dialogWidth.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Width(dialogWidth + 2).
		Render(body)
}

// controlString renders the value side of a setting: "[✓]", "◄ 4 ►" or
// "◄ accent ►".
func controlString(s setting, cfg *config.Config) string {
	switch s.kind {
	case kindToggle:
		if s.getBool(cfg) {
			return "[✓]"
		}
		return "[ ]"
	case kindStepper:
		return fmt.Sprintf("◄ %d ►", s.getInt(cfg))
	case kindCycle:
		val := s.getCycle(cfg)
		maxLen := controlWidth - utf8.RuneCountInString("◄  ►")
		if maxLen > 1 && utf8.RuneCountInString(val) > maxLen {
			val = truncateRune(val, maxLen-1) + "…"
		}
		return "◄ " + val + " ►"
	}
	return ""
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (m Model) activeSettings() []setting {
	return m.groups[m.activeGroup].settings
}

func (m *Model) clampCursor() {
	n := len(m.activeSettings())
	if n > 0 && m.cursor >= n {
		m.cursor = n - 1
	}
}

// padRune pads s with trailing spaces to width, measured in runes so
// multi-byte glyphs (✓ ▸ ◄ ►) keep alignment.
func padRune(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

// padL pads s with leading spaces to width, measured in runes.
func padL(s string, width int) string {
	if n := utf8.RuneCountInString(s); n < width {
		return strings.Repeat(" ", width-n) + s
	}
	return s
}

// truncateRune truncates s to n runes.
func truncateRune(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// autoSaveDelayOptions lists the selectable inactivity delays for auto-save.
var autoSaveDelayOptions = []string{"100ms", "250ms", "300ms", "500ms", "1s", "2s", "5s", "10s"}

// autoSaveDelayLabel renders an AutoSaveDelayMs value as a setting option.
// Values that are not one of the presets fall back to the default label.
func autoSaveDelayLabel(ms int) string {
	switch ms {
	case 100, 250, 300, 500:
		return fmt.Sprintf("%dms", ms)
	}
	if ms >= 1000 && ms%1000 == 0 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	return "300ms"
}

// autoSaveDelayMS parses a delay option label back into milliseconds.
// Unrecognized labels fall back to the default delay.
func autoSaveDelayMS(label string) int {
	if strings.HasSuffix(label, "ms") {
		if n, err := strconv.Atoi(strings.TrimSuffix(label, "ms")); err == nil {
			return n
		}
	}
	if strings.HasSuffix(label, "s") {
		if n, err := strconv.Atoi(strings.TrimSuffix(label, "s")); err == nil {
			return n * 1000
		}
	}
	return 300
}

// buildGroups defines the editable settings. Only config fields that are
// wired into the editor/sidebar today are exposed.
func buildGroups(themes []string) []group {
	return []group{
		{
			name: "Editor",
			settings: []setting{
				{
					label: "Tab Width", kind: kindStepper, min: 1, max: 16,
					getInt: func(c *config.Config) int { return c.Editor.TabWidth },
					setInt: func(c *config.Config, v int) { c.Editor.TabWidth = v },
				},
				{
					label: "Auto Indent", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Editor.AutoIndent },
					setBool: func(c *config.Config, v bool) { c.Editor.AutoIndent = v },
				},
				{
					label: "Trim Trailing Whitespace", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Editor.TrimTrailingWhitespaceOnSave },
					setBool: func(c *config.Config, v bool) { c.Editor.TrimTrailingWhitespaceOnSave = v },
				},
				{
					label: "Insert Final Newline", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Editor.InsertFinalNewlineOnSave },
					setBool: func(c *config.Config, v bool) { c.Editor.InsertFinalNewlineOnSave = v },
				},
				{
					label: "Bottom Padding", kind: kindStepper, min: 0, max: 20,
					getInt: func(c *config.Config) int { return c.Editor.BottomPadding },
					setInt: func(c *config.Config, v int) { c.Editor.BottomPadding = v },
				},
				{
					label: "Auto Save", kind: kindCycle,
					options:  []string{"auto", "manual"},
					getCycle: func(c *config.Config) string { return c.Editor.AutoSave },
					setCycle: func(c *config.Config, v string) { c.Editor.AutoSave = v },
				},
				{
					label: "Auto Save Delay", kind: kindCycle, options: autoSaveDelayOptions,
					getCycle: func(c *config.Config) string { return autoSaveDelayLabel(c.Editor.AutoSaveDelayMs) },
					setCycle: func(c *config.Config, v string) { c.Editor.AutoSaveDelayMs = autoSaveDelayMS(v) },
				},
			},
		},
		{
			name: "Appearance",
			settings: []setting{
				{
					label: "Theme", kind: kindCycle, options: themes,
					getCycle: func(c *config.Config) string { return c.Theme },
					setCycle: func(c *config.Config, v string) { c.Theme = v },
				},
			},
		},
		{
			name: "Sidebar",
			settings: []setting{
				{
					label: "Visible", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Sidebar.Visible },
					setBool: func(c *config.Config, v bool) { c.Sidebar.Visible = v },
				},
				{
					label: "Width", kind: kindStepper, min: 15, max: 80,
					getInt: func(c *config.Config) int { return c.Sidebar.Width },
					setInt: func(c *config.Config, v int) { c.Sidebar.Width = v },
				},
				{
					label: "Confirm Delete", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Sidebar.ConfirmDelete },
					setBool: func(c *config.Config, v bool) { c.Sidebar.ConfirmDelete = v },
				},
				{
					label: "File Icons", kind: kindToggle,
					getBool: func(c *config.Config) bool { return c.Sidebar.FileIcons.Enabled },
					setBool: func(c *config.Config, v bool) { c.Sidebar.FileIcons.Enabled = v },
				},
				{
					label: "Icon Color Mode", kind: kindCycle,
					options:  []string{"accent", "semantic", "none"},
					getCycle: func(c *config.Config) string { return c.Sidebar.FileIcons.ColorMode },
					setCycle: func(c *config.Config, v string) { c.Sidebar.FileIcons.ColorMode = v },
				},
			},
		},
	}
}
