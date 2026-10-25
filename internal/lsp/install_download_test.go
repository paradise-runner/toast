package lsp

import (
	"archive/tar"
	"archive/zip"
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
	if info.Mode()&0o111 == 0 {
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

// TestInstallFromDownloadRawBinary covers the Next LS strategy: the release
// asset is a plain executable named per platform, installed as a fixed name.
func TestInstallFromDownloadRawBinary(t *testing.T) {
	fakeBinary := "#!/bin/sh\necho fake-next-ls\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/next_ls_" + runtime.GOOS + "_" + runtime.GOARCH + exeSuffix()
		if r.URL.Path != want {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(fakeBinary))
	}))
	defer srv.Close()

	cfg := config.Defaults()
	cfg.LSP = map[string]config.LSPCmd{
		"elixir": {
			Command:        "nextls",
			Extensions:     []string{".ex", ".exs"},
			ManagedCommand: "{install_dir}/bin/nextls{exe}",
			Install: &config.LSPInstall{
				Name: "Next LS",
				Download: &config.LSPDownload{
					URL:    srv.URL + "/next_ls_{os}_{arch}{exe}",
					Format: "raw",
					Binary: "nextls{exe}",
				},
			},
		},
	}

	sendCh := make(chan tea.Msg, 16)
	m := NewManager(cfg, t.TempDir(), func(msg tea.Msg) { sendCh <- msg })
	m.installRoot = t.TempDir()

	go m.Install("elixir")
	waitForInstallSuccess(t, sendCh)

	bin := filepath.Join(m.installDir("elixir"), "bin", "nextls"+exeSuffix())
	output, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("binary not installed at %s: %v", bin, err)
	}
	if !strings.Contains(string(output), "fake-next-ls") {
		t.Fatalf("unexpected binary content: %q", output)
	}
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("binary not executable: %v", info.Mode())
	}
}

// waitForInstallSuccess drains install statuses until success or failure.
func waitForInstallSuccess(t *testing.T, sendCh <-chan tea.Msg) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case msg := <-sendCh:
			status, ok := msg.(messages.LSPInstallStatusMsg)
			if !ok {
				continue
			}
			switch status.Status {
			case messages.LSPInstallSucceeded:
				return
			case messages.LSPInstallFailed:
				t.Fatalf("install failed: %s", status.Message)
			}
		case <-deadline:
			t.Fatal("timed out waiting for install success")
		}
	}
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// TestExtractServerExecutable covers the formats a language server release can
// ship in, plus rejection of formats nobody has taught the manager yet.
func TestExtractServerExecutable(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("elixir-ls/language_server.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("zip-binary")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		payload []byte
		format  string
		want    string
		wantErr bool
	}{
		{name: "raw", payload: []byte("raw-binary"), format: "raw", want: "raw-binary"},
		{name: "zip", payload: zipBuf.Bytes(), format: "zip", want: "zip-binary"},
		{name: "empty format means tar.gz", payload: tarball(t, "server", "tar-binary"), want: "tar-binary"},
		{name: "unknown format", payload: []byte("x"), format: "rar", wantErr: true},
		{name: "raw without payload", payload: nil, format: "raw", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractServerExecutable(tc.payload, tc.format)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got.data) != tc.want {
				t.Fatalf("data = %q, want %q", got.data, tc.want)
			}
		})
	}
}

func TestExpandPlatformPlaceholders(t *testing.T) {
	m := NewManager(config.Defaults(), t.TempDir(), func(tea.Msg) {})
	got := m.expand("elixir", "next_ls_{os}_{arch}{exe}")
	want := "next_ls_" + runtime.GOOS + "_" + runtime.GOARCH + exeSuffix()
	if got != want {
		t.Fatalf("expand = %q, want %q", got, want)
	}
}

// tarball builds a .tar.gz with a single executable inside a wrapping dir.
func tarball(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "dir/" + name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
