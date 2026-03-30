# Markdown Line Wrapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable soft line wrapping for editable markdown files (`.md`, `.markdown`) in the toast terminal editor.

**Architecture:** Add a `wrapMode bool` field to `editor.Model`, set on file load based on extension. All wrapping logic lives in a new `wrap.go` file containing visual-row helpers. `clampViewport`, `moveCursorUp/Down`, `screenToBuffer`, and `View` each get a wrap-mode branch that uses those helpers.

**Tech Stack:** Go, bubbletea v2, lipgloss v2, tree-sitter (unchanged)

---

## File Map

| File | Change |
|---|---|
| `internal/components/editor/editor.go` | Add `wrapMode` field; update `fileLoadedMsg` handler, `clampViewport`, `moveCursorUp`, `moveCursorDown`, `moveCursorLeft`, `moveCursorRight`, `screenToBuffer`, `View` |
| `internal/components/editor/wrap.go` | **Create** — `wrapWidth`, `visualRowsForLine`, `visualRowFromTop`, `visualRowOfCursor`, `bufPosFromVisualRow` |
| `internal/components/editor/wrap_test.go` | **Create** — unit tests for all helpers and wrap-mode navigation |

---

### Task 1: Add `wrapMode` field and set it on file load

**Files:**
- Modify: `internal/components/editor/editor.go`
- Test: `internal/components/editor/editor_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/components/editor/editor_test.go`:

```go
func TestWrapMode_SetOnMarkdownLoad(t *testing.T) {
	m := newTestModel("")
	if m.wrapMode {
		t.Fatal("wrapMode should be false before any file load")
	}

	// Simulate a markdown file load.
	mdMsg := fileLoadedMsg{bufferID: 0, path: "notes.md", content: "hello\n"}
	model, _ := m.Update(mdMsg)
	m = model.(Model)
	if !m.wrapMode {
		t.Fatal("wrapMode should be true after loading .md file")
	}

	// Simulate a Go file load.
	goMsg := fileLoadedMsg{bufferID: 0, path: "main.go", content: "package main\n"}
	model, _ = m.Update(goMsg)
	m = model.(Model)
	if m.wrapMode {
		t.Fatal("wrapMode should be false after loading .go file")
	}

	// Simulate a .markdown extension.
	mdMsg2 := fileLoadedMsg{bufferID: 0, path: "README.markdown", content: "# hi\n"}
	model, _ = m.Update(mdMsg2)
	m = model.(Model)
	if !m.wrapMode {
		t.Fatal("wrapMode should be true after loading .markdown file")
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```
go test ./internal/components/editor/... -v -run TestWrapMode_SetOnMarkdownLoad
```

Expected: `FAIL` — `m.wrapMode undefined`

- [ ] **Step 3: Add the field and set it in the `fileLoadedMsg` handler**

In `internal/components/editor/editor.go`, add `wrapMode bool` to the `Model` struct after `binaryFile`:

```go
	binaryFile  bool
	wrapMode    bool
	highlighter *syntax.Highlighter
```

In the `fileLoadedMsg` case (around line 149), add `m.wrapMode = isMarkdownPath(msg.path)` after `m.viewportLeft = 0`:

```go
		m.viewportLeft = 0
		m.wrapMode = isMarkdownPath(msg.path)
		if msg.isBinary {
```

Add the helper at the bottom of `editor.go` (before the last closing brace of the file, near the other small helpers):

```go
// isMarkdownPath returns true if path has a .md or .markdown extension.
func isMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}
```

- [ ] **Step 4: Run test to confirm it passes**

```
go test ./internal/components/editor/... -v -run TestWrapMode_SetOnMarkdownLoad
```

Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/components/editor/editor.go internal/components/editor/editor_test.go
git commit -m "feat(editor): add wrapMode field, set on markdown file load"
```

---

### Task 2: Add visual row helpers in `wrap.go`

**Files:**
- Create: `internal/components/editor/wrap.go`
- Create: `internal/components/editor/wrap_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/components/editor/wrap_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/components/editor/... -v -run "TestWrapWidth|TestVisualRow|TestBufPos"
```

Expected: `FAIL` — undefined methods

- [ ] **Step 3: Create `wrap.go` with all helpers**

Create `internal/components/editor/wrap.go`:

```go
package editor

// wrapWidth returns the number of content columns available for wrapped lines.
// Returns at least 1 to avoid division by zero.
func (m *Model) wrapWidth() int {
	w := m.viewWidth - m.gutterWidth
	if w < 1 {
		return 1
	}
	return w
}

// visualRowsForLine returns the number of screen rows that buffer line bufLine
// occupies in wrap mode. Always returns at least 1. In non-wrap mode, always 1.
func (m *Model) visualRowsForLine(bufLine int) int {
	if !m.wrapMode {
		return 1
	}
	raw := m.buf.LineAt(bufLine)
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	n := len(raw)
	w := m.wrapWidth()
	if n == 0 {
		return 1
	}
	rows := (n + w - 1) / w
	if rows < 1 {
		rows = 1
	}
	return rows
}

// visualRowFromTop returns the 0-based absolute visual row index of the first
// visual row of bufLine, counting from the top of the buffer.
func (m *Model) visualRowFromTop(bufLine int) int {
	row := 0
	for l := 0; l < bufLine; l++ {
		row += m.visualRowsForLine(l)
	}
	return row
}

// visualRowOfCursor returns the 0-based absolute visual row index for the
// current cursor position.
func (m *Model) visualRowOfCursor() int {
	row := m.visualRowFromTop(m.cursor.line)
	if m.wrapMode {
		row += m.cursor.col / m.wrapWidth()
	}
	return row
}

// bufPosFromVisualRow maps an absolute visual row index to a (bufLine, bufCol)
// pair. bufCol is the byte offset of the start of that visual chunk within the
// buffer line. If targetVR is past the last visual row, the last buffer position
// is returned.
func (m *Model) bufPosFromVisualRow(targetVR int) (bufLine, bufCol int) {
	lineCount := m.buf.LineCount()
	vr := 0
	for l := 0; l < lineCount; l++ {
		rows := m.visualRowsForLine(l)
		if vr+rows > targetVR {
			chunkIndex := targetVR - vr
			bufLine = l
			bufCol = chunkIndex * m.wrapWidth()
			lineLen := m.lineContentLen(l)
			if bufCol > lineLen {
				bufCol = lineLen
			}
			return
		}
		vr += rows
	}
	// Past end of buffer.
	if lineCount > 0 {
		bufLine = lineCount - 1
		bufCol = m.lineContentLen(bufLine)
	}
	return
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```
go test ./internal/components/editor/... -v -run "TestWrapWidth|TestVisualRow|TestBufPos"
```

Expected: all `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/components/editor/wrap.go internal/components/editor/wrap_test.go
git commit -m "feat(editor): add visual row helpers for wrap mode"
```

---

### Task 3: Update `clampViewport` for wrap mode

**Files:**
- Modify: `internal/components/editor/editor.go`
- Test: `internal/components/editor/wrap_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/components/editor/wrap_test.go`:

```go
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

	// viewportTop must be at least line 1 so visual row 3 is within [topVR, topVR+2).
	// Visual row 3 is in [topVR, topVR+viewHeight) means topVR <= 3 < topVR+2 → topVR >= 2.
	// Line 1 starts at visual row 1; line 2 starts at visual row 3.
	// We need topVR+2 > 3, so topVR >= 2. Line 2 starts at visual row 3 → viewportTop=2 works.
	if m.viewportTop < 1 {
		t.Fatalf("clampViewport did not scroll down: viewportTop = %d", m.viewportTop)
	}
	// Cursor visual row must be within viewport.
	topVR := m.visualRowFromTop(m.viewportTop)
	cursorVR := m.visualRowOfCursor()
	if cursorVR < topVR || cursorVR >= topVR+m.viewHeight {
		t.Fatalf("cursor visual row %d not in viewport [%d, %d)", cursorVR, topVR, topVR+m.viewHeight)
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/components/editor/... -v -run "TestClampViewport_WrapMode"
```

Expected: `FAIL` — existing clampViewport ignores wrapMode

- [ ] **Step 3: Update `clampViewport`**

Replace the body of `clampViewport` in `internal/components/editor/editor.go`. The full updated function:

```go
func (m *Model) clampViewport() {
	if m.wrapMode {
		m.viewportLeft = 0
		if m.viewHeight <= 0 {
			return
		}

		cursorVR := m.visualRowOfCursor()
		topVR := m.visualRowFromTop(m.viewportTop)

		// Cursor above viewport: scroll up — find the buffer line whose first
		// visual row is <= cursorVR.
		if cursorVR < topVR {
			vr := 0
			for l := 0; l < m.buf.LineCount(); l++ {
				rows := m.visualRowsForLine(l)
				if vr+rows > cursorVR {
					m.viewportTop = l
					return
				}
				vr += rows
			}
			m.viewportTop = 0
			return
		}

		// Cursor below viewport: advance viewportTop until cursor is visible.
		if cursorVR >= topVR+m.viewHeight {
			targetTopVR := cursorVR - m.viewHeight + 1
			vr := 0
			for l := 0; l < m.buf.LineCount(); l++ {
				rows := m.visualRowsForLine(l)
				if vr+rows > targetTopVR {
					m.viewportTop = l
					return
				}
				vr += rows
			}
		}
		return
	}

	// ── Non-wrap mode (original logic) ──────────────────────────────────────
	// Vertical.
	if m.cursor.line < m.viewportTop {
		m.viewportTop = m.cursor.line
	}
	if m.viewHeight > 0 && m.cursor.line >= m.viewportTop+m.viewHeight {
		m.viewportTop = m.cursor.line - m.viewHeight + 1
	}
	if m.viewportTop < 0 {
		m.viewportTop = 0
	}

	// Horizontal: ensure cursor col is visible.
	cursorScreenCol := m.cursor.col - m.viewportLeft
	contentWidth := m.viewWidth - m.gutterWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	if cursorScreenCol < 0 {
		m.viewportLeft += cursorScreenCol
		if m.viewportLeft < 0 {
			m.viewportLeft = 0
		}
	}
	if cursorScreenCol >= contentWidth {
		m.viewportLeft += cursorScreenCol - contentWidth + 1
	}
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/components/editor/... -v -run "TestClampViewport_WrapMode"
```

Expected: all `PASS`

- [ ] **Step 5: Run full test suite to ensure no regressions**

```
go test ./internal/components/editor/... -v
```

Expected: all existing tests still `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/components/editor/editor.go internal/components/editor/wrap_test.go
git commit -m "feat(editor): update clampViewport for wrap mode"
```

---

### Task 4: Update cursor up/down navigation for wrap mode

**Files:**
- Modify: `internal/components/editor/editor.go`
- Test: `internal/components/editor/wrap_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/components/editor/wrap_test.go`:

```go
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
	// Down moves into second chunk of line 0 → col should be 20+5=25, but line
	// is only 21 bytes, so clamped to 21. Then down again → line 1, col 3 (clamped).
	m := newWrapModel("123456789012345678901\nend\n")
	m.viewHeight = 10
	m.cursor = cursorPos{line: 0, col: 5}
	m.preferredCol = 5 // visual col = 5 % 20 = 5
	m.moveCursorDown(1)

	// Should be on second chunk: col = min(20+5, 21) = 21. Line 0 content is 21 bytes.
	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0", m.cursor.line)
	}
	if m.cursor.col != 21 {
		t.Fatalf("cursor.col = %d, want 21", m.cursor.col)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```
go test ./internal/components/editor/... -v -run "TestMoveCursor.*WrapMode"
```

Expected: `FAIL` — existing moveCursorUp/Down ignore wrapMode

- [ ] **Step 3: Update `moveCursorUp` in `editor.go`**

Replace the entire `moveCursorUp` method:

```go
func (m *Model) moveCursorUp(n int) {
	if m.wrapMode {
		currentVR := m.visualRowOfCursor()
		targetVR := currentVR - n
		if targetVR < 0 {
			targetVR = 0
		}
		bufLine, chunkStart := m.bufPosFromVisualRow(targetVR)
		w := m.wrapWidth()
		chunkEnd := chunkStart + w
		lineLen := m.lineContentLen(bufLine)
		if chunkEnd > lineLen {
			chunkEnd = lineLen
		}
		targetCol := chunkStart + m.preferredCol
		if targetCol > chunkEnd {
			targetCol = chunkEnd
		}
		m.cursor.line = bufLine
		m.cursor.col = targetCol
		return
	}
	if m.cursor.line == 0 {
		m.cursor.col = 0
		m.preferredCol = 0
		return
	}
	m.cursor.line -= n
	if m.cursor.line < 0 {
		m.cursor.line = 0
	}
	m.cursor.col = m.clampCol(m.cursor.line, m.preferredCol)
}
```

Replace the entire `moveCursorDown` method:

```go
func (m *Model) moveCursorDown(n int) {
	if m.wrapMode {
		currentVR := m.visualRowOfCursor()
		targetVR := currentVR + n
		// Total visual rows in the buffer.
		lineCount := m.buf.LineCount()
		maxVR := m.visualRowFromTop(lineCount) - 1
		if maxVR < 0 {
			maxVR = 0
		}
		if targetVR > maxVR {
			targetVR = maxVR
		}
		bufLine, chunkStart := m.bufPosFromVisualRow(targetVR)
		w := m.wrapWidth()
		chunkEnd := chunkStart + w
		lineLen := m.lineContentLen(bufLine)
		if chunkEnd > lineLen {
			chunkEnd = lineLen
		}
		targetCol := chunkStart + m.preferredCol
		if targetCol > chunkEnd {
			targetCol = chunkEnd
		}
		m.cursor.line = bufLine
		m.cursor.col = targetCol
		return
	}
	lastLine := m.buf.LineCount() - 1
	if lastLine < 0 {
		lastLine = 0
	}
	if m.cursor.line >= lastLine {
		m.cursor.col = m.lineContentLen(lastLine)
		m.preferredCol = m.cursor.col
		return
	}
	m.cursor.line += n
	if m.cursor.line > lastLine {
		m.cursor.line = lastLine
	}
	m.cursor.col = m.clampCol(m.cursor.line, m.preferredCol)
}
```

- [ ] **Step 4: Update `preferredCol` assignment in `moveCursorLeft` and `moveCursorRight`**

In `moveCursorLeft`, the last line is `m.preferredCol = m.cursor.col`. Change it to:

```go
	if m.wrapMode {
		m.preferredCol = m.cursor.col % m.wrapWidth()
	} else {
		m.preferredCol = m.cursor.col
	}
```

In `moveCursorRight`, the last line is `m.preferredCol = m.cursor.col`. Change it to:

```go
	if m.wrapMode {
		m.preferredCol = m.cursor.col % m.wrapWidth()
	} else {
		m.preferredCol = m.cursor.col
	}
```

- [ ] **Step 5: Run tests**

```
go test ./internal/components/editor/... -v -run "TestMoveCursor.*WrapMode"
```

Expected: all `PASS`

- [ ] **Step 6: Run full test suite**

```
go test ./internal/components/editor/... -v
```

Expected: all `PASS`

- [ ] **Step 7: Commit**

```bash
git add internal/components/editor/editor.go internal/components/editor/wrap_test.go
git commit -m "feat(editor): update cursor up/down navigation for wrap mode"
```

---

### Task 5: Update `View` rendering for wrap mode

**Files:**
- Modify: `internal/components/editor/editor.go`

No unit test — `View` produces terminal escape sequences. Verify by opening a `.md` file and observing line wrapping. The existing `View` tests (if any) continue to work unchanged.

- [ ] **Step 1: Run full tests to establish a baseline before editing View**

```
go test ./internal/components/editor/... -v
```

Expected: all `PASS`

- [ ] **Step 2: Update the render loop in `View`**

Locate the render loop in `View` that starts with `for screenRow := 0; screenRow < m.viewHeight; screenRow++` (around line 1085). Replace the entire loop with:

```go
	for screenRow := 0; screenRow < m.viewHeight; screenRow++ {
		// In wrap mode, map screenRow → (bufLine, chunkIndex).
		// In non-wrap mode, chunkIndex is always 0 and bufLine = viewportTop + screenRow.
		var bufLine, chunkIndex int
		if m.wrapMode {
			topVR := m.visualRowFromTop(m.viewportTop)
			bl, chunkStart := m.bufPosFromVisualRow(topVR + screenRow)
			bufLine = bl
			if m.wrapWidth() > 0 {
				chunkIndex = chunkStart / m.wrapWidth()
			}
		} else {
			bufLine = m.viewportTop + screenRow
		}

		// ── Gutter ──────────────────────────────────────────────────────────
		lineNumStr := ""
		diffBar := ""
		if bufLine < lineCount {
			if !m.wrapMode || chunkIndex == 0 {
				lineNumStr = fmt.Sprintf("%*d", m.gutterWidth-3, bufLine+1)
				var kind messages.GitLineKind
				if bufLine < len(m.lineKinds) {
					kind = m.lineKinds[bufLine]
				}
				diffBar = gitDiffBar(kind, m.theme)
			} else {
				// Continuation row: blank line number, no diff bar colour.
				lineNumStr = strings.Repeat(" ", m.gutterWidth-3)
				diffBar = gitDiffBar(messages.GitLineUnchanged, m.theme)
			}
		} else {
			lineNumStr = strings.Repeat(" ", m.gutterWidth-3)
			diffBar = gitDiffBar(messages.GitLineUnchanged, m.theme)
		}

		gutter := gutterStyle.Render(lineNumStr+" ") + diffBar + gutterStyle.Render(" ")

		// ── Content ─────────────────────────────────────────────────────────
		isCurrentLine := bufLine == m.cursor.line

		var lineContent string
		if bufLine < lineCount {
			raw := m.buf.LineAt(bufLine)
			if len(raw) > 0 && raw[len(raw)-1] == '\n' {
				raw = raw[:len(raw)-1]
			}

			if m.wrapMode {
				// Slice to the chunk for this visual row.
				w := m.wrapWidth()
				chunkStart := chunkIndex * w
				chunkEnd := chunkStart + w
				if chunkStart > len(raw) {
					chunkStart = len(raw)
				}
				if chunkEnd > len(raw) {
					chunkEnd = len(raw)
				}
				lineContent = raw[chunkStart:chunkEnd]
			} else {
				lineContent = raw
				// Apply viewport left offset (horizontal scroll).
				if m.viewportLeft > 0 && len(lineContent) > m.viewportLeft {
					lineContent = lineContent[m.viewportLeft:]
				} else if m.viewportLeft > 0 {
					lineContent = ""
				}
			}
		}

		contentWidth := m.viewWidth - m.gutterWidth
		if contentWidth < 0 {
			contentWidth = 0
		}

		// In non-wrap mode, truncate to contentWidth.
		if !m.wrapMode && len(lineContent) > contentWidth {
			lineContent = lineContent[:contentWidth]
		}

		// Compute selection range for this chunk.
		// selRange is raw line-relative byte offsets (before any viewportLeft adjustment).
		var lineSelRange *[2]int
		if start, end, active := m.selectionRange(); active {
			lineStart := m.buf.OffsetForLine(bufLine)
			selStartOff := m.buf.OffsetForLine(start.line) + start.col
			selEndOff := m.buf.OffsetForLine(end.line) + end.col
			lineContentLen := m.lineContentLen(bufLine)
			lineContentEnd := lineStart + lineContentLen

			// Clamp selection to this chunk when in wrap mode.
			chunkByteStart := 0
			chunkByteEnd := lineContentLen
			if m.wrapMode {
				w := m.wrapWidth()
				chunkByteStart = chunkIndex * w
				chunkByteEnd = chunkByteStart + w
				if chunkByteEnd > lineContentLen {
					chunkByteEnd = lineContentLen
				}
				_ = lineContentEnd
			}
			_ = lineContentEnd

			if lineStart+chunkByteEnd > selStartOff && lineStart+chunkByteStart < selEndOff {
				rawStart := selStartOff - lineStart - chunkByteStart
				if rawStart < 0 {
					rawStart = 0
				}
				rawEnd := selEndOff - lineStart - chunkByteStart
				if rawEnd > chunkByteEnd-chunkByteStart {
					rawEnd = chunkByteEnd - chunkByteStart
				}
				r := [2]int{rawStart, rawEnd}
				lineSelRange = &r
			}
		}

		// The lineOffset passed to renderHighlightedLine is the raw line-relative
		// byte index where lineContent starts (used for highlight span adjustment).
		lineOffset := m.viewportLeft
		if m.wrapMode {
			lineOffset = chunkIndex * m.wrapWidth()
		}

		var renderedContent string
		if isCurrentLine && m.focused {
			if !m.wrapMode && len(lineContent) > contentWidth {
				lineContent = lineContent[:contentWidth]
			}
			renderedContent = m.renderHighlightedLine(bufLine, lineContent, lineHighlight, contentWidth, lineSelRange, lineOffset)
		} else {
			if !m.wrapMode && len(lineContent) > contentWidth {
				lineContent = lineContent[:contentWidth]
			}
			renderedContent = m.renderHighlightedLine(bufLine, lineContent, bgColor, contentWidth, lineSelRange, lineOffset)
		}

		if screenRow > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(gutter)
		sb.WriteString(renderedContent)
	}
```

- [ ] **Step 3: Update cursor screen position calculation in `View`**

Find the cursor position block after the loop (around line 1178):

```go
	v := tea.NewView(sb.String())
	if m.focused {
		cursorScreenX := m.gutterWidth + (m.cursor.col - m.viewportLeft)
		cursorScreenY := m.cursor.line - m.viewportTop
```

Replace with:

```go
	v := tea.NewView(sb.String())
	if m.focused {
		var cursorScreenX, cursorScreenY int
		if m.wrapMode {
			w := m.wrapWidth()
			chunkStart := (m.cursor.col / w) * w
			cursorScreenX = m.gutterWidth + (m.cursor.col - chunkStart)
			topVR := m.visualRowFromTop(m.viewportTop)
			cursorScreenY = m.visualRowOfCursor() - topVR
		} else {
			cursorScreenX = m.gutterWidth + (m.cursor.col - m.viewportLeft)
			cursorScreenY = m.cursor.line - m.viewportTop
		}
```

- [ ] **Step 4: Run full test suite**

```
go test ./internal/components/editor/... -v
```

Expected: all `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/components/editor/editor.go
git commit -m "feat(editor): update View to render wrapped lines for markdown"
```

---

### Task 6: Update `screenToBuffer` for wrap mode mouse clicks

**Files:**
- Modify: `internal/components/editor/editor.go`
- Test: `internal/components/editor/wrap_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/components/editor/wrap_test.go`:

```go
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
```

- [ ] **Step 2: Run test to confirm it fails**

```
go test ./internal/components/editor/... -v -run "TestScreenToBuffer_WrapMode"
```

Expected: `FAIL` — existing screenToBuffer ignores wrapMode

- [ ] **Step 3: Update `screenToBuffer` in `editor.go`**

Replace the `screenToBuffer` method:

```go
func (m *Model) screenToBuffer(x, y int) (line, col int) {
	if m.wrapMode {
		topVR := m.visualRowFromTop(m.viewportTop)
		targetVR := topVR + y
		bufLine, chunkStart := m.bufPosFromVisualRow(targetVR)
		visualCol := x - m.gutterWidth
		if visualCol < 0 {
			visualCol = 0
		}
		bufCol := chunkStart + visualCol
		lineLen := m.lineContentLen(bufLine)
		if bufCol > lineLen {
			bufCol = lineLen
		}
		return bufLine, bufCol
	}

	line = m.viewportTop + y
	lineCount := m.buf.LineCount()
	if line >= lineCount {
		line = lineCount - 1
	}
	if line < 0 {
		line = 0
	}
	col = m.viewportLeft + (x - m.gutterWidth)
	if col < 0 {
		col = 0
	}
	lineLen := m.lineContentLen(line)
	if col > lineLen {
		col = lineLen
	}
	return line, col
}
```

- [ ] **Step 4: Run tests**

```
go test ./internal/components/editor/... -v -run "TestScreenToBuffer_WrapMode"
```

Expected: `PASS`

- [ ] **Step 5: Run full test suite**

```
go test ./internal/components/editor/... -v
```

Expected: all `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/components/editor/editor.go internal/components/editor/wrap_test.go
git commit -m "feat(editor): update screenToBuffer for wrap mode mouse clicks"
```

---

### Task 7: Manual verification

- [ ] **Step 1: Build and run**

```bash
go build ./cmd/toast/... && ./toast
```

- [ ] **Step 2: Open a long-line markdown file**

Open a `.md` file that has lines longer than the terminal width. Confirm:
- Long lines wrap onto continuation rows
- Line numbers show only on the first visual row of each buffer line
- Cursor moves through wrapped lines with up/down arrow keys
- Clicking on a wrapped continuation row positions the cursor correctly
- Editing (typing, backspace) works normally
- Opening a `.go` file shows no wrapping (horizontal scroll still works)

- [ ] **Step 3: Commit if any small fixes were needed from manual testing**

```bash
git add -p && git commit -m "fix(editor): wrap mode corrections from manual testing"
```
