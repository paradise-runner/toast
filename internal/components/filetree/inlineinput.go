package filetree

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/theme"
)

// InlineInput holds the state for the inline name-entry row shown when creating
// a new file or folder.
type InlineInput struct {
	targetDir string
	value     string
	isDir     bool
}

// NewInlineInput creates an inline input for creating a new entry under targetDir.
// Set isDir to true when creating a directory.
func NewInlineInput(targetDir string, isDir bool) *InlineInput {
	return &InlineInput{targetDir: targetDir, isDir: isDir}
}

// Insert appends a rune to the current value.
func (i *InlineInput) Insert(ch rune) {
	i.value += string(ch)
}

// Backspace removes the last rune from the current value.
func (i *InlineInput) Backspace() {
	if i.value == "" {
		return
	}
	_, size := utf8.DecodeLastRuneInString(i.value)
	i.value = i.value[:len(i.value)-size]
}

// RenderRow renders the inline-edit row at the given indent depth.
// The row is padded to exactly width visual columns.
func (i InlineInput) RenderRow(depth, width int, tm *theme.Manager) string {
	style := lipgloss.NewStyle()
	if selBG := tm.UI("sidebar_selected_bg"); selBG != "" {
		style = style.Background(lipgloss.Color(selBG))
	}
	if selFG := tm.UI("sidebar_selected_fg"); selFG != "" {
		style = style.Foreground(lipgloss.Color(selFG))
	}

	indent := strings.Repeat("  ", depth)
	prefix := "  "
	if i.isDir {
		prefix = "▶ "
	}

	content := indent + prefix + i.value + "▌"
	var padLen int
	if lipgloss.Width(content) > width {
		content = ansi.Truncate(content, width, "")
		padLen = 0
	} else {
		padLen = width - lipgloss.Width(content)
		if padLen < 0 {
			padLen = 0
		}
	}
	return style.Render(content + strings.Repeat(" ", padLen))
}
