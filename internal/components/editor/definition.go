package editor

import (
	"unicode/utf16"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

type definitionLinkState struct {
	line, start, end int
	sourceCol        int
	pending          bool
	visible          bool
	targetPath       string
	targetLine       int
	targetCol        int
}

func (s *definitionLinkState) hide() { *s = definitionLinkState{} }

func (s definitionLinkState) contains(line, col int) bool {
	return s.visible && line == s.line && col >= s.start && col < s.end
}

func (m *Model) wordRangeAt(line, col int) (start, end int, ok bool) {
	if m.buf == nil || line < 0 || line >= m.buf.LineCount() {
		return 0, 0, false
	}
	text := m.lineContent(line)
	if col < 0 {
		col = 0
	}
	if col > len(text) {
		col = len(text)
	}
	if col == len(text) && col > 0 {
		_, size := utf8.DecodeLastRuneInString(text[:col])
		if r, _ := utf8.DecodeRuneInString(text[col-size : col]); isWordChar(r) {
			col -= size
		}
	}
	if col >= len(text) {
		return 0, 0, false
	}
	r, _ := utf8.DecodeRuneInString(text[col:])
	if !isWordChar(r) {
		return 0, 0, false
	}
	start = col
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if !isWordChar(r) {
			break
		}
		start -= size
	}
	end = col
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if !isWordChar(r) {
			break
		}
		end += size
	}
	return start, end, start < end
}

func (m Model) handleGoToPosition(msg messages.GoToPositionMsg) (tea.Model, tea.Cmd) {
	if m.buf == nil || m.buf.LineCount() == 0 {
		return m, nil
	}
	line := msg.Line
	if line < 0 {
		line = 0
	}
	if line >= m.buf.LineCount() {
		line = m.buf.LineCount() - 1
	}
	m.cursor = cursorPos{line: line, col: utf16ColToByte(m.lineContent(line), msg.Col)}
	m.preferredCol = m.cursor.col
	m.selectionAnchor = nil
	m.definitionLink.hide()
	m.clampViewport()
	return m, nil
}

func utf16ColToByte(line string, col int) int {
	if col <= 0 {
		return 0
	}
	units := 0
	for byteOffset, r := range line {
		width := len(utf16.Encode([]rune{r}))
		if units+width > col {
			return byteOffset
		}
		units += width
		if units == col {
			return byteOffset + utf8.RuneLen(r)
		}
	}
	return len(line)
}
