// Package r binds the tree-sitter R grammar (r-lib/tree-sitter-r, MIT) to the
// go-tree-sitter runtime already used by internal/syntax.
//
// The grammar has no binding for github.com/smacker/go-tree-sitter, so the C
// sources are vendored here and linked against that runtime:
//
//   - parser.c, scanner.c: generated grammar sources, commit pinned below.
//     Only their include lines were rewritten ("tree_sitter/parser.h" ->
//     "parser.h", "tree_sitter/alloc.h" -> "alloc.h") to match the layout the
//     smacker runtime's own grammar packages use.
//   - parser.h, alloc.h: copies of the smacker runtime headers. The generated
//     code is compiled against these so the TSLanguage/TSLexer layout is
//     exactly what the runtime expects. tree-sitter-r's own copy of parser.h
//     adds a trailing `log` field to TSLexer; the scanner never dereferences
//     it, so the older shape is safe and avoids an ABI mismatch.
//
// To upgrade: fetch parser.c and scanner.c from
//
//	https://github.com/r-lib/tree-sitter-r/tree/main/src
//
// verify the generated LANGUAGE_VERSION is still <= the runtime's
// TREE_SITTER_LANGUAGE_VERSION (14), redo the include rewrite, and run
// `go test ./internal/syntax/`.
//
// Upstream: https://github.com/r-lib/tree-sitter-r
// Pinned:   main @ 2026-06-22 (parser LANGUAGE_VERSION 14)
//
// Licensing: parser.c and scanner.c are MIT (see LICENSE in this directory);
// parser.h and alloc.h come from the MIT-licensed tree-sitter runtime.
package r

//#include "parser.h"
//TSLanguage *tree_sitter_r();
import "C"
import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

// GetLanguage returns the R grammar for the go-tree-sitter runtime.
func GetLanguage() *sitter.Language {
	return sitter.NewLanguage(unsafe.Pointer(C.tree_sitter_r()))
}
