package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/components/editor"
	"github.com/yourusername/toast/internal/messages"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// FileMovedMsg must re-point an open buffer (editor, tab bar map) at the new
// path without closing the tab or losing the buffer ID.
func TestApp_FileMovedMsg_RebindsOpenBuffer(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "foo.go")
	writeFile(t, oldPath)

	m := newTestApp(t, dir)
	_, cmd := m.Update(messages.FileSelectedMsg{Path: oldPath})
	runAppCmd(t, m, cmd, 0)

	if _, ok := m.openBuffers[oldPath]; !ok {
		t.Fatalf("expected %q open", oldPath)
	}
	if m.editor.Path() != oldPath {
		t.Fatalf("editor path = %q, want %q", m.editor.Path(), oldPath)
	}

	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(subDir, "foo.go")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	m.Update(messages.FileMovedMsg{OldPath: oldPath, NewPath: newPath, IsDir: false})

	if _, ok := m.openBuffers[oldPath]; ok {
		t.Error("old path still keyed in openBuffers")
	}
	if _, ok := m.openBuffers[newPath]; !ok {
		t.Error("new path not keyed in openBuffers")
	}
	if m.editor.Path() != newPath {
		t.Errorf("editor path = %q, want %q", m.editor.Path(), newPath)
	}
	if tab := m.tabBar.ActiveTab(); tab == nil || tab.Path != newPath {
		t.Errorf("active tab path = %v, want %q", tab, newPath)
	}
}

// A directory move must rebase every open buffer under the old prefix.
func TestApp_FileMovedMsg_DirMoveRebindsNestedBuffers(t *testing.T) {
	dir := t.TempDir()
	oldDir := filepath.Join(dir, "old")
	newDir := filepath.Join(dir, "new")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(oldDir, "a.go")
	writeFile(t, nested)

	m := newTestApp(t, dir)
	m.openBuffers[nested] = 7

	m.Update(messages.FileMovedMsg{OldPath: oldDir, NewPath: newDir, IsDir: true})

	if _, ok := m.openBuffers[nested]; ok {
		t.Error("nested old path still keyed")
	}
	want := filepath.Join(newDir, "a.go")
	if id, ok := m.openBuffers[want]; !ok || id != 7 {
		t.Errorf("openBuffers[%q] = %d, %v; want 7, true", want, id, ok)
	}
}

// Snapshots for buffers closed after editing at the old path must be re-keyed
// so unsaved changes survive a move.
func TestApp_FileMovedMsg_RekeysClosedSnapshots(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "gone.go")
	newPath := filepath.Join(dir, "moved.go")

	m := newTestApp(t, dir)
	m.closedSnapshots[oldPath] = editor.BufferSnapshot{Path: oldPath}

	m.Update(messages.FileMovedMsg{OldPath: oldPath, NewPath: newPath, IsDir: false})

	if _, ok := m.closedSnapshots[oldPath]; ok {
		t.Error("old snapshot key still present")
	}
	if _, ok := m.closedSnapshots[newPath]; !ok {
		t.Error("snapshot not re-keyed to new path")
	}
}

// End-to-end routing: a press/motion/release over the sidebar must reach the
// file tree and move the file. Global Y is offset by the tab bar (1 row).
func TestApp_SidebarDragMovesFile(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "a.txt")
	writeFile(t, filePath)

	m := newTestApp(t, dir)
	// Sidebar rows: 0 root, 1 sub, 2 a.txt. Global Y = row + 1.
	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 2, Y: 3})
	_, _ = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: 2, Y: 2})
	if !m.fileTree.IsDragging() {
		t.Fatal("expected file tree drag to be active after motion")
	}
	_, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: 2, Y: 2})

	newPath := filepath.Join(subDir, "a.txt")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected %q to exist after drag: %v", newPath, err)
	}
	runAppCmd(t, m, cmd, 0) // deliver FileMovedMsg to the app
}

// Releasing a sidebar drag on the status bar must cancel it — not leave the
// tree stuck in a drag, and not move the file.
func TestApp_SidebarDragReleasedOnStatusBarCancels(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "a.txt")
	writeFile(t, filePath)

	m := newTestApp(t, dir)
	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 2, Y: 3}) // press a.txt
	_, _ = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: 2, Y: 2})
	_, _ = m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: 2, Y: m.height - 1})

	if m.fileTree.IsDragging() {
		t.Error("drag should be cancelled after releasing on the status bar")
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("file should not have moved: %v", err)
	}
	if m.movingPath != "" {
		t.Errorf("movingPath = %q, want empty after cancel", m.movingPath)
	}
}
