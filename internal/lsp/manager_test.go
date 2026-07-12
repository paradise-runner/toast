package lsp

import (
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

func TestParseDefinitionLocationVariants(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		path string
		line int
	}{
		{"location", `{"uri":"file:///tmp/one.go","range":{"start":{"line":2,"character":3},"end":{"line":2,"character":6}}}`, "/tmp/one.go", 2},
		{"locations", `[{"uri":"file:///tmp/two.go","range":{"start":{"line":4,"character":1},"end":{"line":4,"character":2}}}]`, "/tmp/two.go", 4},
		{"location links", `[{"targetUri":"file:///tmp/three.go","targetRange":{"start":{"line":5,"character":0},"end":{"line":8,"character":0}},"targetSelectionRange":{"start":{"line":6,"character":2},"end":{"line":6,"character":7}}}]`, "/tmp/three.go", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location, ok := parseDefinitionLocation(json.RawMessage(tt.raw))
			if !ok || PathFromURI(location.URI) != tt.path || location.Range.Start.Line != tt.line {
				t.Fatalf("unexpected location: %#v ok=%v", location, ok)
			}
		})
	}
	if _, ok := parseDefinitionLocation(json.RawMessage("null")); ok {
		t.Fatal("expected null definition result to be unavailable")
	}
}

func TestLanguageForPathConfigSupportsCustomLanguage(t *testing.T) {
	configured := map[string]config.LSPCmd{
		"zig": {Command: "zls", Extensions: []string{".zig", "build.zig"}},
	}
	for _, path := range []string{"main.zig", "/tmp/project/build.zig"} {
		if got := LanguageForPathConfig(path, configured); got != "zig" {
			t.Fatalf("LanguageForPathConfig(%q) = %q, want zig", path, got)
		}
	}
}

func TestPositionColumnConversions(t *testing.T) {
	line := "a😀éz"
	tests := []struct {
		byteCol int
		utf16   int
	}{
		{0, 0}, {1, 1}, {5, 3}, {7, 4}, {8, 5},
	}
	for _, tt := range tests {
		if got := byteColToUTF16(line, tt.byteCol); got != tt.utf16 {
			t.Fatalf("byteColToUTF16(%d) = %d, want %d", tt.byteCol, got, tt.utf16)
		}
	}
}

func TestLanguageIDForReactFiles(t *testing.T) {
	if got := languageIDForPath("view.tsx", "typescript", config.LSPCmd{}); got != "typescriptreact" {
		t.Fatalf("tsx language id = %q", got)
	}
	if got := languageIDForPath("view.jsx", "javascript", config.LSPCmd{}); got != "javascriptreact" {
		t.Fatalf("jsx language id = %q", got)
	}
	if got := languageIDForPath("view.tsx", "custom", config.LSPCmd{LanguageID: "override"}); got != "override" {
		t.Fatalf("configured language id = %q", got)
	}
}

func TestEnsureServerPromptsForConfiguredManagedInstall(t *testing.T) {
	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"test": {
			Command: "toast-language-server-that-does-not-exist",
			Install: &config.LSPInstall{Name: "Test LS", Command: "missing-installer"},
		},
	}
	var sent []tea.Msg
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sent = append(sent, msg) })
	m.installRoot = t.TempDir()

	m.EnsureServer("test")
	m.EnsureServer("test")

	if len(sent) != 1 {
		t.Fatalf("expected one prompt, got %d messages", len(sent))
	}
	prompt, ok := sent[0].(messages.LSPInstallPromptMsg)
	if !ok {
		t.Fatalf("expected LSPInstallPromptMsg, got %T", sent[0])
	}
	if prompt.Language != "test" || prompt.Name != "Test LS" {
		t.Fatalf("unexpected prompt: %#v", prompt)
	}
}

func TestInstallReportsMissingInstaller(t *testing.T) {
	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"test": {
			Command: "missing-server",
			Install: &config.LSPInstall{Name: "Test LS", Command: "toast-installer-that-does-not-exist"},
		},
	}
	var sent []tea.Msg
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sent = append(sent, msg) })
	m.installRoot = t.TempDir()

	m.Install("test")

	if len(sent) != 2 {
		t.Fatalf("expected running and failed statuses, got %d messages", len(sent))
	}
	if status, ok := sent[0].(messages.LSPInstallStatusMsg); !ok || status.Status != messages.LSPInstallRunning {
		t.Fatalf("expected running status, got %#v", sent[0])
	}
	if status, ok := sent[1].(messages.LSPInstallStatusMsg); !ok || status.Status != messages.LSPInstallFailed {
		t.Fatalf("expected failed status, got %#v", sent[1])
	}
}

func TestDidChangeTracksEveryFullDocumentVersion(t *testing.T) {
	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{"test": {Command: "missing-server"}}
	m := NewManager(cfg, t.TempDir(), func(tea.Msg) {})

	m.DidOpen("/tmp/main.test", "test", "one")
	m.DidChange("/tmp/main.test", "test", "two")
	m.DidChange("/tmp/main.test", "test", "three")

	m.mu.Lock()
	doc := m.documents["/tmp/main.test"]
	m.mu.Unlock()
	if doc.version != 3 || doc.text != "three" {
		t.Fatalf("unexpected tracked document: version=%d text=%q", doc.version, doc.text)
	}
}
