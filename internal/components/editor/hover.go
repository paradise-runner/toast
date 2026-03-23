package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/theme"
)

const hoverMaxWidth = 60
const hoverMaxLines = 10

type hoverState struct {
	contents string
	visible  bool
}

func (hs *hoverState) show(contents string) { hs.contents = contents; hs.visible = contents != "" }
func (hs *hoverState) hide()                { hs.visible = false; hs.contents = "" }

func renderHover(hs hoverState, x, y, screenHeight int, tm *theme.Manager) string {
	if !hs.visible || hs.contents == "" {
		return ""
	}
	bg := lipgloss.Color(tm.UI("hover_bg"))
	fg := lipgloss.Color(tm.UI("hover_fg"))
	border := lipgloss.Color(tm.UI("hover_border"))
	style := lipgloss.NewStyle().Background(bg).Foreground(fg).
		BorderStyle(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).MaxWidth(hoverMaxWidth)
	lines := strings.Split(hs.contents, "\n")
	if len(lines) > hoverMaxLines {
		lines = lines[:hoverMaxLines]
		lines = append(lines, "…")
	}
	return style.Render(strings.Join(lines, "\n"))
}
