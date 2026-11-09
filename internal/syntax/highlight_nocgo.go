//go:build !cgo

package syntax

import (
	"strings"

	"github.com/yourusername/toast/internal/theme"
)

// Span represents a highlighted byte range within a line.
type Span struct {
	Start int
	End   int
	Style string
}

// Highlighter provides JSON/JSONC highlighting when tree-sitter is not
// available. Other recognized languages safely fall back to plain text.
type Highlighter struct {
	lang              *LangDef
	jsonTokens        []jsonToken
	jsonAllowComments bool
	content           []byte
	theme             *theme.Manager
}

// TreeSitterAvailable reports whether this build includes the cgo-backed
// tree-sitter highlighter.
func TreeSitterAvailable() bool { return false }

func NewHighlighter(path string, tm *theme.Manager) (*Highlighter, error) {
	h := &Highlighter{lang: ForPath(path), theme: tm}
	if h.lang != nil && h.lang.Name == "json" {
		h.jsonAllowComments = strings.HasSuffix(strings.ToLower(path), ".jsonc")
	}
	return h, nil
}

func (h *Highlighter) HasQuery() bool {
	return h.lang != nil && h.lang.Name == "json"
}

func (h *Highlighter) Parse(src []byte) {
	if h.lang == nil || h.lang.Name != "json" {
		return
	}
	h.jsonTokens = scanJSON(src, h.jsonAllowComments)
	h.content = src
}

func (h *Highlighter) Edit(
	src []byte,
	startByte, oldEndByte, newEndByte uint32,
	startRow, startCol, oldEndRow, oldEndCol, newEndRow, newEndCol uint32,
) {
	h.Parse(src)
}

func (h *Highlighter) HighlightLine(lineStart int, lineContent string) []Span {
	if h.lang == nil || h.lang.Name != "json" || h.jsonTokens == nil {
		return nil
	}

	lineStartByte := 0
	if lineStart > 0 {
		nlCount := 0
		for i, b := range h.content {
			if b == '\n' {
				nlCount++
				if nlCount == lineStart {
					lineStartByte = i + 1
					break
				}
			}
		}
	}
	lineEndByte := lineStartByte + len(lineContent)

	var spans []Span
	for _, tok := range h.jsonTokens {
		if tok.endByte <= lineStartByte || tok.startByte >= lineEndByte {
			continue
		}
		start := max(tok.startByte-lineStartByte, 0)
		end := min(tok.endByte-lineStartByte, len(lineContent))
		if start < end {
			spans = append(spans, Span{Start: start, End: end, Style: tok.style})
		}
	}
	return spans
}
