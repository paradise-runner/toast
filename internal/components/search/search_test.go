package search

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme.NewManager: %v", err)
	}
	m := New(tm, t.TempDir())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = m.Update(messages.SearchOpenMsg{})
	return m
}

func collectCmdMessages(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, batchCmd := range batch {
			msgs = append(msgs, collectCmdMessages(t, batchCmd)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func TestMouseClickCloseEmitsSearchClose(t *testing.T) {
	m := newTestModel(t)

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.width - 1,
		Y:      0,
	})

	if updated.active {
		t.Fatal("expected search to become inactive after close click")
	}
	msgs := collectCmdMessages(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if _, ok := msgs[0].(messages.SearchCloseMsg); !ok {
		t.Fatalf("expected SearchCloseMsg, got %T", msgs[0])
	}
}

func TestMouseClickResultEmitsFileSelected(t *testing.T) {
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
		Y:      searchHeaderRows,
	})

	if updated.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", updated.cursor)
	}
	msgs := collectCmdMessages(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg, ok := msgs[0].(messages.FileSelectedMsg)
	if !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", msgs[0])
	}
	if msg.Path != path {
		t.Fatalf("Path = %q, want %q", msg.Path, path)
	}
}

func TestMouseClickInputMovesQueryCursor(t *testing.T) {
	m := newTestModel(t)
	m.query.value = "foobar"
	m.query.cursor = 0

	updated, _ := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      3,
		Y:      1,
	})

	if updated.query.cursor != 3 {
		t.Fatalf("query cursor = %d, want 3", updated.query.cursor)
	}
}

func TestMouseWheelMovesCursor(t *testing.T) {
	m := newTestModel(t)
	for i := 0; i < 3; i++ {
		m, _ = m.Update(messages.SearchResultMsg{
			Path:       filepath.Join(m.rootDir, "main.go"),
			Line:       i + 1,
			Content:    "func main() {}\n",
			MatchStart: 5,
			MatchEnd:   9,
		})
	}

	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	if updated.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", updated.cursor)
	}
}
