# Word-Boundary Line Wrapping Design

**Date:** 2026-03-30
**Status:** Approved
**Builds on:** `docs/superpowers/specs/2026-03-30-markdown-linewrap-design.md`

## Overview

Upgrade the existing character-boundary soft wrap (for markdown files) to wrap at word boundaries. The change is confined to `internal/components/editor/wrap.go` — a single new pure function replaces the arithmetic that determines chunk positions.

## Scope

- Wraps at the last ASCII space (0x20) before the column limit
- Falls back to character-boundary break if no space exists within a chunk
- Navigation, viewport, mouse, and selection logic are unchanged — they already consume `visualRowsForLine` and `bufPosFromVisualRow`
- Non-markdown files are unaffected

## New Function

```go
// wordWrapChunks returns the byte offsets of the start of each visual chunk
// when line is broken at word boundaries with the given column width.
// Each chunk starts at a non-space character. If no space exists within a
// chunk, the line is broken at the column boundary (character fallback).
func wordWrapChunks(line string, width int) []int
```

### Algorithm

```
chunks = [0]
start  = 0

while start + width < len(line):
    end = start + width
    // Scan backward for last ASCII space within this chunk.
    sp = last index of ' ' in line[start:end]
    if sp found:
        next = start + sp + 1   // skip the space
    else:
        next = end              // character-boundary fallback
    chunks = append(chunks, next)
    start  = next

return chunks
```

`wordWrapChunks` is a standalone (non-method) function so it can be unit-tested without constructing a model.

## Call Site Changes (wrap.go only)

| Function | Before | After |
|---|---|---|
| `visualRowsForLine` | `(n + w - 1) / w` | `len(wordWrapChunks(raw, w))` |
| `bufPosFromVisualRow` | `chunkIndex * w` | `wordWrapChunks(raw, w)[chunkIndex]` |
| View render loop (chunk slicing) | `chunkStart = chunkIndex * w` | `chunkStart = chunks[chunkIndex]`; `chunkEnd = chunks[chunkIndex+1]` or `len(raw)` |

The View render loop calls `wordWrapChunks` once per visual row rendered, which is fine for typical markdown file sizes.

## Non-Goals

- Unicode whitespace (tabs, non-breaking spaces) as break points
- Hyphenation
- Per-line word-wrap toggle
- Caching chunk positions across renders
