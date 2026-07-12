package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

func collectAppCmdMessages(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, batchCmd := range batch {
			msgs = append(msgs, collectAppCmdMessages(t, batchCmd)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func runAppCmd(t *testing.T, m *Model, cmd tea.Cmd, depth int) {
	t.Helper()
	if cmd == nil || depth > 10 {
		return
	}
	for _, msg := range collectAppCmdMessages(t, cmd) {
		_, next := m.Update(msg)
		runAppCmd(t, m, next, depth+1)
	}
}

// TestApp_FileSavingMsg_SetsSavingPath verifies that receiving FileSavingMsg sets isSavingPath.
func TestApp_FileSavingMsg_SetsSavingPath(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	if m.isSavingPath != "" {
		t.Fatal("isSavingPath should start empty")
	}
	m.Update(messages.FileSavingMsg{Path: "/some/file.go"})
	if m.isSavingPath != "/some/file.go" {
		t.Fatalf("isSavingPath = %q, want %q", m.isSavingPath, "/some/file.go")
	}
}

// TestApp_FileSavedMsg_ClearsSavingPath verifies that receiving FileSavedMsg clears isSavingPath.
func TestApp_FileSavedMsg_ClearsSavingPath(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	m.isSavingPath = "/some/file.go"
	m.Update(messages.FileSavedMsg{Path: "/some/file.go"})
	if m.isSavingPath != "" {
		t.Fatalf("isSavingPath should be empty after FileSavedMsg, got %q", m.isSavingPath)
	}
}

// TestApp_FileSaveFailedMsg_ClearsSavingPath verifies that receiving FileSaveFailedMsg clears isSavingPath.
func TestApp_FileSaveFailedMsg_ClearsSavingPath(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	m.isSavingPath = "/some/file.go"
	m.Update(messages.FileSaveFailedMsg{Path: "/some/file.go", Err: errors.New("disk full")})
	if m.isSavingPath != "" {
		t.Fatalf("isSavingPath should be empty after FileSaveFailedMsg, got %q", m.isSavingPath)
	}
}

// TestApp_FileChangedOnDisk_SkippedWhileSaving verifies that a FileChangedOnDiskMsg
// for the currently-saving path is suppressed entirely (no git status, no reload).
func TestApp_FileChangedOnDisk_SkippedWhileSaving(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	m.isSavingPath = "/some/file.go"

	_, _ = m.Update(messages.FileChangedOnDiskMsg{Path: "/some/file.go"})
	// isSavingPath should still be set — FileChangedOnDiskMsg does not clear it
	if m.isSavingPath != "/some/file.go" {
		t.Fatalf("isSavingPath should be unchanged, got %q", m.isSavingPath)
	}
}

func TestApp_EscapeClosesFindReplaceImmediately(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	m.openFindReplace("foo")
	if !m.findReplaceOpen {
		t.Fatal("expected find/replace overlay to be open")
	}

	cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("expected escape close to be handled synchronously")
	}
	if m.findReplaceOpen {
		t.Fatal("expected find/replace overlay to close")
	}
	if m.findReplace.IsOpen() {
		t.Fatal("expected find/replace component to close")
	}
}

func TestApp_FileSelectedClosesSearch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m := newTestApp(t, dir)
	m.openSearch()

	_, _ = m.Update(messages.FileSelectedMsg{Path: path})

	if m.searchOpen {
		t.Fatal("expected search to close after selecting a file")
	}
	if m.focus != FocusEditor {
		t.Fatalf("focus = %v, want FocusEditor", m.focus)
	}
}

func TestApp_SearchCloseButtonClickClosesSearch(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	m.sidebarVisible = false
	m.resizeComponents()
	m.openSearch()

	cmd := m.handleMouseClick(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.width - 1,
		Y:      2,
	})
	msgs := collectAppCmdMessages(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if _, ok := msgs[0].(messages.SearchCloseMsg); !ok {
		t.Fatalf("expected SearchCloseMsg, got %T", msgs[0])
	}

	_, _ = m.Update(msgs[0])
	if m.searchOpen {
		t.Fatal("expected search to close after close-button message")
	}
}

func TestApp_DefinitionNavigationOpensTargetAndMovesToPosition(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.go")
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(source, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(target, []byte("package main\n\nfunc target() {}\n"), 0644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	m := newTestApp(t, dir)
	_, openSource := m.Update(messages.FileSelectedMsg{Path: source})
	runAppCmd(t, m, openSource, 0)

	_, navigate := m.Update(messages.DefinitionResultMsg{Path: target, Line: 2, Col: 5, Navigate: true})
	runAppCmd(t, m, navigate, 0)

	if m.editor.Path() != target {
		t.Fatalf("editor path = %q, want %q", m.editor.Path(), target)
	}
	if m.editor.CursorLine() != 2 || m.editor.CursorCol() != 5 {
		t.Fatalf("cursor = (%d,%d), want (2,5)", m.editor.CursorLine(), m.editor.CursorCol())
	}
}

func TestApp_LSPInstallPromptRendersAndDismisses(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_, _ = m.Update(messages.LSPInstallPromptMsg{Language: "go", Name: "gopls"})
	if !strings.Contains(m.View().Content, "Language support for go") {
		t.Fatal("expected managed install prompt in app view")
	}

	cmd := m.handleKey(tea.KeyPressMsg{Code: 'n'})
	if cmd != nil || m.lspInstall.Visible() {
		t.Fatal("expected not-now key to dismiss prompt")
	}
}

func TestApp_ViewEnablesHoverMouseMotion(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	view := m.View()
	if view.MouseMode != tea.MouseModeAllMotion {
		t.Fatalf("mouse mode = %v, want MouseModeAllMotion for Ctrl-hover", view.MouseMode)
	}
}
