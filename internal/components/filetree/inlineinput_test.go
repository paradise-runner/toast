package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
)

func TestInlineInput_Insert(t *testing.T) {
	inp := &InlineInput{}
	inp.Insert('h')
	inp.Insert('i')
	if inp.value != "hi" {
		t.Errorf("expected value 'hi', got %q", inp.value)
	}
}

func TestInlineInput_Backspace(t *testing.T) {
	inp := &InlineInput{value: "foo"}
	inp.Backspace()
	if inp.value != "fo" {
		t.Errorf("expected 'fo' after backspace, got %q", inp.value)
	}
}

func TestInlineInput_Backspace_Empty(t *testing.T) {
	inp := &InlineInput{}
	inp.Backspace() // should not panic
	if inp.value != "" {
		t.Errorf("expected empty value, got %q", inp.value)
	}
}

func TestInlineInput_Backspace_MultibyteRune(t *testing.T) {
	inp := &InlineInput{value: "café"}
	inp.Backspace()
	if inp.value != "caf" {
		t.Errorf("expected 'caf' after backspace on multi-byte rune, got %q", inp.value)
	}
}

func TestInlineInput_RenderRow_ContainsCursor(t *testing.T) {
	tm := newTestTheme(t)
	inp := &InlineInput{value: "main", isDir: false}
	row := inp.RenderRow(0, 30, tm)
	if !strings.Contains(row, "▌") {
		t.Errorf("expected cursor '▌' in render, got: %q", row)
	}
}

func TestInlineInput_RenderRow_DirShowsArrow(t *testing.T) {
	tm := newTestTheme(t)
	inp := &InlineInput{value: "pkg", isDir: true}
	row := inp.RenderRow(0, 30, tm)
	if !strings.Contains(row, "▶") {
		t.Errorf("expected '▶' icon for directory, got: %q", row)
	}
}

func TestInlineInput_RenderRow_WidthIsRespected(t *testing.T) {
	tm := newTestTheme(t)
	inp := &InlineInput{value: "main", isDir: false}
	const requestedWidth = 30
	row := inp.RenderRow(0, requestedWidth, tm)
	// The visual width of the rendered row (stripping ANSI) should be <= requestedWidth
	// We use lipgloss.Width which handles ANSI stripping
	w := lipgloss.Width(row)
	if w > requestedWidth {
		t.Errorf("expected rendered width <= %d, got %d", requestedWidth, w)
	}
}

func TestInlineInput_RenderRow_FileHasNoArrow(t *testing.T) {
	tm := newTestTheme(t)
	inp := &InlineInput{value: "main.go", isDir: false}
	row := inp.RenderRow(0, 30, tm)
	if strings.Contains(row, "▶") {
		t.Errorf("expected no '▶' icon for file, got: %q", row)
	}
}

func TestInlineInput_RenderRow_Depth(t *testing.T) {
	tm := newTestTheme(t)
	inp := &InlineInput{value: "sub", isDir: false}
	// Both depths should produce rows containing the value
	for _, depth := range []int{0, 1, 2} {
		row := inp.RenderRow(depth, 40, tm)
		if !strings.Contains(row, "sub") {
			t.Errorf("depth %d: expected row to contain 'sub', got: %q", depth, row)
		}
	}
}

// ── Disk-creation integration tests ──────────────────────────────────────────

// openInlineEdit drives: right-click at (2,0) → optionally down-arrow → enter
// to put the model into inline edit mode for a file (isDir=false) or dir (isDir=true).
func openInlineEdit(t *testing.T, m Model, isDir bool) Model {
	t.Helper()
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if isDir {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return m
}

// typeAndEnter types each character in name then presses Enter.
func typeAndEnter(t *testing.T, m Model, name string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, ch := range name {
		m, cmd = m.Update(tea.KeyPressMsg{Text: string(ch)})
		_ = cmd
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return m, cmd
}

func TestInlineInput_EnterCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)
	rootDir := m.rootDir

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set after opening inline edit")
	}

	m, gotCmd := typeAndEnter(t, m, "newfile.go")

	wantPath := filepath.Join(rootDir, "newfile.go")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file %q to exist: %v", wantPath, err)
	}
	if gotCmd == nil {
		t.Fatal("expected non-nil cmd after creating file")
	}
	msg := gotCmd()
	fc, ok := msg.(messages.FileCreatedMsg)
	if !ok {
		t.Fatalf("expected FileCreatedMsg, got %T", msg)
	}
	if fc.Path != wantPath {
		t.Errorf("expected Path %q, got %q", wantPath, fc.Path)
	}
}

func TestInlineInput_EnterCreatesDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)
	rootDir := m.rootDir

	m = openInlineEdit(t, m, true)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set after opening inline edit for dir")
	}

	m, gotCmd := typeAndEnter(t, m, "newpkg")

	wantPath := filepath.Join(rootDir, "newpkg")
	info, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("expected dir %q to exist: %v", wantPath, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %q to be a directory", wantPath)
	}
	if gotCmd == nil {
		t.Fatal("expected non-nil cmd after creating dir")
	}
	msg := gotCmd()
	dc, ok := msg.(messages.DirCreatedMsg)
	if !ok {
		t.Fatalf("expected DirCreatedMsg, got %T", msg)
	}
	if dc.Path != wantPath {
		t.Errorf("expected Path %q, got %q", wantPath, dc.Path)
	}
}

func TestInlineInput_EmptyNameCancels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set")
	}

	// Press Enter immediately with empty name.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.inlineInput != nil {
		t.Error("expected inlineInput to be nil after empty name + Enter")
	}
	if cmd != nil {
		t.Error("expected nil cmd after cancelling with empty name")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in rootDir, got %d", len(entries))
	}
}

func TestInlineInput_EscapeCancels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set")
	}

	// Type a character then Escape.
	m, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.inlineInput != nil {
		t.Error("expected inlineInput to be nil after Escape")
	}
}

func TestInlineInput_NameCollisionAborts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)
	rootDir := m.rootDir

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set")
	}

	m, gotCmd := typeAndEnter(t, m, "main.go")

	if gotCmd != nil {
		t.Error("expected nil cmd when target already exists")
	}

	// Existing file should still be 0 bytes.
	info, err := os.Stat(filepath.Join(rootDir, "main.go"))
	if err != nil {
		t.Fatalf("expected main.go to still exist: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected main.go to still be 0 bytes, got %d", info.Size())
	}
}

func TestInlineInput_NestedPath_CreatesIntermediateDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)
	rootDir := m.rootDir

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set")
	}

	m, gotCmd := typeAndEnter(t, m, "sub/nested.go")

	if gotCmd == nil {
		t.Fatal("expected non-nil cmd after creating nested file")
	}
	msg := gotCmd()
	if _, ok := msg.(messages.FileCreatedMsg); !ok {
		t.Fatalf("expected FileCreatedMsg, got %T", msg)
	}

	wantPath := filepath.Join(rootDir, "sub", "nested.go")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file %q to exist: %v", wantPath, err)
	}
}

func TestInlineInput_PathTraversal_IsRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestModel(t, dir)
	rootDir := m.rootDir

	m = openInlineEdit(t, m, false)
	if m.inlineInput == nil {
		t.Fatalf("expected inlineInput to be set")
	}

	m, gotCmd := typeAndEnter(t, m, "../escape.go")

	if gotCmd != nil {
		t.Error("expected nil cmd when path traversal attempted")
	}

	escapePath := filepath.Join(filepath.Dir(rootDir), "escape.go")
	if _, err := os.Stat(escapePath); !os.IsNotExist(err) {
		t.Errorf("expected escape.go NOT to exist at %q, but found it (err=%v)", escapePath, err)
	}
}
