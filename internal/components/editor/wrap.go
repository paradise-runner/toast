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

// ensureWrapCache rebuilds visualRowCache if the buffer or wrap width has
// changed since the last build. visualRowCache[i] holds the total number of
// visual rows occupied by buffer lines 0..i-1 (a prefix-sum array), so
// visualRowCache[0] == 0 and visualRowCache[lineCount] == total visual rows.
//
// After this call all callers can do O(1) lookups instead of O(n) scans.
func (m *Model) ensureWrapCache() {
	if !m.wrapMode {
		m.visualRowCache = nil
		return
	}
	lineCount := m.buf.LineCount()
	gen := m.buf.Generation()
	w := m.wrapWidth()
	if m.visualRowCache != nil &&
		len(m.visualRowCache) == lineCount+1 &&
		m.wrapCacheGen == gen &&
		m.wrapCacheWidth == w {
		return // still valid
	}
	if cap(m.visualRowCache) >= lineCount+1 {
		m.visualRowCache = m.visualRowCache[:lineCount+1]
	} else {
		m.visualRowCache = make([]int, lineCount+1)
	}
	m.visualRowCache[0] = 0
	for l := 0; l < lineCount; l++ {
		m.visualRowCache[l+1] = m.visualRowCache[l] + len(m.lineChunks(l))
	}
	m.wrapCacheGen = gen
	m.wrapCacheWidth = w
}

// visualRowsForLine returns the number of screen rows that buffer line bufLine
// occupies in wrap mode. Always returns at least 1. In non-wrap mode, always 1.
func (m *Model) visualRowsForLine(bufLine int) int {
	if !m.wrapMode {
		return 1
	}
	m.ensureWrapCache()
	if bufLine+1 < len(m.visualRowCache) {
		return m.visualRowCache[bufLine+1] - m.visualRowCache[bufLine]
	}
	return len(m.lineChunks(bufLine))
}

// visualRowFromTop returns the 0-based absolute visual row index of the first
// visual row of bufLine, counting from the top of the buffer.
func (m *Model) visualRowFromTop(bufLine int) int {
	if !m.wrapMode {
		return bufLine
	}
	m.ensureWrapCache()
	if bufLine < len(m.visualRowCache) {
		return m.visualRowCache[bufLine]
	}
	if len(m.visualRowCache) > 0 {
		return m.visualRowCache[len(m.visualRowCache)-1]
	}
	return 0
}

// visualRowOfCursor returns the 0-based absolute visual row index for the
// current cursor position.
func (m *Model) visualRowOfCursor() int {
	row := m.visualRowFromTop(m.cursor.line)
	if m.wrapMode {
		chunks := m.lineChunks(m.cursor.line)
		row += chunkContaining(chunks, m.cursor.col)
	}
	return row
}

// wordWrapChunks returns the byte offsets of the start of each visual chunk
// when line is broken at word boundaries with the given column width.
// Breaking occurs at the last ASCII space before the width limit; if no space
// exists in the chunk, the break falls back to the column boundary.
func wordWrapChunks(line string, width int) []int {
	chunks := []int{0}
	start := 0
	for start+width < len(line) {
		end := start + width
		// Scan backward for last ASCII space within [start, end).
		sp := -1
		for i := end - 1; i >= start; i-- {
			if line[i] == ' ' {
				sp = i
				break
			}
		}
		if sp >= 0 {
			chunks = append(chunks, sp+1) // next chunk starts after the space
			start = sp + 1
		} else {
			chunks = append(chunks, end) // character-boundary fallback
			start = end
		}
	}
	return chunks
}

// chunkContaining returns the index of the chunk in chunks that contains
// byteOffset. chunks must be non-empty and sorted ascending.
func chunkContaining(chunks []int, byteOffset int) int {
	for i := 0; i+1 < len(chunks); i++ {
		if byteOffset < chunks[i+1] {
			return i
		}
	}
	return len(chunks) - 1
}

// lineChunks returns the word-wrap chunk start offsets for buffer line bufLine.
// In non-wrap mode it always returns [0] (single chunk).
func (m *Model) lineChunks(bufLine int) []int {
	if !m.wrapMode {
		return []int{0}
	}
	raw := m.buf.LineAt(bufLine)
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	return wordWrapChunks(raw, m.wrapWidth())
}

// bufPosFromVisualRow maps an absolute visual row index to a (bufLine, bufCol)
// pair. bufCol is the byte offset of the start of that visual chunk within the
// buffer line. If targetVR is past the last visual row, the last buffer position
// is returned.
//
// Uses a binary search on the visual row cache for O(log n) performance.
func (m *Model) bufPosFromVisualRow(targetVR int) (bufLine, bufCol int) {
	lineCount := m.buf.LineCount()
	if lineCount == 0 {
		return 0, 0
	}
	m.ensureWrapCache()

	// Binary search: find the largest l such that visualRowCache[l] <= targetVR.
	lo, hi := 0, lineCount
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if mid < len(m.visualRowCache) && m.visualRowCache[mid] <= targetVR {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	bufLine = lo
	if bufLine >= lineCount {
		bufLine = lineCount - 1
		bufCol = m.lineContentLen(bufLine)
		return
	}
	chunkIndex := targetVR - m.visualRowCache[bufLine]
	chunks := m.lineChunks(bufLine)
	if chunkIndex < len(chunks) {
		bufCol = chunks[chunkIndex]
	} else {
		bufCol = m.lineContentLen(bufLine)
	}
	return
}
