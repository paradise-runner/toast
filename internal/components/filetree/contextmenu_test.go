package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestTheme(t *testing.T) *theme.Manager {
	t.Helper()
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("failed to load theme: %v", err)
	}
	return tm
}

func TestContextMenu_Render_ContainsItems(t *testing.T) {
	tm := newTestTheme(t)
	cm := &ContextMenu{X: 0, Y: 0, items: []string{"New File", "New Folder"}, focused: 0, theme: tm}
	out := cm.Render()
	if !strings.Contains(out, "New File") {
		t.Error("expected 'New File' in context menu render")
	}
	if !strings.Contains(out, "New Folder") {
		t.Error("expected 'New Folder' in context menu render")
	}
}

func TestContextMenu_Render_HighlightsFocused(t *testing.T) {
	tm := newTestTheme(t)
	cm0 := &ContextMenu{X: 0, Y: 0, items: []string{"New File", "New Folder"}, focused: 0, theme: tm}
	cm1 := &ContextMenu{X: 0, Y: 0, items: []string{"New File", "New Folder"}, focused: 1, theme: tm}
	// The renders should differ (different item highlighted)
	if cm0.Render() == cm1.Render() {
		t.Error("expected different renders for focused=0 vs focused=1")
	}
}

func TestNewContextMenu_DefaultItems(t *testing.T) {
	tm := newTestTheme(t)
	cm := newContextMenu(5, 10, "/some/dir", nil, tm)
	if len(cm.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(cm.items))
	}
	if cm.X != 5 || cm.Y != 10 {
		t.Errorf("expected X=5 Y=10, got X=%d Y=%d", cm.X, cm.Y)
	}
	if cm.targetDir != "/some/dir" {
		t.Errorf("expected targetDir '/some/dir', got %q", cm.targetDir)
	}
}

func TestNewContextMenu_IncludesRevealItem(t *testing.T) {
	tm := newTestTheme(t)
	cm := newContextMenu(0, 0, "/some/dir", nil, tm)
	if len(cm.items) < 3 {
		t.Fatalf("expected at least 3 items, got %d", len(cm.items))
	}
	if cm.items[2] != revealItemLabel() {
		t.Errorf("expected items[2]=%q (OS reveal label), got %q", revealItemLabel(), cm.items[2])
	}
}

// ── Integration tests (require filetree.Model) ────────────────────────────────

func newTestModel(t *testing.T, dir string) Model {
	t.Helper()
	tm := newTestTheme(t)
	cfg := config.Defaults()
	m := New(tm, cfg, dir)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	return m
}

func TestContextMenu_RightClickOpensMenu(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu to be set after right-click")
	}
}

func TestContextMenu_EscapeDismisses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.ctxMenu != nil {
		t.Fatal("expected ctxMenu to be nil after Escape")
	}
}

func TestContextMenu_SelectNewFile_EntersInlineEdit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	// focused starts at 0 = "New File"; press Enter
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.inlineInput == nil {
		t.Fatal("expected inlineInput to be set after selecting New File")
	}
	if m.inlineInput.isDir {
		t.Error("expected isDir=false for New File selection")
	}
	if m.ctxMenu != nil {
		t.Error("expected ctxMenu to be cleared after entering inline edit")
	}
}

func TestContextMenu_SelectNewFolder_EntersInlineEdit(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	// Move focus down to "New Folder" (index 1)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.inlineInput == nil {
		t.Fatal("expected inlineInput to be set after selecting New Folder")
	}
	if !m.inlineInput.isDir {
		t.Error("expected isDir=true for New Folder selection")
	}
}

func TestContextMenu_MoveUpDown(t *testing.T) {
	tm := newTestTheme(t)
	cm := newContextMenu(0, 0, "", nil, tm)
	if len(cm.items) != 3 {
		t.Fatalf("expected 3 default items, got %d", len(cm.items))
	}

	// Initial state
	if cm.focused != 0 {
		t.Errorf("expected initial focused=0, got %d", cm.focused)
	}
	// Can't go above 0
	cm.moveUp()
	if cm.focused != 0 {
		t.Errorf("expected focused to stay 0 after moveUp at top, got %d", cm.focused)
	}
	// Move down
	cm.moveDown()
	if cm.focused != 1 {
		t.Errorf("expected focused=1 after moveDown, got %d", cm.focused)
	}
	// Move down to the last item (index 2 = reveal item)
	cm.moveDown()
	if cm.focused != 2 {
		t.Errorf("expected focused=2 after moveDown, got %d", cm.focused)
	}
	// Can't go past last item
	cm.moveDown()
	if cm.focused != 2 {
		t.Errorf("expected focused to stay at 2 after moveDown at bottom, got %d", cm.focused)
	}
	// Move back up
	cm.moveUp()
	if cm.focused != 1 {
		t.Errorf("expected focused=1 after moveUp, got %d", cm.focused)
	}
}

func TestContextMenu_HandleClick_HitsItem(t *testing.T) {
	tm := newTestTheme(t)
	// Menu at (5, 3), 2 items. Item 0 is at row 3+1=4, item 1 at row 5.
	cm := &ContextMenu{X: 5, Y: 3, items: []string{"New File", "New Folder"}, focused: 0, theme: tm}
	// Click directly on item 0
	if got := cm.HandleClick(6, 4); got != 0 {
		t.Errorf("expected item 0, got %d", got)
	}
	// Click directly on item 1
	if got := cm.HandleClick(6, 5); got != 1 {
		t.Errorf("expected item 1, got %d", got)
	}
}

func TestContextMenu_HoverItem_Geometry(t *testing.T) {
	tm := newTestTheme(t)
	// Menu at (5, 3), 2 items. Item 0 is at row 3+1=4, item 1 at row 5.
	cm := &ContextMenu{X: 5, Y: 3, items: []string{"New File", "New Folder"}, focused: 0, theme: tm}
	if got := cm.HoverItem(6, 4); got != 0 {
		t.Errorf("hover item 0: expected 0, got %d", got)
	}
	if got := cm.HoverItem(6, 5); got != 1 {
		t.Errorf("hover item 1: expected 1, got %d", got)
	}
	// Hover on the top border row is not an item.
	if got := cm.HoverItem(6, 3); got != -1 {
		t.Errorf("hover top border: expected -1, got %d", got)
	}
	// Hover outside the box entirely.
	if got := cm.HoverItem(30, 30); got != -1 {
		t.Errorf("hover far outside: expected -1, got %d", got)
	}
}

func TestContextMenu_HandleClick_MissesOutside(t *testing.T) {
	tm := newTestTheme(t)
	cm := &ContextMenu{X: 5, Y: 3, items: []string{"New File", "New Folder"}, focused: 0, theme: tm}
	// Click above menu
	if got := cm.HandleClick(6, 2); got != -1 {
		t.Errorf("expected -1 for click above menu, got %d", got)
	}
	// Click left of menu
	if got := cm.HandleClick(4, 4); got != -1 {
		t.Errorf("expected -1 for click left of menu, got %d", got)
	}
	// Click right of menu (X >= c.X + contextMenuInnerW + 4)
	if got := cm.HandleClick(5+contextMenuInnerW+4, 4); got != -1 {
		t.Errorf("expected -1 for click right of menu, got %d", got)
	}
	// Click on bottom border row (no item)
	if got := cm.HandleClick(6, 3+len(cm.items)+1); got != -1 {
		t.Errorf("expected -1 for bottom border row, got %d", got)
	}
}

func TestNewContextMenu_WithTargetNode_HasDeleteItem(t *testing.T) {
	tm := newTestTheme(t)
	node := &TreeNode{Name: "foo.go", Path: "/tmp/foo.go", IsDir: false}
	cm := newContextMenu(0, 0, "/tmp", node, tm)
	if len(cm.items) != 4 {
		t.Fatalf("expected 4 items with targetNode, got %d", len(cm.items))
	}
	if cm.items[3] != "Delete" {
		t.Errorf("expected items[3]='Delete', got %q", cm.items[3])
	}
	if cm.items[2] != revealItemLabel() {
		t.Errorf("expected items[2]=%q, got %q", revealItemLabel(), cm.items[2])
	}
	if cm.targetPath != "/tmp/foo.go" {
		t.Errorf("expected targetPath '/tmp/foo.go', got %q", cm.targetPath)
	}
	if cm.targetIsDir {
		t.Error("expected targetIsDir=false for a file node")
	}
}

func TestNewContextMenu_WithNilNode_NoDeleteItem(t *testing.T) {
	tm := newTestTheme(t)
	cm := newContextMenu(0, 0, "/tmp", nil, tm)
	if len(cm.items) != 3 {
		t.Fatalf("expected 3 items without targetNode, got %d", len(cm.items))
	}
}

func TestNewContextMenu_WithDirNode_HasDeleteItem(t *testing.T) {
	tm := newTestTheme(t)
	node := &TreeNode{Name: "pkg", Path: "/tmp/pkg", IsDir: true}
	cm := newContextMenu(0, 0, "/tmp/pkg", node, tm)
	if len(cm.items) != 4 {
		t.Fatalf("expected 4 items with dir targetNode, got %d", len(cm.items))
	}
	if !cm.targetIsDir {
		t.Error("expected targetIsDir=true for a dir node")
	}
}

func TestContextMenu_LeftClickActivatesItem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	// Open context menu at (2, 0)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}
	// Left-click on item 0 (New File): item 0 is at row ctxMenu.Y+1
	itemY := m.ctxMenu.Y + 1
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.ctxMenu.X + 2, Y: itemY})
	if m.ctxMenu != nil {
		t.Error("expected ctxMenu to be cleared after left-click activation")
	}
	if m.inlineInput == nil {
		t.Error("expected inlineInput to be set after activating New File via click")
	}
}

func TestContextMenu_LeftClickOutsideDismisses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	// Click far outside menu
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 28, Y: 8})
	if m.ctxMenu != nil {
		t.Error("expected ctxMenu to be dismissed after click outside")
	}
}

func TestContextMenu_RightClickOnNode_HasDeleteItem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	// Right-click on row 0 (which has the file node)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu")
	}
	if len(m.ctxMenu.items) != 4 {
		t.Errorf("expected 4 items for node right-click, got %d", len(m.ctxMenu.items))
	}
	if m.ctxMenu.items[3] != "Delete" {
		t.Errorf("expected items[3]='Delete', got %q", m.ctxMenu.items[3])
	}
}

func TestContextMenu_RightClickEmptySpace_NoDeleteItem(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	// Right-click on row 5 (beyond any nodes — empty space)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 5})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu even for empty space")
	}
	if len(m.ctxMenu.items) != 3 {
		t.Errorf("expected 3 items for empty-space right-click, got %d", len(m.ctxMenu.items))
	}
	if len(m.ctxMenu.items) > 3 && m.ctxMenu.items[3] == "Delete" {
		t.Error("expected no Delete item for empty-space right-click")
	}
}

func TestContextMenu_MouseMotionHover_HighlightsItem(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	// Open context menu at (2, 0). Item 0 = New File (row 1), item 1 = New Folder (row 2).
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}
	if m.ctxMenu.focused != 0 {
		t.Fatalf("expected initial focused=0, got %d", m.ctxMenu.focused)
	}

	// Hover over item 1 (New Folder).
	m, _ = m.Update(tea.MouseMotionMsg{X: m.ctxMenu.X + 2, Y: m.ctxMenu.Y + 2})
	if m.ctxMenu == nil || m.ctxMenu.focused != 1 {
		t.Fatalf("expected focused=1 after hovering item 1, got %d", m.ctxMenu.focused)
	}

	// Hover back over item 0.
	m, _ = m.Update(tea.MouseMotionMsg{X: m.ctxMenu.X + 2, Y: m.ctxMenu.Y + 1})
	if m.ctxMenu == nil || m.ctxMenu.focused != 0 {
		t.Fatalf("expected focused=0 after hovering item 0, got %d", m.ctxMenu.focused)
	}
}

func TestContextMenu_MouseMotionHover_OutsideLeavesFocus(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 0})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}

	// Hover over item 1.
	m, _ = m.Update(tea.MouseMotionMsg{X: m.ctxMenu.X + 2, Y: m.ctxMenu.Y + 2})
	// Move the mouse far outside the menu: focus should stay where it was.
	m, _ = m.Update(tea.MouseMotionMsg{X: 0, Y: 30})
	if m.ctxMenu == nil || m.ctxMenu.focused != 1 {
		t.Fatalf("expected focused to stay 1 after hovering outside, got %d", m.ctxMenu.focused)
	}
}

func TestContextMenu_MouseMotionHover_NoMenu_NoOp(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	// Motion without a menu open must not crash or change state.
	m, _ = m.Update(tea.MouseMotionMsg{X: 2, Y: 1})
	if m.ctxMenu != nil {
		t.Error("expected no ctxMenu from plain mouse motion")
	}
}

func TestContextMenu_Bounds(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	if _, _, _, _, ok := m.ContextMenuBounds(); ok {
		t.Error("expected ContextMenuBounds ok=false with no menu open")
	}
	if m.HasContextMenu() {
		t.Error("expected HasContextMenu=false with no menu open")
	}

	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 3})
	x, y, w, h, ok := m.ContextMenuBounds()
	if !ok {
		t.Fatal("expected ContextMenuBounds ok=true with menu open")
	}
	if x != 2 || y != 3 {
		t.Errorf("expected bounds origin (2,3), got (%d,%d)", x, y)
	}
	if w != contextMenuInnerW+4 {
		t.Errorf("expected width %d, got %d", contextMenuInnerW+4, w)
	}
	if h != m.ctxMenu.visibleCount()+2 {
		t.Errorf("expected height %d, got %d", m.ctxMenu.visibleCount()+2, h)
	}
	if !m.HasContextMenu() {
		t.Error("expected HasContextMenu=true with menu open")
	}
}

// ── Reveal in file manager (View in Finder / View in Folder) ──────────────────

// TestContextMenu_SelectReveal_OnNode_EmitsMsg verifies that activating the
// reveal item (index 2) on a file emits RevealInFileManagerMsg with the node's
// path and clears the context menu.
func TestContextMenu_SelectReveal_OnNode_EmitsMsg(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	// flat[0]=root, flat[1]=main.go — right-click the file row.
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}
	// Move focus 0 → 1 → 2 (reveal item).
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.ctxMenu != nil {
		t.Error("expected ctxMenu to be cleared after selecting reveal")
	}
	if cmd == nil {
		t.Fatal("expected RevealInFileManagerMsg cmd")
	}
	msg := cmd()
	rm, ok := msg.(messages.RevealInFileManagerMsg)
	if !ok {
		t.Fatalf("expected RevealInFileManagerMsg, got %T", msg)
	}
	if rm.Path != filePath {
		t.Errorf("RevealInFileManagerMsg.Path = %q, want %q", rm.Path, filePath)
	}
}

// TestContextMenu_SelectReveal_EmptySpace_EmitsMsg verifies that activating the
// reveal item after right-clicking empty space reveals the target directory.
func TestContextMenu_SelectReveal_EmptySpace_EmitsMsg(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir)
	// Right-click empty space (row 5 — beyond any nodes).
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 5})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}
	// Move focus 0 → 1 → 2 (reveal item).
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected RevealInFileManagerMsg cmd")
	}
	msg := cmd()
	rm, ok := msg.(messages.RevealInFileManagerMsg)
	if !ok {
		t.Fatalf("expected RevealInFileManagerMsg, got %T", msg)
	}
	if rm.Path != dir {
		t.Errorf("RevealInFileManagerMsg.Path = %q, want %q", rm.Path, dir)
	}
}

// TestContextMenu_LeftClickReveal_EmitsMsg verifies the reveal item works via
// mouse click as well.
func TestContextMenu_LeftClickReveal_EmitsMsg(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
	if m.ctxMenu == nil {
		t.Fatal("expected ctxMenu after right-click")
	}
	// Left-click item 2 (reveal item): row = ctxMenu.Y + 1 + 2.
	m, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.ctxMenu.X + 2, Y: m.ctxMenu.Y + 3})
	if m.ctxMenu != nil {
		t.Error("expected ctxMenu to be cleared after clicking reveal")
	}
	if cmd == nil {
		t.Fatal("expected RevealInFileManagerMsg cmd")
	}
	msg := cmd()
	rm, ok := msg.(messages.RevealInFileManagerMsg)
	if !ok {
		t.Fatalf("expected RevealInFileManagerMsg, got %T", msg)
	}
	if rm.Path != filePath {
		t.Errorf("RevealInFileManagerMsg.Path = %q, want %q", rm.Path, filePath)
	}
}

// ── Overlay clamping ─────────────────────────────────────────────────────────
//
// The context menu is composited over the whole screen, so its stored position
// must already fit the visible area. If the renderer has to shift the box (as
// overlayAt does as a safety net), clicks keep testing the unshifted position
// and land on the wrong item or outside the box entirely.

func TestContextMenu_Clamp_KeepsBoxOnScreen(t *testing.T) {
	items := []string{"New File", "New Folder", revealItemLabel()}
	boxW := contextMenuInnerW + 4 // boxWidth, independent of item count
	tests := []struct {
		name       string
		x, y       int
		maxW, maxH int
		wantX      int
		wantY      int
	}{
		{name: "already fits", x: 2, y: 3, maxW: 100, maxH: 40, wantX: 2, wantY: 3},
		{name: "flips up at bottom", x: 2, y: 38, maxW: 100, maxH: 40, wantX: 2, wantY: 35},
		{name: "slides left at right edge", x: 95, y: 3, maxW: 100, maxH: 40, wantX: 100 - boxW, wantY: 3},
		{name: "clamps negative", x: -5, y: -2, maxW: 100, maxH: 40, wantX: 0, wantY: 0},
		{name: "unbounded stays put", x: 90, y: 90, maxW: 0, maxH: 0, wantX: 90, wantY: 90},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm := &ContextMenu{X: tc.x, Y: tc.y, items: items}
			cm.clamp(tc.maxW, tc.maxH)
			if cm.X != tc.wantX || cm.Y != tc.wantY {
				t.Errorf("clamp(%d,%d) from (%d,%d) = (%d,%d), want (%d,%d)",
					tc.maxW, tc.maxH, tc.x, tc.y, cm.X, cm.Y, tc.wantX, tc.wantY)
			}
			if tc.maxW > 0 && cm.X+cm.boxWidth() > tc.maxW {
				t.Errorf("box width overflows: X=%d w=%d maxW=%d", cm.X, cm.boxWidth(), tc.maxW)
			}
			if tc.maxH > 0 && cm.Y+cm.boxHeight() > tc.maxH {
				t.Errorf("box height overflows: Y=%d h=%d maxH=%d", cm.Y, cm.boxHeight(), tc.maxH)
			}
		})
	}
}

// TestContextMenu_RightClickNearBottom_FlipsUpAndStaysClickable covers the
// reported bug: right-clicking on the last rows of the tree used to open a menu
// that the compositor shifted upward, after which clicking the drawn "New File"
// row missed the box (or hit the wrong item).
func TestContextMenu_RightClickNearBottom_FlipsUpAndStaysClickable(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir) // 30x10 viewport

	// Bottom row of the tree.
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 9})
	if m.ctxMenu == nil {
		t.Fatal("expected context menu after right-click")
	}
	if got, want := m.ctxMenu.Y, 10-m.ctxMenu.boxHeight(); got != want {
		t.Fatalf("menu Y = %d, want %d (flipped up to fit %d rows)", got, want, m.height)
	}
	if m.ctxMenu.Y+m.ctxMenu.boxHeight() > m.height {
		t.Fatalf("menu box (%d rows from Y=%d) exceeds viewport height %d",
			m.ctxMenu.boxHeight(), m.ctxMenu.Y, m.height)
	}

	// Clicking the first item where the menu is drawn must select "New File".
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 4, Y: m.ctxMenu.Y + 1})
	if m.ctxMenu != nil {
		t.Fatal("click on the drawn 'New File' row missed the menu box")
	}
	if m.inlineInput == nil {
		t.Fatal("click on 'New File' did not start the inline create row")
	}
	if m.inlineInput.isDir {
		t.Error("click on 'New File' started a folder create row")
	}
}

// TestContextMenu_RightEdge_SlidesLeftAndStaysClickable covers the horizontal
// case: a wide sidebar lets the menu overflow the window, which the compositor
// used to shift left while hit-testing kept the original X.
func TestContextMenu_RightEdge_SlidesLeftAndStaysClickable(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir) // 30 cols wide
	m.SetScreenWidth(30)

	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 28, Y: 2})
	if m.ctxMenu == nil {
		t.Fatal("expected context menu after right-click")
	}
	if got, want := m.ctxMenu.X, 30-m.ctxMenu.boxWidth(); got != want {
		t.Fatalf("menu X = %d, want %d (slid left to fit 30 cols)", got, want)
	}

	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.ctxMenu.X + 2, Y: m.ctxMenu.Y + 1})
	if m.ctxMenu != nil {
		t.Fatal("click on the drawn first item missed the menu box")
	}
	if m.inlineInput == nil {
		t.Fatal("click on 'New File' did not start the inline create row")
	}
}

// TestContextMenu_ResizeReclamps verifies that shrinking the window while the
// menu is open repositions it, so it stays clickable where it is drawn.
func TestContextMenu_ResizeReclamps(t *testing.T) {
	dir := t.TempDir()
	m := newTestModel(t, dir) // 30x10
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 3})
	if m.ctxMenu == nil {
		t.Fatal("expected context menu after right-click")
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 6})
	if m.ctxMenu.Y+m.ctxMenu.boxHeight() > m.height {
		t.Fatalf("menu not re-clamped after resize: Y=%d h=%d height=%d",
			m.ctxMenu.Y, m.ctxMenu.boxHeight(), m.height)
	}
}

// ── Viewport: menus taller than the window ───────────────────────────────────
//
// A menu with more items than the content area can show must scroll inside a
// viewport that fits the window. The drawn rows and the hit-test rows stay
// one-to-one (row i ↔ items[scroll+i]) so clicks land on the item they overlap.

// contextMenuTestItems returns the four-item menu (incl. Delete) a right-click on
// a file row produces.
func contextMenuTestItems() []string {
	return []string{"New File", "New Folder", revealItemLabel(), "Delete"}
}

func TestContextMenu_SetRows_FitsItems(t *testing.T) {
	items := contextMenuTestItems()
	tests := []struct {
		name       string
		maxHeight  int
		focused    int
		wantRows   int
		wantScroll int
	}{
		{name: "unlimited", maxHeight: 0, focused: 0, wantRows: 0, wantScroll: 0},
		{name: "taller than items", maxHeight: 40, focused: 3, wantRows: 0, wantScroll: 0},
		{name: "exactly fits items", maxHeight: 6, focused: 0, wantRows: 0, wantScroll: 0},
		{name: "one item hidden", maxHeight: 5, focused: 0, wantRows: 3, wantScroll: 0},
		{name: "two rows", maxHeight: 4, focused: 0, wantRows: 2, wantScroll: 0},
		{name: "single row", maxHeight: 3, focused: 0, wantRows: 1, wantScroll: 0},
		{name: "focus below viewport scrolls", maxHeight: 4, focused: 3, wantRows: 2, wantScroll: 2},
		{name: "degenerate height keeps one row", maxHeight: 2, focused: 1, wantRows: 1, wantScroll: 1},
		{name: "height of one keeps one row", maxHeight: 1, focused: 0, wantRows: 1, wantScroll: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cm := &ContextMenu{items: items, focused: tc.focused}
			cm.setRows(tc.maxHeight)
			if cm.rows != tc.wantRows {
				t.Errorf("rows = %d, want %d", cm.rows, tc.wantRows)
			}
			if cm.scroll != tc.wantScroll {
				t.Errorf("scroll = %d, want %d", cm.scroll, tc.wantScroll)
			}
			if got, want := cm.boxHeight(), cm.visibleCount()+2; got != want {
				t.Errorf("boxHeight = %d, want %d", got, want)
			}
			if tc.maxHeight >= 3 && cm.boxHeight() > tc.maxHeight {
				t.Errorf("boxHeight %d exceeds maxHeight %d", cm.boxHeight(), tc.maxHeight)
			}
			if cm.visibleCount() > len(cm.items)-cm.scroll {
				t.Errorf("viewport shows %d items but only %d remain from scroll=%d",
					cm.visibleCount(), len(cm.items)-cm.scroll, cm.scroll)
			}
		})
	}
}

func TestContextMenu_ScrollBy_Bounded(t *testing.T) {
	cm := &ContextMenu{items: contextMenuTestItems(), rows: 2}
	cm.scrollBy(-3)
	if cm.scroll != 0 {
		t.Errorf("scroll = %d, want 0 after scrolling up past the first item", cm.scroll)
	}
	cm.scrollBy(1)
	if cm.scroll != 1 {
		t.Errorf("scroll = %d, want 1", cm.scroll)
	}
	cm.scrollBy(99)
	if want := len(cm.items) - cm.rows; cm.scroll != want {
		t.Errorf("scroll = %d, want %d (last full page)", cm.scroll, want)
	}
	// Without a viewport the menu never scrolls.
	unbounded := &ContextMenu{items: contextMenuTestItems()}
	unbounded.scrollBy(2)
	if unbounded.scroll != 0 {
		t.Errorf("unbounded menu scrolled to %d, want 0", unbounded.scroll)
	}
}

func TestContextMenu_HandleClick_MapsThroughScroll(t *testing.T) {
	cm := &ContextMenu{X: 5, Y: 3, items: contextMenuTestItems(), rows: 2, scroll: 2}
	// Row Y+1 is the first visible row → item index scroll+0 = 2.
	if got := cm.HandleClick(6, 4); got != 2 {
		t.Errorf("first visible row: got %d, want 2", got)
	}
	if got := cm.HandleClick(6, 5); got != 3 {
		t.Errorf("second visible row: got %d, want 3", got)
	}
	// Bottom border row is not an item.
	if got := cm.HandleClick(6, 6); got != -1 {
		t.Errorf("bottom border: got %d, want -1", got)
	}
	// Hover uses the same mapping.
	if got := cm.HoverItem(6, 5); got != 3 {
		t.Errorf("hover second visible row: got %d, want 3", got)
	}
}

// TestContextMenu_ShortViewport_EveryItemReachable is the issue-81 regression
// test: in a window too short for the whole menu, every item must still be
// reachable by keyboard and clickable where it is drawn.
func TestContextMenu_ShortViewport_EveryItemReachable(t *testing.T) {
	tests := []struct {
		name   string
		index  int
		verify func(t *testing.T, m Model)
	}{
		{name: "New File", index: 0, verify: func(t *testing.T, m Model) {
			t.Helper()
			if m.inlineInput == nil {
				t.Fatal("click on 'New File' did not start the inline create row")
			}
			if m.inlineInput.isDir {
				t.Error("click on 'New File' started a folder create row")
			}
		}},
		{name: "New Folder", index: 1, verify: func(t *testing.T, m Model) {
			t.Helper()
			if m.inlineInput == nil {
				t.Fatal("click on 'New Folder' did not start the inline create row")
			}
			if !m.inlineInput.isDir {
				t.Error("click on 'New Folder' started a file create row")
			}
		}},
		{name: "Reveal", index: 2, verify: func(t *testing.T, m Model) {
			t.Helper()
			if m.deleteDialog != nil {
				t.Error("click on the reveal item opened the delete dialog")
			}
		}},
		{name: "Delete", index: 3, verify: func(t *testing.T, m Model) {
			t.Helper()
			if m.deleteDialog == nil {
				t.Error("click on 'Delete' did not open the delete dialog")
			}
			if m.inlineInput != nil {
				t.Error("click on 'Delete' started an inline create row")
			}
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			m := newTestModel(t, dir)
			// Content area of 5 rows: room for 3 item rows plus borders.
			m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 5})

			// Right-click the file row so the menu carries all four items.
			m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
			if m.ctxMenu == nil {
				t.Fatal("expected context menu after right-click")
			}
			if got, want := len(m.ctxMenu.items), 4; got != want {
				t.Fatalf("menu has %d items, want %d", got, want)
			}

			// Walk down to the target item, checking the box never leaves the
			// content area and the focused item stays inside the viewport.
			for i := 0; i < tc.index; i++ {
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
				assertMenuFits(t, m)
				c := m.ctxMenu
				if c.focused < c.scroll || c.focused >= c.scroll+c.visibleCount() {
					t.Fatalf("focused %d outside viewport [%d,%d)",
						c.focused, c.scroll, c.scroll+c.visibleCount())
				}
			}
			if got := m.ctxMenu.focused; got != tc.index {
				t.Fatalf("focused = %d, want %d", got, tc.index)
			}

			// Click the row where the focused item is drawn.
			c := m.ctxMenu
			drawnY := c.Y + 1 + (c.focused - c.scroll)
			if idx := c.HandleClick(c.X+2, drawnY); idx != tc.index {
				t.Fatalf("HandleClick at the drawn row = %d, want %d", idx, tc.index)
			}
			m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: c.X + 2, Y: drawnY})
			if m.ctxMenu != nil {
				t.Fatal("expected ctxMenu to close after activating an item")
			}
			tc.verify(t, m)
		})
	}
}

// TestContextMenu_ShortViewport_ShowsScrollMarkers verifies the viewport
// advertises the items it hides.
func TestContextMenu_ShortViewport_ShowsScrollMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 5})
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
	if m.ctxMenu == nil {
		t.Fatal("expected context menu after right-click")
	}

	if out := m.ctxMenu.Render(); !strings.Contains(out, markerDown) {
		t.Errorf("expected a down marker while items are hidden below:\n%s", out)
	}
	if out := m.ctxMenu.Render(); strings.Contains(out, markerUp) {
		t.Errorf("unexpected up marker at the top of the list:\n%s", out)
	}

	// Scroll to the end: the marker flips.
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 2, Y: 2})
	if out := m.ctxMenu.Render(); !strings.Contains(out, markerUp) {
		t.Errorf("expected an up marker after scrolling down:\n%s", out)
	}
}

// TestContextMenu_WheelScrollsViewport verifies the wheel moves the menu
// viewport instead of scrolling the tree behind it.
func TestContextMenu_WheelScrollsViewport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 4}) // 2 item rows
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
	if m.ctxMenu == nil {
		t.Fatal("expected context menu after right-click")
	}
	if got, want := m.ctxMenu.visibleCount(), 2; got != want {
		t.Fatalf("visibleCount = %d, want %d", got, want)
	}
	treeOffset := m.offset

	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 2, Y: 2})
	if m.ctxMenu.scroll != 1 {
		t.Errorf("scroll = %d, want 1 after one wheel down", m.ctxMenu.scroll)
	}
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 2, Y: 2})
	if m.ctxMenu.scroll != 0 {
		t.Errorf("scroll = %d, want 0 after one wheel up", m.ctxMenu.scroll)
	}
	m, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: 2, Y: 2})
	if m.ctxMenu.scroll != 0 {
		t.Errorf("scroll = %d, want 0 (bounded)", m.ctxMenu.scroll)
	}
	if m.offset != treeOffset {
		t.Errorf("tree offset moved to %d (was %d): wheel leaked past the menu", m.offset, treeOffset)
	}
}

// TestContextMenu_RenderHeightMatchesBounds pins the invariant that keeps the
// compositor from shifting the box: the rendered height/width must equal the
// geometry used for hit-testing, for every window height.
func TestContextMenu_RenderHeightMatchesBounds(t *testing.T) {
	for h := 3; h <= 12; h++ {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		m := newTestModel(t, dir)
		m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: h})
		m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
		if m.ctxMenu == nil {
			t.Fatalf("height %d: expected context menu after right-click", h)
		}

		out, x, y, ok := m.ContextMenuOverlay()
		bx, by, bw, bh, okBounds := m.ContextMenuBounds()
		if !ok || !okBounds {
			t.Fatalf("height %d: expected overlay and bounds", h)
		}
		if x != bx || y != by {
			t.Errorf("height %d: overlay at (%d,%d), bounds at (%d,%d)", h, x, y, bx, by)
		}

		lines := strings.Split(out, "\n")
		if len(lines) != bh {
			t.Errorf("height %d: rendered %d rows, bounds say %d", h, len(lines), bh)
		}
		if w := lipgloss.Width(lines[0]); w != bw {
			t.Errorf("height %d: rendered width %d, bounds say %d", h, w, bw)
		}
		// The box must fit the content area; overlayAt must never have to shift it.
		if by+bh > m.height {
			t.Errorf("height %d: menu box rows %d..%d exceed content area", h, by, by+bh)
		}
		if bx+bw > m.screenWidth && m.screenWidth > 0 {
			t.Errorf("height %d: menu box cols %d..%d exceed window width %d",
				h, bx, bx+bw, m.screenWidth)
		}
	}
}

// TestContextMenu_ResizeShrinkKeepsFocusVisible verifies that shrinking the
// window while a late item is focused scrolls it back into view.
func TestContextMenu_ResizeShrinkKeepsFocusVisible(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, dir) // 30x10
	m, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 2, Y: 1})
	for i := 0; i < 3; i++ { // focus the "Delete" item
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ctxMenu.focused != 3 {
		t.Fatalf("focused = %d, want 3", m.ctxMenu.focused)
	}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 4})
	assertMenuFits(t, m)
	c := m.ctxMenu
	if c.focused < c.scroll || c.focused >= c.scroll+c.visibleCount() {
		t.Fatalf("focused %d left the viewport [%d,%d) after resize",
			c.focused, c.scroll, c.scroll+c.visibleCount())
	}
	// It must still be clickable where it is drawn.
	drawnY := c.Y + 1 + (c.focused - c.scroll)
	if idx := c.HandleClick(c.X+2, drawnY); idx != 3 {
		t.Fatalf("HandleClick at the drawn focused row = %d, want 3", idx)
	}
}

// assertMenuFits fails if the open context menu does not fit the tree's content
// area.
func assertMenuFits(t *testing.T, m Model) {
	t.Helper()
	if m.ctxMenu == nil {
		t.Fatal("expected an open context menu")
	}
	if bottom := m.ctxMenu.Y + m.ctxMenu.boxHeight(); bottom > m.height {
		t.Fatalf("menu rows %d..%d exceed the %d-row content area",
			m.ctxMenu.Y, bottom, m.height)
	}
	if m.ctxMenu.Y < 0 {
		t.Fatalf("menu Y = %d, want >= 0", m.ctxMenu.Y)
	}
}
