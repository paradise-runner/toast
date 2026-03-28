package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/components/breadcrumbs"
	"github.com/yourusername/toast/internal/components/closedialog"
	"github.com/yourusername/toast/internal/components/editor"
	"github.com/yourusername/toast/internal/components/filetree"
	"github.com/yourusername/toast/internal/components/gotoline"
	"github.com/yourusername/toast/internal/components/quitdialog"
	"github.com/yourusername/toast/internal/components/search"
	"github.com/yourusername/toast/internal/components/statusbar"
	"github.com/yourusername/toast/internal/components/tabbar"
	"github.com/yourusername/toast/internal/components/themepicker"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/git"
	"github.com/yourusername/toast/internal/lsp"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
	"github.com/yourusername/toast/internal/watcher"
)

// FocusTarget identifies which component currently has keyboard focus.
type FocusTarget int

const (
	FocusEditor FocusTarget = iota
	FocusFileTree
	FocusSearch
)

// Model is the top-level application model that composes all components.
type Model struct {
	cfg   config.Config
	theme *theme.Manager
	focus FocusTarget

	width, height  int
	sidebarWidth   int
	sidebarVisible bool
	ready          bool

	// Sidebar resize state
	sidebarDragging bool
	dragStartX      int
	dragStartWidth  int

	rootDir    string
	searchOpen bool

	themePickerOpen bool
	themePicker     themepicker.Model
	themeDir        string
	configPath      string

	closeDialogOpen  bool
	closePendingID   int
	closePendingPath string
	closeDialog      closedialog.Model

	goToLineOpen bool
	goToLine     gotoline.Model

	quitDialogOpen bool
	quitDialog     quitdialog.Model

	// pendingQuit is set while a save-then-quit sequence is in progress.
	pendingQuit bool

	// File to open immediately on startup (empty = none).
	initialFile string

	// Next buffer ID counter for opening files.
	nextBufferID int

	// isSavingPath holds the path of the file being written to disk.
	// It is non-empty while a save is in-flight and suppresses the
	// watcher-triggered reload for that specific path on FileChangedOnDiskMsg.
	isSavingPath string

	// Mapping of path -> bufferID for open files.
	openBuffers map[string]int

	// Component models
	fileTree   filetree.Model
	tabBar     tabbar.Model
	editor     editor.Model
	statusBar  statusbar.Model
	breadcrumb breadcrumbs.Model
	search     search.Model

	// Services (initialized lazily via SetLSPSend)
	lspMgr  *lsp.Manager
	watcher *watcher.Watcher
}

// New creates a new application model with all component sub-models.
// initialFile is an optional absolute path to open immediately on startup.
func New(cfg config.Config, themeDir, rootDir, initialFile string) (*Model, error) {
	tm, err := theme.NewManager(cfg.Theme, themeDir)
	if err != nil {
		return nil, err
	}

	configPath, err := config.DefaultPath()
	if err != nil {
		configPath = ""
	}

	return &Model{
		cfg:            cfg,
		theme:          tm,
		focus:          FocusEditor,
		sidebarWidth:   cfg.Sidebar.Width,
		sidebarVisible: cfg.Sidebar.Visible,
		rootDir:        rootDir,
		initialFile:    initialFile,
		nextBufferID:   1,
		openBuffers:    make(map[string]int),
		themeDir:       themeDir,
		configPath:     configPath,
		themePicker:    themepicker.New(tm, themeDir, cfg.Theme),
		goToLine:       gotoline.NewWithTheme(tm),

		fileTree:   filetree.New(tm, cfg, rootDir),
		tabBar:     tabbar.New(tm),
		editor:     editor.New(tm, cfg),
		statusBar:  statusbar.New(tm),
		breadcrumb: breadcrumbs.New(tm, rootDir),
		search:     search.New(tm, rootDir),
	}, nil
}

// SetLSPSend creates the LSP manager and file watcher using the provided
// send function (typically bubbletea's Program.Send).
func (m *Model) SetLSPSend(send func(tea.Msg)) {
	m.lspMgr = lsp.NewManager(m.cfg, m.rootDir, send)
	w, err := watcher.New(send)
	if err == nil {
		m.watcher = w
	}
}

// ShutdownLSP shuts down all LSP servers and closes the file watcher.
func (m *Model) ShutdownLSP() {
	if m.lspMgr != nil {
		m.lspMgr.ShutdownAll()
	}
	if m.watcher != nil {
		m.watcher.Close()
	}
}

// Init satisfies tea.Model.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.runGitStatus())
	if m.initialFile != "" {
		path := m.initialFile
		cmds = append(cmds, func() tea.Msg {
			return messages.FileSelectedMsg{Path: path}
		})
	}
	return tea.Batch(cmds...)
}

// Update handles all incoming messages, routing them to the appropriate
// component(s) and orchestrating cross-component communication.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		cmds = append(cmds, m.resizeComponents()...)

	case tea.KeyPressMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseClickMsg:
		cmd := m.handleMouseClick(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseReleaseMsg:
		cmd := m.handleMouseRelease(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseMotionMsg:
		cmd := m.handleMouseMotion(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.FileSelectedMsg:
		cmd := m.handleFileSelected(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.CloseTabRequestMsg:
		cmd := m.requestCloseTab(msg.BufferID, msg.Path)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.CloseTabConfirmedMsg:
		m.closeDialogOpen = false
		if msg.Cancelled {
			break
		}
		if msg.Save {
			updated, saveCmd := m.editor.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
			m.editor = updated.(editor.Model)
			closeCmd := func() tea.Msg {
				return messages.BufferClosedMsg{BufferID: msg.BufferID}
			}
			cmds = append(cmds, saveCmd, closeCmd)
		} else {
			bID := msg.BufferID
			cmds = append(cmds, func() tea.Msg {
				return messages.BufferClosedMsg{BufferID: bID}
			})
		}

	case messages.BufferClosedMsg:
		// Remove from openBuffers map and unwatch.
		for path, id := range m.openBuffers {
			if id == msg.BufferID {
				delete(m.openBuffers, path)
				if m.watcher != nil {
					_ = m.watcher.Unwatch(filepath.Dir(path))
				}
				break
			}
		}
		// Update the tab bar.
		var closeCmd tea.Cmd
		m.tabBar, closeCmd = m.tabBar.Update(msg)
		if closeCmd != nil {
			cmds = append(cmds, closeCmd)
		}
		// Switch to the new active tab, or clear the editor if none remain.
		if active := m.tabBar.ActiveTab(); active != nil {
			switchCmd := m.handleFileSelected(messages.FileSelectedMsg{Path: active.Path})
			if switchCmd != nil {
				cmds = append(cmds, switchCmd)
			}
		} else {
			// No tabs remain: reset the editor to its empty state.
			m.editor = editor.New(m.theme, m.cfg)
			sidebarW := 0
			if m.sidebarVisible {
				sidebarW = m.sidebarWidth
			}
			editorWidth := m.width - sidebarW
			if editorWidth < 0 {
				editorWidth = 0
			}
			contentHeight := m.height - 3 // tabBar + breadcrumb + statusBar
			if contentHeight < 0 {
				contentHeight = 0
			}
			updated, _ := m.editor.Update(tea.WindowSizeMsg{Width: editorWidth, Height: contentHeight})
			m.editor = updated.(editor.Model)
		}

	case messages.BufferOpenedMsg:
		var cmd tea.Cmd
		m.tabBar, cmd = m.tabBar.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.updateBreadcrumbs(messages.ActiveBufferChangedMsg{BufferID: msg.BufferID, Path: msg.Path})
		m.updateStatusBar(messages.ActiveBufferChangedMsg{BufferID: msg.BufferID, Path: msg.Path})
		// Start git diff for the newly opened buffer
		cmds = append(cmds, m.runGitDiff(msg.BufferID, msg.Path))

	case messages.BufferModifiedMsg:
		m.tabBar, _ = m.tabBar.Update(msg)
		m.updateStatusBar(msg)

	case messages.FileSavingMsg:
		m.isSavingPath = msg.Path

	case messages.FileSavedMsg:
		if m.isSavingPath == msg.Path {
			m.isSavingPath = ""
		}
		// Refresh git status after save
		cmds = append(cmds, m.runGitStatus())
		// If a save-then-quit was requested, now quit.
		if m.pendingQuit {
			m.pendingQuit = false
			return m, tea.Batch(append(cmds, tea.Quit)...)
		}

	case messages.FileCreatedMsg:
		cmds = append(cmds, m.runGitStatus())
		cmd := m.handleFileSelected(messages.FileSelectedMsg{Path: msg.Path})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.DirCreatedMsg:
		cmds = append(cmds, m.runGitStatus())

	case messages.FileDeletedMsg:
		cmds = append(cmds, m.runGitStatus())
		if cmd := m.forceCloseTab(msg.Path); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.fileTree, _ = m.fileTree.Update(msg)

	case messages.DirDeletedMsg:
		cmds = append(cmds, m.runGitStatus())
		prefix := msg.Path + string(filepath.Separator)
		for path := range m.openBuffers {
			if strings.HasPrefix(path, prefix) {
				if cmd := m.forceCloseTab(path); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		m.fileTree, _ = m.fileTree.Update(msg)

	case messages.SaveConfigMsg:
		if cfg, ok := msg.Config.(config.Config); ok {
			m.cfg = cfg
			_ = config.Save(cfg, m.configPath)
		}

	case messages.ActiveBufferChangedMsg:
		m.tabBar, _ = m.tabBar.Update(msg)
		m.updateBreadcrumbs(msg)
		m.updateStatusBar(msg)
		// Load the file in the editor if switching tabs
		cmd := m.handleFileSelected(messages.FileSelectedMsg{Path: msg.Path})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.DiagnosticsUpdatedMsg:
		m.updateEditor(msg)
		m.updateStatusBar(msg)

	case messages.GitStatusUpdatedMsg:
		m.fileTree, _ = m.fileTree.Update(msg)
		m.updateStatusBar(msg)

	case messages.GitDiffUpdatedMsg:
		m.updateEditor(msg)

	case messages.CompletionResultMsg:
		m.updateEditor(msg)

	case messages.HoverResultMsg:
		m.updateEditor(msg)

	case messages.DefinitionResultMsg:
		// Open the definition file
		cmd := m.handleFileSelected(messages.FileSelectedMsg{Path: msg.Path})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.SearchOpenMsg:
		m.searchOpen = true
		m.setFocus(FocusSearch)
		m.search, _ = m.search.Update(msg)

	case messages.SearchCloseMsg:
		m.searchOpen = false
		m.setFocus(FocusEditor)
		m.search, _ = m.search.Update(msg)

	case messages.LSPServerStatusMsg:
		m.updateStatusBar(msg)

	case messages.FileSaveFailedMsg:
		if m.isSavingPath == msg.Path {
			m.isSavingPath = ""
		}
		fmt.Fprintf(os.Stderr, "toast: save failed for %s: %v\n", msg.Path, msg.Err)

	case messages.FileChangedOnDiskMsg:
		// Skip git status and reload when this is our own save write.
		if m.isSavingPath == msg.Path {
			break
		}
		// Refresh git status on external file changes
		cmds = append(cmds, m.runGitStatus())
		// If the changed file is currently open and unmodified, silently reload it.
		if msg.Path == m.editor.Path() && !m.editor.IsModified() {
			cmd := m.editor.OpenFile(m.editor.BufferID(), msg.Path)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case messages.SidebarToggleMsg:
		m.sidebarVisible = !m.sidebarVisible
		cmds = append(cmds, m.resizeComponents()...)

	case messages.QuitRequestMsg:
		cmd := m.requestQuit()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.QuitConfirmedMsg:
		m.quitDialogOpen = false
		if msg.Cancelled {
			break
		}
		if msg.Save {
			// Save the active buffer, then quit once FileSavedMsg arrives.
			m.pendingQuit = true
			updated, saveCmd := m.editor.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
			m.editor = updated.(editor.Model)
			if saveCmd != nil {
				cmds = append(cmds, saveCmd)
			} else {
				// Nothing to save (empty path); quit immediately.
				m.pendingQuit = false
				return m, tea.Quit
			}
		} else {
			return m, tea.Quit
		}

	case messages.ThemePickerOpenMsg:
		m.themePicker = themepicker.New(m.theme, m.themeDir, m.theme.Name())
		m.themePicker, _ = m.themePicker.Init()
		m.themePickerOpen = true

	case messages.GoToLineCancelMsg:
		m.goToLineOpen = false

	case messages.GoToLineMsg:
		m.goToLineOpen = false
		// Forward to editor so it moves the cursor.
		cmd := m.updateEditor(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.ThemePickerClosedMsg:
		m.themePickerOpen = false
		_ = m.theme.Reload(msg.ThemeName, m.themeDir)
		m.cfg.Theme = msg.ThemeName
		_ = config.Save(m.cfg, m.configPath)

	case messages.ThemeChangedMsg:
		_ = m.theme.Reload(msg.ThemeName, m.themeDir)

	default:
		// Forward unknown messages to focused component
		cmd := m.updateFocused(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Update cursor position in status bar after any message
	m.statusBar.SetCursor(m.editor.CursorLine(), m.editor.CursorCol())

	return m, tea.Batch(cmds...)
}

// handleKey processes key messages, checking app-level bindings first then
// forwarding to the focused component.
func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.quitDialogOpen {
		// Ctrl+Q while dialog is open force-quits without saving.
		if isQuit(msg) {
			m.quitDialogOpen = false
			return tea.Quit
		}
		updated, cmd := m.quitDialog.Update(msg)
		m.quitDialog = updated
		return cmd
	}

	if m.closeDialogOpen {
		updated, cmd := m.closeDialog.Update(msg)
		m.closeDialog = updated
		return cmd
	}

	if m.themePickerOpen {
		updated, cmd := m.themePicker.Update(msg)
		m.themePicker = updated
		return cmd
	}

	if m.goToLineOpen {
		updated, cmd := m.goToLine.Update(msg)
		m.goToLine = updated
		return cmd
	}

	// App-level keys always checked first.
	switch {
	case isQuit(msg):
		return m.requestQuit()

	case isToggleSidebar(msg):
		m.sidebarVisible = !m.sidebarVisible
		cmds := m.resizeComponents()
		return tea.Batch(cmds...)

	case isCloseTab(msg):
		if tab := m.tabBar.ActiveTab(); tab != nil {
			return m.requestCloseTab(tab.BufferID, tab.Path)
		}
		return nil

	case isSearch(msg):
		m.searchOpen = true
		m.setFocus(FocusSearch)
		m.search, _ = m.search.Update(messages.SearchOpenMsg{})
		return nil

	case isGoToLine(msg):
		m.goToLine = m.goToLine.Open(m.editor.LineCount())
		m.goToLineOpen = true
		return nil

	case isNextTab(msg):
		cmd := m.tabBar.NextTab()
		return cmd

	case isPrevTab(msg):
		cmd := m.tabBar.PrevTab()
		return cmd

	case msg.String() == "escape":
		if m.searchOpen {
			m.searchOpen = false
			m.setFocus(FocusEditor)
			m.search, _ = m.search.Update(messages.SearchCloseMsg{})
			return nil
		}
		if m.focus == FocusFileTree {
			m.setFocus(FocusEditor)
			return nil
		}

	case msg.String() == "ctrl+shift+e":
		// Toggle focus between editor and file tree
		if m.focus == FocusFileTree {
			m.setFocus(FocusEditor)
		} else {
			m.setFocus(FocusFileTree)
		}
		return nil
	}

	// Forward to focused component.
	return m.updateFocused(msg)
}

// requestCloseTab checks if the buffer is modified and either closes it
// immediately or opens the confirmation dialog.
// requestQuit checks for unsaved changes and either quits immediately or opens
// the quit confirmation dialog.
func (m *Model) requestQuit() tea.Cmd {
	if !m.editor.IsModified() {
		return tea.Quit
	}
	m.quitDialog = quitdialog.New(m.theme, m.editor.Path())
	m.quitDialogOpen = true
	return nil
}

func (m *Model) requestCloseTab(bufferID int, path string) tea.Cmd {
	// Is this the currently displayed (dirty) buffer?
	if m.editor.BufferID() == bufferID && m.editor.IsModified() {
		m.closeDialog = closedialog.New(m.theme, bufferID, path)
		m.closeDialogOpen = true
		m.closePendingID = bufferID
		m.closePendingPath = path
		return nil
	}
	// Clean buffer — close immediately.
	return func() tea.Msg {
		return messages.BufferClosedMsg{BufferID: bufferID}
	}
}

// forceCloseTab closes the tab for path without prompting to save.
// It removes the entry from openBuffers, tells the tab bar, and switches focus.
func (m *Model) forceCloseTab(path string) tea.Cmd {
	bufID, ok := m.openBuffers[path]
	if !ok {
		return nil
	}
	delete(m.openBuffers, path)
	if m.watcher != nil {
		_ = m.watcher.Unwatch(filepath.Dir(path))
	}
	var closeCmd tea.Cmd
	m.tabBar, closeCmd = m.tabBar.Update(messages.BufferClosedMsg{BufferID: bufID})
	if active := m.tabBar.ActiveTab(); active != nil {
		switchCmd := m.handleFileSelected(messages.FileSelectedMsg{Path: active.Path})
		return tea.Batch(closeCmd, switchCmd)
	}
	// No tabs remain: reset editor
	m.editor = editor.New(m.theme, m.cfg)
	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}
	editorWidth := m.width - sidebarW
	if editorWidth < 0 {
		editorWidth = 0
	}
	contentHeight := m.height - 3
	if contentHeight < 0 {
		contentHeight = 0
	}
	updated, _ := m.editor.Update(tea.WindowSizeMsg{Width: editorWidth, Height: contentHeight})
	m.editor = updated.(editor.Model)
	return closeCmd
}

// handleMouseClick routes mouse click events to the appropriate component based on position.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if m.quitDialogOpen {
		ow, oh := m.quitDialog.Dimensions()
		startX := (m.width - ow) / 2
		startY := (m.height - oh) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		localX := msg.X - startX
		localY := msg.Y - startY
		if localX < 0 || localX >= ow || localY < 0 || localY >= oh {
			// Click outside the dialog: cancel.
			return func() tea.Msg { return messages.QuitConfirmedMsg{Cancelled: true} }
		}
		local := tea.MouseClickMsg{Button: msg.Button, X: localX, Y: localY}
		updated, cmd := m.quitDialog.Update(local)
		m.quitDialog = updated
		return cmd
	}

	if m.themePickerOpen {
		ow, oh := m.themePicker.Dimensions()
		startX := (m.width - ow) / 2
		startY := (m.height - oh) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		localX := msg.X - startX
		localY := msg.Y - startY
		if localX < 0 || localX >= ow || localY < 0 || localY >= oh {
			// Click outside the picker: revert and close.
			orig := m.themePicker.ActiveTheme()
			return func() tea.Msg { return messages.ThemePickerClosedMsg{ThemeName: orig} }
		}
		local := tea.MouseClickMsg{Button: msg.Button, X: localX, Y: localY}
		updated, cmd := m.themePicker.Update(local)
		m.themePicker = updated
		return cmd
	}

	// If delete dialog is open in the filetree, forward all left-clicks to it.
	if msg.Button == tea.MouseLeft && m.fileTree.HasDeleteDialog() {
		var cmd tea.Cmd
		m.fileTree, cmd = m.fileTree.Update(msg)
		return cmd
	}

	tabBarHeight := 1
	statusBarHeight := 1
	breadcrumbHeight := 1

	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}

	x := msg.Mouse().X
	y := msg.Mouse().Y

	// Handle sidebar resize drag start
	if msg.Button == tea.MouseLeft && m.sidebarVisible {
		if x >= sidebarW-1 && x <= sidebarW+1 && y > tabBarHeight && y < m.height-statusBarHeight {
			m.sidebarDragging = true
			m.dragStartX = x
			m.dragStartWidth = m.sidebarWidth
			return nil
		}
	}

	// Route click to appropriate component region
	if y == 0 {
		// Tab bar
		var cmd tea.Cmd
		m.tabBar, cmd = m.tabBar.Update(msg)
		return cmd
	}

	if y >= m.height-statusBarHeight {
		// Normalize Y to 0 — statusbar occupies exactly one row and
		// its click handler expects row-local coordinates.
		normalizedMsg := tea.MouseClickMsg{Button: msg.Button, Mod: msg.Mod, X: x, Y: 0}
		updated, cmd := m.statusBar.Update(normalizedMsg)
		m.statusBar = updated.(statusbar.Model)
		return cmd
	}

	contentY := y - tabBarHeight

	if m.sidebarVisible && x < sidebarW {
		m.setFocus(FocusFileTree)
		adjustedMsg := tea.MouseClickMsg{
			Button: msg.Button,
			Mod:    msg.Mod,
			X:      x,
			Y:      contentY,
		}
		var cmd tea.Cmd
		m.fileTree, cmd = m.fileTree.Update(adjustedMsg)
		return cmd
	}

	editorX := x - sidebarW
	if editorX < 0 {
		editorX = 0
	}

	if y == tabBarHeight && !m.searchOpen {
		m.setFocus(FocusEditor)
		return nil
	}

	adjustedY := contentY - breadcrumbHeight
	if adjustedY < 0 {
		adjustedY = 0
	}

	if m.searchOpen {
		m.setFocus(FocusSearch)
		adjustedMsg := tea.MouseClickMsg{
			Button: msg.Button,
			Mod:    msg.Mod,
			X:      editorX,
			Y:      adjustedY,
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(adjustedMsg)
		return cmd
	}

	m.setFocus(FocusEditor)
	adjustedMsg := tea.MouseClickMsg{
		Button: msg.Button,
		Mod:    msg.Mod,
		X:      editorX,
		Y:      adjustedY,
	}
	updated, cmd := m.editor.Update(adjustedMsg)
	m.editor = updated.(editor.Model)
	return cmd
}

// handleMouseRelease handles mouse button release events (ends sidebar drag and editor drag).
func (m *Model) handleMouseRelease(msg tea.MouseReleaseMsg) tea.Cmd {
	if m.themePickerOpen || m.closeDialogOpen {
		return nil
	}
	if msg.Button == tea.MouseLeft {
		m.sidebarDragging = false
	}
	// Route tab bar row to tab bar (close button × and middle-click).
	if msg.Y == 0 {
		var cmd tea.Cmd
		m.tabBar, cmd = m.tabBar.Update(msg)
		return cmd
	}
	// Forward release to editor so it can reset its drag state.
	updated, cmd := m.editor.Update(msg)
	m.editor = updated.(editor.Model)
	return cmd
}

// handleMouseMotion routes mouse motion events, handling sidebar drag.
func (m *Model) handleMouseMotion(msg tea.MouseMotionMsg) tea.Cmd {
	if m.themePickerOpen {
		return nil
	}
	tabBarHeight := 1
	statusBarHeight := 1
	breadcrumbHeight := 1

	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}

	x := msg.Mouse().X
	y := msg.Mouse().Y

	if m.sidebarDragging {
		delta := x - m.dragStartX
		newWidth := m.dragStartWidth + delta
		if newWidth < 15 {
			newWidth = 15
		}
		if newWidth > m.width/2 {
			newWidth = m.width / 2
		}
		m.sidebarWidth = newWidth
		cmds := m.resizeComponents()
		return tea.Batch(cmds...)
	}

	// Route motion to appropriate component region
	if y == 0 {
		var cmd tea.Cmd
		m.tabBar, cmd = m.tabBar.Update(msg)
		return cmd
	}

	if y >= m.height-statusBarHeight {
		return nil
	}

	contentY := y - tabBarHeight

	if m.sidebarVisible && x < sidebarW {
		adjustedMsg := tea.MouseMotionMsg{
			Button: msg.Button,
			Mod:    msg.Mod,
			X:      x,
			Y:      contentY,
		}
		var cmd tea.Cmd
		m.fileTree, cmd = m.fileTree.Update(adjustedMsg)
		return cmd
	}

	editorX := x - sidebarW
	if editorX < 0 {
		editorX = 0
	}

	if y == tabBarHeight && !m.searchOpen {
		return nil
	}

	adjustedY := contentY - breadcrumbHeight
	if adjustedY < 0 {
		adjustedY = 0
	}

	if m.searchOpen {
		adjustedMsg := tea.MouseMotionMsg{
			Button: msg.Button,
			Mod:    msg.Mod,
			X:      editorX,
			Y:      adjustedY,
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(adjustedMsg)
		return cmd
	}

	adjustedMsg := tea.MouseMotionMsg{
		Button: msg.Button,
		Mod:    msg.Mod,
		X:      editorX,
		Y:      adjustedY,
	}
	updated, cmd := m.editor.Update(adjustedMsg)
	m.editor = updated.(editor.Model)
	return cmd
}

// handleFileSelected opens a file when selected from file tree or search.
func (m *Model) handleFileSelected(msg messages.FileSelectedMsg) tea.Cmd {
	path := msg.Path

	// Check if file is already open
	if bufID, ok := m.openBuffers[path]; ok {
		// Switch to existing buffer
		m.tabBar, _ = m.tabBar.Update(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		m.updateBreadcrumbs(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		m.updateStatusBar(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		// Re-open in editor to switch the displayed content
		m.setFocus(FocusEditor)
		cmd := m.editor.OpenFile(bufID, path)
		return cmd
	}

	// Assign a new buffer ID
	bufID := m.nextBufferID
	m.nextBufferID++
	m.openBuffers[path] = bufID

	// Open file in editor
	cmd := m.editor.OpenFile(bufID, path)

	// Emit BufferOpenedMsg
	openCmd := func() tea.Msg {
		return messages.BufferOpenedMsg{BufferID: bufID, Path: path}
	}

	// Watch the file for external changes
	if m.watcher != nil {
		_ = m.watcher.Watch(filepath.Dir(path))
	}

	// Start LSP for this file type
	if m.lspMgr != nil {
		lang := lsp.LanguageForPath(path)
		if lang != "" {
			go m.lspMgr.EnsureServer(lang)
		}
	}

	m.setFocus(FocusEditor)

	return tea.Batch(cmd, openCmd)
}

// setFocus changes keyboard focus to the given target.
func (m *Model) setFocus(target FocusTarget) {
	m.focus = target

	// Update focused state on components
	if target == FocusEditor {
		m.editor.Focus()
		m.fileTree.Blur()
	} else if target == FocusFileTree {
		m.editor.Blur()
		m.fileTree.Focus()
	} else if target == FocusSearch {
		m.editor.Blur()
		m.fileTree.Blur()
	}
}

// updateFocused sends a message to whichever component currently has focus.
func (m *Model) updateFocused(msg tea.Msg) tea.Cmd {
	switch m.focus {
	case FocusEditor:
		updated, cmd := m.editor.Update(msg)
		m.editor = updated.(editor.Model)
		return cmd
	case FocusFileTree:
		var cmd tea.Cmd
		m.fileTree, cmd = m.fileTree.Update(msg)
		return cmd
	case FocusSearch:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return cmd
	}
	return nil
}

// updateEditor forwards a message to the editor component with proper type assertion.
func (m *Model) updateEditor(msg tea.Msg) tea.Cmd {
	updated, cmd := m.editor.Update(msg)
	m.editor = updated.(editor.Model)
	return cmd
}

// updateBreadcrumbs forwards a message to the breadcrumbs component.
func (m *Model) updateBreadcrumbs(msg tea.Msg) {
	updated, _ := m.breadcrumb.Update(msg)
	m.breadcrumb = updated.(breadcrumbs.Model)
}

// updateStatusBar forwards a message to the status bar component.
func (m *Model) updateStatusBar(msg tea.Msg) {
	updated, _ := m.statusBar.Update(msg)
	m.statusBar = updated.(statusbar.Model)
}

// resizeComponents recalculates dimensions and forwards WindowSizeMsg to all components.
func (m *Model) resizeComponents() []tea.Cmd {
	tabBarHeight := 1
	statusBarHeight := 1
	breadcrumbHeight := 1

	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}

	contentHeight := m.height - tabBarHeight - statusBarHeight
	if contentHeight < 0 {
		contentHeight = 0
	}
	editorWidth := m.width - sidebarW
	if editorWidth < 0 {
		editorWidth = 0
	}
	editorHeight := contentHeight - breadcrumbHeight
	if editorHeight < 0 {
		editorHeight = 0
	}

	var cmds []tea.Cmd

	// Tab bar: full width, 1 row
	m.tabBar, _ = m.tabBar.Update(tea.WindowSizeMsg{Width: m.width, Height: 1})

	// File tree: sidebar width, content height
	m.fileTree, _ = m.fileTree.Update(tea.WindowSizeMsg{Width: sidebarW, Height: contentHeight})

	// Breadcrumbs: editor width, 1 row
	updated, _ := m.breadcrumb.Update(tea.WindowSizeMsg{Width: editorWidth, Height: 1})
	m.breadcrumb = updated.(breadcrumbs.Model)

	// Editor: editor width, editor height
	edUpdated, _ := m.editor.Update(tea.WindowSizeMsg{Width: editorWidth, Height: editorHeight})
	m.editor = edUpdated.(editor.Model)

	// Search: editor width, editor height (shares space with editor)
	m.search, _ = m.search.Update(tea.WindowSizeMsg{Width: editorWidth, Height: editorHeight})

	// Status bar: full width, 1 row
	sbUpdated, _ := m.statusBar.Update(tea.WindowSizeMsg{Width: m.width, Height: 1})
	m.statusBar = sbUpdated.(statusbar.Model)

	return cmds
}

// runGitStatus runs git status asynchronously and returns a GitStatusUpdatedMsg.
func (m *Model) runGitStatus() tea.Cmd {
	rootDir := m.rootDir
	return func() tea.Msg {
		result, err := git.Run(rootDir)
		if err != nil {
			return nil
		}
		// Convert git.FileStatus to messages.GitStatus
		fileStatuses := make(map[string]messages.GitStatus)
		for path, status := range result.Files {
			// Make paths absolute for matching
			absPath := path
			if !filepath.IsAbs(path) {
				absPath = filepath.Join(rootDir, path)
			}
			switch status {
			case git.StatusModified:
				fileStatuses[absPath] = messages.GitStatusModified
			case git.StatusAdded:
				fileStatuses[absPath] = messages.GitStatusAdded
			case git.StatusDeleted:
				fileStatuses[absPath] = messages.GitStatusDeleted
			case git.StatusUntracked:
				fileStatuses[absPath] = messages.GitStatusUntracked
			case git.StatusConflict:
				fileStatuses[absPath] = messages.GitStatusConflict
			default:
				fileStatuses[absPath] = messages.GitStatusClean
			}
		}
		return messages.GitStatusUpdatedMsg{
			FileStatuses: fileStatuses,
			Branch:       result.Branch,
			Ahead:        result.Ahead,
			Behind:       result.Behind,
		}
	}
}

// runGitDiff runs git diff for a specific file and returns a GitDiffUpdatedMsg.
func (m *Model) runGitDiff(bufferID int, path string) tea.Cmd {
	rootDir := m.rootDir
	return func() tea.Msg {
		kinds, err := git.RunDiff(rootDir, path, 10000) // generous line count
		if err != nil {
			return nil
		}
		return messages.GitDiffUpdatedMsg{
			BufferID:  bufferID,
			Path:      path,
			LineKinds: kinds,
		}
	}
}

// View renders the full application layout.
func (m *Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if !m.ready {
		v.Content = "Loading..."
		return v
	}

	// Tab bar (full width, 1 row)
	tabBarView := m.tabBar.View().Content

	// Sidebar
	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}

	// File tree view
	fileTreeView := ""
	if m.sidebarVisible {
		fileTreeView = m.fileTree.View().Content
	}

	// Breadcrumbs
	breadcrumbView := m.breadcrumb.View().Content

	// Editor or Search
	var mainContentView string
	var editorCursor *tea.Cursor
	if m.searchOpen {
		mainContentView = m.search.View().Content
	} else {
		editorView := m.editor.View()
		mainContentView = editorView.Content
		editorCursor = editorView.Cursor
	}

	// Right pane: breadcrumbs + editor/search
	rightPane := lipgloss.JoinVertical(lipgloss.Left, breadcrumbView, mainContentView)

	// Middle section: sidebar | right pane
	var middleSection string
	if m.sidebarVisible && sidebarW > 0 {
		middleSection = lipgloss.JoinHorizontal(lipgloss.Top, fileTreeView, rightPane)
	} else {
		middleSection = rightPane
	}

	// Status bar (full width, 1 row)
	statusBarView := m.statusBar.View().Content

	// Full layout: tabbar / middle / statusbar
	v.Content = lipgloss.JoinVertical(lipgloss.Left, tabBarView, middleSection, statusBarView)

	// Propagate the editor cursor to the app view with screen offset applied.
	if editorCursor != nil {
		const tabBarHeight = 1
		const breadcrumbHeight = 1
		c := *editorCursor
		c.X += sidebarW
		c.Y += tabBarHeight + breadcrumbHeight
		v.Cursor = &c
	}
	// Context menu overlay: apply after JoinHorizontal so lipgloss layout doesn't destroy it.
	// menuY is content-relative (tab bar already subtracted at click time), so add tabBarHeight
	// back to get full-screen row coordinates.
	if menuStr, menuX, menuY, ok := m.fileTree.ContextMenuOverlay(); ok {
		const tabBarHeight = 1
		v.Content = overlayAt(v.Content, menuStr, menuX, menuY+tabBarHeight, m.width, m.height)
	}
	if m.themePickerOpen {
		overlayStr := m.themePicker.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if m.closeDialogOpen {
		overlayStr := m.closeDialog.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if m.goToLineOpen {
		overlayStr := m.goToLine.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if m.quitDialogOpen {
		overlayStr := m.quitDialog.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if rendered, ok := m.fileTree.DeleteDialogOverlay(m.width, m.height); ok {
		v.Content = overlayCenter(v.Content, rendered, m.width, m.height)
	}
	return v
}
