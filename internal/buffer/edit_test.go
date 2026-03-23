package buffer

import "testing"

func TestUndoSingleInsert(t *testing.T) {
	b := NewEditBuffer("hello")
	b.Insert(5, " world")
	if got := b.String(); got != "hello world" {
		t.Fatalf("after insert: got %q, want %q", got, "hello world")
	}
	b.Undo()
	if got := b.String(); got != "hello" {
		t.Fatalf("after undo: got %q, want %q", got, "hello")
	}
}

func TestUndoSingleDelete(t *testing.T) {
	b := NewEditBuffer("hello world")
	b.Delete(5, 11)
	if got := b.String(); got != "hello" {
		t.Fatalf("after delete: got %q, want %q", got, "hello")
	}
	b.Undo()
	if got := b.String(); got != "hello world" {
		t.Fatalf("after undo: got %q, want %q", got, "hello world")
	}
}

func TestRedoAfterUndo(t *testing.T) {
	b := NewEditBuffer("hello")
	b.Insert(5, " world")
	b.Undo()
	if got := b.String(); got != "hello" {
		t.Fatalf("after undo: got %q, want %q", got, "hello")
	}
	b.Redo()
	if got := b.String(); got != "hello world" {
		t.Fatalf("after redo: got %q, want %q", got, "hello world")
	}
}

func TestUndoMultiple(t *testing.T) {
	b := NewEditBuffer("")
	b.Insert(0, "a")
	b.Insert(1, "b")
	b.Insert(2, "c")
	if got := b.String(); got != "abc" {
		t.Fatalf("after inserts: got %q, want %q", got, "abc")
	}
	b.Undo()
	if got := b.String(); got != "ab" {
		t.Fatalf("after undo 1: got %q, want %q", got, "ab")
	}
	b.Undo()
	if got := b.String(); got != "a" {
		t.Fatalf("after undo 2: got %q, want %q", got, "a")
	}
	b.Undo()
	if got := b.String(); got != "" {
		t.Fatalf("after undo 3: got %q, want %q", got, "")
	}
}

func TestUndoNothingToUndo(t *testing.T) {
	b := NewEditBuffer("hello")
	// Should not panic when nothing to undo.
	b.Undo()
	if got := b.String(); got != "hello" {
		t.Fatalf("after empty undo: got %q, want %q", got, "hello")
	}
}

func TestRedoInvalidatedByNewEdit(t *testing.T) {
	b := NewEditBuffer("hello")
	b.Insert(5, " world")
	b.Undo()
	// New edit should invalidate the redo stack.
	b.Insert(5, "!")
	b.Redo() // Should be a no-op now.
	if got := b.String(); got != "hello!" {
		t.Fatalf("after new edit + redo: got %q, want %q", got, "hello!")
	}
}

func TestModifiedFlag(t *testing.T) {
	b := NewEditBuffer("hello")
	if b.Modified() {
		t.Fatal("new buffer should not be modified")
	}
	b.Insert(5, " world")
	if !b.Modified() {
		t.Fatal("buffer should be modified after insert")
	}
	b.MarkSaved()
	if b.Modified() {
		t.Fatal("buffer should not be modified after MarkSaved")
	}
	b.Undo()
	if !b.Modified() {
		t.Fatal("buffer should be modified after undo past save point")
	}
}

func TestModifiedFlag_EditAndRevertWithoutUndo(t *testing.T) {
	b := NewEditBuffer("hello")
	// Insert a character then delete it — content returns to saved state
	// via edits, not undo. Modified() must report false.
	b.Insert(5, "x")
	b.Delete(5, 6)
	if b.Modified() {
		t.Fatal("buffer content matches saved content; Modified() should be false")
	}
}
