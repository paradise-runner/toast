package editor

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/buffer"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

// newFoldWrapModel builds a Markdown wrap-mode model for fold row-mapping
// tests. viewWidth=24, gutterWidth=8 (1 digit + 7 chrome) → wrapWidth=16.
func newFoldWrapModel(content string) Model {
	m := New(nil, config.Config{})
	m.buf = buffer.NewEditBuffer(content)
	m.path = "notes.md"
	m.wrapMode = true
	m.viewWidth = 24
	m.viewHeight = 10
	m.recomputeGutterWidth()
	return m
}

const nestedCode = `func main() {
	if x {
		do()
	}
	after()
}
`

// foldedCode adds a sibling statement after the foldable function so tests can
// check what remains visible when the function is collapsed.
const foldedCode = nestedCode + "tail()\n"

func foldEnds(regions []foldRegion) [][2]int {
	out := make([][2]int, len(regions))
	for i, r := range regions {
		out[i] = [2]int{r.start, r.end}
	}
	return out
}

func TestComputeIndentRegions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [][2]int
	}{
		{
			name: "nested blocks",
			text: nestedCode,
			want: [][2]int{{0, 5}, {1, 3}},
		},
		{
			name: "flat file has no folds",
			text: "one\ntwo\nthree\n",
			want: nil,
		},
		{
			name: "single body line folds through the closing brace",
			text: "if x {\n\tdone()\n}\n",
			want: [][2]int{{0, 2}},
		},
		{
			name: "trailing blank lines are not hidden",
			text: "a\n\tbody\n\n\nb\n",
			want: [][2]int{{0, 1}},
		},
		{
			name: "blank lines still separate blocks",
			text: "func a() {\n\tx()\n}\n\nfunc b() {\n\ty()\n}\n",
			want: [][2]int{{0, 2}, {4, 6}},
		},
		{
			name: "tabs widen indent consistent with display width",
			text: "root\n\tchild\n\t\tleaf\n\t\tleaf2\n",
			want: [][2]int{{0, 3}, {1, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := foldEnds(computeFoldRegions(splitLines(tt.text), 4, false))
			if len(got) != len(tt.want) {
				t.Fatalf("regions = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("regions = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestComputeHeadingRegions(t *testing.T) {
	doc := `# Title

Intro text.

## Section A

body a

## Section B

body b
`
	regions := computeFoldRegions(splitLines(doc), 4, true)
	want := [][2]int{{0, 10}, {4, 6}, {8, 10}}
	if len(regions) != len(want) {
		t.Fatalf("regions = %v, want %v", foldEnds(regions), want)
	}
	for i, r := range regions {
		if [2]int{r.start, r.end} != want[i] {
			t.Fatalf("regions = %v, want %v", foldEnds(regions), want)
		}
	}
	if regions[0].depth != 0 || regions[1].depth != 1 || regions[2].depth != 1 {
		t.Fatalf("depths = %d,%d,%d, want 0,1,1", regions[0].depth, regions[1].depth, regions[2].depth)
	}

	if got := foldEnds(computeFoldRegions(splitLines("#hashtag\n# Heading\n"), 4, true)); len(got) != 0 {
		t.Fatalf("hashtag must not be a heading, got %v", got)
	}

	fenced := "# A\n```go\n# not a heading\n```\ntext\n"
	got := foldEnds(computeFoldRegions(splitLines(fenced), 4, true))
	if len(got) != 1 || got[0] != [2]int{0, 4} {
		t.Fatalf("fenced regions = %v, want [[0 4]]", got)
	}
}

func TestFoldToggleAndHidden(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.ensureFolds()

	if !m.collapse(0) {
		t.Fatal("expected line 0 to be foldable")
	}
	if !m.isCollapsed(0) {
		t.Fatal("expected line 0 collapsed")
	}
	for line := 1; line <= 5; line++ {
		if !m.isHidden(line) {
			t.Fatalf("line %d should be hidden", line)
		}
	}
	if m.isHidden(0) || m.isHidden(6) {
		t.Fatal("header and following line must stay visible")
	}

	// Collapsing the nested region must replace the outer span, keeping the
	// collapsed set non-overlapping.
	if !m.toggleFoldAt(1) {
		t.Fatal("expected line 1 to be foldable")
	}
	if len(m.collapsed) != 1 || m.collapsed[0].start != 1 {
		t.Fatalf("collapsed = %#v, want only the nested span", m.collapsed)
	}

	m.UnfoldAll()
	if len(m.collapsed) != 0 {
		t.Fatalf("collapsed = %#v after UnfoldAll, want none", m.collapsed)
	}
}

func TestFoldAllOnlyTopLevel(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.FoldAll()
	if len(m.collapsed) != 1 || m.collapsed[0] != (foldSpan{start: 0, end: 5}) {
		t.Fatalf("collapsed = %#v, want the top-level block only", m.collapsed)
	}

	md := newThemedTestModel(t, "# Title\n\nintro\n\n## Section\n\nbody\n")
	md.path = "doc.md"
	md.FoldAll()
	if len(md.collapsed) != 1 || md.collapsed[0] != (foldSpan{start: 0, end: 6}) {
		t.Fatalf("markdown collapsed = %#v, want only the level-1 heading", md.collapsed)
	}
}

func TestFoldRowMappingNonWrap(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.collapse(0)

	if got := m.totalRows(); got != 2 {
		t.Fatalf("totalRows = %d, want 2 (header + tail)", got)
	}
	if got := m.visualRowFromTop(1); got != 1 {
		t.Fatalf("visualRowFromTop(1) = %d, want 1 (hidden line shares the row after the header)", got)
	}
	if got := m.lineAtRow(1); got != 6 {
		t.Fatalf("lineAtRow(1) = %d, want 6", got)
	}
	if prev, ok := m.prevVisibleLine(6); !ok || prev != 0 {
		t.Fatalf("prevVisibleLine(6) = %d,%v, want 0,true", prev, ok)
	}
	if next, ok := m.nextVisibleLine(0); !ok || next != 6 {
		t.Fatalf("nextVisibleLine(0) = %d,%v, want 6,true", next, ok)
	}
	if line, col := m.screenToBuffer(0, 1); line != 6 || col != 0 {
		t.Fatalf("screenToBuffer(y=1) = (%d,%d), want (6,0)", line, col)
	}
}

func TestFoldRowMappingWrap(t *testing.T) {
	m := newFoldWrapModel("# T\naaaa bbbb cccc dddd\n## S\nbody\n")
	m.collapse(2) // hide "body"

	if m.isHidden(3) == false {
		t.Fatal("expected body line hidden")
	}
	// Line 1 wraps into two rows at wrapWidth 16.
	if got := m.visualRowsForLine(1); got != 2 {
		t.Fatalf("visualRowsForLine(1) = %d, want 2", got)
	}
	if got := m.visualRowFromTop(2); got != 3 {
		t.Fatalf("visualRowFromTop(2) = %d, want 3", got)
	}
	if got := m.totalRows(); got != 4 {
		t.Fatalf("totalRows = %d, want 4", got)
	}
	if got := m.visualRowsForLine(3); got != 0 {
		t.Fatalf("hidden line rows = %d, want 0", got)
	}
	if line, _ := m.screenToBuffer(m.gutterWidth, 3); line != 2 {
		t.Fatalf("row 3 maps to line %d, want 2 (hidden body skipped)", line)
	}
}

func TestFoldCursorMovementSkipsHiddenLines(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.collapse(0)
	m.cursor = cursorPos{line: 6, col: 0}
	m.preferredCol = 0

	m.moveCursorUp(1)
	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0 (header above the fold)", m.cursor.line)
	}

	m.cursor = cursorPos{line: 0, col: 0}
	m.moveCursorDown(1)
	if m.cursor.line != 6 {
		t.Fatalf("cursor.line = %d, want 6 (fold skipped)", m.cursor.line)
	}

	// Left at column 0 of the line after the fold must land on the header end,
	// not on a hidden line.
	m.cursor = cursorPos{line: 6, col: 0}
	m.moveCursorLeft()
	if m.cursor.line != 0 || m.cursor.col != m.lineContentLen(0) {
		t.Fatalf("cursor = (%d,%d), want end of header line 0", m.cursor.line, m.cursor.col)
	}

	m.cursor = cursorPos{line: 0, col: m.lineContentLen(0)}
	m.moveCursorRight()
	if m.cursor.line != 6 || m.cursor.col != 0 {
		t.Fatalf("cursor = (%d,%d), want (6,0)", m.cursor.line, m.cursor.col)
	}
}

func TestFoldViewRendersCaretAndHiddenCount(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.viewHeight = 4

	expanded := stripANSI(m.View().Content)
	if !strings.Contains(expanded, "▾") {
		t.Fatalf("expanded fold caret missing from view: %q", expanded)
	}
	if strings.Contains(expanded, "▸") {
		t.Fatalf("unexpected collapsed caret in view: %q", expanded)
	}

	m.collapse(0)
	collapsed := stripANSI(m.View().Content)
	if !strings.Contains(collapsed, "▸  5") {
		t.Fatalf("collapsed caret with hidden count missing from view: %q", collapsed)
	}
	if strings.Contains(collapsed, "do()") || strings.Contains(collapsed, "after()") {
		t.Fatalf("hidden lines rendered: %q", collapsed)
	}
}

func TestFoldCaretClickToggles(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.viewHeight = 5
	m.focused = true

	x := m.foldCaretX() + 1
	updated, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: 0})
	m = updated.(Model)
	if !m.isCollapsed(0) {
		t.Fatalf("click at caret x=%d did not collapse line 0", x)
	}

	// A second click expands it again.
	updated, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: 0})
	m = updated.(Model)
	if m.isCollapsed(0) {
		t.Fatal("second click did not expand line 0")
	}

	// Clicking the same cell on a non-foldable row just moves the caret.
	updated, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: 2})
	m = updated.(Model)
	if m.cursor.line != 2 {
		t.Fatalf("cursor.line = %d, want 2", m.cursor.line)
	}
}

func TestFoldGoToLineRevealsTarget(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.collapse(0)

	updated, _ := m.Update(messages.GoToLineMsg{Line: 2})
	m = updated.(Model)
	if m.isCollapsed(0) {
		t.Fatal("go-to-line inside a fold must expand it")
	}
	if m.cursor.line != 2 {
		t.Fatalf("cursor.line = %d, want 2", m.cursor.line)
	}
}

func TestFoldPruneAfterEditKeepsAnchor(t *testing.T) {
	m := newThemedTestModel(t, "if x {\n\ta()\n}\n")
	m.ensureFolds()
	m.collapse(0)
	if end, ok := m.foldEnd(0); !ok || end != 2 {
		t.Fatalf("foldEnd(0) = %d,%v, want 2,true", end, ok)
	}

	// Insert another indented line inside the block: the folded span grows.
	m.buf.Insert(m.buf.OffsetForLine(2), "    b()\n")
	m.ensureFolds()
	if !m.isCollapsed(0) {
		t.Fatal("fold must stay collapsed while its body grows")
	}
	if end, ok := m.foldEnd(0); !ok || end != 3 {
		t.Fatalf("foldEnd(0) = %d,%v, want 3,true", end, ok)
	}

	// Dedenting the inserted line closes the region earlier again.
	m.cursor = cursorPos{line: 2, col: 4}
	m.dedent()
	m.ensureFolds()
	if end, ok := m.foldEnd(0); !ok || end != 1 {
		t.Fatalf("foldEnd(0) after dedent = %d,%v, want 1,true", end, ok)
	}
}

func TestFoldSnapshotRoundTrip(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.collapse(0)

	snap := m.Snapshot()

	other := newThemedTestModel(t, "unrelated\n")
	other.RestoreSnapshot(snap, m.bufferID, m.path)
	if !other.isCollapsed(0) {
		t.Fatalf("restored collapsed = %#v, want span at line 0", other.collapsed)
	}
	if got := other.totalRows(); got != 2 {
		t.Fatalf("totalRows after restore = %d, want 2", got)
	}
}

func TestFoldClampViewportNeverAnchorsOnHiddenLine(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.viewHeight = 2
	m.viewportTop = 2 // inside the block that is about to be folded

	m.toggleFoldAt(0)
	if m.isHidden(m.viewportTop) {
		t.Fatalf("viewportTop = %d is hidden", m.viewportTop)
	}
	if m.viewportTop != 6 {
		t.Fatalf("viewportTop = %d, want 6 (first visible line after the fold)", m.viewportTop)
	}
}

func TestFoldAllSnapsCursorToHeader(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.cursor = cursorPos{line: 2, col: 1}
	m.preferredCol = 1

	m.FoldAll()
	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0 (fold header)", m.cursor.line)
	}
	if !m.isCollapsed(0) {
		t.Fatal("FoldAll must keep the fold when it snaps the cursor to the header")
	}
}

func TestFoldCaretClickSnapsCursorOutOfFoldedBlock(t *testing.T) {
	m := newThemedTestModel(t, foldedCode)
	m.focused = true
	m.cursor = cursorPos{line: 2, col: 1}
	m.preferredCol = 1

	updated, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.foldCaretX(), Y: 0})
	m = updated.(Model)
	if !m.isCollapsed(0) {
		t.Fatal("click did not collapse the function")
	}
	if m.cursor.line != 0 {
		t.Fatalf("cursor.line = %d, want 0 (fold header)", m.cursor.line)
	}
}
