package filetree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

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
	focused bool

	gitStatuses map[string]messages.GitStatus

	// ── Context menu & inline edit ────────────────────────────────────────────────
	ctxMenu         *ContextMenu
	deleteDialog    *DeleteConfirmDialog
	inlineInput     *InlineInput
	inlineInsertIdx int // index in flat where the edit row is inserted after
}

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
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case tea.KeyPressMsg:
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
				if m.ctxMenu.focused == 2 {
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
				if idx == 2 {
					path := m.ctxMenu.targetPath
					isDir := m.ctxMenu.targetIsDir
					m.ctxMenu = nil
					return m.handleStartDelete(path, isDir)
				}
				m.enterInlineEdit()
				return m, nil
			}
			idx := m.offset + msg.Y
			if idx >= 0 && idx < len(m.flat) {
				return m.activateNode(idx)
			}
		}

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

		label := indent + prefix + node.Name

		var line string
		if i == m.cursor {
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
			var iconStr string
			if hasIcon {
				iconStr = selectedStyle.Render(" ") + iconStyle.Render(icon) + iconStyle.Render(" ")
			} else {
				iconStr = iconStyle.Render("   ")
			}
			// Pad line to full width for highlight
			rendered := label + " " + icon + " "
			padLen := m.width - lipgloss.Width(rendered)
			if padLen < 0 {
				padLen = 0
			}
			line = selectedStyle.Render(label) + iconStr + selectedStyle.Render(strings.Repeat(" ", padLen))
		} else {
			// Build icon with base background
			iconStyle := baseStyle.Copy()
			if hasIcon && gitColor != "" {
				iconStyle = iconStyle.Foreground(lipgloss.Color(gitColor))
			}
			var iconStr string
			if hasIcon {
				iconStr = baseStyle.Render(" ") + iconStyle.Render(icon) + baseStyle.Render(" ")
			} else {
				iconStr = baseStyle.Render("   ")
			}
			// Pad to full width so JoinHorizontal doesn't add unstyled spaces.
			rendered := label + " " + icon + " "
			padLen := m.width - lipgloss.Width(rendered)
			if padLen < 0 {
				padLen = 0
			}
			line = baseStyle.Render(label) + iconStr + baseStyle.Render(strings.Repeat(" ", padLen))
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

// Focus gives keyboard focus to the file tree.
func (m *Model) Focus() { m.focused = true }

// Blur removes keyboard focus from the file tree.
func (m *Model) Blur() { m.focused = false }

// ── Context menu & inline edit ────────────────────────────────────────────────

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

	// For directories with loaded children, aggregate recursively.
	if node.Children != nil {
		var aggregated messages.GitStatus = messages.GitStatusClean
		for _, child := range node.Children {
			cs := applyToNode(child, statuses)
			if cs != messages.GitStatusClean && aggregated == messages.GitStatusClean {
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
		if s != messages.GitStatusClean && strings.HasPrefix(path, prefix) {
			aggregated = s
			break
		}
	}
	node.GitStatus = aggregated
	return aggregated
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
	default:
		return ""
	}
}
