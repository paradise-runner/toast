//go:build !cgo

package syntax

import (
	"path/filepath"
	"strings"
)

// LangDef describes a recognized source language. Query is retained in the
// no-cgo API for compatibility, but is empty because tree-sitter is disabled.
type LangDef struct {
	Name  string
	Query []byte
}

var langByExt map[string]*LangDef
var langByName map[string]*LangDef

func init() {
	names := []string{
		"go", "python", "javascript", "typescript", "rust", "css", "html",
		"yaml", "bash", "hcl", "markdown", "json",
	}
	defs := make([]*LangDef, len(names))
	langByName = make(map[string]*LangDef, len(names))
	for i, name := range names {
		defs[i] = &LangDef{Name: name}
		langByName[name] = defs[i]
	}

	langByExt = map[string]*LangDef{
		".go": defs[0], ".py": defs[1], ".js": defs[2], ".mjs": defs[2],
		".ts": defs[3], ".tsx": defs[3], ".rs": defs[4], ".css": defs[5],
		".html": defs[6], ".htm": defs[6], ".yaml": defs[7], ".yml": defs[7],
		".sh": defs[8], ".bash": defs[8],
		".hcl": defs[9], ".tf": defs[9], ".tfvars": defs[9],
		".md": defs[10], ".markdown": defs[10],
		".json": defs[11], ".jsonc": defs[11],
	}
}

func ForPath(path string) *LangDef { return langByExt[strings.ToLower(filepath.Ext(path))] }
func ForName(name string) *LangDef { return langByName[strings.ToLower(name)] }
