package filetree

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/theme"
)

const deleteDialogW = 42 // total rendered width (border included)

// DeleteConfirmDialog is the modal overlay shown before deleting a file or directory.
type DeleteConfirmDialog struct {
	path    string
	isDir   bool
	focus   int  // 0 = Confirm, 1 = Cancel, 2 = checkbox
	checked bool // "Don't ask again"
	theme   *theme.Manager

	// Set by Render; used by HandleClick.
	originX, originY int
	dialogW, dialogH int
}

func newDeleteConfirmDialog(path string, isDir bool, tm *theme.Manager) *DeleteConfirmDialog {
	return &DeleteConfirmDialog{
		path:  path,
		isDir: isDir,
		theme: tm,
	}
}

// Render computes the centered position, stores geometry, and returns the rendered string.
func (d *DeleteConfirmDialog) Render(totalWidth, totalHeight int) string {
	// deleteDialogW includes: left-border(1) + left-pad(1) + content(innerW) + right-pad(1) + right-border(1)
	innerW := deleteDialogW - 4

	bg := lipgloss.Color(d.theme.UI("completion_bg"))
	fg := lipgloss.Color(d.theme.UI("completion_fg"))
	border := lipgloss.Color(d.theme.UI("hover_border"))
	selColor := lipgloss.Color(d.theme.UI("completion_selected"))

	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	focused := lipgloss.NewStyle().Background(selColor).Foreground(fg).Bold(true)

	// padLine renders s with base style and right-fills to exactly innerW cells using
	// bg-colored spaces. This prevents terminal-default background from bleeding
	// through the side padding columns added by the box border.
	padLine := func(s string, style lipgloss.Style) string {
		w := lipgloss.Width(s)
		fill := max(0, innerW-w)
		return style.Render(s) + base.Render(strings.Repeat(" ", fill))
	}

	name := filepath.Base(d.path)

	// Build title line.
	var title string
	if d.isDir {
		title = "Delete \"" + name + "\" and all its contents?"
	} else {
		title = "Delete \"" + name + "\"?"
	}
	// Truncate title if too wide.
	if lipgloss.Width(title) > innerW {
		maxNameLen := innerW - len("Delete \"\" and all its contents?")
		if maxNameLen < 1 {
			maxNameLen = 1
		}
		runes := []rune(name)
		if len(runes) > maxNameLen {
			name = string(runes[:maxNameLen-1]) + "…"
		}
		if d.isDir {
			title = "Delete \"" + name + "\" and all its contents?"
		} else {
			title = "Delete \"" + name + "\"?"
		}
	}

	titleLine := padLine(title, base)
	warnLine := padLine("This cannot be undone.", base)

	// Separator: border color as foreground gives a subtle divider rather than a glaring line.
	sepStyle := lipgloss.NewStyle().Background(bg).Foreground(border)
	sep := sepStyle.Render(strings.Repeat("─", innerW))

	// Checkbox.
	checkMark := "[ ]"
	if d.checked {
		checkMark = "[x]"
	}
	checkLabel := checkMark + " Don't ask again"
	checkStyle := base
	if d.focus == 2 {
		checkStyle = focused
	}
	checkLine := padLine(checkLabel, checkStyle)

	// Blank spacer line.
	blankLine := base.Render(strings.Repeat(" ", innerW))

	// Button row: all segments carry explicit bg so no terminal-default gaps appear.
	confirmLabel := "[Confirm]"
	cancelLabel := "[Cancel]"
	confirmStyle := base
	cancelStyle := base
	if d.focus == 0 {
		confirmStyle = focused
	}
	if d.focus == 1 {
		cancelStyle = focused
	}

	confirmBtn := confirmStyle.Render(confirmLabel)
	gap := base.Render(strings.Repeat(" ", 8)) // 8 bg-styled spaces between buttons
	cancelBtn := cancelStyle.Render(cancelLabel)
	btnRow := confirmBtn + gap + cancelBtn
	btnPad := innerW - lipgloss.Width(btnRow)
	leftPad := btnPad / 2
	rightPad := btnPad - leftPad
	if leftPad < 0 {
		leftPad = 0
	}
	if rightPad < 0 {
		rightPad = 0
	}
	btnLine := base.Render(strings.Repeat(" ", leftPad)) + btnRow + base.Render(strings.Repeat(" ", rightPad))

	body := strings.Join([]string{titleLine, warnLine, sep, checkLine, blankLine, btnLine}, "\n")

	// BorderBackground(bg) ensures the rounded-border characters (╭─╮│╰╯) sit on
	// the dialog background color rather than showing through to whatever content
	// is behind the overlay — critical for light themes where the terminal content
	// is darker than completion_bg.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Padding(0, 1).
		Width(deleteDialogW).
		Render(body)

	// Measure rendered dimensions.
	boxLines := strings.Split(box, "\n")
	d.dialogH = len(boxLines)
	d.dialogW = 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > d.dialogW {
			d.dialogW = w
		}
	}

	// Center in terminal.
	d.originX = (totalWidth - d.dialogW) / 2
	d.originY = (totalHeight - d.dialogH) / 2
	if d.originX < 0 {
		d.originX = 0
	}
	if d.originY < 0 {
		d.originY = 0
	}

	return box
}

// HandleKey processes a key press. Returns (confirm, cancel, toggleCheck).
// The caller is responsible for applying any state changes (e.g. toggling d.checked).
func (d *DeleteConfirmDialog) HandleKey(msg tea.KeyPressMsg) (confirm, cancel, toggleCheck bool) {
	switch msg.String() {
	case "esc":
		return false, true, false
	case "enter":
		switch d.focus {
		case 0:
			return true, false, false
		case 1:
			return false, true, false
		case 2:
			d.checked = !d.checked
			return false, false, true
		}
	case " ", "space":
		d.checked = !d.checked
		return false, false, true
	case "tab", "right":
		d.focus = (d.focus + 1) % 3
	case "shift+tab", "left":
		d.focus = (d.focus + 2) % 3
	}
	return false, false, false
}

// HandleClick takes raw screen coordinates and returns (confirm, cancel, toggleCheck).
// Render must have been called at least once to populate geometry.
func (d *DeleteConfirmDialog) HandleClick(x, y int) (confirm, cancel, toggleCheck bool) {
	// Bounds check.
	if x < d.originX || x >= d.originX+d.dialogW ||
		y < d.originY || y >= d.originY+d.dialogH {
		return false, false, false
	}

	// The button row is the second-to-last row (last row is bottom border).
	btnRow := d.originY + d.dialogH - 2
	// Row layout (0-indexed from top): border, title, warn, sep, checkbox, blank, buttons, border.
	checkRow := d.originY + 4

	if y == btnRow {
		// Split confirm vs cancel at the dialog midpoint.
		mid := d.originX + d.dialogW/2
		if x < mid {
			d.focus = 0
			return true, false, false
		}
		d.focus = 1
		return false, true, false
	}
	if y == checkRow {
		d.focus = 2
		d.checked = !d.checked
		return false, false, true
	}
	return false, false, false
}
