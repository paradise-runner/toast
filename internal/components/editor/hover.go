package editor

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/messages"
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

	// Truncate long content lines ourselves: lipgloss MaxWidth truncates the
	// content but not the top/bottom borders, producing a broken box (border
	// wider than the content and a missing right edge).
	lines := strings.Split(hs.contents, "\n")
	for i, line := range lines {
		lines[i] = truncateCells(line, hoverMaxWidth)
	}
	if len(lines) > hoverMaxLines {
		lines = lines[:hoverMaxLines]
		lines = append(lines, "…")
	}

	// Render each content line at a fixed width so the box is uniform.
	inner := lipgloss.NewStyle().Background(bg).Foreground(fg).Width(hoverMaxWidth)
	for i, line := range lines {
		lines[i] = inner.Render(line)
	}

	// The border ring (including the corner cells) carries the hover
	// background so the whole tooltip is one themed surface. Terminal cells
	// are square, so the corners read as square — accepted trade-off.
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).BorderForeground(border).BorderBackground(bg).
		Padding(0, 1).Background(bg)
	return style.Render(strings.Join(lines, "\n"))
}

// truncateCells truncates s to at most width display cells, appending an
// ellipsis when content had to be cut.
func truncateCells(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	var sb strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw >= width {
			break
		}
		sb.WriteRune(r)
		w += rw
	}
	return sb.String() + "…"
}

// overlayHover composites the hover tooltip over the editor's rendered ANSI
// content near the mouse position, preferring to place it above the cursor.
func overlayHover(base, tooltip string, x, y, width, height int) string {
	if tooltip == "" || width <= 0 || height <= 0 {
		return base
	}
	baseLines := strings.Split(base, "\n")
	tooltipLines := strings.Split(tooltip, "\n")
	boxHeight := len(tooltipLines)

	// Prefer above the mouse; fall back to below, then clamp.
	boxY := y - boxHeight - 1
	if boxY < 0 {
		boxY = y + 1
	}
	if boxY < 0 {
		boxY = 0
	}
	if boxY+boxHeight > height {
		boxY = height - boxHeight
		if boxY < 0 {
			boxY = 0
		}
	}

	boxWidth := 0
	for _, line := range tooltipLines {
		if lineWidth := lipgloss.Width(line); lineWidth > boxWidth {
			boxWidth = lineWidth
		}
	}
	boxX := x + 1
	if boxX+boxWidth > width {
		boxX = width - boxWidth
		if boxX < 0 {
			boxX = 0
		}
	}

	for i, tooltipLine := range tooltipLines {
		row := boxY + i
		if row >= len(baseLines) || row >= height {
			break
		}
		baseLine := baseLines[row]
		baseWidth := lipgloss.Width(baseLine)
		before := ansi.Truncate(baseLine, boxX, "")
		after := ""
		if boxX+boxWidth < baseWidth {
			after = ansi.Cut(baseLine, boxX+boxWidth, baseWidth)
		}
		baseLines[row] = before + tooltipLine + after
	}
	return strings.Join(baseLines, "\n")
}

// diagnosticAt returns the diagnostic whose range covers (line, col) — col is
// a byte offset in the line — or nil. LSP columns are UTF-16 code units, so
// they are converted before comparison. Multi-line diagnostics span their
// whole intermediate lines.
func (m Model) diagnosticAt(line, col int) *messages.Diagnostic {
	if line < 0 || col < 0 || len(m.diagnostics) == 0 {
		return nil
	}
	raw := m.lineContent(line)
	for i := range m.diagnostics {
		d := &m.diagnostics[i]
		if d.Line > line || d.EndLine < line {
			continue
		}
		start, end := 0, len(raw)
		if d.Line == line {
			start = utf16ColToByte(raw, d.Col)
		}
		if d.EndLine == line {
			end = utf16ColToByte(raw, d.EndCol)
		}
		if col >= start && col < end {
			return d
		}
	}
	return nil
}
