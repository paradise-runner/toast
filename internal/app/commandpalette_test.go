package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
)

func paletteOpenKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl | tea.ModShift}
}

func TestCommandPalette_OpenCloseViaKeybinding(t *testing.T) {
	m := newTestApp(t, t.TempDir())

	cmd := m.handleKey(paletteOpenKey())
	if cmd != nil {
		t.Fatal("expected palette open to be synchronous")
	}
	if !m.commandPaletteOpen {
		t.Fatal("expected palette to open on ctrl+shift+p")
	}
	if !m.commandPalette.IsOpen() {
		t.Fatal("expected palette component to be open")
	}

	// The palette toggle key closes it again.
	_ = m.handleKey(paletteOpenKey())
	if m.commandPaletteOpen {
		t.Fatal("expected palette to close on second ctrl+shift+p")
	}
}

func TestCommandPalette_EscapeCloses(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_ = m.handleKey(paletteOpenKey())
	if !m.commandPaletteOpen {
		t.Fatal("expected palette to open")
	}

	cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("expected escape close to be handled synchronously")
	}
	if m.commandPaletteOpen || m.commandPalette.IsOpen() {
		t.Fatal("expected palette to close on escape")
	}
}

func TestCommandPalette_SelectDispatchesAction(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	before := m.sidebarVisible
	_ = m.handleKey(paletteOpenKey())

	_, cmd := m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionToggleSidebar, Label: "Toggle Sidebar",
	})
	if cmd == nil {
		t.Fatal("expected a cmd (confirmation flash tick)")
	}
	if m.sidebarVisible == before {
		t.Fatal("expected sidebar visibility to toggle")
	}
	if m.commandPaletteOpen {
		t.Fatal("expected palette to close on select")
	}
}

func TestCommandPalette_SelectSaveFocusesEditorAndSchedulesFlash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	openFile(t, m, filePath)
	// Move focus away from the editor first.
	m.setFocus(FocusFileTree)

	_ = m.handleKey(paletteOpenKey())
	_, cmd := m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionSave, Label: "Save",
	})
	if cmd == nil {
		t.Fatal("expected a cmd from save dispatch")
	}
	if m.focus != FocusEditor {
		t.Fatal("expected save dispatch to focus the editor")
	}

	// The flash tick expires and clears the flash.
	_, clearCmd := m.Update(messages.StatusFlashClearMsg{})
	if clearCmd != nil {
		t.Fatal("expected flash clear to be synchronous")
	}
}

func TestCommandPalette_SelectQuickOpenOpensQuickOpen(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_ = m.handleKey(paletteOpenKey())

	_, cmd := m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionQuickOpen, Label: "Open File",
	})
	if cmd == nil {
		t.Fatal("expected a cmd (flash tick + file load)")
	}
	if m.commandPaletteOpen {
		t.Fatal("expected palette to close")
	}
	if !m.quickOpenOpen {
		t.Fatal("expected quick open overlay to open")
	}
}

func TestCommandPalette_SelectPaletteItselfClosesWithoutFlash(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_ = m.handleKey(paletteOpenKey())

	_, cmd := m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionCommandPalette, Label: "Command Palette",
	})
	if cmd != nil {
		t.Fatal("expected no cmd (no flash for palette itself)")
	}
	if m.commandPaletteOpen {
		t.Fatal("expected palette to close")
	}
}

func TestCommandPalette_UndoRedoDispatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	openFile(t, m, filePath)

	// Type text so undo has something to undo.
	typeKey(m, '!')
	_ = m.handleKey(paletteOpenKey())
	_, cmd := m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionUndo, Label: "Undo",
	})
	if cmd == nil {
		t.Fatal("expected a cmd from undo dispatch")
	}
	content := m.editor.Content()
	if strings.Contains(content, "!") {
		t.Fatalf("expected undo to remove typed text, got %q", content)
	}

	_ = m.handleKey(paletteOpenKey())
	_, cmd = m.Update(messages.CommandPaletteSelectMsg{
		ActionID: config.ActionRedo, Label: "Redo",
	})
	if cmd == nil {
		t.Fatal("expected a cmd from redo dispatch")
	}
	content = m.editor.Content()
	if !strings.Contains(content, "!") {
		t.Fatalf("expected redo to restore typed text, got %q", content)
	}
}
