package app

import (
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
