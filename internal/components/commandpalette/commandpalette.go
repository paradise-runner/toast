// Package commandpalette implements the Ctrl+Shift+P command palette overlay:
// fuzzy search over editor actions with keybinding hints, live toggle state,
// MRU ordering, a selected-command preview line, and a rotating tips footer.
package commandpalette

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/fuzzy"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	overlayWidth     = 64
	overlayMinHeight = 8 // border + input + separator + 1 row + description + tip + border + padding
	overlayMaxHeight = 24
	maxDisplayItems  = 16
)

// ── Text input ────────────────────────────────────────────────────────────────

type textInput struct {
	value  string
	cursor int
}

func (t *textInput) Value() string { return t.value }

func (t *textInput) handleKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "backspace":
		if t.cursor > 0 {
			runes := []rune(t.value)
			runes = append(runes[:t.cursor-1], runes[t.cursor:]...)
			t.value = string(runes)
			t.cursor--
		}
	case "delete":
		runes := []rune(t.value)
		if t.cursor < len(runes) {
			runes = append(runes[:t.cursor], runes[t.cursor+1:]...)
			t.value = string(runes)
		}
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		if t.cursor < len([]rune(t.value)) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len([]rune(t.value))
	case "ctrl+u":
		runes := []rune(t.value)
		t.value = string(runes[t.cursor:])
		t.cursor = 0
	case "ctrl+k":
		runes := []rune(t.value)
		t.value = string(runes[:t.cursor])
	default:
		if len(msg.Text) > 0 {
			runes := []rune(t.value)
			text := []rune(msg.Text)
			runes = append(runes[:t.cursor], append(text, runes[t.cursor:]...)...)
			t.value = string(runes)
			t.cursor += len(text)
		}
	}
}

func (t *textInput) View(placeholder string) string {
	v := t.value
	if v == "" {
		return placeholder + " "
	}
	runes := []rune(v)
	if t.cursor >= len(runes) {
		return v + "█"
	}
	return string(runes[:t.cursor]) + "█" + string(runes[t.cursor+1:])
}

// ── Command registry ──────────────────────────────────────────────────────────

// Command is one entry in the palette. ID is a config.Action* identifier that
// the app dispatches via runAction; the other fields are display metadata.
type Command struct {
	ID          string
	Label       string
	Category    string
	Description string
	Toggle      bool // shows a live on/off pill; state supplied via SetToggleState
}

// DefaultCommands returns the initial command registry. Every entry must map
// to an action the app's runAction dispatcher understands.
func DefaultCommands() []Command {
	return []Command{
		{ID: config.ActionQuickOpen, Label: "Open File", Category: "File", Description: "Fuzzy-search and open a file in the workspace"},
		{ID: config.ActionSave, Label: "Save", Category: "File", Description: "Save the active buffer to disk"},
		{ID: config.ActionCloseTab, Label: "Close Tab", Category: "File", Description: "Close the active tab"},

		{ID: config.ActionGoToLine, Label: "Go to Line", Category: "Navigation", Description: "Jump to a line number in the active buffer"},
		{ID: config.ActionGoToDefinition, Label: "Go to Definition", Category: "Navigation", Description: "Jump to the symbol under the cursor"},
		{ID: config.ActionNextTab, Label: "Next Tab", Category: "Navigation", Description: "Switch to the next open tab"},
		{ID: config.ActionPrevTab, Label: "Previous Tab", Category: "Navigation", Description: "Switch to the previous open tab"},
		{ID: config.ActionToggleFocus, Label: "Toggle Focus", Category: "Navigation", Description: "Move focus between the editor and the file tree"},

		{ID: config.ActionUndo, Label: "Undo", Category: "Edit", Description: "Undo the last edit"},
		{ID: config.ActionRedo, Label: "Redo", Category: "Edit", Description: "Redo the last undone edit"},
		{ID: config.ActionToggleFold, Label: "Toggle Fold", Category: "Edit", Description: "Fold or unfold the block under the cursor"},
		{ID: config.ActionFoldAll, Label: "Fold All", Category: "Edit", Description: "Collapse every top-level block or heading"},
		{ID: config.ActionUnfoldAll, Label: "Unfold All", Category: "Edit", Description: "Expand every folded block"},

		{ID: config.ActionToggleSidebar, Label: "Toggle Sidebar", Category: "View", Description: "Show or hide the file tree sidebar", Toggle: true},
		{ID: config.ActionMarkdownPreview, Label: "Toggle Markdown Preview", Category: "View", Description: "Show or hide the markdown preview pane", Toggle: true},

		{ID: config.ActionSearch, Label: "Search Files", Category: "Search", Description: "Search for text across the whole workspace"},
		{ID: config.ActionFindReplace, Label: "Find in File", Category: "Search", Description: "Find and replace text in the active buffer"},

		{ID: config.ActionOpenSettings, Label: "Open Settings", Category: "System", Description: "Open the settings dialog"},
		{ID: config.ActionThemePicker, Label: "Theme Picker", Category: "System", Description: "Browse and apply themes"},
		{ID: config.ActionCommandPalette, Label: "Command Palette", Category: "System", Description: "Close this palette"},
		{ID: config.ActionQuit, Label: "Quit", Category: "System", Description: "Quit toast"},
	}
}

// rotatingTips is a small pool of hints/jokes shown in the palette footer.
var rotatingTips = []string{
	"Tip: Ctrl+P opens a file. Ctrl+Shift+P opens me.",
	"Tip: Ctrl+F searches this file; Ctrl+Shift+F searches the whole workspace.",
	"Tip: Ctrl+Z undoes mistakes. Real life included? Unfortunately not.",
	"Tip: Ctrl+Shift+E moves focus between the editor and the file tree.",
	"Fun fact: this editor is named after a breakfast item.",
	"Tip: Press Escape to dismiss me.",
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the command palette overlay component.
type Model struct {
	theme *theme.Manager
	keys  config.KeybindingMap

	open    bool
	query   textInput
	results []Command
	cursor  int

	usage       map[string]int  // per-command invocation count (MRU ordering)
	toggleState map[string]bool // live state for Toggle commands
	tipIndex    int
}

// New creates a command palette model.
func New(tm *theme.Manager, keys config.KeybindingMap) Model {
	return Model{
		theme:       tm,
		keys:        keys,
		usage:       make(map[string]int),
		toggleState: make(map[string]bool),
	}
}

// IsOpen reports whether the overlay is visible.
func (m Model) IsOpen() bool { return m.open }

// Open activates the overlay, resets the query, and rotates the tips footer.
func (m Model) Open() Model {
	m.open = true
	m.query = textInput{}
	m.cursor = 0
	m.tipIndex = (m.tipIndex + 1) % len(rotatingTips)
	m.filterResults()
	return m
}

// SetToggleState records the live on/off state of a Toggle command so the
// palette can render a state pill. Called by the app on every open.
func (m *Model) SetToggleState(action string, on bool) {
	m.toggleState[action] = on
}

// Update handles messages for the command palette overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}

	switch msg := msg.(type) {
	case messages.CommandPaletteCloseMsg:
		m.open = false

	case tea.KeyPressMsg:
		switch msg.String() {
		case "escape":
			m.open = false
			return m, func() tea.Msg { return messages.CommandPaletteCloseMsg{} }

		case "enter":
			if len(m.results) > 0 && m.cursor >= 0 && m.cursor < len(m.results) {
				cmd := m.results[m.cursor]
				m.usage[cmd.ID]++
				m.open = false
				return m, func() tea.Msg {
					return messages.CommandPaletteSelectMsg{ActionID: cmd.ID, Label: cmd.Label}
				}
			}

		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "ctrl+p", "ctrl+n", "ctrl+j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}

		default:
			prev := m.query.Value()
			m.query.handleKey(msg)
			if m.query.Value() != prev {
				m.cursor = 0
				m.filterResults()
			}
		}
	}

	return m, nil
}

// ── Filtering & ordering ──────────────────────────────────────────────────────

// filterResults updates m.results. With an empty query the full registry is
// shown, most-used first then alphabetically. With a query, fuzzy score
// dominates, with usage count and label as tiebreakers.
func (m *Model) filterResults() {
	query := m.query.Value()
	all := DefaultCommands()

	if query == "" {
		m.results = make([]Command, len(all))
		copy(m.results, all)
		sort.SliceStable(m.results, func(i, j int) bool {
			if ui, uj := m.usage[m.results[i].ID], m.usage[m.results[j].ID]; ui != uj {
				return ui > uj
			}
			return m.results[i].Label < m.results[j].Label
		})
		return
	}

	type scored struct {
		cmd   Command
		score int
	}

	var scoredResults []scored
	for _, c := range all {
		if ok, score := fuzzy.MatchScore(c.Label+" "+c.Category, query); ok {
			scoredResults = append(scoredResults, scored{cmd: c, score: score})
		}
	}

	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		if ui, uj := m.usage[scoredResults[i].cmd.ID], m.usage[scoredResults[j].cmd.ID]; ui != uj {
			return ui > uj
		}
		return scoredResults[i].cmd.Label < scoredResults[j].cmd.Label
	})

	m.results = make([]Command, len(scoredResults))
	for i, s := range scoredResults {
		m.results[i] = s.cmd
	}
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// View renders the command palette overlay. Returns an empty string when closed.
func (m Model) View() string {
	if !m.open {
		return ""
	}

	bgStr := m.theme.UI("completion_bg")
	if bgStr == "" {
		bgStr = "#313244"
	}
	fgStr := m.theme.UI("completion_fg")
	if fgStr == "" {
		fgStr = "#cdd6f4"
	}
	borderStr := m.theme.UI("find_replace_border")
	if borderStr == "" {
		borderStr = m.theme.UI("hover_border")
	}
	selBgStr := m.theme.UI("sidebar_selected_bg")
	if selBgStr == "" {
		selBgStr = "#45475a"
	}
	selFgStr := m.theme.UI("sidebar_selected_fg")
	if selFgStr == "" {
		selFgStr = "#cdd6f4"
	}
	accentStr := m.theme.UI("accent")
	if accentStr == "" {
		accentStr = "#89b4fa"
	}
	mutedStr := m.theme.UI("muted")
	if mutedStr == "" {
		mutedStr = m.theme.UI("statusbar_fg") // older themes without a muted token
	}

	bg := lipgloss.Color(bgStr)
	fg := lipgloss.Color(fgStr)
	border := lipgloss.Color(borderStr)
	selBg := lipgloss.Color(selBgStr)
	selFg := lipgloss.Color(selFgStr)
	accent := lipgloss.Color(accentStr)
	muted := lipgloss.Color(mutedStr)

	innerWidth := overlayWidth - 4 // border (2) + padding (2)

	// Lock-on: with exactly one fuzzy result, the palette locks onto it.
	lockOn := m.query.Value() != "" && len(m.results) == 1
	activeBorder := border
	if lockOn {
		activeBorder = accent
	}

	// ── Build content lines ────────────────────────────────────────────────

	var lines []string

	// Input line
	lines = append(lines, pad("Commands: "+m.query.View("Type a command..."), innerWidth))

	// Separator
	lines = append(lines, strings.Repeat("─", innerWidth))

	// Result rows
	display := m.results
	if len(display) > maxDisplayItems {
		display = display[:maxDisplayItems]
	}

	for _, item := range display {
		lines = append(lines, m.rowLine(item, innerWidth))
	}

	// Description preview line (selected row)
	var desc string
	switch {
	case len(m.results) == 0:
		desc = "No matching commands"
	case m.cursor >= 0 && m.cursor < len(m.results):
		desc = m.results[m.cursor].Description
	default:
		desc = m.results[0].Description
	}
	lines = append(lines, pad(desc, innerWidth))

	// Rotating tip footer
	lines = append(lines, pad(rotatingTips[m.tipIndex], innerWidth))

	// ── Style the lines ────────────────────────────────────────────────────

	var styledLines []string
	for i, line := range lines {
		rowIdx := i - 2 // rows start after input + separator
		switch {
		case i == 0:
			// Input line: bold
			styledLines = append(styledLines,
				lipgloss.NewStyle().Bold(true).Foreground(fg).Background(bg).Render(line))
		case i == 1:
			// Separator
			styledLines = append(styledLines,
				lipgloss.NewStyle().Foreground(activeBorder).Background(bg).Render(line))
		case rowIdx >= 0 && rowIdx < len(display):
			// Result rows
			selected := rowIdx == m.cursor
			marker := " "
			if selected {
				marker = "▸"
			}
			style := lipgloss.NewStyle().Background(bg).Foreground(fg)
			if lockOn {
				style = style.Foreground(accent)
			}
			if selected {
				style = style.Background(selBg).Foreground(selFg)
				if lockOn {
					style = style.Bold(true)
				}
			}
			styledLines = append(styledLines, style.Render(marker+line[1:]))
		case rowIdx == len(display):
			// Description line
			styledLines = append(styledLines,
				lipgloss.NewStyle().Italic(true).Foreground(fg).Background(bg).Render(line))
		default:
			// Tips footer: muted secondary text — dimmer than the description
			// line above it, but clearly legible (no Faint, which would push it
			// back down toward the border color's contrast).
			styledLines = append(styledLines,
				lipgloss.NewStyle().Foreground(muted).Background(bg).Render(line))
		}
	}

	content := strings.Join(styledLines, "\n")

	// Determine box height based on how many result lines we have.
	boxHeight := len(lines) + 2 // border top + bottom
	if boxHeight < overlayMinHeight {
		boxHeight = overlayMinHeight
	}
	if boxHeight > overlayMaxHeight {
		boxHeight = overlayMaxHeight
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(activeBorder).
		BorderBackground(bg).
		Background(bg).
		Foreground(fg).
		Padding(0, 1).
		Width(overlayWidth).
		Height(boxHeight).
		Render(content)

	return box
}

// Render is an alias for View, matching the pattern used by other overlays.
func (m Model) Render() string { return m.View() }

// rowLine renders one result row: label (+ state pill for toggles) on the
// left, keybinding hint right-aligned, both padded to innerWidth.
func (m Model) rowLine(item Command, innerWidth int) string {
	label := item.Label
	if item.Toggle {
		if m.toggleState[item.ID] {
			label += " ●on"
		} else {
			label += " ○off"
		}
	}
	key := ""
	if hint := m.keys.FirstKey(item.ID); hint != "" && hint != "ctrl+shift+p" && hint != "super+shift+p" {
		key = hint
	}
	leftRunes := []rune(label)
	keyRunes := []rune(key)
	padWidth := innerWidth - 1 - 1 // leading space + trailing space
	if len(leftRunes)+len(keyRunes) > padWidth {
		// Truncate the label to make room for the keybinding hint.
		keep := padWidth - len(keyRunes) - 2
		if keep < 8 {
			keep = 8
		}
		label = truncateRunes(label, keep)
		leftRunes = []rune(label)
	}
	line := " " + label
	remaining := innerWidth - len(leftRunes) - len(keyRunes) - 2 // spaces around
	if remaining < 1 {
		remaining = 1
	}
	line += strings.Repeat(" ", remaining) + key + " "
	return line
}

func pad(s string, width int) string {
	runes := []rune(s)
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return truncateRunes(s, width)
}

func truncateRunes(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
