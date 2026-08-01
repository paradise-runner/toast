package lspinstall

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

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

func TestInstallPromptSpinsWhileInstalling(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	m := New(tm)
	m.Show("go", "gopls")

	// Accept the install; running status starts the spinner and schedules ticks.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'i'})
	updated, cmd := updated.Update(messages.LSPInstallStatusMsg{Language: "go", Status: messages.LSPInstallRunning})
	if cmd == nil {
		t.Fatal("expected a spinner tick while installing")
	}
	if _, ok := cmd().(messages.LSPInstallTickMsg); !ok {
		t.Fatalf("expected LSPInstallTickMsg, got %T", cmd())
	}
	if !strings.Contains(updated.Render(), "Installing") {
		t.Fatalf("expected Installing status, got: %q", updated.Render())
	}

	// Ticks keep the spinner cycling while the install is running.
	frame := updated.spinner
	for i := 0; i < 2*len(spinnerFrames); i++ {
		updated, cmd = updated.Update(messages.LSPInstallTickMsg{})
		if cmd == nil {
			t.Fatal("expected ticks to continue while installing")
		}
	}
	if updated.spinner != frame {
		t.Fatalf("spinner frame %d, want %d after a full cycle", updated.spinner, frame)
	}

	// Success stops the spinner and drops the Installing status.
	updated, cmd = updated.Update(messages.LSPInstallStatusMsg{Language: "go", Status: messages.LSPInstallSucceeded})
	if cmd != nil {
		t.Fatal("expected ticks to stop after success")
	}
	if strings.Contains(updated.Render(), "Installing") {
		t.Fatalf("Installing status should be gone, got: %q", updated.Render())
	}
}

func TestInstallPromptSpinnerStopsOnFailure(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	m := New(tm)
	m.Show("go", "gopls")

	updated, _ := m.Update(messages.LSPInstallStatusMsg{Language: "go", Status: messages.LSPInstallRunning})
	if _, cmd := updated.Update(messages.LSPInstallTickMsg{}); cmd == nil {
		t.Fatal("expected tick while installing")
	}
	updated, cmd := updated.Update(messages.LSPInstallStatusMsg{Language: "go", Status: messages.LSPInstallFailed, Message: "boom"})
	if cmd != nil {
		t.Fatal("expected ticks to stop after failure")
	}
	if !updated.failed || !strings.Contains(updated.Render(), "boom") {
		t.Fatalf("expected failure message, got: %q", updated.Render())
	}
}

func TestPadTruncatesHugeMessageWithoutFreezing(t *testing.T) {
	// Regression: installer output (megabytes) once reached the status line and
	// pad() truncated it rune-by-rune with an O(n²) string rebuild, freezing
	// the UI goroutine for minutes.
	huge := strings.Repeat("x", 13*1024*1024)
	start := time.Now()
	got := pad(huge, 42)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("pad of 13MB took %v; UI would freeze", elapsed)
	}
	if lipgloss.Width(got) != 42 {
		t.Fatalf("padded width = %d, want 42", lipgloss.Width(got))
	}
	if !strings.HasSuffix(got, "…") || strings.TrimSuffix(got, "…") != strings.Repeat("x", 41) {
		t.Fatalf("unexpected truncation: %q", got)
	}
}

func TestPadKeepsShortStrings(t *testing.T) {
	if got := pad("hello", 10); got != "hello     " {
		t.Fatalf("pad(hello, 10) = %q", got)
	}
	// Wide runes are counted by display width, not bytes.
	if got := pad("😀😀", 4); got != "😀😀" {
		t.Fatalf("pad(😀😀, 4) = %q", got)
	}
	if got := pad("😀😀", 3); got != "😀…" {
		t.Fatalf("pad(😀😀, 3) = %q", got)
	}
}
