//go:build !cgo

package syntax_test

import (
	"testing"

	"github.com/yourusername/toast/internal/syntax"
	"github.com/yourusername/toast/internal/theme"
)

func TestHighlighterWithoutCgoFallsBackGracefully(t *testing.T) {
	if syntax.TreeSitterAvailable() {
		t.Fatal("tree-sitter must be unavailable in a !cgo build")
	}

	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("main.go", tm)
	if err != nil {
		t.Fatalf("NewHighlighter() error = %v", err)
	}
	h.Parse([]byte("package main\n"))
	h.Edit([]byte("package toast\n"), 8, 12, 13, 0, 8, 0, 12, 0, 13)
	if h.HasQuery() {
		t.Fatal("non-JSON highlighter unexpectedly has a query without cgo")
	}
	if spans := h.HighlightLine(0, "package toast\n"); spans != nil {
		t.Fatalf("HighlightLine() = %#v, want nil fallback", spans)
	}
}
