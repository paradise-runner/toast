# Markdown Line Wrapping Design

**Date:** 2026-03-30
**Status:** Approved

## Overview

Add soft line wrapping for markdown files (`.md`, `.markdown`) in the toast editor. Wrapping is purely a rendering and navigation concern — the buffer model, editing operations, and cursor representation (`bufLine`, `bufCol` as byte offset) are unchanged. Files remain fully editable while wrapped.

## Scope

- Wrap mode activates when a file with extension `.md` or `.markdown` is opened
- Wrap mode is off for all other file types (behavior unchanged)
- Wrapping is character-boundary (byte-boundary), not word-boundary — word wrap can be layered on later
- Horizontal scroll is disabled in wrap mode (`viewportLeft` forced to 0)

## Model Changes

Add two fields to `editor.Model`:

```go
wrapMode    bool // true when the open file is markdown
```

Set in `openFile` (or equivalent) based on `filepath.Ext(path)`.

## Visual Row Abstraction

A "visual row" is one screen line. In wrap mode, one buffer line may occupy multiple visual rows. All three helpers are O(n) in line count — acceptable for typical file sizes.

### `visualRowsForLine(bufLine int) int`
Returns how many screen rows buffer line `bufLine` occupies.

```
max(1, ceil(byteLen(line) / contentWidth))
```

Empty lines return 1.

### `visualRowOfCursor() int`
Returns the absolute 0-based visual row index from the top of the buffer for the current cursor position.

```
sum(visualRowsForLine(l) for l in [0, cursor.line)) + cursor.col / contentWidth
```

### `bufPosFromVisualRow(visualRow int) (bufLine, bufCol int)`
Inverse mapping: walks buffer lines accumulating visual row counts until `visualRow` is reached. Returns the buffer line and the byte offset of the start of that visual chunk:

```
bufCol = chunkIndex * contentWidth
```

where `chunkIndex` is which chunk within the line contains `visualRow`.

### `visualRowFromTop(bufLine int) int`
Returns the absolute visual row index of the first visual row of `bufLine`. Used in `clampViewport`.

## Viewport Management

`clampViewport` in wrap mode:

1. Compute `cursorVR = visualRowOfCursor()`
2. Compute `topVR = visualRowFromTop(viewportTop)`
3. If `cursorVR < topVR`: walk buffer lines backward from `viewportTop` until `topVR <= cursorVR` — set new `viewportTop`
4. If `cursorVR >= topVR + viewHeight`: walk buffer lines forward from `viewportTop` until cursor is within `[0, viewHeight)` — set new `viewportTop`
5. Force `viewportLeft = 0`

## View Rendering

The render loop builds a flat list of `(bufLine, chunkIndex)` pairs starting from `viewportTop`, filling `viewHeight` screen rows.

For each visual row:
- **Gutter**: line number shown only on `chunkIndex == 0`; continuation rows show blank space of the same width
- **Content**: `line[chunkIndex*contentWidth : min((chunkIndex+1)*contentWidth, len(line))]`
- **Syntax highlighting**: spans from `HighlightLine` are clamped to the chunk's byte range and offset-adjusted by `chunkIndex * contentWidth`
- **Selection**: existing per-line `selRange` logic already uses byte offsets — clamp to chunk range as with highlighting
- **Cursor**: `cursorScreenX = gutterWidth + (cursor.col - chunkStart)` where `chunkStart = chunkIndex * contentWidth`

## Navigation

### Up/Down arrows (wrap mode only)

Replace the current `cursor.line ± 1` with:

```
targetVR = visualRowOfCursor() ± 1   (clamped to [0, totalVisualRows))
bufLine, bufCol = bufPosFromVisualRow(targetVR)
cursor = {line: bufLine, col: clampColToLine(bufCol + preferredVisualCol)}
```

`preferredCol` stores the **visual column** (0-based within content area). On left/right movement it is reset to `cursor.col % contentWidth`.

### Left/Right, Home/End, Page Up/Down

Unchanged. Home/End operate on buffer-line boundaries (not visual-row boundaries).

### Mouse clicks

```
visualRow = visualRowFromTop(viewportTop) + screenRow
bufLine, chunkStart = bufPosFromVisualRow(visualRow)
bufCol  = chunkStart + max(0, screenCol - gutterWidth)
```

Clamped to line byte length.

## Non-Goals

- Word-boundary wrapping (future)
- Per-line wrap toggle
- Rendered markdown preview
- Configurable wrap column
