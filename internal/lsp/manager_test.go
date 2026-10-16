package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestLanguageForPathConfigDetectsJava(t *testing.T) {
	if got := LanguageForPathConfig("/tmp/proj/src/main/java/Main.java", config.Defaults().LSP); got != "java" {
		t.Fatalf("LanguageForPathConfig(Main.java) = %q, want java", got)
	}
}

func TestLanguageForPathConfigDetectsR(t *testing.T) {
	configured := config.Defaults().LSP
	for _, path := range []string{
		"/tmp/proj/analysis.R",
		"/tmp/proj/analysis.r",
		"/tmp/proj/.Rprofile",
		"/tmp/proj/.Renviron",
		"/tmp/proj/report.Rmd",
		"/tmp/proj/report.qmd",
	} {
		if got := LanguageForPathConfig(path, configured); got != "r" {
			t.Errorf("LanguageForPathConfig(%q) = %q, want r", path, got)
		}
	}
}

// languageserver decides literate mode from the file extension, so every R
// extension keeps the default "r" language id.
func TestLanguageIDForRFiles(t *testing.T) {
	cmd := config.Defaults().LSP["r"]
	for _, path := range []string{"analysis.R", ".Rprofile", "report.Rmd", "report.qmd"} {
		if got := languageIDForPath(path, "r", cmd); got != "r" {
			t.Errorf("languageIDForPath(%q) = %q, want r", path, got)
		}
	}
}

func TestProjectRootFindsNearestBuildMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "module", "src", "main", "java")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, ok := projectRoot(nested, []string{"settings.gradle", "pom.xml"})
	if !ok || got != root {
		t.Fatalf("projectRoot = %q ok=%v, want %q", got, ok, root)
	}
	if _, ok := projectRoot(nested, []string{"pom.xml"}); ok {
		t.Fatal("expected no root when no marker is present")
	}
}

func TestRootDirForUsesLanguageMarkers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "src", "Main.java")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(config.Defaults(), t.TempDir(), func(tea.Msg) {})
	if got := m.rootDirFor("java", file); got != root {
		t.Fatalf("rootDirFor(java) = %q, want build root %q", got, root)
	}
	// Languages without RootMarkers keep the workspace root.
	if got := m.rootDirFor("go", file); got != m.rootDir {
		t.Fatalf("rootDirFor(go) = %q, want workspace root %q", got, m.rootDir)
	}
}

func TestEnsureServerReportsMissingRequirement(t *testing.T) {
	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"test": {
			Command:  "sh",
			Requires: []string{"toast-requirement-that-does-not-exist"},
		},
	}
	var sent []tea.Msg
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sent = append(sent, msg) })
	m.installRoot = t.TempDir()

	m.EnsureServer("test", "")

	if len(sent) != 1 {
		t.Fatalf("expected one status message, got %d: %#v", len(sent), sent)
	}
	status, ok := sent[0].(messages.LSPServerStatusMsg)
	if !ok || status.Status != messages.LSPServerNotFound {
		t.Fatalf("expected not-found status, got %#v", sent[0])
	}
	if !strings.Contains(status.Message, "toast-requirement-that-does-not-exist") {
		t.Fatalf("message should name the missing requirement, got %q", status.Message)
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

func TestLanguageForPathDetectsMarkdown(t *testing.T) {
	for _, path := range []string{"README.md", "notes.markdown", "docs/page.mdx", "notes.txt", "/tmp/x/README.MD"} {
		if got := LanguageForPath(path); got != "markdown" {
			t.Fatalf("LanguageForPath(%q) = %q, want markdown", path, got)
		}
	}
}

func TestLanguageIDForMarkdownAndPlainText(t *testing.T) {
	if got := languageIDForPath("README.md", "markdown", config.LSPCmd{}); got != "markdown" {
		t.Fatalf("md language id = %q", got)
	}
	if got := languageIDForPath("docs/page.mdx", "markdown", config.LSPCmd{}); got != "markdown" {
		t.Fatalf("mdx language id = %q", got)
	}
	// harper-ls parses .txt with its plain-text parser, not the Markdown one.
	if got := languageIDForPath("notes.txt", "markdown", config.LSPCmd{}); got != "plaintext" {
		t.Fatalf("txt language id = %q, want plaintext", got)
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

	m.EnsureServer("test", "")
	m.EnsureServer("test", "")

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

func TestEnsureServerPromptsWhenProbeFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"test": {
			// The command resolves, but the probe reports that the server is
			// not actually installed (mirrors Rscript without the R package).
			Command: "sh",
			Probe:   &config.LSPProbe{Command: "sh", Args: []string{"-c", "exit 1"}},
			Install: &config.LSPInstall{Name: "Test LS", Command: "missing-installer"},
		},
	}
	var sent []tea.Msg
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sent = append(sent, msg) })
	m.installRoot = t.TempDir()

	m.EnsureServer("test", "")
	m.EnsureServer("test", "")

	if len(sent) != 1 {
		t.Fatalf("expected one prompt, got %d messages: %#v", len(sent), sent)
	}
	prompt, ok := sent[0].(messages.LSPInstallPromptMsg)
	if !ok {
		t.Fatalf("expected LSPInstallPromptMsg, got %T", sent[0])
	}
	if prompt.Language != "test" || prompt.Name != "Test LS" {
		t.Fatalf("unexpected prompt: %#v", prompt)
	}
}

func TestProbeAvailable(t *testing.T) {
	m := NewManager(config.Defaults(), t.TempDir(), func(tea.Msg) {})

	// No probe configured: the resolved command is proof enough.
	if !m.probeAvailable("test", config.LSPCmd{}) {
		t.Error("a nil probe should be available")
	}
	if !m.probeAvailable("test", config.LSPCmd{Probe: &config.LSPProbe{Command: "sh", Args: []string{"-c", "exit 0"}}}) {
		t.Error("a probe exiting 0 should be available")
	}
	if m.probeAvailable("test", config.LSPCmd{Probe: &config.LSPProbe{Command: "sh", Args: []string{"-c", "exit 7"}}}) {
		t.Error("a probe exiting non-zero should not be available")
	}
	if m.probeAvailable("test", config.LSPCmd{Probe: &config.LSPProbe{Command: "toast-probe-that-does-not-exist"}}) {
		t.Error("a probe whose command is missing should not be available")
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

func TestInstallCapsVerboseFailureOutput(t *testing.T) {
	// An installer that floods stdout with a megabyte of noise before failing.
	installer := filepath.Join(t.TempDir(), "noisy-installer")
	if err := os.WriteFile(installer, []byte("#!/bin/sh\nhead -c 1048576 /dev/zero | tr '\\0' 'x'\necho FAILED >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"test": {
			Command: "missing-server",
			Install: &config.LSPInstall{Name: "Test LS", Command: installer},
		},
	}
	var sent []tea.Msg
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sent = append(sent, msg) })
	m.installRoot = t.TempDir()

	m.Install("test")

	if len(sent) != 2 {
		t.Fatalf("expected running and failed statuses, got %d messages", len(sent))
	}
	status, ok := sent[1].(messages.LSPInstallStatusMsg)
	if !ok || status.Status != messages.LSPInstallFailed {
		t.Fatalf("expected failed status, got %#v", sent[1])
	}
	if len(status.Message) > maxInstallOutput+100 {
		t.Fatalf("failure message not capped: %d bytes", len(status.Message))
	}
	if !strings.Contains(status.Message, "FAILED") {
		t.Fatalf("failure message should keep the meaningful tail, got %q", status.Message)
	}
}

func TestExpandTargetTriple(t *testing.T) {
	m := NewManager(config.Defaults(), t.TempDir(), func(tea.Msg) {})
	triple := rustTargetTriple()
	if triple == "" {
		t.Skip("platform has no prebuilt harper binary")
	}
	if got := m.expand("markdown", "https://example.com/harper-ls-{target}.tar.gz"); got != "https://example.com/harper-ls-"+triple+".tar.gz" {
		t.Fatalf("expand({target}) = %q, want triple %q", got, triple)
	}
}
