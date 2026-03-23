package editor

import (
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func gitDiffBar(kind messages.GitLineKind, tm *theme.Manager) string {
	bg := lipgloss.Color(tm.UI("background"))
	style := lipgloss.NewStyle().Background(bg)
	switch kind {
	case messages.GitLineAdded:
		return style.Foreground(lipgloss.Color(tm.Git("added"))).Render("│")
	case messages.GitLineModified:
		return style.Foreground(lipgloss.Color(tm.Git("modified"))).Render("│")
	case messages.GitLineDeleted:
		return style.Foreground(lipgloss.Color(tm.Git("deleted"))).Render("▸")
	default:
		return style.Render(" ")
	}
}
