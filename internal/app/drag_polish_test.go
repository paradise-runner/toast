package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
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
