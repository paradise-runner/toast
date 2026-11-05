//go:build windows

package lsp_test

import (
	"path/filepath"
	"testing"

	"github.com/yourusername/toast/internal/lsp"
)

func TestWindowsPathToURIRoundtrip(t *testing.T) {
	path := `C:\Users\bob\My Project\main.go`
	uri := lsp.URIFromPath(path)
	if uri != "file:///C:/Users/bob/My%20Project/main.go" {
		t.Fatalf("URIFromPath() = %q", uri)
	}
	if back := lsp.PathFromURI(uri); back != filepath.Clean(path) {
		t.Fatalf("PathFromURI() = %q, want %q", back, filepath.Clean(path))
	}
}
