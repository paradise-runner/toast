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

// TestRightClick_NearBottom_MenuIsClickableWhereDrawn opens the sidebar context
// menu on the last usable screen row and then clicks the row where the menu is
// actually drawn. Drawn position and hit-test box must agree.
func TestRightClick_NearBottom_MenuIsClickableWhereDrawn(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// Last row above the status bar.
	clickY := 24 - 2
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: clickY})
	if !model.fileTree.HasContextMenu() {
		t.Fatal("expected context menu to be open after right-click")
	}

	lines := strings.Split(model.View().Content, "\n")
	drawnRow := -1
	for i, l := range lines {
		if strings.Contains(l, "New File") {
			drawnRow = i
			break
		}
	}
	if drawnRow < 0 {
		t.Fatalf("menu not visible in view (rows=%d)", len(lines))
	}
	t.Logf("menu drawn starting at row %d of %d", drawnRow, len(lines))

	model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 7, Y: drawnRow})
	if model.fileTree.HasContextMenu() {
		t.Fatal("click on the drawn 'New File' row missed the menu box")
	}
	if !strings.Contains(model.View().Content, "▌") {
		t.Error("click on the drawn 'New File' row did not start the inline create row")
	}
	if strings.Contains(model.View().Content, "▶ ▌") {
		t.Error("click selected 'New Folder' instead of 'New File'")
	}
}

// TestRightClick_MenuOverhangsSidebar_ClickIsRoutedToMenu verifies that clicks
// on the part of the context menu that spills over the editor are routed to the
// sidebar (which owns the menu) rather than to the editor.
func TestRightClick_MenuOverhangsSidebar_ClickIsRoutedToMenu(t *testing.T) {
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// Right-click near the sidebar's right edge so the 20-column menu overhangs.
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 25, Y: 2})
	menuX, menuY, menuW, _, ok := model.fileTree.ContextMenuBounds()
	if !ok {
		t.Fatal("expected context menu to be open after right-click")
	}
	if menuX+menuW <= 30 {
		t.Fatalf("test setup: menu does not overhang the sidebar (x=%d w=%d)", menuX, menuW)
	}

	// Click the first item at an X inside the sidebar's editor region.
	// Bounds are content-relative (tab bar excluded), so screen row = menuY+1+1.
	clickX := 35
	if clickX >= menuX+menuW {
		t.Fatalf("test setup: clickX %d outside menu box", clickX)
	}
	model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: clickX, Y: menuY + 2})

	if model.fileTree.HasContextMenu() {
		t.Fatal("click on the menu overhang was routed to the editor; menu still open")
	}
	if !strings.Contains(model.View().Content, "▌") {
		t.Error("click on the menu overhang did not activate the 'New File' item")
	}
}

// ── Viewport: menus taller than the content area ──────────────────────────────

// newShortWindowCtxMenuApp opens a sidebar context menu on a file row in a
// window of the given height so the menu carries all four items (incl. Delete).
func newShortWindowCtxMenuApp(t *testing.T, height int) *Model {
	t.Helper()
	cfg := config.Defaults()
	cfg.Sidebar.Visible = true
	cfg.Sidebar.Width = 30

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	model, err := New(cfg, "", rootDir, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 100, Height: height})
	// Screen row 2 = first content row = the file row (root is content row 0).
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 2})
	if !model.fileTree.HasContextMenu() {
		t.Fatal("expected context menu to be open after right-click")
	}
	return model
}

// TestRightClick_ShortWindow_MenuFitsContentArea is the issue-81 regression test
// at the app level: in a window too short for the whole menu, the box must stay
// inside the content area and every item must stay clickable where it is drawn.
func TestRightClick_ShortWindow_MenuFitsContentArea(t *testing.T) {
	const tabBarHeight = 1
	for _, h := range []int{5, 6, 7, 8} {
		model := newShortWindowCtxMenuApp(t, h)
		menuX, menuY, menuW, menuH, ok := model.fileTree.ContextMenuBounds()
		if !ok {
			t.Fatalf("height %d: expected context menu bounds", h)
		}
		contentRows := h - 2 // tab bar and status bar excluded
		if menuY+menuH > contentRows {
			t.Fatalf("height %d: menu content rows %d..%d exceed the %d-row content area",
				h, menuY, menuY+menuH, contentRows)
		}
		if menuX+menuW > 100 {
			t.Fatalf("height %d: menu cols %d..%d exceed the window width", h, menuX, menuX+menuW)
		}

		// Walk focus to the last item, then click it where the menu draws it.
		for i := 0; i < 3; i++ {
			model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		_, menuY, _, menuH, ok = model.fileTree.ContextMenuBounds()
		if !ok {
			t.Fatalf("height %d: menu closed while navigating", h)
		}
		if menuY+menuH > contentRows {
			t.Fatalf("height %d: menu left the content area while navigating", h)
		}
	}
}

// TestRightClick_ShortWindow_ClickOnScrolledDelete verifies that activating an
// item that is only reachable after scrolling selects that item — not whatever
// item happens to sit at the same row in the unscrolled list.
func TestRightClick_ShortWindow_ClickOnScrolledDelete(t *testing.T) {
	const tabBarHeight = 1
	model := newShortWindowCtxMenuApp(t, 6) // 4 content rows → 2-item viewport

	// Focus the fourth item ("Delete"), which scrolls the viewport.
	for i := 0; i < 3; i++ {
		model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	_, menuY, _, menuH, ok := model.fileTree.ContextMenuBounds()
	if !ok {
		t.Fatal("expected context menu bounds")
	}
	if menuH > 4 {
		t.Fatalf("test setup: menu height %d does not overflow the 4-row content area", menuH)
	}

	// The focused row is the last row of the box (bottom border is below it).
	drawnRow := menuY + menuH - 2
	model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 7, Y: drawnRow + tabBarHeight})

	if !model.fileTree.HasDeleteDialog() {
		t.Fatal("click on the drawn 'Delete' row did not open the delete dialog")
	}
	if model.fileTree.HasContextMenu() {
		t.Error("expected the context menu to close after selecting Delete")
	}
}

// TestWheelOverMenuOverhang_ScrollsMenuViewport verifies that the wheel over the
// part of the menu that spills past the sidebar edge scrolls the menu instead of
// the editor behind it.
func TestWheelOverMenuOverhang_ScrollsMenuViewport(t *testing.T) {
	model := newShortWindowCtxMenuApp(t, 6)

	// Move the menu so it overhangs the sidebar's right edge.
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 6})
	model.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 25, Y: 2})
	menuX, menuY, menuW, _, ok := model.fileTree.ContextMenuBounds()
	if !ok {
		t.Fatal("expected context menu bounds after reopening")
	}
	if menuX+menuW <= 30 {
		t.Fatalf("test setup: menu does not overhang the sidebar (x=%d w=%d)", menuX, menuW)
	}

	scrollX := 30 + 5 // inside the overhang, past the sidebar edge
	if scrollX >= menuX+menuW {
		t.Fatalf("test setup: scrollX %d outside menu box (x=%d w=%d)", scrollX, menuX, menuW)
	}
	model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: scrollX, Y: menuY + 1 + 1})

	if content := model.View().Content; !strings.Contains(content, "\u25b2") {
		t.Error("wheel over the menu overhang did not scroll the menu viewport")
	}
}
