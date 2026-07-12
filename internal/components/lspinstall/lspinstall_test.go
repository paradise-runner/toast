package lspinstall

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func TestInstallPromptAcceptsAndClosesWhenServerIsReady(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	m := New(tm)
	m.Show("go", "gopls")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'i'})
	if cmd == nil {
		t.Fatal("expected install request")
	}
	request, ok := cmd().(messages.LSPInstallRequestMsg)
	if !ok || request.Language != "go" {
		t.Fatalf("unexpected install request: %#v", request)
	}

	updated, _ = updated.Update(messages.LSPServerStatusMsg{Language: "go", Status: messages.LSPServerReady})
	if updated.Visible() {
		t.Fatal("expected prompt to close when the server is ready")
	}
}

func TestInstallPromptQueuesLanguages(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	m := New(tm)
	m.Show("go", "gopls")
	m.Show("rust", "rust-analyzer")

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'n'})
	if !updated.Visible() || updated.language != "rust" {
		t.Fatalf("expected queued rust prompt, got visible=%v language=%q", updated.Visible(), updated.language)
	}
}
