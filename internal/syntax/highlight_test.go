package syntax_test

import (
	"testing"

	"github.com/yourusername/toast/internal/syntax"
	"github.com/yourusername/toast/internal/theme"
)

func TestHighlightGoCode(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("test.go", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	h.Parse([]byte(src))
	spans := h.HighlightLine(0, "package main\n")
	if len(spans) == 0 {
		t.Error("expected highlight spans, got none")
	}
}

func TestHighlightUnknownLang(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("test.xyz", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil highlighter")
	}
}
