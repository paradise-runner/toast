package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
)

func newPromptModel(t *testing.T) *Model {
	t.Helper()
	cfg := config.Defaults()
	m, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(*Model)
	m.ShowWorkspacePrompt()
	return m
}

func TestWorkspaceButtonRect_InsideEditorArea(t *testing.T) {
	m := newPromptModel(t)
	x, y, w, h := m.workspaceButtonRect()
	t.Logf("button rect: x=%d y=%d w=%d h=%d (window 100x30)", x, y, w, h)
	if w < 10 || h < 2 {
		t.Fatalf("button too small: %dx%d", w, h)
	}
	// Centered horizontally within the editor area (right of the sidebar).
	editorWidth := m.width
	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
		editorWidth -= m.sidebarWidth
	}
	center := x + w/2
	expectedCenter := sidebarW + editorWidth/2
	if center < expectedCenter-1 || center > expectedCenter+1 {
		t.Fatalf("button not centered: center=%d want ~%d (x=%d w=%d)", center, expectedCenter, x, w)
	}
	// Below the tab bar and breadcrumb (rows 0-1).
	if y < 2 {
		t.Fatalf("button overlaps the chrome: y=%d", y)
	}
}

func TestUpdate_ClickOnButtonOpensPicker(t *testing.T) {
	m := newPromptModel(t)
	x, y, _, _ := m.workspaceButtonRect()

	// Click inside the button → request command.
	updated, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x + 1, Y: y + 1})
	if cmd == nil {
		t.Fatal("expected a command from clicking the button")
	}
	msg := cmd()
	if msg != nil {
		t.Fatalf("expected nil message (marker written to stdout), got %T", msg)
	}
	_ = updated
}

func TestUpdate_ClickOutsideButtonDoesNothing(t *testing.T) {
	m := newPromptModel(t)
	x, y, _, h := m.workspaceButtonRect()

	// Click far away from the button (top-left corner of the editor area).
	updated, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 3})
	if cmd != nil {
		t.Fatalf("expected no command for a click outside the button, got %v", cmd)
	}
	_ = updated

	// Just below the button.
	updated, cmd = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x + 1, Y: y + h + 5})
	if cmd != nil {
		t.Fatalf("expected no command for a click below the button, got %v", cmd)
	}
	_ = updated
}
