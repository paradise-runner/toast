package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/yourusername/toast/internal/components/breadcrumbs"
	"github.com/yourusername/toast/internal/components/closedialog"
	"github.com/yourusername/toast/internal/components/editor"
	"github.com/yourusername/toast/internal/components/filetree"
	"github.com/yourusername/toast/internal/components/findreplace"
	"github.com/yourusername/toast/internal/components/gotoline"
	"github.com/yourusername/toast/internal/components/lspinstall"
	"github.com/yourusername/toast/internal/components/preview"
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

	findReplaceOpen bool
	findReplace     findreplace.Model

	quitDialogOpen bool
	quitDialog     quitdialog.Model

	lspInstall        lspinstall.Model
	pendingDefinition *messages.DefinitionResultMsg

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

	// previewOpen is true when the markdown preview pane is displayed instead of the editor.
	previewOpen bool

	// previewByBuffer tracks which buffer IDs have preview mode enabled, so
	// each markdown tab remembers its preview state across tab switches.
	previewByBuffer map[int]bool

	// bufferSnapshots stores the editor state for each open background buffer
	// so that unsaved edits are preserved when switching between tabs.
	bufferSnapshots map[int]editor.BufferSnapshot

	// closedSnapshots stores the editing state of closed tabs that had
	// unsaved changes. Keyed by file path. Preserved until quit so the user
	// can be prompted to save them. Re-opening the file restores from here.
	closedSnapshots map[string]editor.BufferSnapshot

	// pendingQuitSaves counts how many async saves are still in-flight during
	// a save-then-quit sequence. The app quits when this reaches zero.
	pendingQuitSaves int

	// Component models
	fileTree   filetree.Model
	tabBar     tabbar.Model
	editor     editor.Model
	statusBar  statusbar.Model
	breadcrumb breadcrumbs.Model
	search     search.Model
	preview    preview.Model

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
		cfg:             cfg,
		theme:           tm,
		focus:           FocusEditor,
		sidebarWidth:    cfg.Sidebar.Width,
		sidebarVisible:  cfg.Sidebar.Visible,
		rootDir:         rootDir,
		initialFile:     initialFile,
		nextBufferID:    1,
		openBuffers:     make(map[string]int),
		previewByBuffer: make(map[int]bool),
		bufferSnapshots: make(map[int]editor.BufferSnapshot),
		closedSnapshots: make(map[string]editor.BufferSnapshot),
		themeDir:        themeDir,
		configPath:      configPath,
		themePicker:     themepicker.New(tm, themeDir, cfg.Theme),
		goToLine:        gotoline.NewWithTheme(tm),
		findReplace:     findreplace.New(tm),
		lspInstall:      lspinstall.New(tm),

		fileTree:   filetree.New(tm, cfg, rootDir),
		tabBar:     tabbar.New(tm),
		editor:     editor.New(tm, cfg),
		statusBar:  statusbar.New(tm),
		breadcrumb: breadcrumbs.New(tm, rootDir),
		search:     search.New(tm, rootDir),
		preview:    preview.New(tm),
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

func (m *Model) requestSystemThemeColors() tea.Cmd {
	if !m.theme.IsSystem() {
		return nil
	}
	return tea.Batch(
		tea.RequestBackgroundColor,
		tea.RequestForegroundColor,
		tea.RequestCursorColor,
		tea.Raw(theme.SystemPaletteQuery()),
	)
}

// Init satisfies tea.Model.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.runGitStatus())
	if cmd := m.requestSystemThemeColors(); cmd != nil {
		cmds = append(cmds, cmd)
	}
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

	case tea.BackgroundColorMsg:
		m.theme.ApplySystemBackground(msg.Color, msg.IsDark())

	case tea.ForegroundColorMsg:
		m.theme.ApplySystemForeground(msg.Color)

	case tea.CursorColorMsg:
		m.theme.ApplySystemCursor(msg.Color)

	case uv.UnknownOscEvent:
		if index, c, ok := theme.ParseSystemPaletteResponse(string(msg)); ok {
			m.theme.ApplySystemPaletteColor(index, c)
		}

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

	case tea.MouseWheelMsg:
		cmd := m.handleMouseWheel(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.FileSelectedMsg:
		closeSearch := m.searchOpen
		cmd := m.handleFileSelected(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if closeSearch {
			m.closeSearch()
		}

	case messages.CloseTabRequestMsg:
		cmd := m.requestCloseTab(msg.BufferID, msg.Path)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case messages.CloseTabConfirmedMsg:
		// The close dialog is no longer shown on tab close; this case is kept
		// for safety but should not be reached in normal operation.
		m.closeDialogOpen = false

	case messages.BufferClosedMsg:
		// Remove from openBuffers map and unwatch.
		for path, id := range m.openBuffers {
			if id == msg.BufferID {
				if m.lspMgr != nil {
					if language := m.languageForPath(path); language != "" {
						m.lspMgr.DidClose(path, language)
					}
				}
				delete(m.openBuffers, path)
				delete(m.previewByBuffer, id)
				// Preserve unsaved changes so the user can be prompted at quit.
				if snap, ok := m.bufferSnapshots[id]; ok {
					if snap.Modified() {
						m.closedSnapshots[path] = snap
					}
					delete(m.bufferSnapshots, id)
				} else if m.editor.BufferID() == id && m.editor.IsModified() {
					// Active buffer being closed — snapshot its current state.
					snap := m.editor.Snapshot()
					snap.Path = path
					m.closedSnapshots[path] = snap
				}
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
		// Remove saved file from closedSnapshots if present.
		delete(m.closedSnapshots, msg.Path)
		// Refresh git status after save
		cmds = append(cmds, m.runGitStatus())
		// If a save-then-quit was requested, quit once all saves complete.
		if m.pendingQuit {
			m.pendingQuitSaves--
			if m.pendingQuitSaves <= 0 {
				m.pendingQuit = false
				return m, tea.Batch(append(cmds, tea.Quit)...)
			}
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

	case messages.DefinitionRequestMsg:
		if m.lspMgr != nil {
			if language := m.languageForPath(msg.Path); language != "" {
				m.lspMgr.Definition(msg.BufferID, msg.Path, language, msg.Line, msg.Col, msg.Navigate)
			}
		}

	case messages.DefinitionResultMsg:
		if msg.Navigate {
			if cmd := m.navigateToDefinition(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			m.updateEditor(msg)
		}

	case messages.LSPInstallPromptMsg:
		m.lspInstall.Show(msg.Language, msg.Name)

	case messages.LSPInstallRequestMsg:
		if m.lspMgr != nil {
			go m.lspMgr.Install(msg.Language)
		}

	case messages.LSPInstallStatusMsg:
		m.lspInstall, _ = m.lspInstall.Update(msg)

	case messages.SearchOpenMsg:
		m.openSearch()

	case messages.SearchCloseMsg:
		m.closeSearch()

	case messages.FindReplaceOpenMsg:
		m.openFindReplace("")

	case messages.FindReplaceCloseMsg:
		m.closeFindReplace()

	case messages.FindReplaceQueryChangedMsg:
		if cmd := m.updateEditor(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.syncFindReplaceStatus()

	case messages.FindReplaceNavigateMsg:
		if cmd := m.updateEditor(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.syncFindReplaceStatus()

	case messages.FindReplaceReplaceCurrentMsg:
		if cmd := m.updateEditor(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.syncFindReplaceStatus()

	case messages.FindReplaceReplaceAllMsg:
		if cmd := m.updateEditor(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.syncFindReplaceStatus()

	case messages.LSPServerStatusMsg:
		m.updateStatusBar(msg)
		m.lspInstall, _ = m.lspInstall.Update(msg)

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

	case messages.MarkdownPreviewToggleMsg:
		if isMarkdownPath(m.editor.Path()) {
			m.togglePreview()
		}

	case messages.FileLoadedMsg:
		if m.lspMgr != nil {
			if language := m.languageForPath(msg.Path); language != "" {
				m.lspMgr.DidOpen(msg.Path, language, msg.Content)
			}
		}
		if m.pendingDefinition != nil && m.pendingDefinition.Path == msg.Path {
			m.updateEditor(messages.GoToPositionMsg{Line: m.pendingDefinition.Line, Col: m.pendingDefinition.Col})
			m.pendingDefinition = nil
		}
		// Restore per-buffer preview state when a file finishes loading.
		wasOpen := m.previewByBuffer[msg.BufferID]
		if isMarkdownPath(msg.Path) && wasOpen {
			m.previewOpen = true
			m.preview.SetContent(msg.Content)
		} else {
			m.previewOpen = false
		}
		m.breadcrumb.SetPreviewOpen(m.previewOpen)

	case messages.OpenExternalFileMsg:
		if cmd := openExternalFile(msg.Path); cmd != nil {
			cmds = append(cmds, cmd)
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
			// Save every unsaved buffer (active editor + open snapshots + closed snapshots).
			var saveCmds []tea.Cmd
			if m.editor.IsModified() {
				updated, saveCmd := m.editor.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
				m.editor = updated.(editor.Model)
				if saveCmd != nil {
					saveCmds = append(saveCmds, saveCmd)
				}
			}
			for bufID, snap := range m.bufferSnapshots {
				if snap.Modified() {
					saveCmd := snap.SaveToDisk(bufID, snap.Path, m.cfg)
					m.bufferSnapshots[bufID] = snap
					if saveCmd != nil {
						saveCmds = append(saveCmds, saveCmd)
					}
				}
			}
			for path, snap := range m.closedSnapshots {
				if snap.Modified() {
					saveCmd := snap.SaveToDisk(0, path, m.cfg)
					m.closedSnapshots[path] = snap
					if saveCmd != nil {
						saveCmds = append(saveCmds, saveCmd)
					}
				}
			}
			if len(saveCmds) == 0 {
				return m, tea.Quit
			}
			m.pendingQuit = true
			m.pendingQuitSaves = len(saveCmds)
			cmds = append(cmds, saveCmds...)
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
		if cmd := m.requestSystemThemeColors(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.cfg.Theme = msg.ThemeName
		_ = config.Save(m.cfg, m.configPath)

	case messages.ThemeChangedMsg:
		_ = m.theme.Reload(msg.ThemeName, m.themeDir)
		if cmd := m.requestSystemThemeColors(); cmd != nil {
			cmds = append(cmds, cmd)
		}

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
	if m.lspInstall.Visible() {
		updated, cmd := m.lspInstall.Update(msg)
		m.lspInstall = updated
		return cmd
	}
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

	if m.findReplaceOpen {
		if isEscape(msg) {
			m.closeFindReplace()
			return nil
		}
		if isFindReplace(msg) {
			m.openFindReplace("")
			return nil
		}
		updated, cmd := m.findReplace.Update(msg)
		m.findReplace = updated
		if !m.findReplace.IsOpen() {
			m.findReplaceOpen = false
		}
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
		m.openSearch()
		return nil

	case isFindReplace(msg):
		m.openFindReplace(m.editor.SelectedText())
		return nil

	case isGoToLine(msg):
		m.goToLine = m.goToLine.Open(m.editor.LineCount())
		m.goToLineOpen = true
		return nil

	case isGoToDefinition(msg):
		if m.editor.Path() == "" {
			return nil
		}
		return func() tea.Msg {
			return messages.DefinitionRequestMsg{
				BufferID: m.editor.BufferID(), Path: m.editor.Path(),
				Line: m.editor.CursorLine(), Col: m.editor.CursorCol(), Navigate: true,
			}
		}

	case isMarkdownPreview(msg):
		if isMarkdownPath(m.editor.Path()) {
			m.togglePreview()
		}
		return nil

	case isNextTab(msg):
		cmd := m.tabBar.NextTab()
		return cmd

	case isPrevTab(msg):
		cmd := m.tabBar.PrevTab()
		return cmd

	case isEscape(msg):
		if m.searchOpen {
			m.closeSearch()
			return nil
		}
		if m.focus == FocusFileTree {
			if m.fileTree.HasTransientInteraction() {
				return m.updateFocused(msg)
			}
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
// unsavedFiles returns the count of files with unsaved changes and the path
// of one representative file (for single-file dialog titles).
func (m *Model) unsavedFiles() (count int, path string) {
	if m.editor.IsModified() {
		count++
		path = m.editor.Path()
	}
	for _, snap := range m.bufferSnapshots {
		if snap.Modified() {
			count++
			if path == "" {
				path = snap.Path
			}
		}
	}
	for _, snap := range m.closedSnapshots {
		if snap.Modified() {
			count++
			if path == "" {
				path = snap.Path
			}
		}
	}
	return count, path
}

func (m *Model) requestQuit() tea.Cmd {
	count, path := m.unsavedFiles()
	if count == 0 {
		return tea.Quit
	}
	m.quitDialog = quitdialog.New(m.theme, path, count)
	m.quitDialogOpen = true
	return nil
}

func (m *Model) requestCloseTab(bufferID int, path string) tea.Cmd {
	// Always close immediately — unsaved changes are preserved in closedSnapshots
	// and the user will be prompted at quit time.
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
	if m.lspMgr != nil {
		if language := m.languageForPath(path); language != "" {
			m.lspMgr.DidClose(path, language)
		}
	}
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
	if m.lspInstall.Visible() {
		ow, oh := m.lspInstall.Dimensions()
		startX := m.width - ow - 1
		startY := m.height - oh - 1
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}
		if msg.X >= startX && msg.X < startX+ow && msg.Y >= startY && msg.Y < startY+oh {
			local := tea.MouseClickMsg{Button: msg.Button, Mod: msg.Mod, X: msg.X - startX, Y: msg.Y - startY}
			updated, cmd := m.lspInstall.Update(local)
			m.lspInstall = updated
			return cmd
		}
	}
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

	if m.findReplaceOpen {
		ow, oh := m.findReplace.Dimensions()
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
		if localX >= 0 && localX < ow && localY >= 0 && localY < oh {
			local := tea.MouseClickMsg{Button: msg.Button, Mod: msg.Mod, X: localX, Y: localY}
			updated, cmd := m.findReplace.Update(local)
			m.findReplace = updated
			return cmd
		}
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
		// Forward to breadcrumbs so the preview button can handle clicks.
		crumbMsg := tea.MouseClickMsg{Button: msg.Button, Mod: msg.Mod, X: editorX, Y: 0}
		updated, cmd := m.breadcrumb.Update(crumbMsg)
		m.breadcrumb = updated.(breadcrumbs.Model)
		if cmd != nil {
			return cmd
		}
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
	tabBarHeight := 1
	statusBarHeight := 1
	breadcrumbHeight := 1

	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = m.sidebarWidth
	}

	x := msg.Mouse().X
	y := msg.Mouse().Y

	// Route tab bar row to tab bar (close button × and middle-click).
	if y == 0 {
		var cmd tea.Cmd
		m.tabBar, cmd = m.tabBar.Update(msg)
		return cmd
	}

	if y >= m.height-statusBarHeight {
		return nil
	}

	contentY := y - tabBarHeight

	if m.searchOpen {
		editorX := x - sidebarW
		if editorX < 0 {
			editorX = 0
		}
		adjustedY := contentY - breadcrumbHeight
		if adjustedY < 0 {
			adjustedY = 0
		}
		adjustedMsg := tea.MouseReleaseMsg{
			Button: msg.Button,
			Mod:    msg.Mod,
			X:      editorX,
			Y:      adjustedY,
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(adjustedMsg)
		return cmd
	}

	// Forward release to editor so it can reset its drag state.
	editorX := x - sidebarW
	if editorX < 0 {
		editorX = 0
	}
	adjustedY := contentY - breadcrumbHeight
	if adjustedY < 0 {
		adjustedY = 0
	}
	adjustedMsg := tea.MouseReleaseMsg{
		Button: msg.Button,
		Mod:    msg.Mod,
		X:      editorX,
		Y:      adjustedY,
	}
	updated, cmd := m.editor.Update(adjustedMsg)
	m.editor = updated.(editor.Model)
	return cmd
}

// handleMouseWheel routes scroll-wheel events to the component under the pointer.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.themePickerOpen || m.closeDialogOpen {
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

	if y == 0 || y >= m.height-statusBarHeight {
		return nil
	}

	contentY := y - tabBarHeight

	if m.sidebarVisible && x < sidebarW {
		adjustedMsg := tea.MouseWheelMsg{
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
	adjustedY := contentY - breadcrumbHeight
	if adjustedY < 0 {
		adjustedY = 0
	}
	adjustedMsg := tea.MouseWheelMsg{
		Button: msg.Button,
		Mod:    msg.Mod,
		X:      editorX,
		Y:      adjustedY,
	}

	if m.searchOpen {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(adjustedMsg)
		return cmd
	}
	if m.previewOpen {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(adjustedMsg)
		return cmd
	}
	updated, cmd := m.editor.Update(adjustedMsg)
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

	// Save the current buffer's state before switching away from it, but only
	// if it is still open. When called from BufferClosedMsg the editor still
	// holds the closed buffer; saving it here would duplicate the entry that
	// BufferClosedMsg already moved to closedSnapshots.
	if curID := m.editor.BufferID(); curID != 0 {
		for _, id := range m.openBuffers {
			if id == curID {
				m.bufferSnapshots[curID] = m.editor.Snapshot()
				break
			}
		}
	}

	// Check if file is already open
	if bufID, ok := m.openBuffers[path]; ok {
		// Switch to existing buffer
		m.tabBar, _ = m.tabBar.Update(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		m.updateBreadcrumbs(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		m.updateStatusBar(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: path})
		m.setFocus(FocusEditor)
		// Restore saved buffer state if available, otherwise load from disk.
		if snap, ok := m.bufferSnapshots[bufID]; ok {
			delete(m.bufferSnapshots, bufID)
			m.editor.RestoreSnapshot(snap, bufID, path)
			return nil
		}
		cmd := m.editor.OpenFile(bufID, path)
		return cmd
	}

	// Assign a new buffer ID
	bufID := m.nextBufferID
	m.nextBufferID++
	m.openBuffers[path] = bufID

	// If this file was previously closed with unsaved changes, restore from
	// the preserved snapshot instead of loading the (older) on-disk content.
	if snap, ok := m.closedSnapshots[path]; ok {
		delete(m.closedSnapshots, path)
		m.editor.RestoreSnapshot(snap, bufID, path)
		openCmd := func() tea.Msg {
			return messages.BufferOpenedMsg{BufferID: bufID, Path: path}
		}
		if m.watcher != nil {
			_ = m.watcher.Watch(filepath.Dir(path))
		}
		if m.lspMgr != nil {
			lang := m.languageForPath(path)
			if lang != "" {
				m.lspMgr.DidOpen(path, lang, m.editor.Content())
			}
		}
		m.setFocus(FocusEditor)
		return openCmd
	}

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
		lang := m.languageForPath(path)
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

func (m *Model) openSearch() {
	m.searchOpen = true
	m.setFocus(FocusSearch)
	m.search, _ = m.search.Update(messages.SearchOpenMsg{})
}

func (m *Model) closeSearch() {
	m.searchOpen = false
	m.setFocus(FocusEditor)
	m.search, _ = m.search.Update(messages.SearchCloseMsg{})
}

func (m *Model) openFindReplace(seed string) {
	if strings.Contains(seed, "\n") {
		seed = ""
	}
	if m.searchOpen {
		m.closeSearch()
	}
	m.findReplaceOpen = true
	m.setFocus(FocusEditor)
	m.findReplace = m.findReplace.Open(seed)
	if m.findReplace.Query() != "" {
		_ = m.updateEditor(messages.FindReplaceQueryChangedMsg{
			Query:     m.findReplace.Query(),
			MatchCase: m.findReplace.MatchCase(),
			WholeWord: m.findReplace.WholeWord(),
		})
	}
	m.syncFindReplaceStatus()
}

func (m *Model) closeFindReplace() {
	m.findReplaceOpen = false
	m.findReplace, _ = m.findReplace.Update(messages.FindReplaceCloseMsg{})
	_ = m.updateEditor(messages.FindReplaceCloseMsg{})
}

func (m *Model) syncFindReplaceStatus() {
	current, total := m.editor.FindStatus()
	m.findReplace.SetMatchStatus(current, total)
}

// updateFocused sends a message to whichever component currently has focus.
func (m *Model) updateFocused(msg tea.Msg) tea.Cmd {
	switch m.focus {
	case FocusEditor:
		// When preview is open, user-input events (keys, mouse) go to the
		// preview for scrolling. All other messages — internal editor events,
		// LSP results, git diffs, etc. — must still reach the editor so it
		// stays up to date even while the preview is displayed.
		if m.previewOpen {
			switch msg.(type) {
			case tea.KeyPressMsg, tea.MouseClickMsg, tea.MouseMotionMsg,
				tea.MouseReleaseMsg, tea.MouseWheelMsg:
				var cmd tea.Cmd
				m.preview, cmd = m.preview.Update(msg)
				return cmd
			}
		}
		beforePath := m.editor.Path()
		beforeContent := m.editor.Content()
		updated, cmd := m.editor.Update(msg)
		m.editor = updated.(editor.Model)
		m.syncLSPChange(beforePath, beforeContent)
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
	beforePath := m.editor.Path()
	beforeContent := m.editor.Content()
	updated, cmd := m.editor.Update(msg)
	m.editor = updated.(editor.Model)
	m.syncLSPChange(beforePath, beforeContent)
	return cmd
}

func (m *Model) syncLSPChange(beforePath, beforeContent string) {
	if m.lspMgr == nil || beforePath == "" || beforePath != m.editor.Path() || beforeContent == m.editor.Content() {
		return
	}
	if language := m.languageForPath(beforePath); language != "" {
		m.lspMgr.DidChange(beforePath, language, m.editor.Content())
	}
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

	// Find/replace overlay: sized to the editor pane.
	m.findReplace, _ = m.findReplace.Update(tea.WindowSizeMsg{Width: editorWidth, Height: editorHeight})

	// Preview: editor width, editor height (shares space with editor)
	m.preview, _ = m.preview.Update(tea.WindowSizeMsg{Width: editorWidth, Height: editorHeight})

	// Status bar: full width, 1 row
	sbUpdated, _ := m.statusBar.Update(tea.WindowSizeMsg{Width: m.width, Height: 1})
	m.statusBar = sbUpdated.(statusbar.Model)

	return cmds
}

func openExternalFile(path string) tea.Cmd {
	if path == "" {
		return nil
	}

	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", path)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
		default:
			cmd = exec.Command("xdg-open", path)
		}
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "toast: open external failed for %s: %v\n", path, err)
		}
		return nil
	}
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
			case git.StatusIgnored:
				fileStatuses[absPath] = messages.GitStatusIgnored
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
	// Definition discovery depends on receiving pointer movement without a
	// button held. CellMotion only reports drags; AllMotion reports hover.
	v.MouseMode = tea.MouseModeAllMotion

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

	// Editor, Search, or Markdown Preview
	var mainContentView string
	var editorCursor *tea.Cursor
	if m.searchOpen {
		mainContentView = m.search.View().Content
	} else if m.previewOpen {
		mainContentView = m.preview.View().Content
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
	if m.findReplaceOpen {
		overlayStr := m.findReplace.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if m.quitDialogOpen {
		overlayStr := m.quitDialog.Render()
		v.Content = overlayCenter(v.Content, overlayStr, m.width, m.height)
	}
	if m.lspInstall.Visible() {
		overlayStr := m.lspInstall.Render()
		ow, oh := m.lspInstall.Dimensions()
		v.Content = overlayAt(v.Content, overlayStr, m.width-ow-1, m.height-oh-1, m.width, m.height)
	}
	if rendered, ok := m.fileTree.DeleteDialogOverlay(m.width, m.height); ok {
		v.Content = overlayCenter(v.Content, rendered, m.width, m.height)
	}
	return v
}

func (m *Model) languageForPath(path string) string {
	return lsp.LanguageForPathConfig(path, m.cfg.LSP)
}

func (m *Model) navigateToDefinition(msg messages.DefinitionResultMsg) tea.Cmd {
	if msg.Path == "" {
		return nil
	}
	m.pendingDefinition = &msg
	cmd := m.handleFileSelected(messages.FileSelectedMsg{Path: msg.Path})
	if m.editor.Path() == msg.Path {
		m.updateEditor(messages.GoToPositionMsg{Line: msg.Line, Col: msg.Col})
		m.pendingDefinition = nil
	}
	return cmd
}

// isMarkdownPath returns true when path has a markdown file extension.
func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown") ||
		strings.HasSuffix(lower, ".mdx")
}

// togglePreview opens or closes the markdown preview pane, loading the
// current editor content when opening.
func (m *Model) togglePreview() {
	m.previewOpen = !m.previewOpen
	m.previewByBuffer[m.editor.BufferID()] = m.previewOpen
	if m.previewOpen {
		m.preview.SetContent(m.editor.Content())
	}
	m.breadcrumb.SetPreviewOpen(m.previewOpen)
}
