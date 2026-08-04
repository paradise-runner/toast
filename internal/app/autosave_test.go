package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// typeKey sends a printable-character keypress to the focused editor through
// the app and returns the resulting command.
func typeKey(m *Model, ch rune) tea.Cmd {
	return m.handleKey(tea.KeyPressMsg{Code: rune(ch), Text: string(ch)})
}

// openFile loads path into the editor and drains all resulting commands.
func openFile(t *testing.T, m *Model, path string) {
	t.Helper()
	_, cmd := m.Update(messages.FileSelectedMsg{Path: path})
	runAppCmd(t, m, cmd, 0)
}

// findAutoSaveTick invokes cmd and returns the AutoSaveTickMsg it produces,
// or nil. Requires a short auto-save delay so the debounce timer resolves
// quickly.
func findAutoSaveTick(t *testing.T, cmd tea.Cmd) *messages.AutoSaveTickMsg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	for _, msg := range collectAppCmdMessages(t, cmd) {
		if tick, ok := msg.(messages.AutoSaveTickMsg); ok {
			return &tick
		}
	}
	return nil
}

func TestAutoSave_SchedulesDebouncedTickOnEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, path)

	cmd := typeKey(m, 'x')
	if !m.editor.IsModified() {
		t.Fatal("expected buffer to be modified after typing")
	}
	tick := findAutoSaveTick(t, cmd)
	if tick == nil {
		t.Fatal("expected AutoSaveTickMsg after an edit")
	}
	if tick.Generation != m.autoSaveGen {
		t.Fatalf("tick generation = %d, want current %d", tick.Generation, m.autoSaveGen)
	}
}

func TestAutoSave_ManualModeDoesNotSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSave = "manual"
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, path)

	cmd := typeKey(m, 'x')
	if tick := findAutoSaveTick(t, cmd); tick != nil {
		t.Fatal("expected no AutoSaveTickMsg in manual save mode")
	}
	if m.autoSaveGen != 0 {
		t.Fatalf("autoSaveGen = %d, want 0 in manual mode", m.autoSaveGen)
	}
	if !m.editor.IsModified() {
		t.Fatal("expected buffer to still be modified; manual mode must not save")
	}
}

func TestAutoSave_CursorMovementDoesNotSchedule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, path)

	cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatalf("expected nil command for cursor movement, got %v", cmd)
	}
	if m.autoSaveGen != 0 {
		t.Fatalf("autoSaveGen = %d, want 0 (cursor movement is not an edit)", m.autoSaveGen)
	}
}

func TestAutoSave_StaleTickIgnored_CurrentTickSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, path)

	// Two edits: the first debounce tick becomes stale.
	_ = typeKey(m, 'x') // gen 1
	_ = typeKey(m, 'y') // gen 2
	if m.autoSaveGen != 2 {
		t.Fatalf("autoSaveGen = %d, want 2", m.autoSaveGen)
	}

	// A stale tick (older generation) must not save.
	updated, staleCmd := m.Update(messages.AutoSaveTickMsg{Generation: 1})
	m = updated.(*Model)
	if staleCmd != nil {
		t.Fatalf("expected no command from stale tick, got %v", staleCmd)
	}
	if content, _ := os.ReadFile(path); string(content) != "package main\n" {
		t.Fatalf("stale tick must not write to disk, file = %q", content)
	}

	// The current-generation tick saves.
	updated, saveCmd := m.Update(messages.AutoSaveTickMsg{Generation: m.autoSaveGen})
	m = updated.(*Model)
	if saveCmd == nil {
		t.Fatal("expected save command from current tick")
	}
	msgs := collectAppCmdMessages(t, saveCmd)
	saved := false
	modifiedFalse := false
	for _, msg := range msgs {
		switch sm := msg.(type) {
		case messages.FileSavedMsg:
			if sm.Path == path {
				saved = true
			}
		case messages.BufferModifiedMsg:
			if sm.Modified {
				t.Fatal("expected Modified=false after auto-save")
			}
			modifiedFalse = true
		}
	}
	if !saved {
		t.Fatal("expected FileSavedMsg from auto-save")
	}
	if !modifiedFalse {
		t.Fatal("expected BufferModifiedMsg{Modified:false} after auto-save")
	}
	if content, _ := os.ReadFile(path); string(content) != "xypackage main\n" {
		t.Fatalf("file content after auto-save = %q, want %q", content, "xypackage main\n")
	}
	if m.editor.IsModified() {
		t.Fatal("expected editor buffer to be clean after auto-save")
	}
}

func TestAutoSave_TickOnCleanBufferDoesNotSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	openFile(t, m, path)

	updated, cmd := m.Update(messages.AutoSaveTickMsg{Generation: m.autoSaveGen})
	m = updated.(*Model)
	if cmd != nil {
		t.Fatalf("expected no command when buffer is clean, got %v", cmd)
	}
}

func TestAutoSave_SavesDirtyBackgroundSnapshots(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fileA, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSaveDelayMs = 1
	openFile(t, m, fileA)

	// Edit A, then switch to B — A becomes a dirty background snapshot.
	_ = typeKey(m, 'x')
	openFile(t, m, fileB)

	updated, saveCmd := m.Update(messages.AutoSaveTickMsg{Generation: m.autoSaveGen})
	m = updated.(*Model)
	_ = collectAppCmdMessages(t, saveCmd)

	if content, _ := os.ReadFile(fileA); string(content) != "xone\n" {
		t.Fatalf("background file A after auto-save = %q, want %q", content, "xone\n")
	}
	if content, _ := os.ReadFile(fileB); string(content) != "two" {
		t.Fatalf("clean file B must not be rewritten, got %q", content)
	}
	if m.editor.IsModified() {
		t.Fatal("expected editor (B) to remain clean")
	}
	// The saved snapshot must no longer be dirty.
	if snap, ok := m.bufferSnapshots[m.openBuffers[fileA]]; ok && snap.Modified() {
		t.Fatal("expected background snapshot A to be marked saved")
	}
	if !strings.Contains(m.editor.Content(), "two") {
		t.Fatalf("expected editor to still show B, got %q", m.editor.Content())
	}
}

func TestAutoSave_SavesViaKeybindingInManualMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)
	m.cfg.Editor.AutoSave = "manual"
	openFile(t, m, path)

	_ = typeKey(m, 'x')
	cmd := m.handleKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	msgs := collectAppCmdMessages(t, cmd)
	saved := false
	for _, msg := range msgs {
		if sm, ok := msg.(messages.FileSavedMsg); ok && sm.Path == path {
			saved = true
		}
	}
	if !saved {
		t.Fatal("expected manual ctrl+s to save even in manual mode")
	}
	if content, _ := os.ReadFile(path); string(content) != "xpackage main\n" {
		t.Fatalf("file content after manual save = %q", content)
	}
}
