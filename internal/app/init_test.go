package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

func TestNew_WithInitialFile_InitReturnsFileSelectedCmd(t *testing.T) {
	cfg := config.Defaults()
	model, err := New(cfg, "", t.TempDir(), "/some/file.go")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a non-nil Cmd when initialFile is set")
	}

	// Init now returns a tea.Batch; execute it and look for FileSelectedMsg
	// among the batch results.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}

	var found bool
	for _, inner := range batch {
		if inner == nil {
			continue
		}
		if fileMsg, ok := inner().(messages.FileSelectedMsg); ok {
			if fileMsg.Path != "/some/file.go" {
				t.Fatalf("expected path %q, got %q", "/some/file.go", fileMsg.Path)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("FileSelectedMsg not found in batch")
	}
}

func TestNew_WithoutInitialFile_InitReturnGitStatusCmd(t *testing.T) {
	cfg := config.Defaults()
	model, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a non-nil Cmd for git status")
	}
}
