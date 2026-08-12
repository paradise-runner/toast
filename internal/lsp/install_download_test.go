package lsp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

// TestInstallFromDownloadInstall runs the prebuilt-binary install path end to
// end: the manager downloads a fake release tarball from a local HTTP server,
// extracts the executable into {install_dir}/bin, and reports success. This
// is the install strategy used for harper-ls.
func TestInstallFromDownloadInstall(t *testing.T) {
	fakeBinary := "#!/bin/sh\necho fake-harper-ls\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/harper-ls-") || !strings.HasSuffix(r.URL.Path, ".tar.gz") {
			http.NotFound(w, r)
			return
		}
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		if err := tw.WriteHeader(&tar.Header{Name: "harper-ls", Mode: 0o755, Size: int64(len(fakeBinary))}); err != nil {
			t.Errorf("tar header: %v", err)
			return
		}
		if _, err := tw.Write([]byte(fakeBinary)); err != nil {
			t.Errorf("tar body: %v", err)
			return
		}
		if err := tw.Close(); err != nil {
			t.Errorf("tar close: %v", err)
			return
		}
		if err := gz.Close(); err != nil {
			t.Errorf("gzip close: %v", err)
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"markdown": {
			Command:        "harper-ls",
			Args:           []string{"--stdio"},
			Extensions:     []string{".md"},
			ManagedCommand: "{install_dir}/bin/harper-ls",
			Install: &config.LSPInstall{
				Name:     "harper-ls",
				Download: &config.LSPDownload{URL: srv.URL + "/harper-ls-{target}.tar.gz"},
			},
		},
	}

	sendCh := make(chan tea.Msg, 16)
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sendCh <- msg })
	m.installRoot = t.TempDir()

	// Install runs the server startup after success; drive it on a background
	// goroutine and assert on the install statuses only.
	go m.Install("markdown")

	var sawRunning, sawSucceeded bool
	deadline := time.After(10 * time.Second)
	for !sawSucceeded {
		select {
		case msg := <-sendCh:
			switch status := msg.(type) {
			case messages.LSPInstallStatusMsg:
				switch status.Status {
				case messages.LSPInstallRunning:
					sawRunning = true
				case messages.LSPInstallSucceeded:
					sawSucceeded = true
				case messages.LSPInstallFailed:
					t.Fatalf("install failed: %s", status.Message)
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for install success")
		}
	}
	if !sawRunning {
		t.Fatal("expected an install running status first")
	}

	bin := filepath.Join(m.installDir("markdown"), "bin", "harper-ls")
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("binary not installed at %s: %v", bin, err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Fatalf("binary not executable: %v", info.Mode())
	}
	output, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "fake-harper-ls") {
		t.Fatalf("unexpected binary content: %q", output)
	}
}

// TestPickExecutable verifies archive layout handling: a file at the root is
// preferred and a single wrapping directory is stripped.
func TestPickExecutable(t *testing.T) {
	rootFile := downloadEntry{name: "harper-ls", depth: 0, data: []byte("root")}
	wrapped := downloadEntry{name: "harper-ls", depth: 1, data: []byte("wrapped")}

	got, err := pickExecutable([]downloadEntry{rootFile})
	if err != nil || got.name != "harper-ls" {
		t.Fatalf("root file: %#v, %v", got, err)
	}
	got, err = pickExecutable([]downloadEntry{wrapped})
	if err != nil || got.name != "harper-ls" {
		t.Fatalf("wrapped file: %#v, %v", got, err)
	}
	if _, err := pickExecutable([]downloadEntry{}); err == nil {
		t.Fatal("expected error for empty archive")
	}
	if _, err := pickExecutable([]downloadEntry{
		{name: "a", depth: 0}, {name: "b", depth: 0},
	}); err == nil {
		t.Fatal("expected error for multiple root executables")
	}
}
