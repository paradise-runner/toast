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

func TestPathToURIRoundtripWithSpaces(t *testing.T) {
	path := "/Users/bob/My Project/main.go"
	uri := lsp.URIFromPath(path)
	if uri != "file:///Users/bob/My%20Project/main.go" {
		t.Fatalf("got %q", uri)
	}
	if back := lsp.PathFromURI(uri); back != path {
		t.Fatalf("roundtrip got %q", back)
	}
}
