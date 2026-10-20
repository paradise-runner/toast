package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// Regression test for issue #83: fsnotify echoes the editor's own auto-save
// write back as FileChangedOnDiskMsg. When the event is delivered after
// FileSavedMsg cleared isSavingPath the buffer is already clean, so the app
// used to reload it and jump the cursor to the top.
func TestFileChangedOnDisk_OwnAutoSaveDoesNotResetCursor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\nline4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, path)

	// Put the cursor on line 3 and make an edit so auto-save fires.
	for i := 0; i < 2; i++ {
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	editCmd := typeKey(m, 'x')
	tick := findAutoSaveTick(t, editCmd)
	if tick == nil {
		t.Fatal("expected auto-save tick after edit")
	}
	updated, saveCmd := m.Update(*tick)
	m = updated.(*Model)
	runAppCmd(t, m, saveCmd, 0)
	if m.editor.IsModified() {
		t.Fatal("expected buffer to be clean after auto-save")
	}

	// Simulate the watcher event for our own write arriving late.
	_, cmd := m.Update(messages.FileChangedOnDiskMsg{Path: path})
	runAppCmd(t, m, cmd, 0)

	if got := m.editor.CursorLine(); got != 2 {
		t.Fatalf("cursor line after own-save watcher event = %d, want 2", got)
	}
	if got := m.editor.CursorCol(); got != 1 {
		t.Fatalf("cursor col after own-save watcher event = %d, want 1", got)
	}
}

// TestFileChangedOnDisk_ExternalChangePreservesCursor verifies that a genuine
// external change still reloads the buffer but keeps the caret in place.
func TestFileChangedOnDisk_ExternalChangePreservesCursor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\nline4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	openFile(t, m, path)

	for i := 0; i < 2; i++ {
		m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// An external writer (formatter, git checkout, another editor) rewrites the
	// file with different content but the same shape.
	if err := os.WriteFile(path, []byte("one\ntwo\nTHREE\nfour\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, cmd := m.Update(messages.FileChangedOnDiskMsg{Path: path})
	runAppCmd(t, m, cmd, 0)

	if got := m.editor.Content(); got != "one\ntwo\nTHREE\nfour\n" {
		t.Fatalf("content = %q, want reloaded content", got)
	}
	if got := m.editor.CursorLine(); got != 2 {
		t.Fatalf("cursor line after external reload = %d, want 2", got)
	}
}
