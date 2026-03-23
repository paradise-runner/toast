package filetree

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// overlayAt composites overlay over base, anchored at (x, y).
// The position is clamped so the overlay's bounding box never starts beyond
// (totalWidth-ow, totalHeight-oh). Overlay lines are truncated to fit within
// totalWidth from position x.
func overlayAt(base, overlay string, x, y, totalWidth, totalHeight int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Measure overlay visual dimensions.
	ow := 0
	for _, l := range overlayLines {
		if w := lipgloss.Width(l); w > ow {
			ow = w
		}
	}
	oh := len(overlayLines)

	// Clamp position.
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

		// Pad base line if it doesn't reach x.
		if blWidth < x {
			bl += strings.Repeat(" ", x-blWidth)
		}

		before := ansi.Truncate(bl, x, "")
		// Truncate overlay line to the available canvas width.
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
