package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
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
	if len(cm.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cm.items))
	}
	if cm.X != 5 || cm.Y != 10 {
		t.Errorf("expected X=5 Y=10, got X=%d Y=%d", cm.X, cm.Y)
	}
	if cm.targetDir != "/some/dir" {
		t.Errorf("expected targetDir '/some/dir', got %q", cm.targetDir)
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
	// Can't go past last item
	cm.moveDown()
	if cm.focused != 1 {
		t.Errorf("expected focused to stay at 1 after moveDown at bottom, got %d", cm.focused)
	}
	// Move back up
	cm.moveUp()
	if cm.focused != 0 {
		t.Errorf("expected focused=0 after moveUp, got %d", cm.focused)
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
	if len(cm.items) != 3 {
		t.Fatalf("expected 3 items with targetNode, got %d", len(cm.items))
	}
	if cm.items[2] != "Delete" {
		t.Errorf("expected items[2]='Delete', got %q", cm.items[2])
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
	if len(cm.items) != 2 {
		t.Fatalf("expected 2 items without targetNode, got %d", len(cm.items))
	}
}

func TestNewContextMenu_WithDirNode_HasDeleteItem(t *testing.T) {
	tm := newTestTheme(t)
	node := &TreeNode{Name: "pkg", Path: "/tmp/pkg", IsDir: true}
	cm := newContextMenu(0, 0, "/tmp/pkg", node, tm)
	if len(cm.items) != 3 {
		t.Fatalf("expected 3 items with dir targetNode, got %d", len(cm.items))
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
	if len(m.ctxMenu.items) != 3 {
		t.Errorf("expected 3 items for node right-click, got %d", len(m.ctxMenu.items))
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
	if len(m.ctxMenu.items) != 2 {
		t.Errorf("expected 2 items for empty-space right-click, got %d", len(m.ctxMenu.items))
	}
}
