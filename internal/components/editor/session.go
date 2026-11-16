package editor

import "sort"

// SessionState is the persisted part of one tab's editing state. It is a plain
// value so the app can store it in a session file: the buffer contents, syntax
// highlighter and diagnostics are re-derived from disk on restore.
type SessionState struct {
	CursorLine   int
	CursorCol    int
	ViewportTop  int
	ViewportLeft int
	// Folds holds the header line of every collapsed fold region. Only the
	// start is persisted; ApplySessionState re-derives the extent from the file
	// as it is now.
	Folds []int
}

// SessionState returns the persisted state of the active buffer.
func (m Model) SessionState() SessionState {
	return SessionState{
		CursorLine:   m.cursor.line,
		CursorCol:    m.cursor.col,
		ViewportTop:  m.viewportTop,
		ViewportLeft: m.viewportLeft,
		Folds:        foldHeaderLines(m.collapsed),
	}
}

// SessionState returns the persisted state of a buffered background tab, whose
// editing state lives in the snapshot rather than in the editor.
func (s BufferSnapshot) SessionState() SessionState {
	return SessionState{
		CursorLine:   s.cursor.line,
		CursorCol:    s.cursor.col,
		ViewportTop:  s.viewportTop,
		ViewportLeft: s.viewportLeft,
		Folds:        foldHeaderLines(s.collapsed),
	}
}

// foldHeaderLines lists the header lines of the given spans.
func foldHeaderLines(spans []foldSpan) []int {
	if len(spans) == 0 {
		return nil
	}
	lines := make([]int, 0, len(spans))
	for _, span := range spans {
		lines = append(lines, span.start)
	}
	return lines
}

// ApplySessionState restores a saved state onto the freshly loaded buffer. The
// file may have changed since the state was written, so the cursor and viewport
// are clamped to what is on disk now, folds are re-derived from the current
// text, and the cursor is moved out of any region that ends up hidden.
func (m *Model) ApplySessionState(st SessionState) {
	if m.buf == nil || m.cannotDisplayFile() {
		return
	}
	line := st.CursorLine
	if line < 0 {
		line = 0
	}
	lineCount := m.buf.LineCount()
	if lineCount == 0 {
		line = 0
	} else if line >= lineCount {
		line = lineCount - 1
	}
	col := 0
	if lineCount > 0 {
		// The cursor column is a byte offset; clampCol also backs off to a rune
		// boundary when the file changed under the saved position.
		col = clampByteCol(m.lineContent(line), st.CursorCol)
	}
	m.cursor = cursorPos{line: line, col: col}
	m.preferredCol = col
	m.selectionAnchor = nil
	m.viewportTop = st.ViewportTop
	m.viewportLeft = st.ViewportLeft
	m.applyFoldHeaders(st.Folds)
	m.clampViewport()
}

// applyFoldHeaders collapses the regions that start at the given lines,
// ignoring headers that no longer open a region. ensureFolds must have run, so
// foldEnd reports the current extent of each region.
func (m *Model) applyFoldHeaders(headers []int) {
	m.ensureFolds()
	if len(headers) == 0 {
		return
	}
	// Rebuild from scratch: the buffer was just loaded, so any spans here came
	// from a previous document in this editor.
	collapsed := make([]foldSpan, 0, len(headers))
	prev := -1
	for _, line := range headers {
		if line == prev {
			continue
		}
		prev = line
		end, ok := m.foldEnd(line)
		if !ok || end <= line {
			continue
		}
		collapsed = append(collapsed, foldSpan{start: line, end: end})
	}
	sort.Slice(collapsed, func(i, j int) bool { return collapsed[i].start < collapsed[j].start })
	m.collapsed = collapsed
	m.foldRev++
}
