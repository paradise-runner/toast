package editor

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

func renderCompletion(cs completionState, maxWidth int, tm *theme.Manager) string {
	if !cs.visible || len(cs.items) == 0 {
		return ""
	}
	width := completionWidth
	if maxWidth < width {
		width = maxWidth
	}
	if width < 1 {
		return ""
	}
	bg := lipgloss.Color(tm.UI("completion_bg"))
	fg := lipgloss.Color(tm.UI("completion_fg"))
	selBG := lipgloss.Color(tm.UI("completion_selected"))
	normalStyle := lipgloss.NewStyle().Background(bg).Foreground(fg).Width(width)
	selectedStyle := lipgloss.NewStyle().Background(selBG).Foreground(fg).Width(width)
	start := 0
	if cs.selected >= completionMaxVisible {
		start = cs.selected - completionMaxVisible + 1
	}
	end := start + completionMaxVisible
	if end > len(cs.items) {
		end = len(cs.items)
	}
	var lines []string
	for i := start; i < end; i++ {
		item := cs.items[i]
		label := item.Label
		if item.Detail != "" {
			detail := ansi.Truncate(item.Detail, 20, "…")
			label = label + " " + detail
		}
		label = ansi.Truncate(label, width, "…")
		if i == cs.selected {
			lines = append(lines, selectedStyle.Render(label))
		} else {
			lines = append(lines, normalStyle.Render(label))
		}
	}
	return strings.Join(lines, "\n")
}

// overlayCompletion composites a completion menu over the editor's rendered
// ANSI content. The menu is shown below the cursor when possible and above it
// when there is more room there.
func overlayCompletion(base, menu string, x, cursorY, width, height int) string {
	if menu == "" || width <= 0 || height <= 0 {
		return base
	}
	baseLines := strings.Split(base, "\n")
	menuLines := strings.Split(menu, "\n")
	menuHeight := len(menuLines)
	y := cursorY + 1
	if y+menuHeight > height {
		y = cursorY - menuHeight
	}
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}
	menuWidth := 0
	for _, line := range menuLines {
		if lineWidth := lipgloss.Width(line); lineWidth > menuWidth {
			menuWidth = lineWidth
		}
	}
	if x+menuWidth > width {
		x = width - menuWidth
		if x < 0 {
			x = 0
		}
	}

	for i, menuLine := range menuLines {
		row := y + i
		if row >= len(baseLines) || row >= height {
			break
		}
		baseLine := baseLines[row]
		baseWidth := lipgloss.Width(baseLine)
		before := ansi.Truncate(baseLine, x, "")
		after := ""
		if x+menuWidth < baseWidth {
			after = ansi.Cut(baseLine, x+menuWidth, baseWidth)
		}
		baseLines[row] = before + menuLine + after
	}
	return strings.Join(baseLines, "\n")
}

// expandCompletionSnippet handles the common LSP snippet forms without
// exposing raw ${1:placeholder} syntax in the editor. It returns the plain text
// and the byte offset for the final $0 stop (or the end when none is present).
func expandCompletionSnippet(snippet string) (string, int) {
	var out strings.Builder
	stops := make(map[int]int)
	for i := 0; i < len(snippet); {
		if snippet[i] == '\\' && i+1 < len(snippet) && strings.ContainsRune("\\$}", rune(snippet[i+1])) {
			out.WriteByte(snippet[i+1])
			i += 2
			continue
		}
		if snippet[i] != '$' {
			r, size := utf8.DecodeRuneInString(snippet[i:])
			out.WriteRune(r)
			i += size
			continue
		}

		if i+1 < len(snippet) && snippet[i+1] >= '0' && snippet[i+1] <= '9' {
			j := i + 1
			for j < len(snippet) && snippet[j] >= '0' && snippet[j] <= '9' {
				j++
			}
			index, _ := strconv.Atoi(snippet[i+1 : j])
			if _, exists := stops[index]; !exists {
				stops[index] = out.Len()
			}
			i = j
			continue
		}

		if i+2 < len(snippet) && snippet[i+1] == '{' {
			end := snippetPlaceholderEnd(snippet, i+2)
			if end >= 0 {
				body := snippet[i+2 : end]
				indexEnd := 0
				for indexEnd < len(body) && body[indexEnd] >= '0' && body[indexEnd] <= '9' {
					indexEnd++
				}
				if indexEnd > 0 {
					index, _ := strconv.Atoi(body[:indexEnd])
					if _, exists := stops[index]; !exists {
						stops[index] = out.Len()
					}
					rest := body[indexEnd:]
					switch {
					case strings.HasPrefix(rest, ":"):
						text, _ := expandCompletionSnippet(rest[1:])
						out.WriteString(text)
					case strings.HasPrefix(rest, "|") && strings.HasSuffix(rest, "|"):
						choices := strings.Split(rest[1:len(rest)-1], ",")
						if len(choices) > 0 {
							out.WriteString(choices[0])
						}
					}
					i = end + 1
					continue
				}
			}
		}

		out.WriteByte('$')
		i++
	}

	cursor := out.Len()
	if offset, ok := stops[0]; ok {
		cursor = offset
	}
	return out.String(), cursor
}

func snippetPlaceholderEnd(snippet string, start int) int {
	depth := 0
	for i := start; i < len(snippet); i++ {
		if snippet[i] == '\\' {
			i++
			continue
		}
		if i+1 < len(snippet) && snippet[i] == '$' && snippet[i+1] == '{' {
			depth++
			i++
			continue
		}
		if snippet[i] == '}' {
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}
