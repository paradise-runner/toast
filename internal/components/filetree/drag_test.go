package filetree

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// ── Drag helpers ──────────────────────────────────────────────────────────────

func leftPress(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y}
}

func leftMotion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{Button: tea.MouseLeft, X: x, Y: y}
}

func leftRelease(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{Button: tea.MouseLeft, X: x, Y: y}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// A plain click (press and release on the same row) must still activate the
// node — the drag support must not break click-to-open.
func TestDrag_PlainClickActivates(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	mustWrite(t, filePath)
	m := newTestModel(t, dir)

	// flat: [root, a.txt]
	row := 1
	m, _ = m.Update(leftPress(2, row))
	if m.drag.from == nil {
		t.Fatal("expected a pending click after press")
	}
	m, cmd := m.Update(leftRelease(2, row))

	if cmd == nil {
		t.Fatal("expected FileSelectedMsg after plain click")
	}
	msg, ok := cmd().(messages.FileSelectedMsg)
	if !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", cmd())
	}
	if msg.Path != filePath {
		t.Fatalf("selected %q, want %q", msg.Path, filePath)
	}
	if m.IsDragging() {
		t.Error("plain click must not start a drag")
	}
}

// Dragging a file onto a directory moves it there and emits FileMovedMsg.
func TestDrag_MoveFileIntoDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "a.txt")
	mustWrite(t, filePath)

	m := newTestModel(t, dir)
	// flat: [root, sub, a.txt] — directories sort before files.

	m, _ = m.Update(leftPress(2, 2))  // press a.txt
	m, _ = m.Update(leftMotion(2, 1)) // drag over sub
	if !m.IsDragging() {
		t.Fatal("expected drag to activate after motion")
	}
	if m.drag.dropIdx != 1 {
		t.Fatalf("dropIdx = %d, want 1 (sub)", m.drag.dropIdx)
	}
	m, cmd := m.Update(leftRelease(2, 1))

	newPath := filepath.Join(subDir, "a.txt")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected %q to exist: %v", newPath, err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected original %q to be gone", filePath)
	}
	if m.IsDragging() {
		t.Error("drag should be cleared after release")
	}
	if cmd == nil {
		t.Fatal("expected FileMovedMsg")
	}
	msg, ok := cmd().(messages.FileMovedMsg)
	if !ok {
		t.Fatalf("expected FileMovedMsg, got %T", cmd())
	}
	if msg.OldPath != filePath || msg.NewPath != newPath || msg.IsDir {
		t.Fatalf("FileMovedMsg = %+v", msg)
	}
}

// Dragging a file from a subdirectory up onto the root moves it up.
func TestDrag_MoveFileUpToRoot(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(subDir, "a.txt")
	mustWrite(t, filePath)

	m := newTestModel(t, dir)
	m.expandPath(subDir)
	// flat: [root, sub, a.txt]

	m, _ = m.Update(leftPress(2, 2))  // press a.txt
	m, _ = m.Update(leftMotion(0, 0)) // drag over root
	m, cmd := m.Update(leftRelease(0, 0))

	newPath := filepath.Join(dir, "a.txt")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected %q to exist: %v", newPath, err)
	}
	if cmd == nil {
		t.Fatal("expected FileMovedMsg")
	}
}

// Dropping a directory onto its own descendant must be rejected.
func TestDrag_DescendantRejected(t *testing.T) {
	dir := t.TempDir()
	aDir := filepath.Join(dir, "a")
	bDir := filepath.Join(aDir, "b")
	if err := os.MkdirAll(bDir, 0755); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t, dir)
	m.expandPath(aDir)
	// flat: [root, a, b]

	m, _ = m.Update(leftPress(2, 1))  // press a
	m, _ = m.Update(leftMotion(2, 2)) // drag over b (a descendant)
	if m.drag.dropIdx != -1 {
		t.Fatalf("expected invalid drop target, got dropIdx=%d", m.drag.dropIdx)
	}
	m, cmd := m.Update(leftRelease(2, 2))

	if cmd != nil {
		t.Errorf("expected no move cmd, got %T", cmd())
	}
	if _, err := os.Stat(aDir); err != nil {
		t.Errorf("directory a should be untouched: %v", err)
	}
	if entries, _ := os.ReadDir(bDir); len(entries) != 0 {
		t.Errorf("b should be empty, got %d entries", len(entries))
	}
}

// Dropping a node back onto its current parent is a no-op.
func TestDrag_SameParentNoOp(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	mustWrite(t, filePath)
	m := newTestModel(t, dir)

	m, _ = m.Update(leftPress(2, 1))  // press a.txt
	m, _ = m.Update(leftMotion(2, 0)) // drag over root (its parent)
	m, cmd := m.Update(leftRelease(2, 0))

	if cmd != nil {
		t.Errorf("expected no move cmd, got %T", cmd())
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file should be untouched: %v", err)
	}
}

// A name collision in the destination must be rejected.
func TestDrag_CollisionRejected(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "a.txt")
	dst := filepath.Join(subDir, "a.txt")
	mustWrite(t, src)
	mustWrite(t, dst)

	m := newTestModel(t, dir)
	// flat: [root, sub, a.txt]

	m, _ = m.Update(leftPress(2, 2))
	m, _ = m.Update(leftMotion(2, 1))
	if m.drag.dropIdx != -1 {
		t.Fatalf("expected collision to invalidate the drop, got %d", m.drag.dropIdx)
	}
	m, cmd := m.Update(leftRelease(2, 1))

	if cmd != nil {
		t.Errorf("expected no move cmd, got %T", cmd())
	}
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("source should still exist: %v", err)
	}
	dstData, _ := os.ReadFile(dst)
	if string(srcData) != string(dstData) {
		t.Error("destination should be unchanged")
	}
}

// The root row is not draggable.
func TestDrag_RootRowNotDraggable(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"))
	m := newTestModel(t, dir)

	m, _ = m.Update(leftPress(2, 0)) // press root
	if m.drag.from != nil {
		t.Fatal("root row must not start a drag")
	}
	m, _ = m.Update(leftMotion(2, 1))
	if m.IsDragging() {
		t.Error("motion after pressing root must not start a drag")
	}
}

// Releasing outside the tree viewport cancels the drag rather than dropping on
// the last row the pointer crossed.
func TestDrag_ReleaseOutsideCancels(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "a.txt")
	mustWrite(t, filePath)

	m := newTestModel(t, dir)
	m, _ = m.Update(leftPress(2, 2))
	m, _ = m.Update(leftMotion(2, 1)) // valid target: sub
	if m.drag.dropIdx != 1 {
		t.Fatalf("dropIdx = %d, want 1", m.drag.dropIdx)
	}
	m, cmd := m.Update(leftRelease(2, 99)) // release below the viewport

	if cmd != nil {
		t.Errorf("expected cancel, got %T", cmd())
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file should not have moved: %v", err)
	}
	if m.IsDragging() {
		t.Error("drag should be cleared")
	}
}

// A click that jitters onto another row and back to the source still activates.
func TestDrag_JitterBackToSourceActivates(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	mustWrite(t, filePath)
	m := newTestModel(t, dir)

	m, _ = m.Update(leftPress(2, 1))
	m, _ = m.Update(leftMotion(2, 0))
	m, _ = m.Update(leftMotion(2, 1))
	m, cmd := m.Update(leftRelease(2, 1))

	if cmd == nil {
		t.Fatal("expected FileSelectedMsg")
	}
	if _, ok := cmd().(messages.FileSelectedMsg); !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", cmd())
	}
}
