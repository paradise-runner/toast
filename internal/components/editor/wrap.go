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
