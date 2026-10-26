package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestApp_ToggleFoldViaKeybinding checks that the fold keybinding is consumed
// by the app (not typed into the buffer) and that the editor really collapses
// the block, observable through fold-aware cursor movement.
func TestApp_ToggleFoldViaKeybinding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	content := "func a() {\n\tb()\n}\nnext()\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	m := newTestApp(t, dir)
	openFile(t, m, path)
	m.setFocus(FocusEditor)

	if cmd := m.handleKey(tea.KeyPressMsg{Code: 't', Mod: tea.ModAlt}); cmd != nil {
		t.Fatalf("toggle fold should be synchronous, got cmd %T", cmd)
	}
	if got := m.editor.Content(); got != content {
		t.Fatalf("buffer changed by fold keybinding: %q", got)
	}

	_ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := m.editor.CursorLine(); got != 3 {
		t.Fatalf("cursor line after folding = %d, want 3 (folded block skipped)", got)
	}
}
