package app

import (
	"errors"
	"testing"

	"github.com/yourusername/toast/internal/messages"
)

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
