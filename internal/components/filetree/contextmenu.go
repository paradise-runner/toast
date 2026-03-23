package filetree

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/theme"
)

const (
	contextMenuInnerW = 16 // content width: "Delete" (6) + padding to 16
)

// ContextMenu is a small floating menu shown on right-click in the sidebar.
type ContextMenu struct {
	X, Y        int
	targetDir   string // parent dir for New File / New Folder
	targetPath  string // exact path to delete (empty when targetNode==nil)
	targetIsDir bool   // true if targetNode is a directory
	items       []string
	focused     int
	theme       *theme.Manager
}

// newContextMenu creates a context menu at the given sidebar-relative position.
// targetDir is the directory in which New File / New Folder will be created.
// targetNode, when non-nil, causes a "Delete" item to be appended and
// targetPath / targetIsDir to be populated from the node.
func newContextMenu(x, y int, targetDir string, targetNode *TreeNode, tm *theme.Manager) *ContextMenu {
	items := []string{"New File", "New Folder"}
	c := &ContextMenu{
		X:         x,
		Y:         y,
		targetDir: targetDir,
		items:     items,
		theme:     tm,
	}
	if targetNode != nil {
		c.items = append(c.items, "Delete")
		c.targetPath = targetNode.Path
		c.targetIsDir = targetNode.IsDir
	}
	return c
}

// HandleClick returns the index of the item at sidebar-local (x, y),
// or -1 if the click missed all items or landed outside the menu box.
//
// Box geometry:
//   - X span: [c.X, c.X + contextMenuInnerW + 4)   (border 1 + padding 1 on each side = 4)
//   - Y span: [c.Y, c.Y + len(c.items) + 2)         (top border + items + bottom border)
//   - Item i is at row c.Y + 1 + i
func (c *ContextMenu) HandleClick(x, y int) int {
	boxRight := c.X + contextMenuInnerW + 4
	boxBottom := c.Y + len(c.items) + 2

	if x < c.X || x >= boxRight || y < c.Y || y >= boxBottom {
		return -1
	}
	// Top border row and bottom border row are not items.
	itemRow := y - c.Y - 1
	if itemRow < 0 || itemRow >= len(c.items) {
		return -1
	}
	return itemRow
}

// Render returns the menu as a lipgloss-styled string for overlay composition.
func (c ContextMenu) Render() string {
	base := lipgloss.NewStyle()
	if bg := c.theme.UI("sidebar_bg"); bg != "" {
		base = base.Background(lipgloss.Color(bg))
	}
	if fg := c.theme.UI("sidebar_fg"); fg != "" {
		base = base.Foreground(lipgloss.Color(fg))
	}

	sel := base
	if selBG := c.theme.UI("completion_selected"); selBG != "" {
		sel = sel.Background(lipgloss.Color(selBG))
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)
	if borderColor := c.theme.UI("hover_border"); borderColor != "" {
		boxStyle = boxStyle.BorderForeground(lipgloss.Color(borderColor))
	}
	if bg := c.theme.UI("sidebar_bg"); bg != "" {
		boxStyle = boxStyle.
			Background(lipgloss.Color(bg)).
			BorderBackground(lipgloss.Color(bg))
	}

	var rows []string
	for i, item := range c.items {
		itemStyle := base
		if i == c.focused {
			itemStyle = sel
		}
		rows = append(rows, itemStyle.Width(contextMenuInnerW).Render(item))
	}

	body := strings.Join(rows, "\n")
	return boxStyle.Render(body)
}

// moveUp moves focus up, clamped to 0.
func (c *ContextMenu) moveUp() {
	if c.focused > 0 {
		c.focused--
	}
}

// moveDown moves focus down, clamped to len(items)-1.
func (c *ContextMenu) moveDown() {
	if c.focused < len(c.items)-1 {
		c.focused++
	}
}
