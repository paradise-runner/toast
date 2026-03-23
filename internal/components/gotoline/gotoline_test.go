package gotoline

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// keyMsg converts a key string to a KeyPressMsg.
func keyMsg(key string) tea.KeyPressMsg {
	switch key {
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc", "escape":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		if len(key) == 1 {
			return tea.KeyPressMsg{Code: rune(key[0])}
		}
		return tea.KeyPressMsg{}
	}
}

// extractMsg runs the cmd (if non-nil) and returns the resulting message.
func extractMsg(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestNew_IsClosedByDefault(t *testing.T) {
	m := New()
	if m.IsOpen() {
		t.Fatal("expected new Model to be closed")
	}
}

func TestOpen_SetsOpenTrue(t *testing.T) {
	m := New().Open(100)
	if !m.IsOpen() {
		t.Fatal("expected model to be open after Open()")
	}
}

func TestOpen_ClearsInput(t *testing.T) {
	m := New().Open(100)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	// Re-open should clear input
	m = m.Open(100)
	if m.input != "" {
		t.Fatalf("expected input to be cleared on Open, got %q", m.input)
	}
}

func TestTypingDigits_BuildsInput(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	if m.input != "42" {
		t.Fatalf("expected input \"42\", got %q", m.input)
	}
}

func TestTypingNonDigits_IsIgnored(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("a"))
	m, _ = m.Update(keyMsg("z"))
	m, _ = m.Update(keyMsg("/"))
	if m.input != "" {
		t.Fatalf("expected input to remain empty, got %q", m.input)
	}
}

func TestBackspace_RemovesLastDigit(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	m, _ = m.Update(keyMsg("backspace"))
	if m.input != "4" {
		t.Fatalf("expected input \"4\" after backspace, got %q", m.input)
	}
}

func TestBackspace_OnEmptyInput_IsNoop(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("backspace"))
	if m.input != "" {
		t.Fatalf("expected input to remain empty after backspace, got %q", m.input)
	}
}

func TestEnter_WithDigits_EmitsGoToLineMsg(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	_, cmd := m.Update(keyMsg("enter"))
	msg := extractMsg(cmd)
	gotoline, ok := msg.(messages.GoToLineMsg)
	if !ok {
		t.Fatalf("expected GoToLineMsg, got %T", msg)
	}
	// User typed "42" (1-based) → should be line 41 (0-based)
	if gotoline.Line != 41 {
		t.Fatalf("expected Line=41, got %d", gotoline.Line)
	}
}

func TestEnter_WithEmptyInput_EmitsCancelMsg(t *testing.T) {
	m := New().Open(200)
	_, cmd := m.Update(keyMsg("enter"))
	msg := extractMsg(cmd)
	_, ok := msg.(messages.GoToLineCancelMsg)
	if !ok {
		t.Fatalf("expected GoToLineCancelMsg, got %T", msg)
	}
}

func TestEnter_OutOfRange_ClampedToLastLine(t *testing.T) {
	// lineCount=10 means valid lines are 0–9; user types "999" (1-based)
	m := New().Open(10)
	m, _ = m.Update(keyMsg("9"))
	m, _ = m.Update(keyMsg("9"))
	m, _ = m.Update(keyMsg("9"))
	_, cmd := m.Update(keyMsg("enter"))
	msg := extractMsg(cmd)
	gotoline, ok := msg.(messages.GoToLineMsg)
	if !ok {
		t.Fatalf("expected GoToLineMsg, got %T", msg)
	}
	// Clamped to lineCount-1 = 9
	if gotoline.Line != 9 {
		t.Fatalf("expected Line=9 (clamped), got %d", gotoline.Line)
	}
}

func TestEscape_EmitsCancelMsg(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	_, cmd := m.Update(keyMsg("esc"))
	msg := extractMsg(cmd)
	_, ok := msg.(messages.GoToLineCancelMsg)
	if !ok {
		t.Fatalf("expected GoToLineCancelMsg from escape, got %T", msg)
	}
}

func TestEnter_ClosesOverlay(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("5"))
	m, _ = m.Update(keyMsg("enter"))
	if m.IsOpen() {
		t.Fatal("expected model to be closed after enter")
	}
}

func TestEscape_ClosesOverlay(t *testing.T) {
	m := New().Open(200)
	m, _ = m.Update(keyMsg("esc"))
	if m.IsOpen() {
		t.Fatal("expected model to be closed after escape")
	}
}

func TestView_ShowsLineCount(t *testing.T) {
	m := New().Open(247)
	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	// Should mention the line count guidance
	if !containsStr(view, "247") {
		t.Fatalf("expected view to contain \"247\", got: %s", view)
	}
}

func TestView_ShowsCurrentInput(t *testing.T) {
	m := New().Open(100)
	m, _ = m.Update(keyMsg("4"))
	m, _ = m.Update(keyMsg("2"))
	view := m.View()
	if !containsStr(view, "42") {
		t.Fatalf("expected view to contain \"42\", got: %s", view)
	}
}

func TestView_WhenClosed_ReturnsEmpty(t *testing.T) {
	m := New()
	if m.View() != "" {
		t.Fatal("expected empty view when closed")
	}
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
