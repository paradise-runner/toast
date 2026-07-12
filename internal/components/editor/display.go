package editor

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

const defaultTabWidth = 4

func normalizedTabWidth(width int) int {
	if width < 1 {
		return defaultTabWidth
	}
	return width
}

func nextDisplayColumn(column int, r rune, tabWidth int) int {
	if r == '\t' {
		tabWidth = normalizedTabWidth(tabWidth)
		return column + tabWidth - column%tabWidth
	}
	return column + lipgloss.Width(string(r))
}

func displayColumnAtByte(text string, byteCol, tabWidth int) int {
	byteCol = clampByteCol(text, byteCol)
	column := 0
	for offset := 0; offset < byteCol; {
		r, size := utf8.DecodeRuneInString(text[offset:])
		if offset+size > byteCol {
			break
		}
		column = nextDisplayColumn(column, r, tabWidth)
		offset += size
	}
	return column
}

func displayWidthForByteRange(text string, start, end, tabWidth int) int {
	start = clampByteCol(text, start)
	end = clampByteCol(text, end)
	if end < start {
		end = start
	}
	return displayColumnAtByte(text, end, tabWidth) - displayColumnAtByte(text, start, tabWidth)
}

// byteColForDisplayOffset maps an offset from start's display column back to
// a byte boundary. Cells occupied by a wide rune or expanded tab map to the
// position before that character; its right boundary maps after it.
func byteColForDisplayOffset(text string, start, displayOffset, tabWidth int) int {
	start = clampByteCol(text, start)
	if displayOffset <= 0 {
		return start
	}
	target := displayColumnAtByte(text, start, tabWidth) + displayOffset
	column := displayColumnAtByte(text, start, tabWidth)
	previous := start
	for offset := start; offset < len(text); {
		r, size := utf8.DecodeRuneInString(text[offset:])
		next := offset + size
		nextColumn := nextDisplayColumn(column, r, tabWidth)
		if nextColumn > target {
			return previous
		}
		if nextColumn == target {
			return next
		}
		previous = next
		offset = next
		column = nextColumn
	}
	return len(text)
}

// byteColAtOrAfterDisplayColumn finds a safe horizontal viewport boundary.
// When the requested column falls inside a tab or wide rune, it advances past
// that character so the cursor is guaranteed to become visible.
func byteColAtOrAfterDisplayColumn(text string, target, tabWidth int) int {
	if target <= 0 {
		return 0
	}
	column := 0
	for offset := 0; offset < len(text); {
		r, size := utf8.DecodeRuneInString(text[offset:])
		next := offset + size
		nextColumn := nextDisplayColumn(column, r, tabWidth)
		if column >= target {
			return offset
		}
		if nextColumn >= target {
			return next
		}
		offset = next
		column = nextColumn
	}
	return len(text)
}

func expandTabs(text string, startColumn, tabWidth int) string {
	if !strings.ContainsRune(text, '\t') {
		return text
	}
	var out strings.Builder
	column := startColumn
	for _, r := range text {
		if r == '\t' {
			next := nextDisplayColumn(column, r, tabWidth)
			out.WriteString(strings.Repeat(" ", next-column))
			column = next
			continue
		}
		out.WriteRune(r)
		column = nextDisplayColumn(column, r, tabWidth)
	}
	return out.String()
}
