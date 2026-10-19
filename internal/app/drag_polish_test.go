package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// The app composites the tree's drag ghost into the full-screen layout; make
// sure an in-flight drag reaches DragOverlay and View still renders.
func TestApp_DragOverlayRendersDuringDrag(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.txt"))

	m := newTestApp(t, dir)
	// Sidebar rows: 0 root, 1 sub, 2 a.txt. Global Y = row + 1.
	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 2, Y: 3})
	_, _ = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: 2, Y: 2})

	if _, _, _, ok := m.fileTree.DragOverlay(); !ok {
		t.Fatal("expected the drag overlay while dragging")
	}
	// Composing the overlay must not panic (it is clipped to the screen).
	_ = m.View()

	_, _ = m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: 2, Y: 2})
	if _, _, _, ok := m.fileTree.DragOverlay(); ok {
		t.Error("expected the drag overlay to be gone after the drop")
	}
}

// runCmds executes a command tree and feeds every resulting message back into
// the app so asynchronous work (such as opening a file) completes.
func runCmds(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	switch v := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range v {
			runCmds(t, m, c)
		}
	default:
		_, next := m.Update(v)
		runCmds(t, m, next)
	}
}

// An editor selection drag must keep tracking the pointer when it leaves the
// text area (issue #58): the drag would otherwise freeze at the last in-viewport
// row and the release would never reach the editor to stop its scroll loop.
func TestApp_EditorSelectionDragSurvivesLeavingTheTextArea(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long.txt")
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "line%d\n", i)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}

	m := newTestApp(t, dir)
	_, cmd := m.Update(messages.FileSelectedMsg{Path: path})
	runCmds(t, m, cmd)
	if m.editor.Path() != path {
		t.Fatalf("editor path = %q, want %q", m.editor.Path(), path)
	}

	// Editor rows start below the tab bar and breadcrumb. The gutter for a
	// 200-line file is 8 cells wide, so column 14 is safely inside the text.
	editorX := m.sidebarWidth + 14
	topY := 2

	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: editorX, Y: topY})
	if !m.editor.IsDraggingSelection() {
		t.Fatal("expected a selection drag to start in the editor")
	}

	// Drag below the text area, into the status bar row.
	_, _ = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: editorX, Y: m.height - 1})
	if !m.editor.IsDraggingSelection() {
		t.Fatal("expected the drag to survive leaving the text area")
	}
	if m.editor.SelectedText() == "" {
		t.Fatal("expected the below-viewport motion to extend the selection")
	}

	// Release over the status bar still ends the drag.
	_, _ = m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: editorX, Y: m.height - 1})
	if m.editor.IsDraggingSelection() {
		t.Fatal("expected the release over the status bar to end the drag")
	}
}

// dragScrollTicks drives the editor's edge auto-scroll loop through the app,
// feeding each tick back without any new motion events. It returns the cursor
// line after each tick.
func dragScrollTicks(m *Model, cmd tea.Cmd, n int) ([]int, tea.Cmd) {
	var lines []int
	for i := 0; i < n && cmd != nil; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		_, cmd = m.Update(msg)
		lines = append(lines, m.editor.CursorLine())
	}
	return lines, cmd
}

// Both viewport edges must auto-scroll while the button is held, without the
// pointer having to keep moving and without leaving the text area (issue #58).
func TestApp_EditorSelectionDragAutoScrollsAtBothEdges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long.txt")
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "line%d\n", i)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}

	m := newTestApp(t, dir)
	_, cmd := m.Update(messages.FileSelectedMsg{Path: path})
	runCmds(t, m, cmd)
	if m.editor.Path() != path {
		t.Fatalf("editor path = %q, want %q", m.editor.Path(), path)
	}

	editorX := m.sidebarWidth + 14
	topRow := 2               // first text row: tab bar + breadcrumbs sit above it
	bottomRow := m.height - 2 // last text row: the status bar sits below it

	// Scroll down a bit so the upward direction has room to move.
	for i := 0; i < 40; i++ {
		_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: editorX, Y: bottomRow})
	}

	// ── Top edge ────────────────────────────────────────────────────────────
	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: editorX, Y: bottomRow - 5})
	_, cmd = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: editorX, Y: topRow})
	if cmd == nil {
		t.Fatal("expected auto-scroll to start on the top text row")
	}
	lines, cmd := dragScrollTicks(m, cmd, 4)
	if len(lines) < 4 {
		t.Fatalf("top edge: loop stopped after %d ticks, want 4", len(lines))
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] >= lines[i-1] {
			t.Fatalf("top edge: cursor did not advance upward: %v", lines)
		}
	}
	_, _ = m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: editorX, Y: topRow})

	// ── Bottom edge ─────────────────────────────────────────────────────────
	_, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: editorX, Y: 10})
	_, cmd = m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: editorX, Y: bottomRow})
	if cmd == nil {
		t.Fatal("expected auto-scroll to start on the bottom text row")
	}
	lines, cmd = dragScrollTicks(m, cmd, 4)
	if len(lines) < 4 {
		t.Fatalf("bottom edge: loop stopped after %d ticks, want 4", len(lines))
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] <= lines[i-1] {
			t.Fatalf("bottom edge: cursor did not advance downward: %v", lines)
		}
	}
}
