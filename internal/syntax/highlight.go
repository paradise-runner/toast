package syntax

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/yourusername/toast/internal/theme"
)

// Span represents a highlighted byte range within a line. Start and End are
// byte offsets relative to the start of the line. Style is the theme key used
// to look up the colour (e.g. "keyword", "string", "comment").
type Span struct {
	Start int
	End   int
	Style string
}

// Highlighter holds a tree-sitter parser, the parsed tree and the compiled
// highlight query for a single file.
type Highlighter struct {
	lang    *LangDef
	parser  *sitter.Parser
	query   *sitter.Query
	tree    *sitter.Tree
	content []byte
	theme   *theme.Manager
}

// NewHighlighter creates a Highlighter for the given file path. If the
// extension is not recognised, a no-op highlighter (lang == nil) is returned
// so callers never have to handle a nil value.
func NewHighlighter(path string, tm *theme.Manager) (*Highlighter, error) {
	h := &Highlighter{theme: tm}

	lang := ForPath(path)
	if lang == nil {
		// Unknown language – highlighting will be a no-op.
		return h, nil
	}

	p := sitter.NewParser()
	p.SetLanguage(lang.Language)

	var q *sitter.Query
	if len(lang.Query) > 0 {
		var err error
		q, err = sitter.NewQuery(lang.Query, lang.Language)
		if err != nil {
			// Bad query – fall back to no highlighting rather than hard-failing.
			q = nil
		}
	}

	h.lang = lang
	h.parser = p
	h.query = q
	return h, nil
}

// HasQuery returns true if a highlight query was successfully compiled.
func (h *Highlighter) HasQuery() bool {
	return h.query != nil
}

// Parse does a full parse of src and stores the resulting tree.
func (h *Highlighter) Parse(src []byte) {
	if h.parser == nil {
		return
	}
	h.content = src
	tree, err := h.parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return
	}
	h.tree = tree
}

// Edit applies an incremental edit to the stored tree and re-parses. All
// index/row/col values use the same conventions as tree-sitter's EditInput.
func (h *Highlighter) Edit(
	src []byte,
	startByte, oldEndByte, newEndByte uint32,
	startRow, startCol, oldEndRow, oldEndCol, newEndRow, newEndCol uint32,
) {
	if h.parser == nil || h.tree == nil {
		h.Parse(src)
		return
	}

	h.tree.Edit(sitter.EditInput{
		StartIndex:  startByte,
		OldEndIndex: oldEndByte,
		NewEndIndex: newEndByte,
		StartPoint:  sitter.Point{Row: startRow, Column: startCol},
		OldEndPoint: sitter.Point{Row: oldEndRow, Column: oldEndCol},
		NewEndPoint: sitter.Point{Row: newEndRow, Column: newEndCol},
	})

	h.content = src
	tree, err := h.parser.ParseCtx(context.Background(), h.tree, src)
	if err != nil {
		return
	}
	h.tree = tree
}

// HighlightLine returns the highlight spans for a single line. lineStart is
// the 0-based line number. lineContent is the raw text of that line (including
// any trailing newline). The returned Span offsets are relative to the
// beginning of lineContent.
func (h *Highlighter) HighlightLine(lineStart int, lineContent string) []Span {
	if h.query == nil || h.tree == nil {
		return nil
	}

	// Compute the byte offset of this line within h.content.
	lineNum := lineStart // preserve original for the query cursor
	lineStartByte := uint32(0)
	if lineNum > 0 {
		nlCount := 0
		for i, b := range h.content {
			if b == '\n' {
				nlCount++
				if nlCount == lineNum {
					lineStartByte = uint32(i + 1)
					break
				}
			}
		}
	}
	lineEndByte := lineStartByte + uint32(len(lineContent))

	qc := sitter.NewQueryCursor()
	defer qc.Close()

	// Restrict the query to the byte range of this line via point range.
	qc.SetPointRange(
		sitter.Point{Row: uint32(lineNum), Column: 0},
		sitter.Point{Row: uint32(lineNum), Column: ^uint32(0)},
	)
	qc.Exec(h.query, h.tree.RootNode())

	var spans []Span
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			nodeStart := node.StartByte()
			nodeEnd := node.EndByte()

			// Clamp to line boundaries.
			if nodeEnd <= lineStartByte || nodeStart >= lineEndByte {
				continue
			}
			if nodeStart < lineStartByte {
				nodeStart = lineStartByte
			}
			if nodeEnd > lineEndByte {
				nodeEnd = lineEndByte
			}

			name := h.query.CaptureNameForId(cap.Index)
			// Strip dotted suffix: "function.call" -> "function"
			if dot := strings.IndexByte(name, '.'); dot != -1 {
				name = name[:dot]
			}

			spans = append(spans, Span{
				Start: int(nodeStart - lineStartByte),
				End:   int(nodeEnd - lineStartByte),
				Style: name,
			})
		}
	}

	return spans
}
