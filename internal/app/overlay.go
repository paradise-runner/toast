package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// overlayAt composites overlay over base, anchored at (x, y).
// The position is clamped so the overlay never extends beyond totalWidth/totalHeight.
func overlayAt(base, overlay string, x, y, totalWidth, totalHeight int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	ow := 0
	for _, l := range overlayLines {
		if w := lipgloss.Width(l); w > ow {
			ow = w
		}
	}
	oh := len(overlayLines)

	if x+ow > totalWidth {
		x = totalWidth - ow
	}
	if x < 0 {
		x = 0
	}
	if y+oh > totalHeight {
		y = totalHeight - oh
	}
	if y < 0 {
		y = 0
	}

	for i, ol := range overlayLines {
		row := y + i
		if row >= len(baseLines) || row >= totalHeight {
			break
		}
		bl := baseLines[row]
		blWidth := lipgloss.Width(bl)
		if blWidth < x {
			bl += strings.Repeat(" ", x-blWidth)
		}
		before := ansi.Truncate(bl, x, "")
		availWidth := totalWidth - x
		if lipgloss.Width(ol) > availWidth {
			ol = ansi.Truncate(ol, availWidth, "")
		}
		after := ""
		if x+ow < blWidth {
			after = ansi.Cut(bl, x+ow, blWidth)
		}
		baseLines[row] = before + ol + after
	}
	return strings.Join(baseLines, "\n")
}

// overlayCenter composites overlayStr centered over base.
// base is a multi-line string that may contain ANSI escape sequences (from lipgloss).
// overlayStr is a lipgloss-rendered box (also ANSI-encoded).
func overlayCenter(base, overlay string, totalWidth, totalHeight int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Measure overlay visual width (lipgloss.Width strips ANSI)
	ow := 0
	for _, l := range overlayLines {
		if w := lipgloss.Width(l); w > ow {
			ow = w
		}
	}
	oh := len(overlayLines)

	startX := (totalWidth - ow) / 2
	startY := (totalHeight - oh) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	for i, ol := range overlayLines {
		y := startY + i
		if y >= len(baseLines) {
			break
		}
		bl := baseLines[y]
		blWidth := lipgloss.Width(bl)

		// Pad base line if it doesn't reach startX
		if blWidth < startX {
			bl += strings.Repeat(" ", startX-blWidth)
		}

		// ANSI-aware split: take first startX visual cols, then columns startX+ow onward.
		// ansi.Cut correctly re-emits any SGR state active at the cut point so that the
		// "after" segment has the right background without relying on the terminal's
		// accumulated state (which was reset by the overlay's own sequences).
		before := ansi.Truncate(bl, startX, "")
		after := ""
		if startX+ow < blWidth {
			after = ansi.Cut(bl, startX+ow, blWidth)
		}
		baseLines[y] = before + ol + after
	}
	return strings.Join(baseLines, "\n")
}
