package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

// TestApp_OpenMarkdownPromptsForHarperInstall reproduces the stale-config
// regression: a config saved by an older toast snapshots the built-in servers
// that existed at the time and omits later ones (markdown/harper). Opening a
// Markdown file must still discover the managed harper-ls default and prompt
// for its opt-in install.
func TestApp_OpenMarkdownPromptsForHarperInstall(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(mdPath, []byte("# Hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.json")
	stale := `{"lsp": {"go": {"command": "gopls", "args": ["serve"]}}}`
	if err := os.WriteFile(cfgPath, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if _, ok := cfg.LSP["markdown"]; !ok {
		t.Fatal("stale config should have gained the managed markdown default")
	}
	// Make the test hermetic: whatever this machine has installed (e.g. a
	// real harper-ls in the user's install root) must not satisfy the server
	// lookup, or no prompt would be emitted.
	markdown := cfg.LSP["markdown"]
	markdown.Command = "toast-test-nonexistent-harper-ls"
	markdown.ManagedCommand = "{install_dir}/toast-test-nonexistent-harper-ls"
	cfg.LSP["markdown"] = markdown

	m, err := New(cfg, "", dir, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	var sentCh = make(chan tea.Msg, 16)
	m.SetLSPSend(func(msg tea.Msg) { sentCh <- msg })

	_, cmd := m.Update(messages.FileSelectedMsg{Path: mdPath})
	runAppCmd(t, m, cmd, 10)

	// The manager starts servers asynchronously; wait for its prompt.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-sentCh:
			p, ok := msg.(messages.LSPInstallPromptMsg)
			if !ok {
				continue
			}
			if p.Language != "markdown" || p.Name != "harper-ls" {
				t.Fatalf("unexpected install prompt: %#v", p)
			}
			// The app must surface the prompt overlay.
			_, _ = m.Update(p)
			if !m.lspInstall.Visible() {
				t.Fatal("install prompt received but overlay not shown")
			}
			return
		case <-deadline:
			t.Fatal("no harper install prompt within 5s")
		}
	}
}
