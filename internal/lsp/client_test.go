package lsp_test

import (
	"testing"

	"github.com/yourusername/toast/internal/lsp"
)

func TestPathToURIRoundtrip(t *testing.T) {
	path := "/Users/bob/project/main.go"
	uri := lsp.URIFromPath(path)
	if uri != "file:///Users/bob/project/main.go" {
		t.Errorf("got %q", uri)
	}
	back := lsp.PathFromURI(uri)
	if back != path {
		t.Errorf("got %q", back)
	}
}
