package filetree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	// dragExpandDelay is how long the pointer must dwell on a collapsed
	// directory during a drag before it expands.
	dragExpandDelay = 500 * time.Millisecond
	// dragScrollInterval is the auto-scroll cadence while the pointer is
	// dragged past the top or bottom edge of the tree.
	dragScrollInterval = 60 * time.Millisecond
)

// dragExpandTickMsg fires after the hover-to-expand dwell. path identifies the
// directory that was hovered when the timer was armed.
type dragExpandTickMsg struct{ path string }

// dragScrollTickMsg advances the edge auto-scroll loop one row.
type dragScrollTickMsg struct{}

// Model is the Bubbletea model for the file tree sidebar.
type Model struct {
	theme   *theme.Manager
	cfg     config.Config
	root    *TreeNode
	rootDir string
	flat    []*TreeNode
	cursor  int
	offset  int
	height  int
	width   int
	// screenWidth is the full window width in columns. The context menu is
	// composited over the whole screen, so it is clamped against this rather
	// than against the sidebar width.
	screenWidth int
	focused     bool

	gitStatuses map[string]messages.GitStatus

	// ── Context menu & inline edit ────────────────────────────────────────────────
	ctxMenu         *ContextMenu
	deleteDialog    *DeleteConfirmDialog
	inlineInput     *InlineInput
	inlineInsertIdx int // index in flat where the edit row is inserted after

	// ── Drag & drop ───────────────────────────────────────────────────────────────
	drag              dragState
	dragScrolling     bool   // edge auto-scroll loop is running
	dragExpandPending string // collapsed dir awaiting the hover-to-expand dwell
}

// dragState tracks a left-button press that may become a file move. A press
// only records the source row; moving the pointer onto a different row while
// the button is held activates the drag, and release either activates the node
// (plain click) or performs the move (drag).
type dragState struct {
	fromIdx int // flat index pressed, or -1 when idle
	from    *TreeNode
	active  bool
	dropIdx int // flat index of the current drop target, -1 when invalid
	edge    int // -1 = pointer above the viewport, +1 = below, 0 = inside
	mouseX  int // last tree-local pointer X, for the drag ghost label
	mouseY  int // last tree-local pointer Y
}

// idleDrag returns the zero drag state (no pending press).
func idleDrag() dragState { return dragState{fromIdx: -1, dropIdx: -1} }

// clearDrag abandons any pending press or in-flight drag and its timers.
func (m *Model) clearDrag() {
	m.drag = idleDrag()
	m.dragScrolling = false
	m.dragExpandPending = ""
}

// IsDragging reports whether a file/folder drag is in flight. The app uses
// this to keep routing mouse motion to the tree even when the pointer leaves
// the sidebar.
func (m Model) IsDragging() bool { return m.drag.active }

// DragSourcePath returns the path being dragged, or "" when no press is
// pending. The app uses it to suppress watcher noise from its own rename.
func (m Model) DragSourcePath() string {
	if m.drag.from == nil {
		return ""
	}
	return m.drag.from.Path
}

// RootPath returns the root directory the tree is browsing.
func (m Model) RootPath() string { return m.rootDir }

// New creates a new file tree model, loads children for the root, expands the root,
// and builds the initial flat list.
func New(tm *theme.Manager, cfg config.Config, rootDir string) Model {
	root := &TreeNode{
		Name:  filepath.Base(rootDir),
		Path:  rootDir,
		IsDir: true,
	}
	_ = root.LoadChildren(cfg.IgnoredPatterns)
	root.Expanded = true

	m := Model{
		theme:   tm,
		cfg:     cfg,
		root:    root,
		rootDir: rootDir,
	}
	m.rebuildFlat()
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.SettingsChangedMsg:
		m.cfg = msg.Config
		return m, nil

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.clampContextMenu()

	case tea.KeyPressMsg:
		// Keyboard input abandons any pending or in-flight mouse drag.
		m.clearDrag()
		// Delete dialog is open — route keys to it.
		if m.deleteDialog != nil {
			confirm, cancel, _ := m.deleteDialog.HandleKey(msg)
			if cancel {
				m.deleteDialog = nil
				return m, nil
			}
			if confirm {
				return m.confirmDelete()
			}
			return m, nil
		}
		// Context menu is open — handle its navigation first (regardless of focus).
		if m.ctxMenu != nil {
			switch msg.String() {
			case "up", "k":
				m.ctxMenu.moveUp()
			case "down", "j":
				m.ctxMenu.moveDown()
			case "enter", " ":
				switch m.ctxMenu.focused {
				case 2: // View in Finder / View in Folder / View in Explorer
					return m.revealInFileManager()
				case 3: // Delete
					path := m.ctxMenu.targetPath
					isDir := m.ctxMenu.targetIsDir
					m.ctxMenu = nil
					return m.handleStartDelete(path, isDir)
				}
				m.enterInlineEdit()
			case "esc":
				m.ctxMenu = nil
			}
			return m, nil
		}
		// Inline input is active — capture all keys.
		if m.inlineInput != nil {
			return m.handleInlineInputKey(msg)
		}
		if !m.focused {
			break
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.flat)-1 {
				m.cursor++
				if m.height > 0 && m.cursor >= m.offset+m.height {
					m.offset = m.cursor - m.height + 1
				}
			}
		case "enter", "space":
			return m.activateNode(m.cursor)
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseRight {
			// Ignore right-click while inline input is active.
			if m.inlineInput != nil {
				return m, nil
			}
			m.clearDrag()
			idx := m.offset + msg.Y
			targetDir := m.rootDir
			var targetNode *TreeNode
			if idx >= 0 && idx < len(m.flat) {
				node := m.flat[idx]
				targetNode = node
				targetDir = node.Path
				if !node.IsDir {
					targetDir = filepath.Dir(node.Path)
				}
			}
			m.ctxMenu = newContextMenu(msg.X, msg.Y, targetDir, targetNode, m.theme)
			m.clampContextMenu()
			return m, nil
		}
		if msg.Button == tea.MouseLeft {
			// Route left-click to delete dialog when open.
			if m.deleteDialog != nil {
				confirm, cancel, _ := m.deleteDialog.HandleClick(msg.X, msg.Y)
				if cancel {
					m.deleteDialog = nil
					return m, nil
				}
				if confirm {
					return m.confirmDelete()
				}
				return m, nil
			}
			// Route left-click to context menu when open.
			if m.ctxMenu != nil {
				idx := m.ctxMenu.HandleClick(msg.X, msg.Y)
				if idx == -1 {
					m.ctxMenu = nil
					return m, nil
				}
				m.ctxMenu.focused = idx
				switch idx {
				case 2: // View in Finder / View in Folder / View in Explorer
					return m.revealInFileManager()
				case 3: // Delete
					path := m.ctxMenu.targetPath
					isDir := m.ctxMenu.targetIsDir
					m.ctxMenu = nil
					return m.handleStartDelete(path, isDir)
				}
				m.enterInlineEdit()
				return m, nil
			}
			idx := m.offset + msg.Y
			m.clearDrag()
			if idx >= 0 && idx < len(m.flat) {
				node := m.flat[idx]
				m.cursor = idx
				// The root row cannot be moved, but still activates on release.
				if node != m.root {
					m.drag = dragState{fromIdx: idx, from: node, dropIdx: -1}
				}
			}
		}

	case tea.MouseMotionMsg:
		// Context menu hover: highlight the item under the mouse.
		if m.ctxMenu != nil {
			if idx := m.ctxMenu.HoverItem(msg.X, msg.Y); idx >= 0 {
				m.ctxMenu.focused = idx
			}
			return m, nil
		}
		// Drag detection: a button-held move to a different row starts a drag.
		if m.drag.from != nil && msg.Button == tea.MouseLeft {
			idx := m.offset + msg.Y
			m.drag.mouseX, m.drag.mouseY = msg.X, msg.Y
			if !m.drag.active && idx != m.drag.fromIdx {
				m.drag.active = true
			}
			if !m.drag.active {
				return m, nil
			}
			m.drag.dropIdx = m.dropTargetAt(idx, msg.Y)
			// Edge auto-scroll when the pointer is dragged past the viewport.
			m.drag.edge = 0
			if msg.Y < 0 {
				m.drag.edge = -1
			} else if m.height > 0 && msg.Y >= m.height {
				m.drag.edge = 1
			}
			var cmds []tea.Cmd
			if m.drag.edge != 0 && !m.dragScrolling {
				m.dragScrolling = true
				cmds = append(cmds, dragScrollTick())
			}
			// Hover-to-expand: arm a one-shot dwell timer for a collapsed dir.
			if path := m.pendingExpandPath(); path != "" {
				if path != m.dragExpandPending {
					m.dragExpandPending = path
					cmds = append(cmds, dragExpandTick(path))
				}
			} else {
				m.dragExpandPending = ""
			}
			if len(cmds) == 0 {
				return m, nil
			}
			return m, tea.Batch(cmds...)
		}

	case tea.MouseReleaseMsg:
		if msg.Button != tea.MouseLeft || m.drag.from == nil {
			return m, nil
		}
		d := m.drag
		idx := m.offset + msg.Y
		inViewport := msg.Y >= 0 && (m.height == 0 || msg.Y < m.height)
		if inViewport && idx == d.fromIdx {
			// Press and release on the same row: a plain click, even if the
			// pointer jittered onto another row in between.
			m.clearDrag()
			return m.activateNode(idx)
		}
		// Releasing outside the tree cancels the drag rather than dropping on
		// the last row the pointer happened to cross.
		if !d.active || !inViewport {
			m.clearDrag()
			return m, nil
		}
		// Resolve the destination before clearing the drag state — canDropInto
		// reads m.drag.from.
		destDir, ok := m.dropDirAt(d.dropIdx)
		m.clearDrag()
		if !ok {
			return m, nil
		}
		return m.moveNode(d.from, destDir)

	case dragScrollTickMsg:
		if !m.drag.active || m.drag.edge == 0 {
			m.dragScrolling = false
			return m, nil
		}
		if m.scrollViewport(m.drag.edge) {
			return m, dragScrollTick()
		}
		m.dragScrolling = false
		return m, nil

	case dragExpandTickMsg:
		// Ignore a stale timer: the pointer must still be hovering the same
		// collapsed directory when the dwell elapses.
		if !m.drag.active || m.dragExpandPending != msg.path {
			return m, nil
		}
		m.dragExpandPending = ""
		if m.drag.dropIdx < 0 || m.drag.dropIdx >= len(m.flat) {
			return m, nil
		}
		node := m.flat[m.drag.dropIdx]
		if !node.IsDir || node.Expanded || node.Path != msg.path {
			return m, nil
		}
		if !node.Loaded() {
			_ = node.LoadChildren(m.cfg.IgnoredPatterns)
		}
		node.Expanded = true
		if m.gitStatuses != nil {
			m.applyGitStatus(m.gitStatuses)
		}
		m.rebuildFlat()
		m.refreshDragFromIdx()
		return m, nil

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.offset > 0 {
				m.offset--
				if m.cursor > m.offset+m.height-1 {
					m.cursor = m.offset + m.height - 1
				}
			}
		case tea.MouseWheelDown:
			maxOffset := len(m.flat) - m.height
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.offset < maxOffset {
				m.offset++
				if m.cursor < m.offset {
					m.cursor = m.offset
				}
			}
		}

	case messages.GitStatusUpdatedMsg:
		m.gitStatuses = msg.FileStatuses
		m.applyGitStatus(msg.FileStatuses)
	}

	return m, nil
}

// View renders the file tree as a styled string.
func (m Model) View() tea.View {
	if m.height == 0 || m.width == 0 {
		return tea.NewView("")
	}

	sidebarBG := m.theme.UI("sidebar_bg")
	sidebarFG := m.theme.UI("sidebar_fg")
	selectedBG := m.theme.UI("sidebar_selected_bg")
	selectedFG := m.theme.UI("sidebar_selected_fg")

	baseStyle := lipgloss.NewStyle()
	if sidebarBG != "" {
		baseStyle = baseStyle.Background(lipgloss.Color(sidebarBG))
	}
	if sidebarFG != "" {
		baseStyle = baseStyle.Foreground(lipgloss.Color(sidebarFG))
	}

	selectedStyle := lipgloss.NewStyle()
	if selectedBG != "" {
		selectedStyle = selectedStyle.Background(lipgloss.Color(selectedBG))
	}
	if selectedFG != "" {
		selectedStyle = selectedStyle.Foreground(lipgloss.Color(selectedFG))
	}
	ignoredFG := m.ignoredColor()

	// Drag feedback: dim the source row, underline the current drop target.
	dragging := m.drag.active
	sourceRow, dropRow := -1, -1
	if dragging {
		sourceRow = m.drag.fromIdx
		dropRow = m.drag.dropIdx
	}

	var sb strings.Builder
	end := m.offset + m.height
	if end > len(m.flat) {
		end = len(m.flat)
	}

	for i := m.offset; i < end; i++ {
		node := m.flat[i]
		depth := m.nodeDepth(node)
		indent := strings.Repeat("  ", depth)

		prefix := "  "
		if node.IsDir {
			if node.Expanded {
				prefix = "▼ "
			} else {
				prefix = "▶ "
			}
		}

		icon := m.gitIcon(node.GitStatus)
		hasIcon := icon != " "
		// Don't show git icon on the root folder itself
		if node == m.root {
			icon = " "
			hasIcon = false
		}
		gitColor := ""
		if hasIcon {
			gitColor = m.gitColor(node.GitStatus)
		}

		iconSlotWidth := 3
		if m.width < iconSlotWidth {
			iconSlotWidth = 0
		}
		labelWidth := m.width - iconSlotWidth
		if labelWidth < 0 {
			labelWidth = 0
		}
		ignored := node.GitStatus == messages.GitStatusIgnored

		var line string
		if i == m.cursor {
			labelStyle := selectedStyle.Copy()
			if ignored && ignoredFG != "" {
				labelStyle = labelStyle.Foreground(lipgloss.Color(ignoredFG))
			}
			if dragging && i == sourceRow {
				labelStyle = labelStyle.Faint(true)
			}
			if dragging && i == dropRow {
				labelStyle = labelStyle.Bold(true).Underline(true)
			}
			// Build icon with selected background
			iconStyle := baseStyle.Copy()
			if selectedBG != "" {
				iconStyle = iconStyle.Background(lipgloss.Color(selectedBG))
			}
			if selectedFG != "" {
				iconStyle = iconStyle.Foreground(lipgloss.Color(selectedFG))
			}
			if hasIcon && gitColor != "" {
				iconStyle = iconStyle.Foreground(lipgloss.Color(gitColor))
			}
			markerStyle := m.fileIconStyle(node, selectedStyle, ignored, ignoredFG)
			label, labelVisualWidth := m.renderNodeLabel(node, indent, prefix, labelWidth, labelStyle, markerStyle)
			var iconStr string
			if iconSlotWidth == 0 {
				iconStr = ""
			} else if hasIcon {
				iconStr = selectedStyle.Render(" ") + iconStyle.Render(icon) + iconStyle.Render(" ")
			} else {
				iconStr = iconStyle.Render("   ")
			}
			padLen := m.width - labelVisualWidth - iconSlotWidth
			if padLen < 0 {
				padLen = 0
			}
			line = label + iconStr + selectedStyle.Render(strings.Repeat(" ", padLen))
		} else {
			labelStyle := baseStyle.Copy()
			if ignored && ignoredFG != "" {
				labelStyle = labelStyle.Foreground(lipgloss.Color(ignoredFG))
			}
			if dragging && i == sourceRow {
				labelStyle = labelStyle.Faint(true)
			}
			if dragging && i == dropRow {
				labelStyle = labelStyle.Bold(true).Underline(true)
			}
			// Build icon with base background
			iconStyle := baseStyle.Copy()
			if hasIcon && gitColor != "" {
				iconStyle = iconStyle.Foreground(lipgloss.Color(gitColor))
			}
			markerStyle := m.fileIconStyle(node, baseStyle, ignored, ignoredFG)
			label, labelVisualWidth := m.renderNodeLabel(node, indent, prefix, labelWidth, labelStyle, markerStyle)
			var iconStr string
			if iconSlotWidth == 0 {
				iconStr = ""
			} else if hasIcon {
				iconStr = baseStyle.Render(" ") + iconStyle.Render(icon) + baseStyle.Render(" ")
			} else {
				iconStr = baseStyle.Render("   ")
			}
			// Pad to full width so JoinHorizontal doesn't add unstyled spaces.
			padLen := m.width - labelVisualWidth - iconSlotWidth
			if padLen < 0 {
				padLen = 0
			}
			line = label + iconStr + baseStyle.Render(strings.Repeat(" ", padLen))
		}

		sb.WriteString(line)
		// After rendering the directory row, inject the inline-edit row.
		if m.inlineInput != nil && m.inlineInsertIdx > 0 && i == m.inlineInsertIdx-1 {
			sb.WriteRune('\n')
			sb.WriteString(m.inlineInput.RenderRow(depth+1, m.width, m.theme))
		}
		if i < end-1 {
			sb.WriteRune('\n')
		}
	}

	// Fill remaining lines with background color
	rendered := sb.String()
	lineCount := end - m.offset
	if m.inlineInput != nil && m.inlineInsertIdx > 0 &&
		m.inlineInsertIdx-1 >= m.offset && m.inlineInsertIdx-1 < end {
		lineCount++
	}
	for i := lineCount; i < m.height; i++ {
		rendered += "\n" + baseStyle.Width(m.width).Render("")
	}

	return tea.NewView(rendered)
}

// ContextMenuOverlay returns the rendered context menu and its filetree-local position
// for the app to composite over the full screen after layout. Returns ok=false if no
// menu is open.
func (m Model) ContextMenuOverlay() (rendered string, x, y int, ok bool) {
	if m.ctxMenu == nil {
		return "", 0, 0, false
	}
	return m.ctxMenu.Render(), m.ctxMenu.X, m.ctxMenu.Y, true
}

// HasContextMenu returns true when a context menu is open.
func (m Model) HasContextMenu() bool {
	return m.ctxMenu != nil
}

// ContextMenuBounds returns the filetree-local bounding box of the open context
// menu (X, Y, width, height) and ok=true. Coordinates match those used for mouse
// routing; ok=false when no menu is open.
func (m Model) ContextMenuBounds() (x, y, w, h int, ok bool) {
	if m.ctxMenu == nil {
		return 0, 0, 0, 0, false
	}
	c := m.ctxMenu
	return c.X, c.Y, c.boxWidth(), c.boxHeight(), true
}

func (m Model) renderNodeLabel(node *TreeNode, indent, prefix string, maxWidth int, labelStyle, markerStyle lipgloss.Style) (string, int) {
	if maxWidth <= 0 {
		return "", 0
	}

	var parts strings.Builder
	used := 0

	prefixText := ansi.Truncate(indent+prefix, maxWidth, "")
	if prefixText != "" {
		parts.WriteString(labelStyle.Render(prefixText))
		used += lipgloss.Width(prefixText)
	}
	if used >= maxWidth {
		return parts.String(), used
	}

	if m.cfg.Sidebar.FileIcons.Enabled && !node.IsDir {
		icon := fileIconForName(node.Name)
		markerText := ansi.Truncate(icon.marker+" ", maxWidth-used, "")
		if markerText != "" {
			parts.WriteString(markerStyle.Render(markerText))
			used += lipgloss.Width(markerText)
		}
		if used >= maxWidth {
			return parts.String(), used
		}
	}

	nameText := ansi.Truncate(node.Name, maxWidth-used, "")
	if nameText != "" {
		parts.WriteString(labelStyle.Render(nameText))
		used += lipgloss.Width(nameText)
	}

	return parts.String(), used
}

func (m Model) fileIconStyle(node *TreeNode, base lipgloss.Style, ignored bool, ignoredFG string) lipgloss.Style {
	style := base.Copy().Bold(true)
	if !m.cfg.Sidebar.FileIcons.Enabled || node.IsDir {
		return style
	}
	if ignored && ignoredFG != "" {
		return style.Foreground(lipgloss.Color(ignoredFG))
	}
	if color := fileIconColor(m.theme, m.cfg.Sidebar.FileIcons, fileIconForName(node.Name)); color != "" {
		return style.Foreground(lipgloss.Color(color))
	}
	return style
}

// activateNode toggles directories or emits FileSelectedMsg for files.
func (m Model) activateNode(idx int) (Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.flat) {
		return m, nil
	}
	m.cursor = idx
	node := m.flat[idx]

	if node.IsDir {
		if node.Expanded {
			node.Expanded = false
		} else {
			if !node.Loaded() {
				_ = node.LoadChildren(m.cfg.IgnoredPatterns)
			}
			node.Expanded = true
			if m.gitStatuses != nil {
				m.applyGitStatus(m.gitStatuses)
			}
		}
		m.rebuildFlat()
		return m, nil
	}

	path := node.Path
	return m, func() tea.Msg {
		return messages.FileSelectedMsg{Path: path}
	}
}

// ── Drag & drop ───────────────────────────────────────────────────────────────

// dropTargetAt returns the flat row a drag would drop onto, or -1 when the row
// under the pointer is not a valid destination. screenY is tree-local and is
// used to reject pointers outside the visible viewport (the app keeps feeding
// motion to the tree after the pointer leaves the sidebar).
func (m Model) dropTargetAt(idx, screenY int) int {
	if m.height > 0 && (screenY < 0 || screenY >= m.height) {
		return -1
	}
	if _, ok := m.dropDirAt(idx); !ok {
		return -1
	}
	return idx
}

// dropDirAt resolves a flat row to its destination directory. Dropping on a
// file row targets the file's parent; dropping on a directory row targets the
// directory itself.
func (m Model) dropDirAt(idx int) (string, bool) {
	if idx < 0 || idx >= len(m.flat) {
		return "", false
	}
	node := m.flat[idx]
	destDir := node.Path
	if !node.IsDir {
		destDir = filepath.Dir(node.Path)
	}
	if !m.canDropInto(destDir) {
		return "", false
	}
	return destDir, true
}

// refreshDragFromIdx re-locates the dragged node in the flat list after a
// rebuild (hover-to-expand changes the indices of rows below the target).
func (m *Model) refreshDragFromIdx() {
	if m.drag.from == nil {
		return
	}
	for i, node := range m.flat {
		if node == m.drag.from {
			m.drag.fromIdx = i
			return
		}
	}
}

// pendingExpandPath returns the path of a collapsed directory that is
// currently the drop target, or "" when the target is not a collapsed dir.
func (m Model) pendingExpandPath() string {
	if m.drag.dropIdx < 0 || m.drag.dropIdx >= len(m.flat) {
		return ""
	}
	node := m.flat[m.drag.dropIdx]
	if node.IsDir && !node.Expanded {
		return node.Path
	}
	return ""
}

// dragExpandTick schedules the hover-to-expand dwell timer.
func dragExpandTick(path string) tea.Cmd {
	return tea.Tick(dragExpandDelay, func(time.Time) tea.Msg {
		return dragExpandTickMsg{path: path}
	})
}

// dragScrollTick schedules the next edge auto-scroll step.
func dragScrollTick() tea.Cmd {
	return tea.Tick(dragScrollInterval, func(time.Time) tea.Msg {
		return dragScrollTickMsg{}
	})
}

// scrollViewport nudges the visible window by delta rows and keeps the cursor
// inside it. It reports whether the window actually moved.
func (m *Model) scrollViewport(delta int) bool {
	if m.height <= 0 {
		return false
	}
	maxOffset := len(m.flat) - m.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	next := m.offset + delta
	if next < 0 {
		next = 0
	}
	if next > maxOffset {
		next = maxOffset
	}
	if next == m.offset {
		return false
	}
	m.offset = next
	if m.cursor < m.offset {
		m.cursor = m.offset
	}
	if m.cursor > m.offset+m.height-1 {
		m.cursor = m.offset + m.height - 1
	}
	return true
}

// DragOverlay returns the floating "name → destination" label for an in-flight
// drag and its tree-local anchor, for the app to composite over the layout.
func (m Model) DragOverlay() (rendered string, x, y int, ok bool) {
	if !m.drag.active || m.drag.from == nil {
		return "", 0, 0, false
	}
	label := m.drag.from.Name
	if dest := m.dropLabel(); dest != "" {
		label += " → " + dest
	}
	style := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	if bg := m.theme.UI("completion_selected"); bg != "" {
		style = style.Background(lipgloss.Color(bg))
	}
	if fg := m.theme.UI("completion_fg"); fg != "" {
		style = style.Foreground(lipgloss.Color(fg))
	}
	return style.Render(label), m.drag.mouseX + 2, m.drag.mouseY + 1, true
}

// dropLabel renders the current drop destination relative to the tree root
// ("sub/dir/", "./" for the root), or "" when there is no valid target.
func (m Model) dropLabel() string {
	if m.drag.dropIdx < 0 {
		return ""
	}
	dest, ok := m.dropDirAt(m.drag.dropIdx)
	if !ok {
		return ""
	}
	rel, err := filepath.Rel(m.rootDir, dest)
	if err != nil || rel == "." {
		return "./"
	}
	return rel + "/"
}

// canDropInto reports whether the in-flight drag may move into destDir. It
// rejects the current parent (no-op), the source itself, descendants of the
// source, and destinations that already hold an entry with the same name.
func (m Model) canDropInto(destDir string) bool {
	from := m.drag.from
	if from == nil || destDir == "" {
		return false
	}
	if destDir == filepath.Dir(from.Path) {
		return false
	}
	if from.IsDir {
		sep := string(filepath.Separator)
		if destDir == from.Path || strings.HasPrefix(destDir+sep, from.Path+sep) {
			return false
		}
	}
	if _, err := os.Lstat(filepath.Join(destDir, from.Name)); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

// moveNode renames from into destDir on disk, refreshes the source and
// destination directories, and emits FileMovedMsg so the app can rebind any
// open buffers, tabs, and language-server documents to the new path.
func (m *Model) moveNode(from *TreeNode, destDir string) (Model, tea.Cmd) {
	oldPath := from.Path
	isDir := from.IsDir
	newPath := filepath.Join(destDir, from.Name)
	if err := movePath(oldPath, newPath); err != nil {
		fmt.Fprintf(os.Stderr, "toast: move %q: %v\n", oldPath, err)
		return *m, nil
	}
	m.reloadDir(filepath.Dir(oldPath))
	m.expandPath(destDir)
	m.reloadDir(destDir)
	m.selectPath(newPath)
	return *m, func() tea.Msg {
		return messages.FileMovedMsg{OldPath: oldPath, NewPath: newPath, IsDir: isDir}
	}
}

// movePath renames src to dst. When src and dst live on different filesystems
// os.Rename fails with EXDEV, so it falls back to copying then removing the
// source.
func movePath(src, dst string) error {
	return movePathWith(src, dst, os.Rename)
}

// movePathWith is movePath with an injectable rename, so the cross-device
// fallback can be exercised in tests.
func movePathWith(src, dst string, rename func(string, string) error) error {
	err := rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyPath(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// copyPath recursively copies a file, directory, or symlink.
func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()|0700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return os.Chmod(dst, info.Mode().Perm())
	default:
		return copyFile(src, dst, info.Mode().Perm())
	}
}

// copyFile copies a single regular file, preserving its permission bits.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// selectPath moves the cursor (and viewport) onto the row for path when it is
// visible in the flat list.
func (m *Model) selectPath(path string) {
	for i, node := range m.flat {
		if node.Path == path {
			m.cursor = i
			if m.height > 0 {
				if m.cursor < m.offset {
					m.offset = m.cursor
				} else if m.cursor >= m.offset+m.height {
					m.offset = m.cursor - m.height + 1
				}
			}
			return
		}
	}
}

// Focus gives keyboard focus to the file tree.
func (m *Model) Focus() { m.focused = true }

// Blur removes keyboard focus from the file tree.
func (m *Model) Blur() { m.focused = false }

// ── Context menu & inline edit ────────────────────────────────────────────────

// SetScreenWidth records the full window width in columns. The context menu is
// composited in screen space, so the app must tell the tree how wide the window
// is for the overlay to be clamped to visible columns.
func (m *Model) SetScreenWidth(width int) {
	m.screenWidth = width
	m.clampContextMenu()
}

// clampContextMenu repositions the open context menu so the whole box stays
// inside the tree's content rows and the window width. Rendering and
// hit-testing share the stored position, so clamping here (instead of letting
// the overlay compositor shift the box) keeps clicks aligned with what is drawn.
func (m *Model) clampContextMenu() {
	if m.ctxMenu == nil {
		return
	}
	m.ctxMenu.clamp(m.screenWidth, m.height)
}

// enterInlineEdit transitions from context menu to inline edit mode.
func (m *Model) enterInlineEdit() {
	if m.ctxMenu == nil {
		return
	}
	isDir := m.ctxMenu.focused == 1
	targetDir := m.ctxMenu.targetDir
	m.ctxMenu = nil

	// Find the target directory in the flat list.
	idx := -1
	for i, node := range m.flat {
		if node.Path == targetDir {
			idx = i
			break
		}
	}
	// If not found (collapsed), expand and rebuild.
	if idx == -1 {
		m.expandPath(targetDir)
		for i, node := range m.flat {
			if node.Path == targetDir {
				idx = i
				break
			}
		}
	}
	if idx == -1 {
		return // still not found — give up silently
	}
	m.inlineInsertIdx = idx + 1
	m.inlineInput = NewInlineInput(targetDir, isDir)
}

// revealInFileManager handles the "View in Finder / View in Folder / View in
// Explorer" context menu item. When a node is targeted it reveals that node;
// otherwise it reveals the target directory (e.g. right-clicking empty space).
func (m Model) revealInFileManager() (Model, tea.Cmd) {
	path := m.ctxMenu.targetPath
	if path == "" {
		path = m.ctxMenu.targetDir
	}
	m.ctxMenu = nil
	return m, func() tea.Msg {
		return messages.RevealInFileManagerMsg{Path: path}
	}
}

// expandPath finds the node with the given path and expands it.
func (m *Model) expandPath(path string) {
	var expand func(n *TreeNode) bool
	expand = func(n *TreeNode) bool {
		if n.Path == path {
			if !n.Loaded() {
				_ = n.LoadChildren(m.cfg.IgnoredPatterns)
			}
			n.Expanded = true
			return true
		}
		for _, child := range n.Children {
			if expand(child) {
				return true
			}
		}
		return false
	}
	expand(m.root)
	if m.gitStatuses != nil {
		m.applyGitStatus(m.gitStatuses)
	}
	m.rebuildFlat()
}

// handleInlineInputKey handles key presses while inline input is active.
func (m Model) handleInlineInputKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inlineInput = nil
		m.inlineInsertIdx = 0
		return m, nil
	case "enter":
		return m.confirmInlineInput()
	case "backspace":
		m.inlineInput.Backspace()
		return m, nil
	default:
		if msg.Text != "" {
			for _, ch := range msg.Text {
				m.inlineInput.Insert(ch)
			}
		}
		return m, nil
	}
}

// confirmInlineInput creates the file or directory on disk and emits the appropriate message.
func (m Model) confirmInlineInput() (Model, tea.Cmd) {
	inp := m.inlineInput
	name := strings.TrimSpace(inp.value)
	if name == "" {
		m.inlineInput = nil
		m.inlineInsertIdx = 0
		return m, nil
	}

	fullPath := filepath.Join(inp.targetDir, name)
	// Guard: prevent path traversal (e.g. name = "../../etc/passwd")
	cleanTarget := filepath.Clean(inp.targetDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(fullPath)+string(filepath.Separator), cleanTarget) {
		fmt.Fprintf(os.Stderr, "toast: invalid path %q\n", name)
		m.inlineInput = nil
		m.inlineInsertIdx = 0
		return m, nil
	}

	// Check for collision.
	if _, err := os.Stat(fullPath); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			fmt.Fprintf(os.Stderr, "toast: cannot create %q: already exists\n", fullPath)
		} else {
			fmt.Fprintf(os.Stderr, "toast: cannot create %q: %v\n", fullPath, err)
		}
		m.inlineInput = nil
		m.inlineInsertIdx = 0
		return m, nil
	}

	var createErr error
	if inp.isDir {
		createErr = os.MkdirAll(fullPath, 0755)
	} else {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			createErr = err
		} else {
			f, err := os.Create(fullPath)
			if err != nil {
				createErr = err
			} else {
				f.Close()
			}
		}
	}

	if createErr != nil {
		fmt.Fprintf(os.Stderr, "toast: create %q: %v\n", fullPath, createErr)
		m.inlineInput = nil
		m.inlineInsertIdx = 0
		return m, nil
	}

	// Reload the target directory node.
	m.reloadDir(inp.targetDir)
	m.inlineInput = nil
	m.inlineInsertIdx = 0

	created := fullPath
	if inp.isDir {
		return m, func() tea.Msg { return messages.DirCreatedMsg{Path: created} }
	}
	return m, func() tea.Msg { return messages.FileCreatedMsg{Path: created} }
}

// ── Delete ────────────────────────────────────────────────────────────────────

// handleStartDelete is called when the user activates the "Delete" item.
// If ConfirmDelete is false, deletes immediately. Otherwise opens the dialog.
func (m Model) handleStartDelete(path string, isDir bool) (Model, tea.Cmd) {
	if !m.cfg.Sidebar.ConfirmDelete {
		return m, m.doPerformDelete(path, isDir)
	}
	m.deleteDialog = newDeleteConfirmDialog(path, isDir, m.theme)
	return m, nil
}

// confirmDelete is called when the user confirms deletion (from dialog or direct).
// It reads state from deleteDialog, clears it, and calls doPerformDelete.
func (m Model) confirmDelete() (Model, tea.Cmd) {
	d := m.deleteDialog
	m.deleteDialog = nil

	path := d.path
	isDir := d.isDir

	var cmds []tea.Cmd
	if d.checked {
		m.cfg.Sidebar.ConfirmDelete = false
		cfg := m.cfg
		cmds = append(cmds, func() tea.Msg {
			return messages.SaveConfigMsg{Config: cfg}
		})
	}

	deleteCmd := m.doPerformDelete(path, isDir)
	if deleteCmd != nil {
		cmds = append(cmds, deleteCmd)
	}
	return m, tea.Batch(cmds...)
}

// doPerformDelete removes path from disk, reloads the parent dir, and returns a
// cmd that emits FileDeletedMsg or DirDeletedMsg.
func (m *Model) doPerformDelete(path string, isDir bool) tea.Cmd {
	if err := os.RemoveAll(path); err != nil {
		fmt.Fprintf(os.Stderr, "toast: delete %q: %v\n", path, err)
		return nil
	}
	m.reloadDir(filepath.Dir(path))
	if isDir {
		return func() tea.Msg { return messages.DirDeletedMsg{Path: path} }
	}
	return func() tea.Msg { return messages.FileDeletedMsg{Path: path} }
}

// DeleteDialogOverlay returns the rendered delete confirmation dialog (centered)
// and ok=true when the dialog is open.
func (m Model) DeleteDialogOverlay(totalWidth, totalHeight int) (rendered string, ok bool) {
	if m.deleteDialog == nil {
		return "", false
	}
	return m.deleteDialog.Render(totalWidth, totalHeight), true
}

// HasDeleteDialog returns true when the delete confirmation dialog is open.
func (m Model) HasDeleteDialog() bool {
	return m.deleteDialog != nil
}

// HasTransientInteraction returns true when the file tree is handling a
// dismissible context menu, confirmation dialog, or inline create input.
func (m Model) HasTransientInteraction() bool {
	return m.ctxMenu != nil || m.deleteDialog != nil || m.inlineInput != nil
}

// reloadDir finds the node for dirPath, reloads its children, and rebuilds flat.
func (m *Model) reloadDir(dirPath string) {
	var reload func(n *TreeNode) bool
	reload = func(n *TreeNode) bool {
		if n.Path == dirPath {
			_ = n.LoadChildren(m.cfg.IgnoredPatterns)
			if !n.Expanded {
				n.Expanded = true
			}
			return true
		}
		for _, child := range n.Children {
			if reload(child) {
				return true
			}
		}
		return false
	}
	reload(m.root)
	if m.gitStatuses != nil {
		m.applyGitStatus(m.gitStatuses)
	}
	m.rebuildFlat()
}

// rebuildFlat walks the expanded tree into a flat slice.
func (m *Model) rebuildFlat() {
	m.flat = m.flat[:0]
	m.walkNode(m.root)
}

func (m *Model) walkNode(node *TreeNode) {
	m.flat = append(m.flat, node)
	if node.IsDir && node.Expanded {
		for _, child := range node.Children {
			m.walkNode(child)
		}
	}
}

// applyGitStatus applies git status to tree nodes and propagates dirty status to parent dirs.
func (m *Model) applyGitStatus(statuses map[string]messages.GitStatus) {
	applyToNode(m.root, statuses)
}

func applyToNode(node *TreeNode, statuses map[string]messages.GitStatus) messages.GitStatus {
	if !node.IsDir {
		if s, ok := statuses[node.Path]; ok {
			node.GitStatus = s
		} else {
			node.GitStatus = messages.GitStatusClean
		}
		return node.GitStatus
	}

	if s, ok := statuses[node.Path]; ok && s == messages.GitStatusIgnored {
		node.GitStatus = s
		return s
	}

	// For directories with loaded children, aggregate recursively.
	if node.Children != nil {
		var aggregated messages.GitStatus = messages.GitStatusClean
		for _, child := range node.Children {
			cs := applyToNode(child, statuses)
			if isDirtyGitStatus(cs) && aggregated == messages.GitStatusClean {
				aggregated = cs
			}
		}
		node.GitStatus = aggregated
		return aggregated
	}

	// For unloaded directories, scan the status map for any path under this directory.
	prefix := node.Path + string(filepath.Separator)
	var aggregated messages.GitStatus = messages.GitStatusClean
	for path, s := range statuses {
		if isDirtyGitStatus(s) && strings.HasPrefix(path, prefix) {
			aggregated = s
			break
		}
	}
	node.GitStatus = aggregated
	return aggregated
}

func isDirtyGitStatus(status messages.GitStatus) bool {
	return status != messages.GitStatusClean && status != messages.GitStatusIgnored
}

// nodeDepth calculates the depth of a node relative to the root directory.
func (m *Model) nodeDepth(node *TreeNode) int {
	rel, err := filepath.Rel(m.rootDir, node.Path)
	if err != nil {
		return 0
	}
	if rel == "." {
		return 0
	}
	parts := strings.Split(rel, string(filepath.Separator))
	// Subtract 1 so the root's immediate children are at depth 0.
	depth := len(parts) - 1
	if depth < 0 {
		return 0
	}
	return depth
}

// gitIcon returns a single-character icon for a git status.
func (m *Model) gitIcon(status messages.GitStatus) string {
	switch status {
	case messages.GitStatusModified:
		return "●"
	case messages.GitStatusAdded:
		return "●"
	case messages.GitStatusDeleted:
		return "●"
	case messages.GitStatusUntracked:
		return "●"
	case messages.GitStatusConflict:
		return "●"
	default:
		return " "
	}
}

// gitColor returns the theme color string for a git status.
func (m *Model) gitColor(status messages.GitStatus) string {
	switch status {
	case messages.GitStatusModified:
		return m.theme.Git("modified")
	case messages.GitStatusAdded:
		return m.theme.Git("added")
	case messages.GitStatusDeleted:
		return m.theme.Git("deleted")
	case messages.GitStatusUntracked:
		return m.theme.Git("untracked")
	case messages.GitStatusConflict:
		return m.theme.Git("conflict")
	case messages.GitStatusIgnored:
		return m.theme.Git("ignored")
	default:
		return ""
	}
}

func (m Model) ignoredColor() string {
	for _, color := range []string{
		m.theme.Git("ignored"),
		m.theme.Git("untracked"),
		m.theme.UI("gutter_fg"),
		m.theme.UI("sidebar_fg"),
	} {
		if color != "" {
			return color
		}
	}
	return ""
}
