package editor

import (
	"testing"

	"github.com/yourusername/toast/internal/buffer"
	"github.com/yourusername/toast/internal/config"
)

// newWrapModel creates a model configured for wrap-mode testing.
// viewWidth=24, gutterWidth=4 → wrapWidth=20
func newWrapModel(content string) Model {
	m := New(nil, config.Config{})
	m.buf = buffer.NewEditBuffer(content)
	m.wrapMode = true
	m.viewWidth = 24
	m.viewHeight = 10
	m.gutterWidth = 4 // 1 digit + 3 padding
	return m
}

func TestWrapWidth(t *testing.T) {
	m := newWrapModel("")
	if got := m.wrapWidth(); got != 20 {
		t.Fatalf("wrapWidth = %d, want 20", got)
	}
}

func TestVisualRowsForLine_Short(t *testing.T) {
	// "hello" is 5 bytes → fits in 20 cols → 1 visual row
	m := newWrapModel("hello\n")
	if got := m.visualRowsForLine(0); got != 1 {
		t.Fatalf("visualRowsForLine(0) = %d, want 1", got)
	}
}

func TestVisualRowsForLine_Exact(t *testing.T) {
	// 20 bytes exactly → 1 visual row
	m := newWrapModel("12345678901234567890\n")
	if got := m.visualRowsForLine(0); got != 1 {
		t.Fatalf("visualRowsForLine(0) = %d, want 1", got)
	}
}

func TestVisualRowsForLine_Wraps(t *testing.T) {
	// 21 bytes → 2 visual rows
	m := newWrapModel("123456789012345678901\n")
	if got := m.visualRowsForLine(0); got != 2 {
		t.Fatalf("visualRowsForLine(0) = %d, want 2", got)
	}
}

func TestVisualRowsForLine_Empty(t *testing.T) {
	m := newWrapModel("\n")
	if got := m.visualRowsForLine(0); got != 1 {
		t.Fatalf("empty line: visualRowsForLine(0) = %d, want 1", got)
	}
}

func TestVisualRowFromTop(t *testing.T) {
	// Line 0: "hello\n" (5 bytes) → 1 row
	// Line 1: "123456789012345678901\n" (21 bytes) → 2 rows
	// Line 2: "x\n" (1 byte) → 1 row
	m := newWrapModel("hello\n123456789012345678901\nx\n")
	if got := m.visualRowFromTop(0); got != 0 {
		t.Fatalf("visualRowFromTop(0) = %d, want 0", got)
	}
	if got := m.visualRowFromTop(1); got != 1 {
		t.Fatalf("visualRowFromTop(1) = %d, want 1", got)
	}
	if got := m.visualRowFromTop(2); got != 3 {
		t.Fatalf("visualRowFromTop(2) = %d, want 3", got)
	}
}

func TestVisualRowOfCursor(t *testing.T) {
	// Same content as above.
	m := newWrapModel("hello\n123456789012345678901\nx\n")

	// Cursor at line 0, col 0 → visual row 0
	m.cursor = cursorPos{line: 0, col: 0}
	if got := m.visualRowOfCursor(); got != 0 {
		t.Fatalf("cursor {0,0}: visualRowOfCursor = %d, want 0", got)
	}

	// Cursor at line 1, col 0 → visual row 1 (first chunk of line 1)
	m.cursor = cursorPos{line: 1, col: 0}
	if got := m.visualRowOfCursor(); got != 1 {
		t.Fatalf("cursor {1,0}: visualRowOfCursor = %d, want 1", got)
	}

	// Cursor at line 1, col 20 → visual row 2 (second chunk of line 1)
	m.cursor = cursorPos{line: 1, col: 20}
	if got := m.visualRowOfCursor(); got != 2 {
		t.Fatalf("cursor {1,20}: visualRowOfCursor = %d, want 2", got)
	}

	// Cursor at line 2, col 0 → visual row 3
	m.cursor = cursorPos{line: 2, col: 0}
	if got := m.visualRowOfCursor(); got != 3 {
		t.Fatalf("cursor {2,0}: visualRowOfCursor = %d, want 3", got)
	}
}

func TestClampViewport_WrapMode_ScrollDown(t *testing.T) {
	// 3 lines each taking 1 visual row + line 1 is long (2 visual rows).
	// Total visual rows: 1 + 2 + 1 = 4.
	// viewHeight = 2, so cursor on visual row 3 must scroll viewportTop.
	m := newWrapModel("hello\n123456789012345678901\nx\n")
	m.viewHeight = 2
	m.viewportTop = 0

	// Cursor on visual row 3 (line 2, col 0).
	m.cursor = cursorPos{line: 2, col: 0}
	m.clampViewport()

	// Cursor visual row must be within viewport.
	topVR := m.visualRowFromTop(m.viewportTop)
	cursorVR := m.visualRowOfCursor()
	if cursorVR < topVR || cursorVR >= topVR+m.viewHeight {
		t.Fatalf("cursor visual row %d not in viewport [%d, %d)", cursorVR, topVR, topVR+m.viewHeight)
	}
	if m.viewportTop < 1 {
		t.Fatalf("clampViewport did not scroll down: viewportTop = %d", m.viewportTop)
	}
}

func TestClampViewport_WrapMode_ScrollUp(t *testing.T) {
	m := newWrapModel("hello\n123456789012345678901\nx\n")
	m.viewHeight = 2
	m.viewportTop = 2 // currently showing from line 2

	// Cursor at line 0, col 0 → above viewport.
	m.cursor = cursorPos{line: 0, col: 0}
	m.clampViewport()

	if m.viewportTop != 0 {
		t.Fatalf("clampViewport did not scroll up: viewportTop = %d, want 0", m.viewportTop)
	}
}

func TestClampViewport_WrapMode_ForcesViewportLeftZero(t *testing.T) {
	m := newWrapModel("hello\n")
	m.viewportLeft = 5
	m.cursor = cursorPos{line: 0, col: 0}
	m.clampViewport()
	if m.viewportLeft != 0 {
		t.Fatalf("clampViewport did not zero viewportLeft in wrap mode: got %d", m.viewportLeft)
	}
}

func TestBufPosFromVisualRow(t *testing.T) {
	// Same content as above.
	m := newWrapModel("hello\n123456789012345678901\nx\n")

	bufLine, bufCol := m.bufPosFromVisualRow(0)
	if bufLine != 0 || bufCol != 0 {
		t.Fatalf("visualRow 0 → (%d,%d), want (0,0)", bufLine, bufCol)
	}

	bufLine, bufCol = m.bufPosFromVisualRow(1)
	if bufLine != 1 || bufCol != 0 {
		t.Fatalf("visualRow 1 → (%d,%d), want (1,0)", bufLine, bufCol)
	}

	bufLine, bufCol = m.bufPosFromVisualRow(2)
	if bufLine != 1 || bufCol != 20 {
		t.Fatalf("visualRow 2 → (%d,%d), want (1,20)", bufLine, bufCol)
	}

	bufLine, bufCol = m.bufPosFromVisualRow(3)
	if bufLine != 2 || bufCol != 0 {
		t.Fatalf("visualRow 3 → (%d,%d), want (2,0)", bufLine, bufCol)
	}
}
