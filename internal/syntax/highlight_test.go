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

func TestHighlightTerraformHCL(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("main.tf", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	t.Logf("HasQuery: %v", h.HasQuery())
	lang := syntax.ForPath("main.tf")
	if lang == nil {
		t.Fatal("ForPath returned nil for .tf")
	}
	t.Logf("lang.Name=%s, len(Query)=%d", lang.Name, len(lang.Query))
	if !h.HasQuery() {
		t.Fatal("expected a highlight query for .tf files")
	}

	src := "resource \"aws_instance\" \"example\" {\n  ami = \"ami-abc123\"\n}\n"
	h.Parse([]byte(src))

	// Line 0: resource "aws_instance" "example" {
	spans := h.HighlightLine(0, src[:45])
	t.Logf("Line 0 spans: %+v", spans)
	if len(spans) == 0 {
		t.Error("expected highlight spans for resource block header")
	}
	foundKeyword := false
	for _, s := range spans {
		if s.Style == "keyword" {
			foundKeyword = true
			break
		}
	}
	if !foundKeyword {
		t.Error("expected 'resource' to be highlighted as keyword")
	}

	// Line 1:   ami = "ami-abc123"
	spans2 := h.HighlightLine(1, "  ami = \"ami-abc123\"\n")
	t.Logf("Line 1 spans: %+v", spans2)
	if len(spans2) == 0 {
		t.Error("expected highlight spans for attribute line")
	}
}

func TestHighlightTerraformExtensions(t *testing.T) {
	tests := []struct {
		path string
	}{
		{"main.tf"},
		{"variables.tfvars"},
		{"config.hcl"},
	}
	tm, _ := theme.NewManager("toast-dark", "")
	for _, tc := range tests {
		h, err := syntax.NewHighlighter(tc.path, tm)
		if err != nil {
			t.Fatalf("NewHighlighter(%q) error: %v", tc.path, err)
		}
		if !h.HasQuery() {
			t.Errorf("expected HasQuery for %s", tc.path)
		}
	}
}
