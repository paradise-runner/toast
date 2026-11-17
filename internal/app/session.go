package app

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/components/editor"
	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/session"
)

const (
	// sessionMaxAge is how long a session file for a root that is never opened
	// again is kept before pruning removes it.
	sessionMaxAge = 30 * 24 * time.Hour
	// minRestoredSidebarWidth matches the floor the drag-resize handler
	// enforces, so a stored width can never be smaller than a dragged one.
	minRestoredSidebarWidth = 15
)

// sessionSaveDelay is the quiet period after a session-relevant change before
// the state file is rewritten, so a burst of tab switches, tree expansions or a
// sidebar drag costs one write. It is a variable so tests can shorten the
// debounce instead of waiting it out.
var sessionSaveDelay = time.Second

// ConfigureSession enables per-root session persistence. stateDir holds the
// session files and restore selects whether this launch loads the saved session
// (the --no-restore flag and the bundled app's fresh-open state turn it off).
// Without this call session state is neither read nor written.
func (m *Model) ConfigureSession(stateDir string, cfg config.SessionConfig, restore bool) {
	if stateDir == "" {
		return
	}
	m.sessionStore = session.NewStore(stateDir)
	m.sessionPersist = cfg.Persist
	m.sessionRestore = cfg.Restore && restore
	m.setSessionRoot(m.rootDir)
}

// setSessionRoot re-keys the session store on a new workspace root, discarding
// the per-tab state that belonged to the previous one.
func (m *Model) setSessionRoot(rootDir string) {
	m.pendingTabState = nil
	if m.sessionStore == nil {
		return
	}
	root, err := session.ResolveRoot(rootDir)
	if err != nil {
		m.sessionRoot = ""
		return
	}
	m.sessionRoot = root
}

// sessionWritable reports whether a session file can be written for this root.
// The bundled app's fresh-open state has no user-chosen workspace yet — it just
// points at the home directory — so it must not record a session for it.
func (m *Model) sessionWritable() bool {
	return m.sessionStore != nil && m.sessionRoot != "" && m.sessionPersist && !m.workspacePromptVisible
}

// sessionRelevant reports whether msg can change anything a session stores, so
// that unrelated traffic (keystrokes, cursor moves, LSP replies) never arms the
// save timer.
func sessionRelevant(msg tea.Msg) bool {
	switch msg.(type) {
	case messages.FileSelectedMsg,
		messages.BufferOpenedMsg,
		messages.BufferClosedMsg,
		messages.ActiveBufferChangedMsg,
		messages.SidebarToggleMsg,
		messages.SessionDirtyMsg,
		messages.SettingsChangedMsg,
		messages.FileSavedMsg,
		messages.FileCreatedMsg,
		messages.DirCreatedMsg,
		messages.FileDeletedMsg,
		messages.DirDeletedMsg,
		messages.FileMovedMsg:
		return true
	}
	return false
}

// scheduleSessionSave arms the debounced session write. An earlier pending tick
// is superseded by bumping the generation, so only the last change in a burst
// writes.
func (m *Model) scheduleSessionSave() tea.Cmd {
	if !m.sessionWritable() {
		return nil
	}
	m.sessionGen++
	gen := m.sessionGen
	return tea.Tick(sessionSaveDelay, func(time.Time) tea.Msg {
		return messages.SessionTickMsg{Generation: gen}
	})
}

// writeSession captures the current state and persists it off the update loop.
// The state is captured here, not in the command, so a later mutation cannot
// race the write.
func (m *Model) writeSession() tea.Cmd {
	st, ok := m.captureSession()
	if !ok {
		return nil
	}
	store := m.sessionStore
	return func() tea.Msg {
		if err := store.Save(st); err != nil {
			return messages.SessionSaveFailedMsg{Err: err}
		}
		return nil
	}
}

// flushSession writes the session synchronously. Quit uses it so the state that
// is on screen is the state on disk; failures are only a warning because a
// session is advisory data.
func (m *Model) flushSession() {
	st, ok := m.captureSession()
	if !ok {
		return
	}
	if err := m.sessionStore.Save(st); err != nil {
		fmt.Fprintf(os.Stderr, "toast: session save failed: %v\n", err)
	}
}

// quitCmd flushes the session and quits, so every quit path leaves the current
// session behind.
func (m *Model) quitCmd() tea.Cmd {
	m.flushSession()
	return tea.Quit
}

// pruneSessionsCmd drops session files for roots that have not been opened in
// sessionMaxAge. It runs once per launch, off the update loop.
func (m *Model) pruneSessionsCmd() tea.Cmd {
	store := m.sessionStore
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		_ = store.Prune(sessionMaxAge)
		return nil
	}
}

// restoreSession reopens the tabs saved for the current root and returns the
// command that loads the active one. Only the active tab is read during
// restore: the editor loads one file at a time, so the remaining tabs are
// restored cold, with their saved editing state held in pendingTabState until
// each file is first opened.
//
// A visible workspace prompt means there is no root to restore yet (the bundled
// app's fresh-open state points at the home directory), so restoring waits for
// the user to pick a folder.
func (m *Model) restoreSession() tea.Cmd {
	if m.sessionStore == nil || m.sessionRoot == "" || !m.sessionRestore || m.workspacePromptVisible {
		return nil
	}
	st, err := m.sessionStore.Load(m.sessionRoot)
	if err != nil || st == nil || len(st.Tabs) == 0 {
		return nil
	}

	type restoredTab struct {
		path  string
		index int
	}
	var restored []restoredTab
	pending := make(map[string]editor.SessionState, len(st.Tabs))
	for i, tab := range st.Tabs {
		path := session.AbsPath(m.rootDir, tab.Path)
		if path == "" || !isRegularFile(path) {
			continue
		}
		if _, open := m.openBuffers[path]; open {
			continue
		}
		bufID := m.nextBufferID
		m.nextBufferID++
		m.openBuffers[path] = bufID
		m.tabBar, _ = m.tabBar.Update(messages.BufferOpenedMsg{BufferID: bufID, Path: path})
		pending[path] = editor.SessionState{
			CursorLine:   tab.CursorLine,
			CursorCol:    tab.CursorCol,
			ViewportTop:  tab.ViewportTop,
			ViewportLeft: tab.ViewportLeft,
			Folds:        tab.Folds,
		}
		if tab.Preview {
			m.previewByBuffer[bufID] = true
		}
		restored = append(restored, restoredTab{path: path, index: i})
	}
	if len(restored) == 0 {
		return nil
	}
	m.pendingTabState = pending

	// Prefer the tab that was active when the session was saved; fall back to
	// the first one that still exists when it was deleted meanwhile.
	active := restored[0].path
	for _, tab := range restored {
		if tab.index == st.ActiveTab {
			active = tab.path
			break
		}
	}
	if bufID, ok := m.openBuffers[active]; ok {
		m.tabBar, _ = m.tabBar.Update(messages.ActiveBufferChangedMsg{BufferID: bufID, Path: active})
	}

	m.sidebarVisible = st.Sidebar.Visible
	if st.Sidebar.Width >= minRestoredSidebarWidth {
		m.sidebarWidth = st.Sidebar.Width
	}
	m.fileTree.RestoreState(st.Tree.Expanded, st.Tree.Selected, st.Tree.Offset)

	path := active
	return func() tea.Msg { return messages.FileSelectedMsg{Path: path} }
}

// captureSession builds the state to persist for the current root. It reports
// false when nothing can be written: no store, no resolved root, or persistence
// switched off.
func (m *Model) captureSession() (session.State, bool) {
	if !m.sessionWritable() {
		return session.State{}, false
	}
	st := session.State{
		Version:   session.Version,
		Root:      m.sessionRoot,
		SavedAt:   time.Now().Unix(),
		ActiveTab: -1,
		Sidebar:   session.SidebarState{Visible: m.sidebarVisible, Width: m.sidebarWidth},
		Tree: session.TreeState{
			Expanded: m.fileTree.ExpandedPaths(),
			Selected: m.fileTree.SelectedPath(),
			Offset:   m.fileTree.Offset(),
		},
	}
	activeIndex := m.tabBar.ActiveIndex()
	for i, tab := range m.tabBar.Tabs() {
		// Files deleted or renamed while Toast was running must not resurrect as
		// tabs on the next launch.
		if !isRegularFile(tab.Path) {
			continue
		}
		es, _ := m.tabState(tab.BufferID, tab.Path)
		if i == activeIndex {
			st.ActiveTab = len(st.Tabs)
		}
		st.Tabs = append(st.Tabs, session.Tab{
			Path:         session.RelPath(m.rootDir, tab.Path),
			CursorLine:   es.CursorLine,
			CursorCol:    es.CursorCol,
			ViewportTop:  es.ViewportTop,
			ViewportLeft: es.ViewportLeft,
			Folds:        es.Folds,
			Preview:      m.previewByBuffer[tab.BufferID],
		})
	}
	return st, true
}

// tabState returns the editing state that belongs to one open tab: the live
// editor for the active tab, the background snapshot for a tab that has been
// visited this run, or the state restored from an earlier session for a cold
// tab that has not been activated yet.
func (m *Model) tabState(bufferID int, path string) (editor.SessionState, bool) {
	if bufferID == m.editor.BufferID() && m.editor.Path() == path {
		return m.editor.SessionState(), true
	}
	if snap, ok := m.bufferSnapshots[bufferID]; ok {
		return snap.SessionState(), true
	}
	if st, ok := m.pendingTabState[path]; ok {
		return st, true
	}
	return editor.SessionState{}, false
}

// isRegularFile reports whether path is an existing regular file. Restore and
// capture both use it to drop vanished entries instead of resurrecting them.
func isRegularFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
