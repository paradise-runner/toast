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

func TestHighlightJSON(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("test.json", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !h.HasQuery() {
		t.Fatal("expected HasQuery for .json")
	}

	src := `{
  "name": "Alice",
  "age": 30,
  "admin": true,
  "data": null,
  "scores": [9.5, 10.0],
  "address": {
    "city": "NYC",
    "zip": 10001
  }
}`
	h.Parse([]byte(src))

	// Line 0: {
	spans := h.HighlightLine(0, "{")
	t.Logf("Line 0 spans: %+v", spans)
	if len(spans) == 0 {
		t.Error("expected highlight span for '{'")
	} else if spans[0].Style != "punctuation" {
		t.Errorf("expected '{' to be punctuation, got %q", spans[0].Style)
	}

	// Line 1:   "name": "Alice",
	line1 := "  \"name\": \"Alice\","
	spans = h.HighlightLine(1, line1)
	t.Logf("Line 1 spans: %+v", spans)
	// Expected spans: property("name"), punctuation(:), string("Alice"), punctuation(,)
	if len(spans) < 3 {
		t.Errorf("expected at least 3 spans for key-value pair, got %d", len(spans))
	}
	// First span should be the key (property)
	if len(spans) > 0 && spans[0].Style != "property" {
		t.Errorf("expected key 'name' to be property, got %q", spans[0].Style)
	}
	// The string value should be present
	foundString := false
	for _, s := range spans {
		if s.Style == "string" {
			foundString = true
			break
		}
	}
	if !foundString {
		t.Error("expected string highlight for 'Alice'")
	}

	// Line 2:   "age": 30,
	line2 := "  \"age\": 30,"
	spans = h.HighlightLine(2, line2)
	t.Logf("Line 2 spans: %+v", spans)
	foundNumber := false
	for _, s := range spans {
		if s.Style == "number" {
			foundNumber = true
			break
		}
	}
	if !foundNumber {
		t.Error("expected number highlight for 30")
	}

	// Line 3:   "admin": true,
	line3 := "  \"admin\": true,"
	spans = h.HighlightLine(3, line3)
	t.Logf("Line 3 spans: %+v", spans)
	foundConstant := false
	for _, s := range spans {
		if s.Style == "constant" {
			foundConstant = true
			break
		}
	}
	if !foundConstant {
		t.Error("expected constant highlight for true")
	}

	// Line 7 (nested object):     "city": "NYC"
	line7 := "    \"city\": \"NYC\""
	spans = h.HighlightLine(7, line7)
	t.Logf("Line 7 spans: %+v", spans)
	if len(spans) > 0 && spans[0].Style != "property" {
		t.Errorf("expected nested key 'city' to be property, got %q", spans[0].Style)
	}
}

func TestHighlightJSONC(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("test.jsonc", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !h.HasQuery() {
		t.Fatal("expected HasQuery for .jsonc")
	}

	src := `{
  // This is a comment
  "key": "value"
}`
	h.Parse([]byte(src))

	// Line 1:   // This is a comment
	spans := h.HighlightLine(1, "  // This is a comment")
	t.Logf("JSONC Line 1 spans: %+v", spans)
	if len(spans) == 0 {
		t.Error("expected comment highlight for .jsonc")
	} else if spans[0].Style != "comment" {
		t.Errorf("expected style 'comment', got %q", spans[0].Style)
	}
}

func TestHighlightJSONArray(t *testing.T) {
	tm, _ := theme.NewManager("toast-dark", "")
	h, err := syntax.NewHighlighter("data.json", tm)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	src := `["a", "b", true, null, 42]`
	h.Parse([]byte(src))

	spans := h.HighlightLine(0, src)
	t.Logf("Array spans: %+v", spans)
	if len(spans) == 0 {
		t.Fatal("expected highlight spans for array")
	}

	// Strings should NOT be marked as property in an array.
	for _, s := range spans {
		if s.Style == "property" {
			t.Errorf("array elements should not be 'property', got span %+v", s)
		}
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
