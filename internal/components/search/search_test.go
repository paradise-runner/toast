package search

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme.NewManager: %v", err)
	}
	m := New(tm, t.TempDir(), config.Defaults().Search)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m, _ = m.Update(messages.SearchOpenMsg{})
	return m
}

// collectCommands runs a tea.Cmd and collects every message it produces,
// unwrapping tea.BatchMsg transparently.
func collectCommands(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	// tea.BatchMsg is []Cmd — execute each and collect their messages.
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, collectCommands(t, c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// ── Open / Close ──────────────────────────────────────────────────────────────

func TestOpenClose(t *testing.T) {
	m := newTestModel(t)

	if !m.active {
		t.Fatal("expected active after SearchOpenMsg")
	}

	updated, cmd := m.Update(messages.SearchCloseMsg{})
	if updated.active {
		t.Fatal("expected inactive after SearchCloseMsg")
	}
	// Close emits no extra command (active flag is enough).
	_ = collectCommands(t, cmd)
}

func TestEscapeCloses(t *testing.T) {
	m := newTestModel(t)

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "escape"})
	if updated.active {
		t.Fatal("expected inactive after escape")
	}
	msgs := collectCommands(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if _, ok := msgs[0].(messages.SearchCloseMsg); !ok {
		t.Fatalf("expected SearchCloseMsg, got %T", msgs[0])
	}
}

// ── Close button ──────────────────────────────────────────────────────────────

func TestMouseCloseButton(t *testing.T) {
	m := newTestModel(t)

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.width - 1,
		Y:      0,
	})

	if updated.active {
		t.Fatal("expected inactive after close click")
	}
	msgs := collectCommands(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if _, ok := msgs[0].(messages.SearchCloseMsg); !ok {
		t.Fatalf("expected SearchCloseMsg, got %T", msgs[0])
	}
}

func TestMouseCloseButtonNarrow(t *testing.T) {
	m := newTestModel(t)
	// Shrink to where close button does not fit.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 5, Height: 12})

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      4,
		Y:      0,
	})

	if !updated.active {
		t.Fatal("expected active — panel too narrow for close button")
	}
}

// ── Result click → FileSelectedMsg ───────────────────────────────────────────

func TestMouseClickResultOpensFile(t *testing.T) {
	m := newTestModel(t)
	path := filepath.Join(m.rootDir, "main.go")
	m, _ = m.Update(messages.SearchResultMsg{
		Path:       path,
		Line:       12,
		Content:    "func main() {}\n",
		MatchStart: 5,
		MatchEnd:   9,
	})

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      0,
		Y:      headerRows,
	})

	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.cursor)
	}
	msgs := collectCommands(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	fs, ok := msgs[0].(messages.FileSelectedMsg)
	if !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", msgs[0])
	}
	if fs.Path != path {
		t.Fatalf("Path = %q, want %q", fs.Path, path)
	}
}

// ── Mouse click on input row moves query cursor ──────────────────────────────

func TestMouseClickInputMovesCursor(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "foobar"

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      3,
		Y:      1,
	})

	if updated.query.cursor != 3 {
		t.Fatalf("query cursor = %d, want 3", updated.query.cursor)
	}
}

func TestMouseClickInputClampLeft(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "foo"

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      -5,
		Y:      1,
	})

	if updated.query.cursor != 0 {
		t.Fatalf("query cursor = %d, want 0", updated.query.cursor)
	}
}

func TestMouseClickInputClampRight(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "foo"

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      99,
		Y:      1,
	})

	if updated.query.cursor != 3 {
		t.Fatalf("query cursor = %d, want 3", updated.query.cursor)
	}
}

// ── Mouse wheel ───────────────────────────────────────────────────────────────

func TestMouseWheelMovesCursor(t *testing.T) {
	m := newTestModel(t)
	for i := 0; i < 5; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       filepath.Join(m.rootDir, "main.go"),
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	// Wheel down.
	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if updated.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after wheel down", updated.cursor)
	}

	// Wheel up.
	updated, _ = updated.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after wheel up", updated.cursor)
	}

	// Wheel up at top stays at 0.
	updated, _ = updated.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (clamped)", updated.cursor)
	}
}

// ── Mouse motion ──────────────────────────────────────────────────────────────

func TestMouseMotionMovesCursor(t *testing.T) {
	m := newTestModel(t)
	for i := 0; i < 5; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       filepath.Join(m.rootDir, "main.go"),
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	updated, _ := m.Update(tea.MouseMotionMsg{
		X: 0,
		Y: headerRows + 2,
	})

	if updated.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", updated.cursor)
	}
}

// ── Keyboard navigation ───────────────────────────────────────────────────────

func TestKeyboardNavigation(t *testing.T) {
	m := newTestModel(t)
	for i := 0; i < 5; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       filepath.Join(m.rootDir, "main.go"),
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	// Down.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", updated.cursor)
	}

	// Up.
	updated, _ = updated.Update(tea.KeyPressMsg{Text: "up"})
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.cursor)
	}

	// Up at top.
	updated, _ = updated.Update(tea.KeyPressMsg{Text: "up"})
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (clamped)", updated.cursor)
	}

	// Ctrl-p / Ctrl-n aliases.
	updated, _ = updated.Update(tea.KeyPressMsg{Text: "ctrl+n"})
	if updated.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after ctrl+n", updated.cursor)
	}
	updated, _ = updated.Update(tea.KeyPressMsg{Text: "ctrl+p"})
	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after ctrl+p", updated.cursor)
	}
}

// ── Enter opens file ──────────────────────────────────────────────────────────

func TestEnterOpensFile(t *testing.T) {
	m := newTestModel(t)
	path := filepath.Join(m.rootDir, "main.go")
	m, _ = m.Update(messages.SearchResultMsg{
		Path:       path,
		Line:       1,
		Content:    "package main\n",
		MatchStart: 0,
		MatchEnd:   4,
	})

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})

	msgs := collectCommands(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	fs, ok := msgs[0].(messages.FileSelectedMsg)
	if !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", msgs[0])
	}
	if fs.Path != path {
		t.Fatalf("Path = %q, want %q", fs.Path, path)
	}
	_ = updated
}

func TestEnterEmptyResults(t *testing.T) {
	m := newTestModel(t)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	msgs := collectCommands(t, cmd)
	if len(msgs) != 0 {
		t.Fatalf("expected no messages on enter with no results, got %d", len(msgs))
	}
}

// ── Keyboard ignored when inactive ────────────────────────────────────────────

func TestKeyboardIgnoredWhenInactive(t *testing.T) {
	m := newTestModel(t)
	m, _ = m.Update(messages.SearchCloseMsg{})

	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.cursor != 0 {
		t.Fatal("keyboard should be ignored when inactive")
	}
}

// ── Text input ────────────────────────────────────────────────────────────────

func TestTextInputTyping(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "h"})
	if updated.query.value != "h" {
		t.Fatalf("value = %q, want %q", updated.query.value, "h")
	}
	if updated.query.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", updated.query.cursor)
	}
}

func TestTextInputBackspace(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"
	m.query.cursor = 5

	updated, _ := m.Update(tea.KeyPressMsg{Text: "backspace"})
	if updated.query.value != "hell" {
		t.Fatalf("value = %q, want %q", updated.query.value, "hell")
	}
	if updated.query.cursor != 4 {
		t.Fatalf("cursor = %d, want 4", updated.query.cursor)
	}
}

func TestTextInputDelete(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"
	m.query.cursor = 3

	updated, _ := m.Update(tea.KeyPressMsg{Text: "delete"})
	if updated.query.value != "helo" {
		t.Fatalf("value = %q, want %q", updated.query.value, "helo")
	}
	if updated.query.cursor != 3 {
		t.Fatalf("cursor = %d, want 3", updated.query.cursor)
	}
}

func TestTextInputDeleteAtEnd(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"
	m.query.cursor = 5

	updated, _ := m.Update(tea.KeyPressMsg{Text: "delete"})
	if updated.query.value != "hello" {
		t.Fatalf("value = %q, want %q (unchanged)", updated.query.value, "hello")
	}
}

func TestTextInputHomeEnd(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"
	m.query.cursor = 2

	// Home.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "home"})
	if updated.query.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.query.cursor)
	}

	// End.
	updated, _ = updated.Update(tea.KeyPressMsg{Text: "end"})
	if updated.query.cursor != 5 {
		t.Fatalf("cursor = %d, want 5", updated.query.cursor)
	}
}

func TestTextInputCtrlA_E(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"
	m.query.cursor = 2

	updated, _ := m.Update(tea.KeyPressMsg{Text: "ctrl+a"})
	if updated.query.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after ctrl+a", updated.query.cursor)
	}

	updated, _ = updated.Update(tea.KeyPressMsg{Text: "ctrl+e"})
	if updated.query.cursor != 5 {
		t.Fatalf("cursor = %d, want 5 after ctrl+e", updated.query.cursor)
	}
}

func TestTextInputCtrlU(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello world"
	m.query.cursor = 6

	updated, _ := m.Update(tea.KeyPressMsg{Text: "ctrl+u"})
	if updated.query.value != "world" {
		t.Fatalf("value = %q, want %q", updated.query.value, "world")
	}
	if updated.query.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.query.cursor)
	}
}

func TestTextInputCtrlK(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello world"
	m.query.cursor = 6

	updated, _ := m.Update(tea.KeyPressMsg{Text: "ctrl+k"})
	if updated.query.value != "hello " {
		t.Fatalf("value = %q, want %q", updated.query.value, "hello ")
	}
	if updated.query.cursor != 6 {
		t.Fatalf("cursor = %d, want 6", updated.query.cursor)
	}
}

func TestTextInputUnicode(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "café"})
	if updated.query.value != "café" {
		t.Fatalf("value = %q, want %q", updated.query.value, "café")
	}
	if updated.query.cursor != 4 {
		// café has 4 runes
		t.Fatalf("cursor = %d, want 4", updated.query.cursor)
	}
}

// ── Scroll clamping ───────────────────────────────────────────────────────────

func TestScrollClamp(t *testing.T) {
	m := newTestModel(t)
	m.height = 10 // headerRows(3) + summaryRows(1) + 6 results

	for i := 0; i < 20; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       filepath.Join(m.rootDir, "main.go"),
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	// Cursor at 0, offset at 0.
	if m.offset != 0 {
		t.Fatalf("offset = %d, want 0", m.offset)
	}

	// Move cursor to 10; offset should follow.
	m.cursor = 10
	m.clampScroll()
	if m.offset != 5 {
		t.Fatalf("offset = %d, want 5 (10 - 6 + 1)", m.offset)
	}

	// Move cursor to 19; offset should be 14.
	m.cursor = 19
	m.clampScroll()
	if m.offset != 14 {
		t.Fatalf("offset = %d, want 14 (19 - 6 + 1)", m.offset)
	}
}

// ── batchMsg ──────────────────────────────────────────────────────────────────

func TestBatchMsg(t *testing.T) {
	m := newTestModel(t)

	path := filepath.Join(m.rootDir, "main.go")
	b := batchMsg{
		results: []messages.SearchResultMsg{
			{Path: path, Line: 1, Content: "a\n", MatchStart: 0, MatchEnd: 1},
			{Path: path, Line: 2, Content: "b\n", MatchStart: 0, MatchEnd: 1},
		},
		totalFiles:   1,
		totalMatches: 2,
	}

	updated, _ := m.Update(b)

	if updated.running {
		t.Fatal("expected running = false after batchMsg")
	}
	if !updated.done {
		t.Fatal("expected done = true after batchMsg")
	}
	if len(updated.results) != 2 {
		t.Fatalf("results = %d, want 2", len(updated.results))
	}
	if updated.totalMatches != 2 {
		t.Fatalf("totalMatches = %d, want 2", updated.totalMatches)
	}
	if updated.totalFiles != 1 {
		t.Fatalf("totalFiles = %d, want 1", updated.totalFiles)
	}
}

// ── SearchDoneMsg ─────────────────────────────────────────────────────────────

func TestSearchDoneMsg(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(messages.SearchDoneMsg{
		TotalMatches: 5,
		TotalFiles:   2,
	})

	if updated.running {
		t.Fatal("expected running = false")
	}
	if !updated.done {
		t.Fatal("expected done = true")
	}
	if updated.totalMatches != 5 {
		t.Fatalf("totalMatches = %d, want 5", updated.totalMatches)
	}
	if updated.totalFiles != 2 {
		t.Fatalf("totalFiles = %d, want 2", updated.totalFiles)
	}
}

// ── runSearchMsg dedup ──────────────────────────────────────────────────────────

func TestRunSearchMsgDedup(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "hello"

	// Stale query — should be ignored.
	updated, cmd := m.Update(runSearchMsg{query: "world", rootDir: m.rootDir})
	if cmd != nil {
		t.Fatal("expected nil cmd for stale query")
	}
	_ = updated
}

// ── View smoke test ────────────────────────────────────────────────────────────

func TestViewSmoke(t *testing.T) {
	m := newTestModel(t)

	v := m.View()
	if v.Content == "" {
		t.Fatal("expected non-empty view")
	}

	// Add results and check view still renders.
	m, _ = m.Update(messages.SearchResultMsg{
		Path:       filepath.Join(m.rootDir, "main.go"),
		Line:       1,
		Content:    "package main\n",
		MatchStart: 0,
		MatchEnd:   7,
	})
	v = m.View()
	if v.Content == "" {
		t.Fatal("expected non-empty view with results")
	}
}

func TestViewZeroSize(t *testing.T) {
	m := newTestModel(t)
	m.width = 0
	m.height = 0

	v := m.View()
	if v.Content != "" {
		t.Fatalf("expected empty view for zero size, got %q", v.Content)
	}
}

// ── Edge cases ────────────────────────────────────────────────────────────────

func TestEmptySearchDoesNotCrash(t *testing.T) {
	m := newTestModel(t)

	// Run search with empty query (should be a no-op).
	updated, cmd := m.Update(runSearchMsg{query: "", rootDir: m.rootDir})
	if cmd != nil {
		t.Fatal("expected nil cmd for empty query")
	}
	_ = updated
}

func TestInactiveClickIgnored(t *testing.T) {
	m := newTestModel(t)
	m, _ = m.Update(messages.SearchCloseMsg{})

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      10,
		Y:      headerRows,
	})

	if updated.cursor != 0 {
		t.Fatal("expected click to be ignored when inactive")
	}
}

func TestInactiveWheelIgnored(t *testing.T) {
	m := newTestModel(t)
	m, _ = m.Update(messages.SearchCloseMsg{})
	// Seed results via direct field set (since inactive we can't use Update with
	// SearchResultMsg — it would still append, but we test that wheel is ignored).
	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	if updated.cursor != 0 {
		t.Fatal("wheel should be ignored when inactive")
	}
}

func TestCloseButtonHitOutsideX(t *testing.T) {
	m := newTestModel(t)

	// Click at x=0 on title row should not close.
	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      0,
		Y:      0,
	})

	if !updated.active {
		t.Fatal("expected active — close button is at far right, not x=0")
	}
}

func TestRightClickIgnored(t *testing.T) {
	m := newTestModel(t)
	path := filepath.Join(m.rootDir, "main.go")
	m, _ = m.Update(messages.SearchResultMsg{
		Path:       path,
		Line:       1,
		Content:    "hello\n",
		MatchStart: 0,
		MatchEnd:   5,
	})

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseRight,
		X:      0,
		Y:      headerRows,
	})

	if cmd != nil {
		t.Fatal("right click should be ignored")
	}
	if updated.cursor != 0 {
		t.Fatal("right click should not move cursor")
	}
}

// ── Paste ────────────────────────────────────────────────────────────────────

func TestPasteInsertsText(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.PasteMsg{Content: "hello"})
	if updated.query.value != "hello" {
		t.Fatalf("value = %q, want %q", updated.query.value, "hello")
	}
	if updated.query.cursor != 5 {
		t.Fatalf("cursor = %d, want 5", updated.query.cursor)
	}
}

func TestPasteInsertsAtCursor(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "ab"
	m.query.cursor = 1

	updated, _ := m.Update(tea.PasteMsg{Content: "X"})
	if updated.query.value != "aXb" {
		t.Fatalf("value = %q, want %q", updated.query.value, "aXb")
	}
	if updated.query.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", updated.query.cursor)
	}
}

func TestPasteMultilineTakesFirstLine(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.PasteMsg{Content: "foo\nbar\nbaz"})
	if updated.query.value != "foo" {
		t.Fatalf("value = %q, want %q", updated.query.value, "foo")
	}
}

func TestPasteEmptyIgnored(t *testing.T) {
	m := newTestModel(t)

	updated, _ := m.Update(tea.PasteMsg{Content: ""})
	if updated.query.value != "" {
		t.Fatalf("value = %q, want empty", updated.query.value)
	}
}

func TestPasteIgnoredWhenInactive(t *testing.T) {
	m := newTestModel(t)
	m, _ = m.Update(messages.SearchCloseMsg{})

	updated, _ := m.Update(tea.PasteMsg{Content: "hello"})
	if updated.query.value != "" {
		t.Fatalf("value = %q, want empty (inactive)", updated.query.value)
	}
}

// ── Text input view ───────────────────────────────────────────────────────────

func TestTextInputViewEmpty(t *testing.T) {
	ti := textInput{}
	got := ti.view("Search...")
	if got != "Search... " {
		t.Fatalf("view = %q, want %q", got, "Search... ")
	}
}

func TestTextInputViewWithText(t *testing.T) {
	ti := textInput{value: "hello", cursor: 2}
	got := ti.view("")
	// Should show cursor block at position 2.
	expected := "he█lo"
	if got != expected {
		t.Fatalf("view = %q, want %q", got, expected)
	}
}

func TestTextInputViewCursorAtEnd(t *testing.T) {
	ti := textInput{value: "hello", cursor: 5}
	got := ti.view("")
	if got != "hello█" {
		t.Fatalf("view = %q, want %q", got, "hello█")
	}
}

func TestTextInputViewCursorAtStart(t *testing.T) {
	ti := textInput{value: "hello", cursor: 0}
	got := ti.view("")
	if got != "█ello" {
		t.Fatalf("view = %q, want %q", got, "█ello")
	}
}

// ── Row rendering: truncation + full-width padding ──────────────────────────

func TestTruncateRaw(t *testing.T) {
	tests := []struct {
		input string
		w     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"cafe", 3, "caf"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := ansi.Truncate(tt.input, tt.w, "")
		if got != tt.want {
			t.Errorf("ansi.Truncate(%q, %d) = %q, want %q", tt.input, tt.w, got, tt.want)
		}
	}
}

// TestRenderedRowIsFullWidth verifies that every visible row — including the
// selected one — is padded to the full panel width so the background is solid
// and the selection highlight spans the whole row.
func TestRenderedRowIsFullWidth(t *testing.T) {
	m := newTestModel(t)
	path := filepath.Join(m.rootDir, "main.go")
	for i := 0; i < 3; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       path,
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	view := m.View()
	lines := strings.Split(view.Content, "\n")

	for i, ln := range lines {
		if ln == "" {
			continue
		}
		if w := ansi.StringWidth(ln); w != m.width {
			t.Errorf("row %d visual width = %d, want %d (row=%q)", i, w, m.width, ln)
		}
	}
}

// ── SearchRunner ──────────────────────────────────────────────────────────────

func TestSearchRunnerInvalidCommand(t *testing.T) {
	cmd := runSearch("test", "/tmp", "nonexistent-command-xyz", nil)
	msg := cmd()
	b, ok := msg.(batchMsg)
	if !ok {
		t.Fatalf("expected batchMsg, got %T", msg)
	}
	if len(b.results) != 0 {
		t.Fatalf("expected 0 results for invalid command, got %d", len(b.results))
	}
}

func TestSearchRunnerEmptyArgs(t *testing.T) {
	// Should not crash.
	cmd := runSearch("test", "/tmp", "rg", nil)
	msg := cmd()
	if _, ok := msg.(batchMsg); !ok {
		t.Fatalf("expected batchMsg, got %T", msg)
	}
}
