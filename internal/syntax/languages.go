package syntax

import (
	"embed"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/javascript"
	tree_sitter_markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"
)

//go:embed queries/*.scm
var queriesFS embed.FS

type LangDef struct {
	Name     string
	Language *sitter.Language
	Query    []byte
	// Keywords gates "@keyword" captures. Grammars that expose keywords as
	// anonymous nodes (or as parse rules) leave this nil. When set, a span
	// captured as @keyword is kept only if its source text is in the set;
	// this lets a query capture a broad node type such as (identifier) and
	// still highlight only the real keywords.
	Keywords map[string]bool
}

var langByExt map[string]*LangDef
var langByName map[string]*LangDef

func init() {
	defs := []*LangDef{
		{Name: "go", Language: golang.GetLanguage()},
		{Name: "python", Language: python.GetLanguage()},
		{Name: "javascript", Language: javascript.GetLanguage()},
		{Name: "typescript", Language: typescript.GetLanguage()},
		{Name: "rust", Language: rust.GetLanguage()},
		{Name: "css", Language: css.GetLanguage()},
		{Name: "html", Language: html.GetLanguage()},
		{Name: "yaml", Language: yaml.GetLanguage()},
		{Name: "bash", Language: bash.GetLanguage()},
		{Name: "hcl", Language: hcl.GetLanguage()},
		{Name: "markdown", Language: tree_sitter_markdown.GetLanguage()},
		{
			Name:     "elixir",
			Language: elixir.GetLanguage(),
			Keywords: keywordSet(elixirKeywords...),
		},
		// JSON uses a custom scanner instead of tree-sitter (nil Language).
		{Name: "json"},
	}
	for _, d := range defs {
		q, err := queriesFS.ReadFile("queries/" + d.Name + ".scm")
		if err == nil {
			d.Query = q
		}
	}
	langByName = make(map[string]*LangDef, len(defs))
	for _, d := range defs {
		langByName[d.Name] = d
	}
	// Key the extension table by language name so adding a language never
	// shifts the extensions of the ones registered before it.
	byName := func(name string) *LangDef { return langByName[name] }
	langByExt = map[string]*LangDef{
		".go": byName("go"),
		".py": byName("python"),
		".js": byName("javascript"), ".mjs": byName("javascript"),
		".ts": byName("typescript"), ".tsx": byName("typescript"),
		".rs":   byName("rust"),
		".css":  byName("css"),
		".html": byName("html"), ".htm": byName("html"),
		".yaml": byName("yaml"), ".yml": byName("yaml"),
		".sh": byName("bash"), ".bash": byName("bash"),
		".hcl": byName("hcl"), ".tf": byName("hcl"), ".tfvars": byName("hcl"),
		".md": byName("markdown"), ".markdown": byName("markdown"),
		".ex": byName("elixir"), ".exs": byName("elixir"),
		".json": byName("json"), ".jsonc": byName("json"),
	}
}

// elixirKeywords gates "@keyword" captures. It holds both the keywords the
// vendored grammar parses as plain identifiers (call targets) and the ones it
// exposes as anonymous tokens, so every @keyword span is one of these words.
// See queries/elixir.scm.
var elixirKeywords = []string{
	"alias", "case", "cond", "def", "defdelegate", "defexception",
	"defguard", "defguardp", "defimpl", "defmacro", "defmacrop",
	"defmodule", "defp", "defprotocol", "defstruct", "for", "if",
	"import", "quote", "raise", "receive", "require", "reraise",
	"super", "throw", "try", "unless", "unquote", "use", "with",
	// Anonymous keyword tokens: captured by the query's token list.
	"do", "end", "fn", "when", "and", "or", "not", "in", "after",
	"else", "rescue", "catch",
}

func keywordSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, word := range words {
		set[word] = true
	}
	return set
}

func ForPath(path string) *LangDef { return langByExt[strings.ToLower(filepath.Ext(path))] }
func ForName(name string) *LangDef { return langByName[strings.ToLower(name)] }
