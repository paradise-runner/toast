package closedialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestModel(path string) Model {
	tm, _ := theme.NewManager("", "")
	return New(tm, 1, path)
}

func TestUpdate_SKeyEmitsSaveConfirmed(t *testing.T) {
	m := newTestModel("/a/foo.go")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's'})
	if cmd == nil {
		t.Fatal("expected a cmd, got nil")
	}
	msg := cmd()
	confirmed, ok := msg.(messages.CloseTabConfirmedMsg)
	if !ok {
		t.Fatalf("expected CloseTabConfirmedMsg, got %T", msg)
	}
	if confirmed.Cancelled {
		t.Error("expected Cancelled=false")
	}
	if !confirmed.Save {
		t.Error("expected Save=true")
	}
}

func TestUpdate_DKeyEmitsDiscardConfirmed(t *testing.T) {
	m := newTestModel("/a/foo.go")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
	if cmd == nil {
		t.Fatal("expected a cmd, got nil")
	}
	msg := cmd()
	confirmed, ok := msg.(messages.CloseTabConfirmedMsg)
	if !ok {
		t.Fatalf("expected CloseTabConfirmedMsg, got %T", msg)
	}
	if confirmed.Save {
		t.Error("expected Save=false for discard")
	}
	if confirmed.Cancelled {
		t.Error("expected Cancelled=false")
	}
}

func TestUpdate_EscKeyEmitsCancelled(t *testing.T) {
	m := newTestModel("/a/foo.go")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a cmd, got nil")
	}
	msg := cmd()
	confirmed, ok := msg.(messages.CloseTabConfirmedMsg)
	if !ok {
		t.Fatalf("expected CloseTabConfirmedMsg, got %T", msg)
	}
	if !confirmed.Cancelled {
		t.Error("expected Cancelled=true")
	}
}

func TestUpdate_EnterKeyEmitsSaveConfirmed(t *testing.T) {
	m := newTestModel("/a/foo.go")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a cmd, got nil")
	}
	msg := cmd()
	confirmed, ok := msg.(messages.CloseTabConfirmedMsg)
	if !ok {
		t.Fatalf("expected CloseTabConfirmedMsg, got %T", msg)
	}
	if !confirmed.Save {
		t.Error("expected Save=true for enter")
	}
}

func TestRender_ContainsFilename(t *testing.T) {
	m := newTestModel("/a/foo.go")
	rendered := m.Render()
	if !strings.Contains(rendered, "foo.go") {
		t.Errorf("Render() should contain 'foo.go', got:\n%s", rendered)
	}
}
