package syntax

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

// TestEveryQueryCompiles guards the silent failure mode in NewHighlighter: a
// broken .scm file only logs to stderr and leaves the language unhighlighted.
// JSON has no tree-sitter grammar (custom scanner), so it is skipped.
func TestEveryQueryCompiles(t *testing.T) {
	for name, lang := range langByName {
		if lang.Language == nil {
			continue
		}
		if len(lang.Query) == 0 {
			t.Errorf("%s: no embedded highlight query", name)
			continue
		}
		q, err := sitter.NewQuery(lang.Query, lang.Language)
		if err != nil {
			t.Errorf("%s: query does not compile: %v", name, err)
			continue
		}
		q.Close()
	}
}

// TestEveryLanguageIsReachable fails when a grammar is registered but no
// extension maps to it, or an extension points at an unregistered language.
func TestEveryLanguageIsReachable(t *testing.T) {
	for ext, lang := range langByExt {
		if lang == nil {
			t.Errorf("%s: extension maps to a nil language", ext)
			continue
		}
		if langByName[lang.Name] != lang {
			t.Errorf("%s: extension maps to unregistered language %q", ext, lang.Name)
		}
	}
	for path, want := range map[string]string{
		"lib/app.ex": "elixir", "mix.exs": "elixir", "main.go": "go",
	} {
		if lang := ForPath(path); lang == nil || lang.Name != want {
			t.Errorf("ForPath(%q) = %v, want %s", path, lang, want)
		}
	}
}
