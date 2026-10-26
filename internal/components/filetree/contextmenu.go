package filetree

import (
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/theme"
)

const (
	// contextMenuInnerW is the content width of the box: the longest item label
	// ("View in Explorer", 16 cells) plus the two-cell scroll marker column.
	contextMenuInnerW = 18
)

// Scroll markers drawn in the two-cell marker column at the left of an item row
// when the viewport hides items above or below. Unused markers render as
// spaces, so labels stay aligned and rows map one-to-one to items.
const (
	markerUpDown = "\u2195 " // ↕  items hidden on both sides (single-row viewport)
	markerUp     = "\u25b2 " // ▲  items hidden above
	markerDown   = "\u25bc " // ▼  items hidden below
	markerNone   = "  "
)

// ContextMenu is a small floating menu shown on right-click in the sidebar.
type ContextMenu struct {
	X, Y        int
	targetDir   string // parent dir for New File / New Folder
	targetPath  string // exact path to delete (empty when targetNode==nil)
	targetIsDir bool   // true if targetNode is a directory
	items       []string
	focused     int // absolute item index; never viewport-relative
	scroll      int // index of the first visible item
	rows        int // visible item rows; 0 means "show every item"
	theme       *theme.Manager
}

// newContextMenu creates a context menu at the given sidebar-relative position.
// targetDir is the directory in which New File / New Folder will be created.
// targetNode, when non-nil, causes a "Delete" item to be appended and
// targetPath / targetIsDir to be populated from the node.
//
// Item layout:
//
//	0 = New File
//	1 = New Folder
//	2 = View in Finder / View in Folder / View in Explorer (reveal targetDir
//	    when no node is targeted, otherwise reveal the node itself)
//	3 = Delete (only when targetNode != nil)
func newContextMenu(x, y int, targetDir string, targetNode *TreeNode, tm *theme.Manager) *ContextMenu {
	items := []string{"New File", "New Folder", revealItemLabel()}
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

// revealItemLabel returns the OS-appropriate label for the "reveal in file
// manager" menu item.
func revealItemLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "View in Finder"
	case "windows":
		return "View in Explorer"
	default:
		return "View in Folder"
	}
}

// boxWidth returns the rendered width of the menu box in columns:
// content width plus 1 column of padding and 1 of border on each side.
func (c ContextMenu) boxWidth() int { return contextMenuInnerW + 4 }

// boxHeight returns the rendered height of the menu box in rows:
// top border + visible items + bottom border. It follows the viewport, so a
// menu taller than the window shrinks instead of drawing off-screen.
func (c ContextMenu) boxHeight() int { return c.visibleCount() + 2 }

// visibleCount returns how many item rows the viewport currently draws.
func (c ContextMenu) visibleCount() int {
	if c.rows > 0 && c.rows < len(c.items) {
		return c.rows
	}
	return len(c.items)
}

// maxScroll returns the largest scroll offset that still fills the viewport.
func (c *ContextMenu) maxScroll() int {
	if c.rows <= 0 {
		return 0
	}
	if maxOffset := len(c.items) - c.rows; maxOffset > 0 {
		return maxOffset
	}
	return 0
}

// setRows fits the viewport to a menu that may occupy at most maxHeight rows,
// borders included. A non-positive maxHeight disables the viewport. At least one
// item row is always kept so a degenerate window still shows a clickable item.
func (c *ContextMenu) setRows(maxHeight int) {
	if maxHeight <= 0 || maxHeight-2 >= len(c.items) {
		c.rows, c.scroll = 0, 0
		return
	}
	avail := maxHeight - 2
	if avail < 1 {
		avail = 1
	}
	c.rows = avail
	c.ensureVisible()
}

// ensureVisible scrolls the viewport just enough to contain the focused item.
func (c *ContextMenu) ensureVisible() {
	if c.rows <= 0 {
		c.scroll = 0
		return
	}
	if c.focused < c.scroll {
		c.scroll = c.focused
	}
	if c.focused >= c.scroll+c.rows {
		c.scroll = c.focused - c.rows + 1
	}
	c.clampScroll()
}

// scrollBy moves the viewport by delta rows, bounded to the item list.
func (c *ContextMenu) scrollBy(delta int) {
	c.scroll += delta
	c.clampScroll()
}

// clampScroll keeps scroll within [0, maxScroll].
func (c *ContextMenu) clampScroll() {
	if c.scroll > c.maxScroll() {
		c.scroll = c.maxScroll()
	}
	if c.scroll < 0 {
		c.scroll = 0
	}
}

// clamp repositions the menu so its whole box stays inside a maxWidth ×
// maxHeight area and fits its items into that same height. A non-positive
// bound disables clamping on that axis.
//
// This must happen before rendering: the overlay compositor also clamps, and a
// silent shift there would draw the menu somewhere other than the box that
// HandleClick tests against, leaving items unclickable or selecting the wrong
// one. Clamping the stored position — and the viewport, which changes
// boxHeight — keeps drawing and hit-testing in sync.
func (c *ContextMenu) clamp(maxWidth, maxHeight int) {
	// The viewport changes the box height, so it must be fitted first.
	c.setRows(maxHeight)

	if maxWidth > 0 {
		if maxX := maxWidth - c.boxWidth(); c.X > maxX {
			c.X = maxX
		}
		if c.X < 0 {
			c.X = 0
		}
	}
	if maxHeight > 0 {
		if maxY := maxHeight - c.boxHeight(); c.Y > maxY {
			c.Y = maxY
		}
		if c.Y < 0 {
			c.Y = 0
		}
	}
}

// HandleClick returns the index of the item at sidebar-local (x, y),
// or -1 if the click missed all items or landed outside the menu box.
//
// Box geometry:
//   - X span: [c.X, c.X + boxWidth())
//   - Y span: [c.Y, c.Y + boxHeight())
//   - Item i is at row c.Y + 1 + i
func (c *ContextMenu) HandleClick(x, y int) int {
	boxRight := c.X + c.boxWidth()
	boxBottom := c.Y + c.boxHeight()

	if x < c.X || x >= boxRight || y < c.Y || y >= boxBottom {
		return -1
	}
	// Top border row and bottom border row are not items.
	itemRow := y - c.Y - 1
	if itemRow < 0 || itemRow >= c.visibleCount() {
		return -1
	}
	// Rows map to items through the scroll offset, so a scrolled menu still
	// reports the absolute item index the caller switches on.
	return c.scroll + itemRow
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

	first := c.scroll
	count := c.visibleCount()
	// Reserve the marker column only while the viewport can scroll, so a menu
	// that shows every item keeps its original label alignment.
	scrollable := c.rows > 0

	var rows []string
	for i := first; i < first+count; i++ {
		itemStyle := base
		if i == c.focused {
			itemStyle = sel
		}
		label := c.items[i]
		if scrollable {
			marker := markerNone
			above, below := first > 0, first+count < len(c.items)
			switch {
			case i == first && above && i == first+count-1 && below:
				marker = markerUpDown
			case i == first && above:
				marker = markerUp
			case i == first+count-1 && below:
				marker = markerDown
			}
			label = marker + label
		}
		label = ansi.Truncate(label, contextMenuInnerW, "\u2026")
		rows = append(rows, itemStyle.Width(contextMenuInnerW).Render(label))
	}

	body := strings.Join(rows, "\n")
	return boxStyle.Render(body)
}

// HoverItem returns the index of the item under filetree-local (x, y),
// or -1 if the pointer is not over an item (outside the box or on a border row).
// Used for mouseover highlighting; geometry matches HandleClick.
func (c *ContextMenu) HoverItem(x, y int) int {
	return c.HandleClick(x, y)
}

// moveUp moves focus up, clamped to 0, scrolling the viewport to follow.
func (c *ContextMenu) moveUp() {
	if c.focused > 0 {
		c.focused--
	}
	c.ensureVisible()
}

// moveDown moves focus down, clamped to len(items)-1, scrolling the viewport to
// follow.
func (c *ContextMenu) moveDown() {
	if c.focused < len(c.items)-1 {
		c.focused++
	}
	c.ensureVisible()
}
