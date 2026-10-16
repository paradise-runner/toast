package syntax

import (
	"embed"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	tree_sitter_markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"

	"github.com/yourusername/toast/internal/syntax/grammars/r"
)

//go:embed queries/*.scm
var queriesFS embed.FS

type LangDef struct {
	Name     string
	Language *sitter.Language
	Query    []byte
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
		{Name: "java", Language: java.GetLanguage()},
		{Name: "r", Language: r.GetLanguage()},
		{Name: "markdown", Language: tree_sitter_markdown.GetLanguage()},
		// JSON uses a custom scanner instead of tree-sitter (nil Language).
		{Name: "json"},
	}

	langByName = make(map[string]*LangDef, len(defs))
	for _, d := range defs {
		if q, err := queriesFS.ReadFile("queries/" + d.Name + ".scm"); err == nil {
			d.Query = q
		}
		langByName[d.Name] = d
	}

	// Extensions are declared as lowercase suffixes because filepath.Ext is
	// lowercased before lookup, so ".R" and ".Rprofile" resolve here too.
	// Languages are named rather than indexed so adding a grammar cannot
	// silently shift the extension mapping.
	langByExt = make(map[string]*LangDef)
	for ext, name := range map[string]string{
		".go": "go",
		".py": "python",
		".js": "javascript", ".mjs": "javascript",
		".ts": "typescript", ".tsx": "typescript",
		".rs":   "rust",
		".css":  "css",
		".html": "html", ".htm": "html",
		".yaml": "yaml", ".yml": "yaml",
		".sh": "bash", ".bash": "bash",
		".hcl": "hcl", ".tf": "hcl", ".tfvars": "hcl",
		".java": "java",
		".r":    "r", ".rprofile": "r", ".renviron": "r",
		".md": "markdown", ".markdown": "markdown",
		".json": "json", ".jsonc": "json",
	} {
		if def := langByName[name]; def != nil {
			langByExt[ext] = def
		}
	}
}

func ForPath(path string) *LangDef { return langByExt[strings.ToLower(filepath.Ext(path))] }
func ForName(name string) *LangDef { return langByName[strings.ToLower(name)] }
