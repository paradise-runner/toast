package editor

import (
	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// handleGoToLine moves the cursor to the requested line (zero-based), clamps to
// the valid range, clears any selection, and scrolls the viewport to the target.
func (m Model) handleGoToLine(msg messages.GoToLineMsg) (tea.Model, tea.Cmd) {
	line := msg.Line
	if line < 0 {
		line = 0
	}
	if m.buf != nil {
		if lc := m.buf.LineCount(); lc > 0 && line >= lc {
			line = lc - 1
		}
	}
	m.cursor = cursorPos{line: line, col: 0}
	m.selectionAnchor = nil
	m.preferredCol = 0
	m.clampViewport()
	return m, nil
}

// LineCount returns the number of lines in the current buffer.
func (m Model) LineCount() int {
	if m.buf == nil {
		return 0
	}
	return m.buf.LineCount()
}
