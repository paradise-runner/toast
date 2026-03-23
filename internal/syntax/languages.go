package syntax

import (
	"embed"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/golang"
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
		{Name: "markdown", Language: tree_sitter_markdown.GetLanguage()},
	}
	for _, d := range defs {
		q, err := queriesFS.ReadFile("queries/" + d.Name + ".scm")
		if err == nil {
			d.Query = q
		}
	}
	langByExt = map[string]*LangDef{
		".go": defs[0], ".py": defs[1], ".js": defs[2], ".mjs": defs[2],
		".ts": defs[3], ".tsx": defs[3], ".rs": defs[4], ".css": defs[5],
		".html": defs[6], ".htm": defs[6], ".yaml": defs[7], ".yml": defs[7],
		".sh": defs[8], ".bash": defs[8],
		".md": defs[9], ".markdown": defs[9],
	}
	langByName = make(map[string]*LangDef, len(defs))
	for _, d := range defs {
		langByName[d.Name] = d
	}
}

func ForPath(path string) *LangDef { return langByExt[strings.ToLower(filepath.Ext(path))] }
func ForName(name string) *LangDef { return langByName[strings.ToLower(name)] }
