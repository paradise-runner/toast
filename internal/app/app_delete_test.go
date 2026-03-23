package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

func newTestApp(t *testing.T, rootDir string) *Model {
	t.Helper()
	cfg := config.Defaults()
	m, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("failed to create app model: %v", err)
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func TestApp_FileDeletedMsg_ClosesOpenTab(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestApp(t, dir)

	// Open the file in the editor
	_, _ = m.Update(messages.FileSelectedMsg{Path: filePath})

	// Verify it's open
	if _, ok := m.openBuffers[filePath]; !ok {
		t.Fatal("expected file to be open in openBuffers")
	}

	// Emit FileDeletedMsg
	_, _ = m.Update(messages.FileDeletedMsg{Path: filePath})

	if _, ok := m.openBuffers[filePath]; ok {
		t.Error("expected tab to be closed after FileDeletedMsg")
	}
}

func TestApp_DirDeletedMsg_ClosesTabsUnderDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	file1 := filepath.Join(subDir, "a.go")
	file2 := filepath.Join(subDir, "b.go")
	for _, f := range []string{file1, file2} {
		if err := os.WriteFile(f, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := newTestApp(t, dir)

	// Open both files
	_, _ = m.Update(messages.FileSelectedMsg{Path: file1})
	_, _ = m.Update(messages.FileSelectedMsg{Path: file2})

	// Delete the parent directory
	_, _ = m.Update(messages.DirDeletedMsg{Path: subDir})

	// Both tabs should be gone
	if _, ok := m.openBuffers[file1]; ok {
		t.Errorf("expected file1 tab to be closed after DirDeletedMsg")
	}
	if _, ok := m.openBuffers[file2]; ok {
		t.Errorf("expected file2 tab to be closed after DirDeletedMsg")
	}
}

func TestApp_SaveConfigMsg_PersistsConfig(t *testing.T) {
	dir := t.TempDir()
	m := newTestApp(t, dir)

	// Point configPath to a temp file
	tmpCfg := filepath.Join(dir, "config.json")
	m.configPath = tmpCfg

	newCfg := config.Defaults()
	newCfg.Sidebar.ConfirmDelete = false

	_, _ = m.Update(messages.SaveConfigMsg{Config: newCfg})

	// Config file should exist and contain "confirm_delete": false
	data, err := os.ReadFile(tmpCfg)
	if err != nil {
		t.Fatalf("expected config file to be written: %v", err)
	}
	if !strings.Contains(string(data), `"confirm_delete": false`) {
		t.Errorf("expected confirm_delete=false in saved config, got: %s", data)
	}
}
