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

func TestScreenToBuffer_WrapMode(t *testing.T) {
	// Line 0: "hello\n" (5 bytes) → 1 visual row.
	// Line 1: "123456789012345678901\n" (21 bytes) → 2 visual rows.
	// Gutter width = 4, wrapWidth = 20.
	//
	// Screen row 0 → visual row 0 → bufLine=0, chunkStart=0
	// Screen row 1 → visual row 1 → bufLine=1, chunkStart=0
	// Screen row 2 → visual row 2 → bufLine=1, chunkStart=20
	m := newWrapModel("hello\n123456789012345678901\nx\n")
	m.viewportTop = 0

	// Click at screen (gutterWidth + 3, 0) → line 0, col 3
	line, col := m.screenToBuffer(m.gutterWidth+3, 0)
	if line != 0 || col != 3 {
		t.Fatalf("click row 0 col 3: got (%d,%d), want (0,3)", line, col)
	}

	// Click at screen (gutterWidth + 0, 1) → line 1, col 0
	line, col = m.screenToBuffer(m.gutterWidth+0, 1)
	if line != 1 || col != 0 {
		t.Fatalf("click row 1 col 0: got (%d,%d), want (1,0)", line, col)
	}

	// Click at screen (gutterWidth + 0, 2) → line 1, chunkStart=20, col=20
	line, col = m.screenToBuffer(m.gutterWidth+0, 2)
	if line != 1 || col != 20 {
		t.Fatalf("click row 2 col 0: got (%d,%d), want (1,20)", line, col)
	}

	// Click at screen (gutterWidth + 1, 2) → line 1, col 21 (clamped to lineLen=21)
	line, col = m.screenToBuffer(m.gutterWidth+1, 2)
	if line != 1 || col != 21 {
		t.Fatalf("click row 2 col 1: got (%d,%d), want (1,21)", line, col)
	}
}

func TestMoveCursorDown_WrapMode_WithinLine(t *testing.T) {
	// Line 0: 21 bytes → 2 visual rows (wrapWidth=20).
	// Pressing down from col 0 should move to col 20 (start of second chunk),
	// still on line 0.
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 0, col: 0}
	m.preferredCol = 0
	m.moveCursorDown(1)

	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0 (should stay on same buffer line)", m.cursor.line)
	}
	if m.cursor.col != 20 {
		t.Fatalf("cursor.col = %d, want 20 (start of second chunk)", m.cursor.col)
	}
}

func TestMoveCursorDown_WrapMode_ToNextLine(t *testing.T) {
	// Line 0: 21 bytes → 2 visual rows.
	// Cursor at col 20 (second chunk). Down should go to line 1, col 0.
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 0, col: 20}
	m.preferredCol = 0
	m.moveCursorDown(1)

	if m.cursor.line != 1 {
		t.Fatalf("cursor.line = %d, want 1", m.cursor.line)
	}
	if m.cursor.col != 0 {
		t.Fatalf("cursor.col = %d, want 0", m.cursor.col)
	}
}

func TestMoveCursorUp_WrapMode_WithinLine(t *testing.T) {
	// Line 0: 21 bytes → 2 visual rows.
	// Cursor at col 20 (second chunk). Up should go to col 0 (first chunk).
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 0, col: 20}
	m.preferredCol = 0
	m.moveCursorUp(1)

	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0", m.cursor.line)
	}
	if m.cursor.col != 0 {
		t.Fatalf("cursor.col = %d, want 0", m.cursor.col)
	}
}

func TestMoveCursorUp_WrapMode_ToPrevLine(t *testing.T) {
	// Line 0: 21 bytes → 2 visual rows.
	// Line 1: "end\n" → 1 visual row.
	// Cursor at line 1, col 0. Up should land on second chunk of line 0 (col 20).
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 1, col: 0}
	m.preferredCol = 0
	m.moveCursorUp(1)

	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0", m.cursor.line)
	}
	if m.cursor.col != 20 {
		t.Fatalf("cursor.col = %d, want 20 (start of last chunk of line 0)", m.cursor.col)
	}
}

func TestMoveCursorDown_WrapMode_PreservesPreferredCol(t *testing.T) {
	// Line 0: 21 bytes → 2 visual rows (wrapWidth=20).
	// Line 1: "end\n" → 1 visual row (3 bytes).
	// Cursor at line 0, col 5 (preferredCol in visual row = 5).
	// Down moves into second chunk of line 0 → col should be min(20+5, 21) = 21.
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 0, col: 5}
	m.preferredCol = 5
	m.moveCursorDown(1)

	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0", m.cursor.line)
	}
	if m.cursor.col != 21 {
		t.Fatalf("cursor.col = %d, want 21", m.cursor.col)
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
