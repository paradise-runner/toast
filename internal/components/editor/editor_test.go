package editor

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/buffer"
	"github.com/yourusername/toast/internal/clipboard"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/syntax"
	"github.com/yourusername/toast/internal/theme"
)

// newTestModel builds a minimal editor model with the given content.
func newTestModel(content string) Model {
	m := New(nil, config.Config{})
	m.buf = buffer.NewEditBuffer(content)
	return m
}

// newTestModelWithPath builds a test model with a buffer ID and path set.
// Used by Task 3 save tests.
func newTestModelWithPath(content, path string, bufferID int) Model {
	m := newTestModel(content)
	m.path = path
	m.bufferID = bufferID
	return m
}

func TestSave_EmitsFileSavingMsg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	m := newTestModelWithPath("hello\n", path, 1)
	cmd := m.save()
	if cmd == nil {
		t.Fatal("save() returned nil cmd")
	}

	// Execute the batch and collect all messages.
	// tea.Batch returns a single Cmd; we need to run it.
	// For a batch, the returned Cmd wraps multiple Cmds — run it directly.
	msg := cmd()

	// The first message from a Batch is a tea.BatchMsg ([]tea.Cmd).
	batchMsg, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}
	if len(batchMsg) != 2 {
		t.Fatalf("expected exactly 2 cmds in batch, got %d", len(batchMsg))
	}

	// The first cmd in the batch should synchronously return FileSavingMsg.
	savingMsg := batchMsg[0]()
	if _, ok := savingMsg.(messages.FileSavingMsg); !ok {
		t.Fatalf("expected FileSavingMsg as first cmd result, got %T", savingMsg)
	}
	fsmsg := savingMsg.(messages.FileSavingMsg)
	if fsmsg.BufferID != 1 {
		t.Fatalf("FileSavingMsg.BufferID = %d, want 1", fsmsg.BufferID)
	}
	if fsmsg.Path != path {
		t.Fatalf("FileSavingMsg.Path = %q, want %q", fsmsg.Path, path)
	}

	// The second cmd is the async write; run it and expect FileSavedMsg.
	savedMsg := batchMsg[1]()
	if fsm, ok := savedMsg.(messages.FileSavedMsg); !ok {
		t.Fatalf("expected FileSavedMsg as second cmd result, got %T", savedMsg)
	} else if fsm.Path != path {
		t.Fatalf("FileSavedMsg.Path = %q, want %q", fsm.Path, path)
	}
}

func TestSelectionRange_NoSelection(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	_, _, active := m.selectionRange()
	if active {
		t.Fatal("expected no active selection")
	}
}

func TestSelectionRange_ForwardSingleLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 4}
	start, end, active := m.selectionRange()
	if !active {
		t.Fatal("expected active selection")
	}
	if start != (cursorPos{0, 1}) || end != (cursorPos{0, 4}) {
		t.Fatalf("unexpected range: %v %v", start, end)
	}
}

func TestSelectionRange_BackwardSingleLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 4}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 1}
	start, end, active := m.selectionRange()
	if !active {
		t.Fatal("expected active selection")
	}
	if start != (cursorPos{0, 1}) || end != (cursorPos{0, 4}) {
		t.Fatalf("unexpected range: %v %v", start, end)
	}
}

func TestSelectionRange_MultiLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 3}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 1, col: 2}
	start, end, active := m.selectionRange()
	if !active {
		t.Fatal("expected active selection")
	}
	if start != (cursorPos{0, 3}) || end != (cursorPos{1, 2}) {
		t.Fatalf("unexpected range: %v %v", start, end)
	}
}

func TestSelectedText_SingleLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 4}
	got := m.selectedText()
	if got != "ell" {
		t.Fatalf("expected 'ell', got %q", got)
	}
}

func TestSelectedText_MultiLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 3}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 1, col: 2}
	got := m.selectedText()
	if got != "lo\nwo" {
		t.Fatalf("expected 'lo\\nwo', got %q", got)
	}
}

func TestDeleteSelection_SingleLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 4}
	deleted := m.deleteSelection()
	if deleted != "ell" {
		t.Fatalf("expected deleted='ell', got %q", deleted)
	}
	if m.selectionAnchor != nil {
		t.Fatal("expected anchor cleared after delete")
	}
	if m.cursor != (cursorPos{0, 1}) {
		t.Fatalf("expected cursor at {0,1}, got %v", m.cursor)
	}
	if m.buf.String() != "ho\nworld\n" {
		t.Fatalf("unexpected buffer: %q", m.buf.String())
	}
}

func TestDeleteSelection_MultiLine(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	anchor := cursorPos{line: 0, col: 3}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 1, col: 2}
	deleted := m.deleteSelection()
	if deleted != "lo\nwo" {
		t.Fatalf("expected 'lo\\nwo', got %q", deleted)
	}
	if m.buf.String() != "helrld\n" {
		t.Fatalf("unexpected buffer: %q", m.buf.String())
	}
	if m.cursor != (cursorPos{0, 3}) {
		t.Fatalf("expected cursor at {0,3}, got %v", m.cursor)
	}
}

func TestDeleteSelection_NoOp_WhenNoSelection(t *testing.T) {
	m := newTestModel("hello\n")
	deleted := m.deleteSelection()
	if deleted != "" {
		t.Fatalf("expected empty deleted, got %q", deleted)
	}
	if m.buf.String() != "hello\n" {
		t.Fatal("buffer should be unchanged")
	}
}

// ── Task 2: Keyboard selection tests ─────────────────────────────────────────

func TestShiftRight_SetsAnchorAndMovesCursor(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 0}) {
		t.Fatalf("expected anchor at {0,0}, got %v", *em.selectionAnchor)
	}
	if em.cursor != (cursorPos{0, 1}) {
		t.Fatalf("expected cursor at {0,1}, got %v", em.cursor)
	}
}

func TestShiftLeft_ExtendsSelectionBackward(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	m.cursor = cursorPos{0, 3}
	anchor := cursorPos{0, 3}
	m.selectionAnchor = &anchor
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 2}) {
		t.Fatalf("expected cursor at {0,2}, got %v", em.cursor)
	}
	if em.selectionAnchor == nil || *em.selectionAnchor != (cursorPos{0, 3}) {
		t.Fatalf("expected anchor at {0,3}, got %v", em.selectionAnchor)
	}
}

// ── Task 1: Option+Arrow word movement tests ──────────────────────────────────

func TestOptionLeft_MovesWordLeft(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 11} // end of "world"
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 6}) {
		t.Fatalf("expected cursor at {0,6} (start of 'world'), got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestOptionRight_MovesWordRight(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 5}) {
		t.Fatalf("expected cursor at {0,5} (byte offset past 'hello', before space), got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestCmdDown_EmptyBuffer_NoPanic(t *testing.T) {
	m := newTestModel("")
	m.focused = true
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModSuper})
	// Should not panic; no assertions needed beyond not panicking
}

func TestShiftCmdRight_ExtendsExistingSelection(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	// Start with anchor already set from a prior selection
	anchor := cursorPos{0, 3}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 5}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModSuper | tea.ModShift})
	em := m2.(Model)
	// Anchor should remain at original position, not reset
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor preserved")
	}
	if *em.selectionAnchor != (cursorPos{0, 3}) {
		t.Fatalf("expected anchor preserved at {0,3}, got %v", *em.selectionAnchor)
	}
	expectedCol := m.lineContentLen(0)
	if em.cursor != (cursorPos{0, expectedCol}) {
		t.Fatalf("expected cursor at {0,%d}, got %v", expectedCol, em.cursor)
	}
}

func TestShiftOptionLeft_SelectsWordLeft(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 11}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 11}) {
		t.Fatalf("expected anchor at {0,11}, got %v", *em.selectionAnchor)
	}
	if em.cursor != (cursorPos{0, 6}) {
		t.Fatalf("expected cursor at {0,6}, got %v", em.cursor)
	}
}

func TestShiftOptionRight_SelectsWordRight(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 0}) {
		t.Fatalf("expected anchor at {0,0}, got %v", *em.selectionAnchor)
	}
	if em.cursor != (cursorPos{0, 5}) {
		t.Fatalf("expected cursor at {0,5}, got %v", em.cursor)
	}
}

// ── Alt+b / Alt+f legacy fallback (Ghostty, macOS Terminal) ────────────────────

func TestAltB_MovesWordLeft(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 11} // end of "world"
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModAlt})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 6}) {
		t.Fatalf("expected cursor at {0,6} (start of 'world'), got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestAltF_MovesWordRight(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModAlt})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 5}) {
		t.Fatalf("expected cursor at {0,5} (end of 'hello'), got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestCtrlA_JumpsToLineStart(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 7}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection after ctrl+a")
	}
}

func TestCtrlE_JumpsToLineEnd(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 3}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	em := m2.(Model)
	expectedCol := m.lineContentLen(0)
	if em.cursor != (cursorPos{0, expectedCol}) {
		t.Fatalf("expected cursor at {0,%d}, got %v", expectedCol, em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection after ctrl+e")
	}
}

// ── Task 2: Cmd+Arrow line/document jump tests ────────────────────────────────

func TestCmdLeft_JumpsToLineStart(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 7}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModSuper})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestCmdRight_JumpsToLineEnd(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModSuper})
	em := m2.(Model)
	expectedCol := m.lineContentLen(0)
	if em.cursor != (cursorPos{0, expectedCol}) {
		t.Fatalf("expected cursor at {0,%d}, got %v", expectedCol, em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestCmdUp_JumpsToDocStart(t *testing.T) {
	m := newTestModel("hello\nworld\nfoo\n")
	m.focused = true
	m.cursor = cursorPos{2, 3}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModSuper})
	em := m2.(Model)
	if em.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestCmdDown_JumpsToDocEnd(t *testing.T) {
	m := newTestModel("hello\nworld\nfoo\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModSuper})
	em := m2.(Model)
	lastLine := m.buf.LineCount() - 1
	expectedCol := m.lineContentLen(lastLine)
	if em.cursor != (cursorPos{lastLine, expectedCol}) {
		t.Fatalf("expected cursor at {%d,%d}, got %v", lastLine, expectedCol, em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection")
	}
}

func TestShiftCmdLeft_SelectsToLineStart(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 7}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModSuper | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 7}) {
		t.Fatalf("expected anchor at {0,7}, got %v", *em.selectionAnchor)
	}
	if em.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", em.cursor)
	}
}

func TestShiftCmdRight_SelectsToLineEnd(t *testing.T) {
	m := newTestModel("hello world\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModSuper | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 0}) {
		t.Fatalf("expected anchor at {0,0}, got %v", *em.selectionAnchor)
	}
	expectedCol := m.lineContentLen(0)
	if em.cursor != (cursorPos{0, expectedCol}) {
		t.Fatalf("expected cursor at {0,%d}, got %v", expectedCol, em.cursor)
	}
}

func TestShiftCmdUp_SelectsToDocStart(t *testing.T) {
	m := newTestModel("hello\nworld\nfoo\n")
	m.focused = true
	m.cursor = cursorPos{2, 3}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModSuper | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{2, 3}) {
		t.Fatalf("expected anchor at {2,3}, got %v", *em.selectionAnchor)
	}
	if em.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", em.cursor)
	}
}

func TestShiftCmdDown_SelectsToDocEnd(t *testing.T) {
	m := newTestModel("hello\nworld\nfoo\n")
	m.focused = true
	m.cursor = cursorPos{0, 0}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModSuper | tea.ModShift})
	em := m2.(Model)
	if em.selectionAnchor == nil {
		t.Fatal("expected anchor set")
	}
	if *em.selectionAnchor != (cursorPos{0, 0}) {
		t.Fatalf("expected anchor at {0,0}, got %v", *em.selectionAnchor)
	}
	lastLine := m.buf.LineCount() - 1
	expectedCol := m.lineContentLen(lastLine)
	if em.cursor != (cursorPos{lastLine, expectedCol}) {
		t.Fatalf("expected cursor at {%d,%d}, got %v", lastLine, expectedCol, em.cursor)
	}
}

func TestPlainRight_ClearsSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 0}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 2}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	em := m2.(Model)
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared on plain Right")
	}
}

func TestCtrlA_JumpsToLineStart_MidDocument(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	m.focused = true
	m.cursor = cursorPos{1, 3}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.cursor != (cursorPos{1, 0}) {
		t.Fatalf("expected cursor at {1,0}, got %v", em.cursor)
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected no selection after ctrl+a")
	}
}

// ── Task 3: Copy, cut, paste, delete-selection tests ─────────────────────────

func TestCtrlC_CopiesSelection(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	em := m2.(Model)
	// Selection stays active after copy (VS Code behavior)
	if em.selectionAnchor == nil {
		t.Fatal("expected selection to remain after copy")
	}
	// clipboard.Paste() reads the internal fallback
	got := clipboard.Paste()
	if got != "ell" {
		t.Fatalf("expected clipboard='ell', got %q", got)
	}
}

func TestCtrlX_CutsSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared after cut")
	}
	if em.buf.String() != "ho\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	got := clipboard.Paste()
	if got != "ell" {
		t.Fatalf("expected clipboard='ell', got %q", got)
	}
}

func TestCtrlV_PasteAtCursor(t *testing.T) {
	clipboard.Copy("XYZ")
	m := newTestModel("hello\n")
	m.focused = true
	m.cursor = cursorPos{0, 2}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.buf.String() != "heXYZllo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
}

func TestCtrlV_PasteReplacesSelection(t *testing.T) {
	clipboard.Copy("XYZ")
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.buf.String() != "hXYZo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared after paste")
	}
}

func TestBackspace_DeletesSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	em := m2.(Model)
	if em.buf.String() != "ho\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared")
	}
}

func TestDelete_DeletesSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	em := m2.(Model)
	if em.buf.String() != "ho\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
}

func TestTyping_ReplacesSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'X', Text: "X"})
	em := m2.(Model)
	if em.buf.String() != "hXo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared after typing")
	}
}

func TestCtrlV_DeleteSelection_ThenPaste(t *testing.T) {
	clipboard.Copy("AB")
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 3}
	// "el" selected — replace with "AB"
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	em := m2.(Model)
	if em.buf.String() != "hABlo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
}

// ── Task: Bracketed paste (tea.PasteMsg) ─────────────────────────────────────

func TestPasteMsg_InsertsAtCursor(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	m.cursor = cursorPos{0, 2}
	m2, _ := m.Update(tea.PasteMsg{Content: "XYZ"})
	em := m2.(Model)
	if em.buf.String() != "heXYZllo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.cursor.col != 5 {
		t.Fatalf("expected cursor col 5, got %d", em.cursor.col)
	}
}

func TestPasteMsg_ReplacesSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	anchor := cursorPos{0, 1}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{0, 4}
	m2, _ := m.Update(tea.PasteMsg{Content: "XYZ"})
	em := m2.(Model)
	if em.buf.String() != "hXYZo\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.selectionAnchor != nil {
		t.Fatal("expected selection cleared after paste")
	}
}

func TestPasteMsg_MultiLine(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	m.cursor = cursorPos{0, 5}
	m2, _ := m.Update(tea.PasteMsg{Content: "foo\nbar"})
	em := m2.(Model)
	if em.buf.String() != "hellofoo\nbar\n" {
		t.Fatalf("unexpected buffer: %q", em.buf.String())
	}
	if em.cursor.line != 1 {
		t.Fatalf("expected cursor on line 1, got %d", em.cursor.line)
	}
	if em.cursor.col != 3 {
		t.Fatalf("expected cursor col 3, got %d", em.cursor.col)
	}
}

func TestPasteMsg_EmitsModified(t *testing.T) {
	m := newTestModel("hello")
	m.bufferID = 7
	m.focused = true
	_, cmd := m.Update(tea.PasteMsg{Content: "world"})
	if cmd == nil {
		t.Fatal("expected cmd from PasteMsg")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 7 {
		t.Fatalf("expected BufferID 7, got %d", bm.BufferID)
	}
	if !bm.Modified {
		t.Fatal("expected Modified=true after PasteMsg")
	}
}

// ── Task 4: Mouse selection tests ────────────────────────────────────────────

func TestMouseDrag_SetsSelection(t *testing.T) {
	m := newTestModel("hello\nworld\n")
	m.focused = true
	m.viewHeight = 10
	m.viewWidth = 40
	m.gutterWidth = 4

	// Press at X=4, Y=0 → buffer col = viewportLeft + (4 - gutterWidth) = 0, line = 0
	press := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      4, Y: 0,
	}
	m2, _ := m.Update(press)
	em := m2.(Model)
	if !em.mouseDragging {
		t.Fatal("expected mouseDragging=true after press")
	}

	// Motion to X=8, Y=0 → buffer col 4
	motion := tea.MouseMotionMsg{
		Button: tea.MouseLeft,
		X:      8, Y: 0,
	}
	m3, _ := em.Update(motion)
	em2 := m3.(Model)
	if em2.selectionAnchor == nil {
		t.Fatal("expected selection anchor set during drag")
	}
	if em2.cursor.col != 4 {
		t.Fatalf("expected cursor col 4, got %d", em2.cursor.col)
	}
}

func TestMouseRelease_NoMove_ClearsSelection(t *testing.T) {
	m := newTestModel("hello\n")
	m.focused = true
	m.viewHeight = 10
	m.viewWidth = 40
	m.gutterWidth = 4

	press := tea.MouseClickMsg{Button: tea.MouseLeft, X: 4, Y: 0}
	m2, _ := m.Update(press)
	em := m2.(Model)

	release := tea.MouseReleaseMsg{Button: tea.MouseLeft, X: 4, Y: 0}
	m3, _ := em.Update(release)
	em2 := m3.(Model)
	if em2.selectionAnchor != nil {
		t.Fatal("expected no selection on click without drag")
	}
	if em2.mouseDragging {
		t.Fatal("expected mouseDragging=false after release")
	}
}

// ── Task 1: BufferModifiedMsg emission tests ──────────────────────────────────

func TestHandleKey_TypeCharEmitsModified(t *testing.T) {
	m := newTestModel("hello")
	m.bufferID = 1
	m.focused = true

	_, cmd := m.Update(tea.KeyPressMsg{Text: "x"})

	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T: %v", msg, msg)
	}
	if bm.BufferID != 1 {
		t.Fatalf("expected BufferID 1, got %d", bm.BufferID)
	}
	if !bm.Modified {
		t.Fatal("expected Modified=true after typing")
	}
}

func TestHandleKey_UndoEmitsModified(t *testing.T) {
	m := newTestModel("hello")
	m.bufferID = 2
	m.focused = true
	// Type a character to make buffer dirty, then mark saved, then undo.
	offset := m.cursorOffset()
	m.buf.Insert(offset, "x")
	m.buf.MarkSaved()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("expected a command after undo")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 2 {
		t.Fatalf("expected BufferID 2, got %d", bm.BufferID)
	}
	if !bm.Modified {
		t.Fatal("expected Modified=true after undo")
	}
}

func TestHandleKey_ArrowUpDoesNotEmitModified(t *testing.T) {
	m := newTestModel("hello\nworld")
	m.bufferID = 3
	m.focused = true
	m.cursor = cursorPos{line: 1, col: 0} // start on line 1 so Up actually moves

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.BufferModifiedMsg); ok {
			t.Fatal("arrow up should not emit BufferModifiedMsg")
		}
	}
}

func TestHandleKey_CmdPasteEmitsModified(t *testing.T) {
	m := newTestModel("hello")
	m.bufferID = 4
	m.focused = true
	clipboard.Copy("world")

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModSuper})
	if cmd == nil {
		t.Fatal("expected cmd from cmd+v")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 4 {
		t.Fatalf("expected BufferID 4, got %d", bm.BufferID)
	}
	if !bm.Modified {
		t.Fatal("expected Modified=true after cmd+v paste")
	}
}

func TestHandleKey_UndoToSavedStateEmitsModifiedFalse(t *testing.T) {
	m := newTestModel("")
	m.bufferID = 6
	m.focused = true

	// Type a character to dirty the buffer.
	m.buf.Insert(0, "x")
	// Mark it saved — now buffer is clean.
	m.buf.MarkSaved()
	// Type another character to dirty again (gen=2, savedGen=1).
	m.buf.Insert(1, "y")
	// Now undo once → back to gen=1 == savedGen → Modified=false

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected cmd after undo")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 6 {
		t.Fatalf("expected BufferID 6, got %d", bm.BufferID)
	}
	if bm.Modified {
		t.Fatal("expected Modified=false after undoing back to saved state")
	}
}

func TestHandleKey_RedoEmitsModified(t *testing.T) {
	m := newTestModel("")
	m.bufferID = 7
	m.focused = true

	// Insert, mark saved, insert again, undo (clean), then redo (dirty again).
	m.buf.Insert(0, "x")
	m.buf.MarkSaved()    // savedGen=1, clean
	m.buf.Insert(1, "y") // gen=2, dirty
	m.buf.Undo()         // gen=1, clean again (preModified for the redo test = false)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected cmd after redo")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 7 {
		t.Fatalf("expected BufferID 7, got %d", bm.BufferID)
	}
	if !bm.Modified {
		t.Fatal("expected Modified=true after redo")
	}
}

// TestEmitModified_AfterSave verifies emitModified returns Modified=false after MarkSaved.
func TestEmitModified_AfterSave(t *testing.T) {
	m := newTestModel("hello")
	m.bufferID = 4

	// Dirty the buffer.
	m.buf.Insert(0, "x")
	if !m.buf.Modified() {
		t.Fatal("buffer should be modified after insert")
	}

	// Simulate what save() does synchronously: mark saved.
	m.buf.MarkSaved()

	cmd := m.emitModified()
	if cmd == nil {
		t.Fatal("expected cmd from emitModified")
	}
	msg := cmd()
	bm, ok := msg.(messages.BufferModifiedMsg)
	if !ok {
		t.Fatalf("expected BufferModifiedMsg, got %T", msg)
	}
	if bm.BufferID != 4 {
		t.Fatalf("expected BufferID 4, got %d", bm.BufferID)
	}
	if bm.Modified {
		t.Fatal("expected Modified=false after MarkSaved")
	}
}

// newTestModelWithSyntax builds a test model with a real syntax highlighter.
func newTestModelWithSyntax(content, path string) Model {
	m := newTestModel(content)
	m.path = path
	tm, _ := theme.NewManager("toast-dark", "")
	h, _ := syntax.NewHighlighter(path, tm)
	if h != nil {
		h.Parse([]byte(content))
	}
	m.highlighter = h
	return m
}

// TestSave_EmitsFileSavedMsg verifies the save cmd emits FileSavedMsg on success.
func TestSave_EmitsFileSavedMsg(t *testing.T) {
	f, err := os.CreateTemp("", "toast-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	m := newTestModelWithPath("hello", f.Name(), 4)
	m.buf.Insert(0, "x")

	saveCmd := m.save()
	if saveCmd == nil {
		t.Fatal("expected saveCmd")
	}
	// save() now returns a tea.Batch; unwrap it.
	result := saveCmd()
	batchMsg, ok := result.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from save(), got %T", result)
	}
	if len(batchMsg) != 2 {
		t.Fatalf("expected exactly 2 cmds in batch, got %d", len(batchMsg))
	}
	// Second cmd is the async write.
	msg := batchMsg[1]()
	fs, ok := msg.(messages.FileSavedMsg)
	if !ok {
		t.Fatalf("expected FileSavedMsg, got %T", msg)
	}
	if fs.BufferID != 4 {
		t.Fatalf("expected BufferID 4, got %d", fs.BufferID)
	}
}

// ── Syntax re-parse after edits ──────────────────────────────────────────────

func TestReparseSyntax_AfterTyping(t *testing.T) {
	// Start with a Go file. "func" on line 0 should be highlighted as a keyword.
	src := "func main() {}\n"
	m := newTestModelWithSyntax(src, "main.go")
	m.focused = true
	if m.highlighter == nil {
		t.Skip("no highlighter available")
	}

	// Verify "func" is highlighted before editing.
	spans := m.highlighter.HighlightLine(0, src)
	hasFuncKeyword := false
	for _, s := range spans {
		if s.Style == "keyword" {
			hasFuncKeyword = true
			break
		}
	}
	if !hasFuncKeyword {
		t.Fatal("expected 'keyword' span on line 0 before edit")
	}

	// Type a character — this mutates the buffer.
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	em := m2.(Model)

	// The tree must reflect the new content "xfunc main() {}\n".
	if em.buf.String() != "xfunc main() {}\n" {
		t.Fatalf("unexpected buffer content: %q", em.buf.String())
	}
	// HighlightLine must not panic or use stale byte offsets.
	newLine := em.buf.LineAt(0)
	_ = em.highlighter.HighlightLine(0, newLine)
}

func TestReparseSyntax_AfterDelete(t *testing.T) {
	// Delete a character and verify the tree is updated (no stale-tree panic).
	src := "func main() {}\n"
	m := newTestModelWithSyntax(src, "main.go")
	m.focused = true
	m.cursor = cursorPos{0, 4} // after "func"
	if m.highlighter == nil {
		t.Skip("no highlighter available")
	}

	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	em := m2.(Model)

	newLine := em.buf.LineAt(0)
	_ = em.highlighter.HighlightLine(0, newLine) // must not panic or use stale offsets
}

func TestReparseSyntax_AfterPasteMsg(t *testing.T) {
	// Paste a string literal into a Go buffer and verify highlighting works.
	src := "package main\n"
	m := newTestModelWithSyntax(src, "main.go")
	m.focused = true
	m.cursor = cursorPos{0, 12} // end of "package main"
	if m.highlighter == nil {
		t.Skip("no highlighter available")
	}

	m2, _ := m.Update(tea.PasteMsg{Content: ` // comment`})
	em := m2.(Model)

	newLine := em.buf.LineAt(0)
	spans := em.highlighter.HighlightLine(0, newLine)
	hasComment := false
	for _, s := range spans {
		if s.Style == "comment" {
			hasComment = true
			break
		}
	}
	if !hasComment {
		t.Fatalf("expected 'comment' span after pasting comment text, got spans: %v", spans)
	}
}

func TestReparseSyntax_AfterUndo(t *testing.T) {
	// Type a character, undo it, verify tree matches original content.
	src := "package main\n"
	m := newTestModelWithSyntax(src, "main.go")
	m.focused = true
	if m.highlighter == nil {
		t.Skip("no highlighter available")
	}

	// Type "x"
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	em := m2.(Model)
	if em.buf.String() != "xpackage main\n" {
		t.Fatalf("unexpected buffer after typing: %q", em.buf.String())
	}

	// Undo
	m3, _ := em.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	em2 := m3.(Model)
	if em2.buf.String() != src {
		t.Fatalf("unexpected buffer after undo: %q", em2.buf.String())
	}

	// Tree should reflect original content — HighlightLine must not panic.
	origLine := em2.buf.LineAt(0)
	spans := em2.highlighter.HighlightLine(0, origLine)
	_ = spans // no panic; tree is current
}

// ── Binary file guard ─────────────────────────────────────────────────────────

func TestFileLoadedMsg_Binary_SetsBinaryFile(t *testing.T) {
	m := newTestModel("hello\n")
	updated, _ := m.Update(fileLoadedMsg{bufferID: 1, path: "photo.png", isBinary: true})
	result := updated.(Model)
	if !result.binaryFile {
		t.Fatal("expected binaryFile=true after loading binary fileLoadedMsg")
	}
}

func TestFileLoadedMsg_Binary_EmptyBuffer(t *testing.T) {
	m := newTestModel("hello\n")
	updated, _ := m.Update(fileLoadedMsg{bufferID: 1, path: "photo.png", isBinary: true})
	result := updated.(Model)
	if result.buf.String() != "" {
		t.Fatalf("expected empty buffer for binary file, got %q", result.buf.String())
	}
}

func TestFileLoadedMsg_Binary_NilHighlighter(t *testing.T) {
	m := newTestModel("hello\n")
	updated, _ := m.Update(fileLoadedMsg{bufferID: 1, path: "photo.png", isBinary: true})
	result := updated.(Model)
	if result.highlighter != nil {
		t.Fatal("expected nil highlighter for binary file")
	}
}

func TestFileLoadedMsg_BinaryThenText_ClearsBinaryFile(t *testing.T) {
	m := newTestModel("")
	// First: load a binary file
	updated, _ := m.Update(fileLoadedMsg{bufferID: 1, path: "photo.png", isBinary: true})
	m = updated.(Model)
	// Then: load a text file
	updated, _ = m.Update(fileLoadedMsg{bufferID: 2, path: "main.go", content: "package main\n"})
	m = updated.(Model)
	if m.binaryFile {
		t.Fatal("expected binaryFile=false after loading a text file following a binary")
	}
	if m.buf.String() != "package main\n" {
		t.Fatalf("expected buffer to contain text content, got %q", m.buf.String())
	}
}

// ── deleteWordBackward ────────────────────────────────────────────────────────

func TestDeleteWordBackward_MidWord(t *testing.T) {
	// Cursor after "hello" — should delete the whole word.
	m := newTestModel("hello world\n")
	m.cursor = cursorPos{line: 0, col: 5} // after "hello"
	m.deleteWordBackward()
	if m.buf.String() != " world\n" {
		t.Fatalf("expected ' world\\n', got %q", m.buf.String())
	}
	if m.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", m.cursor)
	}
}

func TestDeleteWordBackward_AfterSpace(t *testing.T) {
	// Cursor after "hello " — should skip the space then delete "hello".
	m := newTestModel("hello world\n")
	m.cursor = cursorPos{line: 0, col: 6} // after "hello "
	m.deleteWordBackward()
	if m.buf.String() != "world\n" {
		t.Fatalf("expected 'world\\n', got %q", m.buf.String())
	}
	if m.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", m.cursor)
	}
}

func TestDeleteWordBackward_AtLineStart_JoinsLines(t *testing.T) {
	// Cursor at start of line 1 — should delete the newline (join lines).
	m := newTestModel("hello\nworld\n")
	m.cursor = cursorPos{line: 1, col: 0}
	m.deleteWordBackward()
	if m.buf.String() != "helloworld\n" {
		t.Fatalf("expected 'helloworld\\n', got %q", m.buf.String())
	}
	if m.cursor != (cursorPos{0, 5}) {
		t.Fatalf("expected cursor at {0,5}, got %v", m.cursor)
	}
}

func TestDeleteWordBackward_AtBufferStart_NoOp(t *testing.T) {
	// Cursor at very start — nothing to delete.
	m := newTestModel("hello\n")
	m.cursor = cursorPos{line: 0, col: 0}
	m.deleteWordBackward()
	if m.buf.String() != "hello\n" {
		t.Fatalf("expected buffer unchanged, got %q", m.buf.String())
	}
	if m.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor unchanged at {0,0}, got %v", m.cursor)
	}
}

func TestDeleteWordBackward_WithActiveSelection_DeletesSelection(t *testing.T) {
	// With active selection, should delete selection (same as plain backspace).
	m := newTestModel("hello world\n")
	anchor := cursorPos{line: 0, col: 0}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 5} // "hello" selected
	m.deleteWordBackward()
	if m.buf.String() != " world\n" {
		t.Fatalf("expected ' world\\n', got %q", m.buf.String())
	}
	if m.selectionAnchor != nil {
		t.Fatal("expected selection cleared after delete")
	}
	if m.cursor != (cursorPos{0, 0}) {
		t.Fatalf("expected cursor at {0,0}, got %v", m.cursor)
	}
}

// ── Go-to-Line ────────────────────────────────────────────────────────────────

func TestGoToLine_SetsCursor(t *testing.T) {
	// Build a 20-line model (lines 0–19).
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line content"
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	m := newTestModel(content)

	// Set a selection anchor to verify it gets cleared.
	anchor := cursorPos{line: 2, col: 3}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: 0, col: 0}

	// Dispatch GoToLineMsg for line 5 (0-based).
	updated, cmd := m.Update(messages.GoToLineMsg{Line: 5})
	if cmd != nil {
		t.Fatal("expected no cmd from GoToLineMsg")
	}
	result := updated.(Model)

	if result.cursor.line != 5 {
		t.Fatalf("expected cursor.line=5, got %d", result.cursor.line)
	}
	if result.cursor.col != 0 {
		t.Fatalf("expected cursor.col=0, got %d", result.cursor.col)
	}
	if result.selectionAnchor != nil {
		t.Fatal("expected selectionAnchor to be nil after GoToLineMsg")
	}
}

func TestGoToLine_ClampsBelowZero(t *testing.T) {
	content := "line1\nline2\nline3\n"
	m := newTestModel(content)

	updated, _ := m.Update(messages.GoToLineMsg{Line: -5})
	result := updated.(Model)
	if result.cursor.line != 0 {
		t.Fatalf("expected cursor.line=0 when clamped from negative, got %d", result.cursor.line)
	}
}

func TestGoToLine_ClampsAboveLastLine(t *testing.T) {
	content := "line1\nline2\nline3\n"
	m := newTestModel(content)

	updated, _ := m.Update(messages.GoToLineMsg{Line: 999})
	result := updated.(Model)
	lineCount := m.buf.LineCount()
	if result.cursor.line != lineCount-1 {
		t.Fatalf("expected cursor.line=%d when clamped from 999, got %d", lineCount-1, result.cursor.line)
	}
}

func TestLineCount_ReturnsBufferLineCount(t *testing.T) {
	content := "line1\nline2\nline3\n"
	m := newTestModel(content)
	if m.LineCount() != m.buf.LineCount() {
		t.Fatalf("expected LineCount()=%d, got %d", m.buf.LineCount(), m.LineCount())
	}
}

func TestWrapMode_SetOnMarkdownLoad(t *testing.T) {
	m := newTestModel("")
	if m.wrapMode {
		t.Fatal("wrapMode should be false before any file load")
	}

	// Simulate a markdown file load.
	mdMsg := fileLoadedMsg{bufferID: 0, path: "notes.md", content: "hello\n"}
	model, _ := m.Update(mdMsg)
	m = model.(Model)
	if !m.wrapMode {
		t.Fatal("wrapMode should be true after loading .md file")
	}

	// Simulate a Go file load.
	goMsg := fileLoadedMsg{bufferID: 0, path: "main.go", content: "package main\n"}
	model, _ = m.Update(goMsg)
	m = model.(Model)
	if m.wrapMode {
		t.Fatal("wrapMode should be false after loading .go file")
	}

	// Simulate a .markdown extension.
	mdMsg2 := fileLoadedMsg{bufferID: 0, path: "README.markdown", content: "# hi\n"}
	model, _ = m.Update(mdMsg2)
	m = model.(Model)
	if !m.wrapMode {
		t.Fatal("wrapMode should be true after loading .markdown file")
	}
}
