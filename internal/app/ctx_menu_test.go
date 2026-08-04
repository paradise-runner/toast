package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
)

func TestRightClick_ShowsContextMenu(t *testing.T) {
	cfg := config.Defaults()
	// Ensure sidebar is visible and has a reasonable width
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	// Initialize with a window size so m.width/m.height are set
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click at x=5, y=1 (inside sidebar; y=1 is tab bar so contentY=0 → first row)
	clickMsg := tea.MouseClickMsg{
		Button: tea.MouseRight,
		X:      5,
		Y:      1,
	}
	model.Update(clickMsg)

	// Check ContextMenuOverlay returns ok=true
	menuStr, menuX, menuY, ok := model.fileTree.ContextMenuOverlay()
	if !ok {
		t.Fatal("ContextMenuOverlay returned ok=false after right-click: ctxMenu was not set")
	}
	t.Logf("ctxMenu: ok=%v x=%d y=%d menuStr=%q", ok, menuX, menuY, menuStr)

	// Check View contains menu text
	view := model.View()
	content := view.Content
	if !strings.Contains(content, "New File") {
		t.Errorf("View() does not contain 'New File' — context menu not visible\ncontent snippet: %q", content[:min(200, len(content))])
	}
}

// TestRightClick_EmptySpace verifies that right-clicking in empty sidebar space
// (below all tree entries) still opens the context menu, targeting the root dir.
func TestRightClick_EmptySpace_FallsBackToRootDir(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click far below the only tree entry (y=20 → contentY=19, way out of range)
	model.Update(tea.MouseClickMsg{
		Button: tea.MouseRight,
		X:      5,
		Y:      20,
	})

	_, _, _, ok := model.fileTree.ContextMenuOverlay()
	if !ok {
		t.Fatal("right-click in empty sidebar space should still open context menu (targeting root dir), but ctxMenu was not set")
	}
}

func TestSidebarInlineCreate_EscapeCancelsFromAppKeyRouting(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 1})
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	name := "cancel_escape_probe.go"
	for _, ch := range name {
		model.Update(tea.KeyPressMsg{Text: string(ch)})
	}

	if !strings.Contains(model.View().Content, name) {
		t.Fatalf("expected inline create row to contain %q before Escape", name)
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if strings.Contains(model.View().Content, name) {
		t.Fatalf("expected Escape to cancel inline create row containing %q", name)
	}
}

// TestApp_EscapeClosesContextMenu_RegardlessOfFocus verifies that Escape
// dismisses the sidebar context menu even when keyboard focus has moved
// elsewhere (e.g. the user right-clicked, then clicked into the editor).
func TestApp_EscapeClosesContextMenu_RegardlessOfFocus(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click opens the context menu (focus moves to the file tree).
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 1})
	if _, _, _, ok := model.fileTree.ContextMenuOverlay(); !ok {
		t.Fatal("expected context menu to be open after right-click")
	}

	// Focus moves to the editor — the floating menu stays open.
	model.setFocus(FocusEditor)

	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if _, _, _, ok := model.fileTree.ContextMenuOverlay(); ok {
		t.Fatal("expected Escape to close the context menu even when focus is on the editor")
	}
}

// TestApp_EscapeCancelsInlineCreate_RegardlessOfFocus verifies that Escape
// cancels an in-progress file/folder creation even when keyboard focus has
// moved elsewhere (e.g. the user clicked into the editor mid-creation).
func TestApp_EscapeCancelsInlineCreate_RegardlessOfFocus(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click → Enter starts inline file creation (focus on the file tree).
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 1})
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	name := "focus_elsewhere_probe.go"
	for _, ch := range name {
		model.Update(tea.KeyPressMsg{Text: string(ch)})
	}
	if !strings.Contains(model.View().Content, name) {
		t.Fatalf("expected inline create row to contain %q before Escape", name)
	}

	// Focus moves to the editor — the inline row stays active.
	model.setFocus(FocusEditor)

	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if strings.Contains(model.View().Content, name) {
		t.Fatalf("expected Escape to cancel inline create row containing %q even when focus is on the editor", name)
	}
}

// TestApp_EscapeDismissesDeleteDialog_RegardlessOfFocus verifies that Escape
// cancels the delete confirmation dialog even when focus has moved elsewhere.
func TestApp_EscapeDismissesDeleteDialog_RegardlessOfFocus(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30
	cfg.Sidebar.ConfirmDelete = true

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "doomed.go"), []byte(""), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click the file row → down to Delete → Enter opens the dialog.
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 1})
	model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !model.fileTree.HasDeleteDialog() {
		t.Fatal("expected delete dialog to be open")
	}

	// Focus moves to the editor — the modal stays open.
	model.setFocus(FocusEditor)

	model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if model.fileTree.HasDeleteDialog() {
		t.Fatal("expected Escape to dismiss the delete dialog even when focus is on the editor")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "doomed.go")); err != nil {
		t.Fatalf("file should not have been deleted: %v", err)
	}
}

// TestApp_ContextMenuHover_PastSidebarEdge verifies that mouse motion is routed to
// the file tree when the pointer is inside the open context menu's box, even when
// that box extends past the sidebar's right edge.
func TestApp_ContextMenuHover_PastSidebarEdge(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Right-click near the sidebar's right edge (x=28, y=1 → contentY=0).
	// Menu opens at (28, 0); its box spans [28, 48) horizontally, past sidebarW=30.
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 28, Y: 1})
	before, _, _, ok := model.fileTree.ContextMenuOverlay()
	if !ok {
		t.Fatal("expected context menu to be open after right-click")
	}

	// Hover over item 1 (New Folder): screen y=3 → contentY=2 → menu row 2.
	// x=35 is past the sidebar edge, so this only reaches the file tree via the
	// context-menu bounds routing.
	model.Update(tea.MouseMotionMsg{X: 35, Y: 3})
	after, _, _, ok := model.fileTree.ContextMenuOverlay()
	if !ok {
		t.Fatal("expected context menu to still be open")
	}
	// The highlight must have moved (item 0 → item 1), so the overlay changed.
	if after == before {
		t.Error("expected context menu highlight to change after hovering an item past the sidebar edge")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
