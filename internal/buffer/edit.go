package buffer

// edit records a single reversible change to the buffer.
type edit struct {
	offset   int
	deleted  string // text that was removed at offset
	inserted string // text that was inserted at offset
}

// EditBuffer wraps a Rope with undo/redo history and a modified flag.
type EditBuffer struct {
	rope         *Rope
	undoStack    []edit
	redoStack    []edit
	savedGen     int
	gen          int
	savedContent string
}

// NewEditBuffer creates a new EditBuffer with the given initial content.
func NewEditBuffer(content string) *EditBuffer {
	return &EditBuffer{
		rope:         NewRope(content),
		savedContent: content,
	}
}

// String returns the full content of the buffer.
func (b *EditBuffer) String() string { return b.rope.String() }

// Len returns the byte length of the buffer content.
func (b *EditBuffer) Len() int { return b.rope.Len() }

// LineCount returns the number of lines in the buffer.
func (b *EditBuffer) LineCount() int { return b.rope.LineCount() }

// LineAt returns the full text of the given zero-based line.
func (b *EditBuffer) LineAt(line int) string { return b.rope.LineAt(line) }

// OffsetForLine returns the byte offset of the start of the given zero-based line.
func (b *EditBuffer) OffsetForLine(line int) int { return b.rope.OffsetForLine(line) }

// LineColForOffset returns the zero-based line and column for a given byte offset.
func (b *EditBuffer) LineColForOffset(offset int) (line, col int) {
	return b.rope.LineColForOffset(offset)
}

// Slice returns the substring of the buffer in the range [start, end).
func (b *EditBuffer) Slice(start, end int) string { return b.rope.Slice(start, end) }

// Insert inserts text at the given byte offset, recording the operation for undo.
func (b *EditBuffer) Insert(offset int, text string) {
	b.rope.Insert(offset, text)
	b.undoStack = append(b.undoStack, edit{
		offset:   offset,
		deleted:  "",
		inserted: text,
	})
	b.redoStack = b.redoStack[:0]
	b.gen++
}

// Delete removes bytes in the range [start, end), recording the operation for undo.
func (b *EditBuffer) Delete(start, end int) {
	deleted := b.rope.Slice(start, end)
	b.rope.Delete(start, end)
	b.undoStack = append(b.undoStack, edit{
		offset:   start,
		deleted:  deleted,
		inserted: "",
	})
	b.redoStack = b.redoStack[:0]
	b.gen++
}

// Replace replaces bytes in [start, end) with text, recorded as a single undo entry.
func (b *EditBuffer) Replace(start, end int, text string) {
	deleted := b.rope.Slice(start, end)
	b.rope.Delete(start, end)
	b.rope.Insert(start, text)
	b.undoStack = append(b.undoStack, edit{
		offset:   start,
		deleted:  deleted,
		inserted: text,
	})
	b.redoStack = b.redoStack[:0]
	b.gen++
}

// Undo reverses the most recent edit. If the undo stack is empty, it is a no-op.
func (b *EditBuffer) Undo() {
	if len(b.undoStack) == 0 {
		return
	}
	e := b.undoStack[len(b.undoStack)-1]
	b.undoStack = b.undoStack[:len(b.undoStack)-1]

	// Inverse: remove what was inserted, then re-insert what was deleted.
	if len(e.inserted) > 0 {
		b.rope.Delete(e.offset, e.offset+len(e.inserted))
	}
	if len(e.deleted) > 0 {
		b.rope.Insert(e.offset, e.deleted)
	}

	b.redoStack = append(b.redoStack, e)
	b.gen--
}

// Redo re-applies the most recently undone edit. If the redo stack is empty, it is a no-op.
func (b *EditBuffer) Redo() {
	if len(b.redoStack) == 0 {
		return
	}
	e := b.redoStack[len(b.redoStack)-1]
	b.redoStack = b.redoStack[:len(b.redoStack)-1]

	// Re-apply: remove deleted text, then insert the originally inserted text.
	if len(e.deleted) > 0 {
		b.rope.Delete(e.offset, e.offset+len(e.deleted))
	}
	if len(e.inserted) > 0 {
		b.rope.Insert(e.offset, e.inserted)
	}

	b.undoStack = append(b.undoStack, e)
	b.gen++
}

// Modified reports whether the buffer has been changed since the last MarkSaved call.
// The fast path (gen == savedGen) avoids a string allocation when no edits have occurred.
// When gens differ, a content comparison catches the case where edits cancel each other out.
func (b *EditBuffer) Modified() bool {
	if b.gen == b.savedGen {
		return false
	}
	return b.rope.String() != b.savedContent
}

// MarkSaved records the current state as the saved state, clearing the modified flag.
func (b *EditBuffer) MarkSaved() {
	b.savedGen = b.gen
	b.savedContent = b.rope.String()
}
