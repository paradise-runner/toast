package editor

import (
	"strings"
)

// nonWrapChunks is the immutable single chunk returned by lineChunks in
// non-wrap mode. Callers only read it, so a shared slice avoids an allocation
// on every cursor movement.
var nonWrapChunks = []int{0}

// splitLines splits full into its component lines (without trailing newlines).
// A trailing newline does not produce an empty final element, matching
// EditBuffer.LineCount semantics.
func splitLines(full string) []string {
	if len(full) == 0 {
		return nil
	}
	lines := strings.Split(full, "\n")
	if full[len(full)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// wrapWidth returns the number of content columns available for wrapped lines.
// Returns at least 1 to avoid division by zero.
func (m *Model) wrapWidth() int {
	w := m.viewWidth - m.gutterWidth
	if w < 1 {
		return 1
	}
	return w
}

// ensureWrapCache rebuilds visualRowCache (and chunkCache in wrap mode) if the
// buffer, wrap width or fold state has changed since the last build.
//
// visualRowCache[i] holds the total number of visual rows occupied by buffer
// lines 0..i-1 (a prefix-sum array), so visualRowCache[0] == 0 and
// visualRowCache[lineCount] == total visual rows. Hidden (folded) lines
// contribute zero rows, which is what makes every row-based lookup fold-aware
// in both wrap and non-wrap mode. chunkCache[l] holds the cached word-wrap
// chunk start offsets for line l (wrap mode only).
//
// Both caches are rebuilt in a single O(total) pass over the buffer text via
// buf.String(), rather than calling LineAt per line — which would be
// O(lineCount × total) because each LineAt rebuilds the whole rope string.
// After this call, lineChunks and visual-row lookups are O(1).
//
// Cache validity is keyed on buf.Generation() (bumped on every edit) plus the
// wrap width and fold revision; cursor-only moves never trigger a rebuild.
func (m *Model) ensureWrapCache() {
	if m.buf == nil {
		m.visualRowCache = nil
		m.chunkCache = nil
		return
	}
	m.ensureFolds()
	gen := m.buf.Generation()
	w := m.wrapWidth()
	if m.visualRowCache != nil && m.wrapCacheGen == gen && m.wrapCacheWidth == w &&
		m.wrapCacheFolds == m.foldRev {
		return // still valid
	}

	rawLines := splitLines(m.buf.String())
	lineCount := len(rawLines)

	if cap(m.visualRowCache) >= lineCount+1 {
		m.visualRowCache = m.visualRowCache[:lineCount+1]
	} else {
		m.visualRowCache = make([]int, lineCount+1)
	}
	if m.wrapMode {
		if cap(m.chunkCache) >= lineCount {
			m.chunkCache = m.chunkCache[:lineCount]
		} else {
			m.chunkCache = make([][]int, lineCount)
		}
	} else {
		m.chunkCache = nil
	}

	m.visualRowCache[0] = 0
	tw := m.cfg.Editor.TabWidth
	for l, raw := range rawLines {
		rows := 0
		if !m.isHidden(l) {
			if m.wrapMode {
				chunks := wordWrapChunksWithTabWidth(raw, w, tw)
				m.chunkCache[l] = chunks
				rows = len(chunks)
			} else {
				rows = 1
			}
		}
		m.visualRowCache[l+1] = m.visualRowCache[l] + rows
	}
	m.wrapCacheGen = gen
	m.wrapCacheWidth = w
	m.wrapCacheFolds = m.foldRev
}

// totalRows returns the number of visible screen rows occupied by the whole
// buffer (folded lines contribute zero).
func (m *Model) totalRows() int {
	if m.buf == nil {
		return 0
	}
	return m.visualRowFromTop(m.buf.LineCount())
}

// lineAtRow maps an absolute visible-row index to its buffer line. Rows past
// the end of the buffer map to the last line.
func (m *Model) lineAtRow(row int) int {
	if m.buf == nil || m.buf.LineCount() == 0 {
		return 0
	}
	line, _ := m.bufPosFromVisualRow(row)
	return line
}

// lineAtRowOrAfter maps a visible-row index to the first line whose first row
// is at or after row. Used when scrolling down so a wrapped line is not cut off
// at the bottom edge.
func (m *Model) lineAtRowOrAfter(row int) int {
	if row <= 0 {
		return 0
	}
	line := m.lineAtRow(row)
	if m.visualRowFromTop(line) >= row {
		return line
	}
	if next, ok := m.nextVisibleLine(line); ok {
		return next
	}
	return line
}

// visualRowsForLine returns the number of screen rows that buffer line bufLine
// occupies. Hidden lines occupy zero rows; every visible line occupies at least
// one.
func (m *Model) visualRowsForLine(bufLine int) int {
	m.ensureWrapCache()
	if bufLine < 0 || bufLine+1 >= len(m.visualRowCache) {
		return 0
	}
	return m.visualRowCache[bufLine+1] - m.visualRowCache[bufLine]
}

// visualRowFromTop returns the 0-based absolute visual row index of the first
// visual row of bufLine, counting from the top of the buffer.
func (m *Model) visualRowFromTop(bufLine int) int {
	m.ensureWrapCache()
	if bufLine < 0 {
		return 0
	}
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
	return wordWrapChunksWithTabWidth(line, width, defaultTabWidth)
}

// wordWrapChunksWithTabWidth returns the byte offsets of the start of each
// visual chunk when line is broken at word boundaries with the given column
// width. Breaking occurs at the last ASCII space before the width limit; if no
// space exists in the chunk, the break falls back to the column boundary.
//
// The implementation is O(len(line)): it precomputes each grapheme-cluster
// boundary's absolute display column in a single forward pass, then walks
// boundaries with O(1) width lookups. It never re-decodes the line from byte 0
// and avoids the per-rune string allocation (`len(string(r))`) of an earlier
// version, which made it O(n²) in the line length with one allocation per rune.
//
// Display columns are absolute (tab stops computed from byte 0 of the line),
// matching displayColumnAtByte and the rendering path so that wrap chunks and
// rendered tab expansion stay consistent.
func wordWrapChunksWithTabWidth(line string, width, tabWidth int) []int {
	// offs[k] = byte offset of the k-th cluster; offs[total] == len(line).
	// cols[k] = absolute display column (tab-expanded) at byte offs[k].
	offs := make([]int, 1, len(line)+1)
	cols := make([]int, 1, len(line)+1)
	absCol := 0
	for i := 0; i < len(line); {
		cluster, _ := firstGraphemeClusterWidth(line[i:])
		absCol = nextDisplayColumn(absCol, cluster, tabWidth)
		i += len(cluster)
		offs = append(offs, i)
		cols = append(cols, absCol)
	}
	total := len(offs) - 1 // number of clusters

	chunks := []int{0}
	startIdx := 0
	for startIdx < total && cols[total]-cols[startIdx] > width {
		endIdx := startIdx
		lastSpaceIdx := -1
		for k := startIdx; k < total; k++ {
			if cols[k+1]-cols[startIdx] > width {
				break
			}
			endIdx = k + 1
			// rune k is an ASCII space iff its first byte is 0x20.
			if line[offs[k]] == ' ' {
				lastSpaceIdx = k + 1
			}
		}
		var breakIdx int
		switch {
		case lastSpaceIdx > startIdx:
			breakIdx = lastSpaceIdx
		case endIdx > startIdx:
			breakIdx = endIdx
		default:
			// A single rune is wider than width: force-break after it so the
			// chunk always advances at least one rune.
			breakIdx = startIdx + 1
		}
		chunks = append(chunks, offs[breakIdx])
		startIdx = breakIdx
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
// In non-wrap mode it returns nonWrapChunks (a shared, immutable single chunk).
// In wrap mode the result is served from chunkCache after ensureWrapCache runs,
// so repeated calls are O(1) and never recompute the wrap.
//
// Callers must treat the returned slice as read-only.
func (m *Model) lineChunks(bufLine int) []int {
	if !m.wrapMode {
		return nonWrapChunks
	}
	m.ensureWrapCache()
	if bufLine >= 0 && bufLine < len(m.chunkCache) {
		return m.chunkCache[bufLine]
	}
	// Fallback for an out-of-range line (e.g. an empty buffer): compute the
	// chunks directly without touching the cache.
	raw := m.buf.LineAt(bufLine)
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	return wordWrapChunksWithTabWidth(raw, m.wrapWidth(), m.cfg.Editor.TabWidth)
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
	if targetVR < 0 {
		targetVR = 0
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
