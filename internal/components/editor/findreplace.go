package editor

import (
	"strings"
	"unicode/utf8"
)

type findOptions struct {
	matchCase bool
	wholeWord bool
}

type findMatch struct {
	start int
	end   int
}

type lineFindMatch struct {
	start   int
	end     int
	current bool
}

// FindStatus returns the current 1-based match index and total match count.
func (m Model) FindStatus() (current, total int) {
	total = len(m.findMatches)
	if total == 0 || m.findCurrent < 0 || m.findCurrent >= total {
		return 0, total
	}
	return m.findCurrent + 1, total
}

func (m *Model) clearFindState() {
	m.findQuery = ""
	m.findMatchCase = false
	m.findWholeWord = false
	m.findMatches = nil
	m.findCurrent = -1
}

func (m *Model) applyFindQuery(query string, opts findOptions) {
	if m.buf == nil || m.binaryFile {
		m.clearFindState()
		return
	}

	anchorOffset := m.findAnchorOffset()
	m.findQuery = query
	m.findMatchCase = opts.matchCase
	m.findWholeWord = opts.wholeWord
	m.refreshFindMatches()
	m.findCurrent = m.findIndexContainingOrAfter(anchorOffset)
	m.selectFindCurrent()
}

func (m *Model) navigateFind(forward bool) {
	if m.buf == nil || len(m.findMatches) == 0 {
		return
	}
	if m.findCurrent < 0 || m.findCurrent >= len(m.findMatches) {
		m.findCurrent = m.findIndexContainingOrAfter(m.cursorOffset())
	} else if forward {
		m.findCurrent = (m.findCurrent + 1) % len(m.findMatches)
	} else {
		m.findCurrent--
		if m.findCurrent < 0 {
			m.findCurrent = len(m.findMatches) - 1
		}
	}
	m.selectFindCurrent()
}

func (m *Model) replaceCurrentFind(query, replacement string, opts findOptions) bool {
	if m.buf == nil || m.binaryFile {
		return false
	}

	m.applyFindQuery(query, opts)
	if len(m.findMatches) == 0 || m.findCurrent < 0 {
		return false
	}

	oldIndex := m.findCurrent
	match := m.findMatches[m.findCurrent]
	old := m.buf.Slice(match.start, match.end)
	nextOffset := match.start + len(replacement)
	changed := old != replacement
	if changed {
		m.buf.Replace(match.start, match.end, replacement)
		m.recomputeGutterWidth()
		m.reparseSyntax()
	}

	m.refreshFindMatches()
	if len(m.findMatches) == 0 {
		m.findCurrent = -1
		m.setCursorOffset(nextOffset)
		m.clearSelection()
		m.clampViewport()
		return changed
	}
	if changed {
		m.findCurrent = m.findIndexStartingAtOrAfter(nextOffset)
	} else {
		m.findCurrent = oldIndex + 1
		if m.findCurrent >= len(m.findMatches) {
			m.findCurrent = 0
		}
	}
	m.selectFindCurrent()
	return changed
}

func (m *Model) replaceAllFind(query, replacement string, opts findOptions) bool {
	if m.buf == nil || m.binaryFile {
		return false
	}

	m.findQuery = query
	m.findMatchCase = opts.matchCase
	m.findWholeWord = opts.wholeWord
	m.refreshFindMatches()
	if len(m.findMatches) == 0 {
		m.findCurrent = -1
		return false
	}

	content := m.buf.String()
	var b strings.Builder
	b.Grow(len(content) + len(m.findMatches)*len(replacement))
	pos := 0
	changed := false
	firstStart := m.findMatches[0].start
	for _, match := range m.findMatches {
		b.WriteString(content[pos:match.start])
		b.WriteString(replacement)
		if content[match.start:match.end] != replacement {
			changed = true
		}
		pos = match.end
	}
	b.WriteString(content[pos:])
	if !changed {
		m.selectFindCurrent()
		return false
	}

	m.buf.Replace(0, len(content), b.String())
	m.recomputeGutterWidth()
	m.reparseSyntax()
	m.refreshFindMatches()
	if len(m.findMatches) == 0 {
		m.findCurrent = -1
		m.setCursorOffset(firstStart)
		m.clearSelection()
		m.clampViewport()
		return true
	}
	m.findCurrent = m.findIndexStartingAtOrAfter(firstStart)
	m.selectFindCurrent()
	return true
}

func (m *Model) refreshFindMatches() {
	if m.buf == nil || m.findQuery == "" {
		m.findMatches = nil
		m.findCurrent = -1
		return
	}
	m.findMatches = findAllMatches(m.buf.String(), m.findQuery, findOptions{
		matchCase: m.findMatchCase,
		wholeWord: m.findWholeWord,
	})
	if len(m.findMatches) == 0 {
		m.findCurrent = -1
		return
	}
	if m.findCurrent >= len(m.findMatches) {
		m.findCurrent = len(m.findMatches) - 1
	}
}

func (m *Model) findAnchorOffset() int {
	if start, _, active := m.selectionRange(); active {
		return m.buf.OffsetForLine(start.line) + start.col
	}
	return m.cursorOffset()
}

func (m *Model) findIndexContainingOrAfter(offset int) int {
	if len(m.findMatches) == 0 {
		return -1
	}
	for i, match := range m.findMatches {
		if offset >= match.start && offset <= match.end {
			return i
		}
		if match.start >= offset {
			return i
		}
	}
	return 0
}

func (m *Model) findIndexStartingAtOrAfter(offset int) int {
	if len(m.findMatches) == 0 {
		return -1
	}
	for i, match := range m.findMatches {
		if match.start >= offset {
			return i
		}
	}
	return 0
}

func (m *Model) selectFindCurrent() {
	if m.buf == nil || m.findCurrent < 0 || m.findCurrent >= len(m.findMatches) {
		return
	}
	match := m.findMatches[m.findCurrent]
	startLine, startCol := m.buf.LineColForOffset(match.start)
	endLine, endCol := m.buf.LineColForOffset(match.end)
	anchor := cursorPos{line: startLine, col: startCol}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: endLine, col: endCol}
	m.clampCursor()
	m.preferredCol = m.cursor.col
	m.clampViewport()
}

func (m *Model) setCursorOffset(offset int) {
	if m.buf == nil {
		m.cursor = cursorPos{}
		m.preferredCol = 0
		return
	}
	if offset < 0 {
		offset = 0
	}
	if offset > m.buf.Len() {
		offset = m.buf.Len()
	}
	line, col := m.buf.LineColForOffset(offset)
	m.cursor = cursorPos{line: line, col: col}
	m.clampCursor()
	m.preferredCol = m.cursor.col
}

func (m Model) findRangesForLineRange(bufLine, rangeStart, rangeEnd int) []lineFindMatch {
	if m.buf == nil || bufLine < 0 || rangeEnd <= rangeStart || len(m.findMatches) == 0 {
		return nil
	}
	lineStart := m.buf.OffsetForLine(bufLine)
	absStart := lineStart + rangeStart
	absEnd := lineStart + rangeEnd
	var ranges []lineFindMatch
	for i, match := range m.findMatches {
		if match.end <= absStart {
			continue
		}
		if match.start >= absEnd {
			break
		}
		start := match.start - lineStart
		if start < rangeStart {
			start = rangeStart
		}
		end := match.end - lineStart
		if end > rangeEnd {
			end = rangeEnd
		}
		if start < end {
			ranges = append(ranges, lineFindMatch{
				start:   start,
				end:     end,
				current: i == m.findCurrent,
			})
		}
	}
	return ranges
}

func findAllMatches(content, query string, opts findOptions) []findMatch {
	if query == "" {
		return nil
	}
	if opts.matchCase {
		return findCaseSensitiveMatches(content, query, opts.wholeWord)
	}
	return findFoldMatches(content, query, opts.wholeWord)
}

func findCaseSensitiveMatches(content, query string, wholeWord bool) []findMatch {
	var matches []findMatch
	for searchStart := 0; searchStart <= len(content)-len(query); {
		idx := strings.Index(content[searchStart:], query)
		if idx < 0 {
			break
		}
		start := searchStart + idx
		end := start + len(query)
		if !wholeWord || hasWholeWordBoundaries(content, start, end) {
			matches = append(matches, findMatch{start: start, end: end})
			searchStart = end
			continue
		}
		searchStart = nextRuneOffset(content, start)
	}
	return matches
}

func findFoldMatches(content, query string, wholeWord bool) []findMatch {
	queryRunes := utf8.RuneCountInString(query)
	if queryRunes == 0 {
		return nil
	}

	var matches []findMatch
	for start := 0; start < len(content); {
		end, ok := offsetAfterRunes(content, start, queryRunes)
		if ok && strings.EqualFold(content[start:end], query) &&
			(!wholeWord || hasWholeWordBoundaries(content, start, end)) {
			matches = append(matches, findMatch{start: start, end: end})
			start = end
			continue
		}
		start = nextRuneOffset(content, start)
	}
	return matches
}

func offsetAfterRunes(s string, start, count int) (int, bool) {
	offset := start
	for i := 0; i < count; i++ {
		if offset >= len(s) {
			return offset, false
		}
		_, size := utf8.DecodeRuneInString(s[offset:])
		if size <= 0 {
			size = 1
		}
		offset += size
	}
	return offset, true
}

func nextRuneOffset(s string, start int) int {
	if start >= len(s) {
		return len(s) + 1
	}
	_, size := utf8.DecodeRuneInString(s[start:])
	if size <= 0 {
		size = 1
	}
	return start + size
}

func hasWholeWordBoundaries(content string, start, end int) bool {
	prevWord := false
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(content[:start])
		prevWord = isWordChar(r)
	}
	nextWord := false
	if end < len(content) {
		r, _ := utf8.DecodeRuneInString(content[end:])
		nextWord = isWordChar(r)
	}
	return !prevWord && !nextWord
}
