package editor

import (
	"testing"

	"github.com/paradise-runner/toast/internal/config"
)

func TestSessionStateRoundTrip(t *testing.T) {
	content := "func main() {\n\tif true {\n\t\tprintln(1)\n\t}\n\tprintln(2)\n}\n"
	m := newTestModel(content)
	m.path = "main.go"
	m.viewWidth = 40
	m.viewHeight = 20
	m.recomputeGutterWidth()
	m.cursor = cursorPos{line: 4, col: 3}
	m.preferredCol = 3
	m.viewportTop = 2
	m.viewportLeft = 1

	st := m.SessionState()
	if st.CursorLine != 4 || st.CursorCol != 3 || st.ViewportTop != 2 || st.ViewportLeft != 1 {
		t.Fatalf("SessionState = %+v, want cursor 4:3 viewport 2/1", st)
	}

	fresh := newTestModel(content)
	fresh.path = "main.go"
	fresh.viewWidth = 40
	fresh.viewHeight = 20
	fresh.recomputeGutterWidth()
	fresh.ApplySessionState(st)

	if fresh.cursor.line != 4 || fresh.cursor.col != 3 {
		t.Fatalf("cursor = %+v, want {4 3}", fresh.cursor)
	}
	if fresh.viewportTop != 2 || fresh.viewportLeft != 1 {
		t.Fatalf("viewport = %d/%d, want 2/1", fresh.viewportTop, fresh.viewportLeft)
	}
}

func TestApplySessionStateSnapsViewportOutOfRestoredFold(t *testing.T) {
	m := foldTestModel(t)

	// A viewport anchored on a line that the restored fold now hides has to move
	// to the first line that is still visible.
	m.ApplySessionState(SessionState{CursorLine: 4, ViewportTop: 2, Folds: []int{1}})

	if m.isHidden(m.viewportTop) {
		t.Fatalf("viewport top %d is hidden by the restored fold", m.viewportTop)
	}
}

func TestSessionStateSnapshot(t *testing.T) {
	m := newTestModel("a\nb\nc\n")
	m.cursor = cursorPos{line: 2, col: 1}
	m.viewportTop = 1
	m.viewportLeft = 2
	m.ensureFolds()
	m.collapsed = []foldSpan{{start: 0, end: 1}}

	snap := m.Snapshot()
	st := snap.SessionState()
	if st.CursorLine != 2 || st.CursorCol != 1 || st.ViewportTop != 1 || st.ViewportLeft != 2 {
		t.Fatalf("snapshot state = %+v, want cursor 2:1 viewport 1/2", st)
	}
	if len(st.Folds) != 1 || st.Folds[0] != 0 {
		t.Fatalf("snapshot folds = %v, want [0]", st.Folds)
	}
}

func TestApplySessionStateClampsToShorterFile(t *testing.T) {
	m := newTestModel("one\ntwo\n")
	m.path = "a.txt"
	m.viewWidth = 40
	m.viewHeight = 5
	m.recomputeGutterWidth()

	m.ApplySessionState(SessionState{CursorLine: 99, CursorCol: 99, ViewportTop: 50})

	if m.cursor.line != 1 {
		t.Fatalf("cursor line = %d, want the last line (1)", m.cursor.line)
	}
	if m.cursor.col != len("two") {
		t.Fatalf("cursor col = %d, want %d", m.cursor.col, len("two"))
	}
	if m.viewportTop > 1 {
		t.Fatalf("viewportTop = %d, want it clamped into the file", m.viewportTop)
	}
}

func TestApplySessionStateBacksOffMidRune(t *testing.T) {
	m := newTestModel("héllo\n")
	m.path = "a.txt"
	m.viewWidth = 40
	m.viewHeight = 5
	m.recomputeGutterWidth()

	// Byte 2 is inside the two-byte é (bytes 1-2).
	m.ApplySessionState(SessionState{CursorLine: 0, CursorCol: 2})
	if m.cursor.col != 1 {
		t.Fatalf("cursor col = %d, want it backed off to the rune start (1)", m.cursor.col)
	}
}

func TestApplySessionStateDropsStaleFolds(t *testing.T) {
	m := newTestModel("one\ntwo\nthree\n")
	m.path = "a.txt"
	m.viewWidth = 40
	m.viewHeight = 5
	m.recomputeGutterWidth()

	// Plain text has no fold regions, so every saved header is dropped.
	m.ApplySessionState(SessionState{CursorLine: 1, Folds: []int{0, 1}})
	if len(m.collapsed) != 0 {
		t.Fatalf("collapsed = %+v, want no folds for a file with no regions", m.collapsed)
	}
}

func TestApplySessionStateRestoresFolds(t *testing.T) {
	m := foldTestModel(t)

	// Cursor on the header line, so the folded block stays folded.
	m.ApplySessionState(SessionState{CursorLine: 0, Folds: []int{1}})

	if len(m.collapsed) != 1 || m.collapsed[0].start != 1 || m.collapsed[0].end != 3 {
		t.Fatalf("collapsed = %+v, want one span 1..3", m.collapsed)
	}
	if len(m.SessionState().Folds) != 1 || m.SessionState().Folds[0] != 1 {
		t.Fatalf("folds = %v, want [1] to survive a save", m.SessionState().Folds)
	}
}

func TestApplySessionStateKeepsCursorOutOfRestoredFold(t *testing.T) {
	m := foldTestModel(t)

	// A cursor saved inside the same block (only possible if the block was open
	// when it was written) must not end up hidden: the cursor wins, the fold is
	// dropped.
	m.ApplySessionState(SessionState{CursorLine: 2, CursorCol: 1, Folds: []int{1}})

	if m.isHidden(m.cursor.line) {
		t.Fatalf("cursor landed on hidden line %d", m.cursor.line)
	}
	if m.cursor.line != 2 {
		t.Fatalf("cursor line = %d, want it kept at 2", m.cursor.line)
	}
}

// foldTestModel is a Go file with a nested block, ready to fold.
func foldTestModel(t *testing.T) Model {
	t.Helper()
	m := newTestModel("func main() {\n\tif true {\n\t\tprintln(1)\n\t}\n}\n")
	m.path = "main.go"
	m.cfg = config.Config{Editor: config.EditorConfig{TabWidth: 4}}
	m.viewWidth = 40
	m.viewHeight = 10
	m.recomputeGutterWidth()
	return m
}

func TestApplySessionStateIgnoresBinaryBuffer(t *testing.T) {
	m := newTestModel("")
	m.path = "photo.png"
	m.binaryFile = true
	m.cursor = cursorPos{line: 0, col: 0}

	m.ApplySessionState(SessionState{CursorLine: 5, CursorCol: 5})
	if m.cursor != (cursorPos{}) {
		t.Fatalf("cursor = %+v, want it untouched for a binary file", m.cursor)
	}
}
