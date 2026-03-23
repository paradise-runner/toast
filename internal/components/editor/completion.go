package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

type completionState struct {
	items      []messages.CompletionItem
	selected   int
	visible    bool
	anchorLine int
	anchorCol  int
}

func (cs *completionState) show(items []messages.CompletionItem, line, col int) {
	cs.items = items
	cs.selected = 0
	cs.visible = len(items) > 0
	cs.anchorLine = line
	cs.anchorCol = col
}

func (cs *completionState) hide() { cs.visible = false; cs.items = nil }

func (cs *completionState) moveDown() {
	if len(cs.items) > 0 {
		cs.selected = (cs.selected + 1) % len(cs.items)
	}
}

func (cs *completionState) moveUp() {
	if len(cs.items) > 0 {
		cs.selected = (cs.selected - 1 + len(cs.items)) % len(cs.items)
	}
}

func (cs *completionState) accept() *messages.CompletionItem {
	if !cs.visible || len(cs.items) == 0 {
		return nil
	}
	item := cs.items[cs.selected]
	cs.hide()
	return &item
}

const completionMaxVisible = 10
const completionWidth = 40

func renderCompletion(cs completionState, x, y, screenHeight int, tm *theme.Manager) string {
	if !cs.visible || len(cs.items) == 0 {
		return ""
	}
	bg := lipgloss.Color(tm.UI("completion_bg"))
	fg := lipgloss.Color(tm.UI("completion_fg"))
	selBG := lipgloss.Color(tm.UI("completion_selected"))
	normalStyle := lipgloss.NewStyle().Background(bg).Foreground(fg).Width(completionWidth)
	selectedStyle := lipgloss.NewStyle().Background(selBG).Foreground(fg).Width(completionWidth)
	end := len(cs.items)
	if end > completionMaxVisible {
		end = completionMaxVisible
	}
	var lines []string
	for i := 0; i < end; i++ {
		item := cs.items[i]
		label := item.Label
		if item.Detail != "" {
			detail := item.Detail
			if len(detail) > 20 {
				detail = detail[:20] + "…"
			}
			label = label + " " + detail
		}
		if i == cs.selected {
			lines = append(lines, selectedStyle.Render(label))
		} else {
			lines = append(lines, normalStyle.Render(label))
		}
	}
	return strings.Join(lines, "\n")
}
