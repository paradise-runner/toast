package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/session"
)

// TestMain removes the session debounce so a message that arms it does not make
// the suite wait a second per event. Tests that care about debouncing arm the
// timer themselves.
func TestMain(m *testing.M) {
	sessionSaveDelay = 0
	os.Exit(m.Run())
}

// newSessionApp builds an app whose session files live in a temp dir, so tests
// never read or write the developer's real state directory.
func newSessionApp(t *testing.T, root string, restore bool) (*Model, *session.Store) {
	t.Helper()
	cfg := config.Defaults()
	m, err := New(cfg, "", root, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	dir := t.TempDir()
	store := session.NewStore(dir)
	m.ConfigureSession(dir, cfg.Session, restore)
	return m, store
}

// writeSessionFile seeds a session file for the app's root.
func writeSessionFile(t *testing.T, m *Model, store *session.Store, st session.State) {
	t.Helper()
	st.Root = m.sessionRoot
	if err := store.Save(st); err != nil {
		t.Fatalf("seeding session: %v", err)
	}
}

// sessionTabFiles creates n files under dir and returns their absolute paths.
func sessionTabFiles(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestRestoreSessionReopensSavedTabs(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go", "b.go", "c.go")
	writeSessionFile(t, m, store, session.State{
		Version:   session.Version,
		ActiveTab: 1,
		Tabs: []session.Tab{
			{Path: "a.go", CursorLine: 3, CursorCol: 2, ViewportTop: 1, Folds: []int{2}},
			{Path: "b.go", CursorLine: 1},
			// The file is gone: it must not come back as a tab.
			{Path: "gone.go", CursorLine: 9},
			{Path: "c.go", CursorLine: 2, Preview: true},
		},
	})

	cmd := m.restoreSession()
	if cmd == nil {
		t.Fatal("restoreSession returned no command")
	}

	tabs := m.tabBar.Tabs()
	if len(tabs) != 3 {
		t.Fatalf("restored %d tabs, want 3: %+v", len(tabs), tabs)
	}
	for i, want := range []string{files[0], files[1], files[2]} {
		if tabs[i].Path != want {
			t.Fatalf("tab %d = %q, want %q", i, tabs[i].Path, want)
		}
		if _, open := m.openBuffers[want]; !open {
			t.Fatalf("%q was not registered in openBuffers", want)
		}
	}
	if got := m.tabBar.ActiveIndex(); got != 1 {
		t.Fatalf("active tab = %d, want 1 (the saved active tab)", got)
	}
	if msg, ok := cmd().(messages.FileSelectedMsg); !ok || msg.Path != files[1] {
		t.Fatalf("restore command = %#v, want FileSelectedMsg for b.go", cmd())
	}

	// Per-tab editing state waits for the file, so nothing is loaded yet.
	if len(m.bufferSnapshots) != 0 {
		t.Fatalf("bufferSnapshots = %d, want restore to stay cold", len(m.bufferSnapshots))
	}
	st, ok := m.pendingTabState[files[0]]
	if !ok || st.CursorLine != 3 || st.CursorCol != 2 || st.ViewportTop != 1 || len(st.Folds) != 1 {
		t.Fatalf("pending state for a.go = %+v, want the saved cursor/viewport/folds", st)
	}
	if !m.previewByBuffer[m.openBuffers[files[2]]] {
		t.Fatal("the saved preview flag was not restored for c.go")
	}
}

func TestRestoreSessionRestoresTreeAndSidebar(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "pkg/inner/deep.go")
	writeSessionFile(t, m, store, session.State{
		Version:   session.Version,
		ActiveTab: 0,
		Tabs:      []session.Tab{{Path: "pkg/inner/deep.go"}},
		Tree: session.TreeState{
			Expanded: []string{"pkg", filepath.Join("pkg", "inner")},
			Selected: "pkg/inner/deep.go",
			Offset:   2,
		},
		Sidebar: session.SidebarState{Visible: false, Width: 44},
	})

	if cmd := m.restoreSession(); cmd == nil {
		t.Fatal("restoreSession returned no command")
	}

	if m.sidebarVisible {
		t.Fatal("sidebar should have been restored hidden")
	}
	if m.sidebarWidth != 44 {
		t.Fatalf("sidebar width = %d, want 44", m.sidebarWidth)
	}
	expanded := m.fileTree.ExpandedPaths()
	if len(expanded) != 2 || expanded[0] != "pkg" || expanded[1] != filepath.Join("pkg", "inner") {
		t.Fatalf("expanded = %v, want pkg and pkg/inner", expanded)
	}
	if got := m.fileTree.SelectedPath(); got != filepath.Join("pkg", "inner", "deep.go") {
		t.Fatalf("selected tree row = %q, want pkg/inner/deep.go", got)
	}
	if got := m.tabBar.Tabs()[0].Path; got != files[0] {
		t.Fatalf("tab path = %q, want %q", got, files[0])
	}
}

func TestRestoreSessionSkipsWhenDisabled(t *testing.T) {
	tests := []struct {
		name    string
		restore bool
		mutate  func(*config.Config)
	}{
		{name: "no-restore flag", restore: false},
		{name: "session.restore false", restore: true, mutate: func(c *config.Config) { c.Session.Restore = false }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sessionTabFiles(t, root, "a.go")

			cfg := config.Defaults()
			if tc.mutate != nil {
				tc.mutate(&cfg)
			}
			m, err := New(cfg, "", root, "")
			if err != nil {
				t.Fatalf("app.New: %v", err)
			}
			dir := t.TempDir()
			store := session.NewStore(dir)
			m.ConfigureSession(dir, cfg.Session, tc.restore)
			writeSessionFile(t, m, store, session.State{
				Version: session.Version,
				Tabs:    []session.Tab{{Path: "a.go"}},
			})

			if cmd := m.restoreSession(); cmd != nil {
				t.Fatalf("restoreSession returned a command, want opt-out to skip restore")
			}
			if got := m.tabBar.Tabs(); len(got) != 0 {
				t.Fatalf("tabs = %+v, want none", got)
			}
		})
	}
}

func TestRestoreSessionIgnoresCorruptFile(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	if err := os.MkdirAll(filepath.Dir(store.Path(m.sessionRoot)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(m.sessionRoot), []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	if cmd := m.restoreSession(); cmd != nil {
		t.Fatal("a corrupt session must not produce a restore command")
	}
	if got := len(m.tabBar.Tabs()); got != 0 {
		t.Fatalf("tabs = %d, want 0", got)
	}
}

func TestRestoreSessionWithNoStoreIsInert(t *testing.T) {
	cfg := config.Defaults()
	m, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	if cmd := m.restoreSession(); cmd != nil {
		t.Fatal("restoreSession without ConfigureSession should do nothing")
	}
	if cmd := m.scheduleSessionSave(); cmd != nil {
		t.Fatal("scheduleSessionSave without ConfigureSession should do nothing")
	}
	if st, ok := m.captureSession(); ok {
		t.Fatalf("captureSession without a store wrote %+v", st)
	}
}

func TestInitRestoresSessionWithoutInitialFile(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go")
	writeSessionFile(t, m, store, session.State{
		Version: session.Version,
		Tabs:    []session.Tab{{Path: "a.go"}},
	})

	msgs := collectAppCmdMessages(t, m.Init())
	var found bool
	for _, msg := range msgs {
		if f, ok := msg.(messages.FileSelectedMsg); ok {
			if f.Path != files[0] {
				t.Fatalf("Init opened %q, want the restored tab %q", f.Path, files[0])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("Init did not open the restored tab: %v", msgs)
	}
}

func TestInitPrefersInitialFileOverSession(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "saved.go", "wanted.go")
	writeSessionFile(t, m, store, session.State{
		Version: session.Version,
		Tabs:    []session.Tab{{Path: "saved.go"}},
	})
	m.initialFile = files[1]

	var selected []string
	for _, msg := range collectAppCmdMessages(t, m.Init()) {
		if f, ok := msg.(messages.FileSelectedMsg); ok {
			selected = append(selected, f.Path)
		}
	}
	if len(selected) != 1 || selected[0] != files[1] {
		t.Fatalf("Init opened %v, want only the command-line file %q", selected, files[1])
	}
	if got := len(m.tabBar.Tabs()); got != 0 {
		t.Fatalf("tabs = %d, want the session untouched when a file is named", got)
	}
}

func TestColdTabStateAppliedWhenTheFileLoads(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	content := "func main() {\n\tif true {\n\t\tprintln(1)\n\t}\n\tprintln(2)\n}\n"
	files := sessionTabFiles(t, root, "warm.go", "cold.go")
	if err := os.WriteFile(files[1], []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSessionFile(t, m, store, session.State{
		Version:   session.Version,
		ActiveTab: 0,
		Tabs: []session.Tab{
			{Path: "warm.go"},
			{Path: "cold.go", CursorLine: 4, CursorCol: 1, ViewportTop: 0, Folds: []int{1}},
		},
	})
	if cmd := m.restoreSession(); cmd == nil {
		t.Fatal("restoreSession returned no command")
	}

	// Activate the cold tab and let the real load sequence run.
	load := m.handleFileSelected(messages.FileSelectedMsg{Path: files[1]})
	runAppCmds(t, m, load)

	if m.editor.Path() != files[1] {
		t.Fatalf("editor holds %q, want the cold tab", m.editor.Path())
	}
	st := m.editor.SessionState()
	if st.CursorLine != 4 || st.CursorCol != 1 {
		t.Fatalf("restored cursor = %d:%d, want 4:1", st.CursorLine, st.CursorCol)
	}
	if len(st.Folds) != 1 || st.Folds[0] != 1 {
		t.Fatalf("restored folds = %v, want [1]", st.Folds)
	}
	if _, pending := m.pendingTabState[files[1]]; pending {
		t.Fatal("pending state should be cleared once the file has loaded")
	}
}

func TestCaptureSessionUsesRelativePathsAndDropsMissingFiles(t *testing.T) {
	root := t.TempDir()
	m, _ := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "internal/app.go")
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "gone.go")

	for _, path := range []string{files[0], outside, missing} {
		bufID := m.nextBufferID
		m.nextBufferID++
		m.openBuffers[path] = bufID
		m.tabBar, _ = m.tabBar.Update(messages.BufferOpenedMsg{BufferID: bufID, Path: path})
	}
	m.sidebarVisible = false
	m.sidebarWidth = 51

	st, ok := m.captureSession()
	if !ok {
		t.Fatal("captureSession refused to build state")
	}
	if st.Root != m.sessionRoot {
		t.Fatalf("root = %q, want the resolved root %q", st.Root, m.sessionRoot)
	}
	if len(st.Tabs) != 2 {
		t.Fatalf("tabs = %+v, want the two existing files", st.Tabs)
	}
	if st.Tabs[0].Path != filepath.Join("internal", "app.go") {
		t.Fatalf("inside path = %q, want it relative to the root", st.Tabs[0].Path)
	}
	if st.Tabs[1].Path != outside {
		t.Fatalf("outside path = %q, want it absolute", st.Tabs[1].Path)
	}
	if st.Sidebar.Visible || st.Sidebar.Width != 51 {
		t.Fatalf("sidebar = %+v, want hidden at width 51", st.Sidebar)
	}
}

func TestCaptureSessionPreservesColdTabState(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	sessionTabFiles(t, root, "a.go", "b.go")
	writeSessionFile(t, m, store, session.State{
		Version:   session.Version,
		ActiveTab: 1,
		Tabs: []session.Tab{
			{Path: "a.go", CursorLine: 7, CursorCol: 3, Folds: []int{0}},
			{Path: "b.go"},
		},
	})
	if cmd := m.restoreSession(); cmd == nil {
		t.Fatal("restoreSession returned no command")
	}

	// a.go was never activated, so its state can only come from the session.
	st, ok := m.captureSession()
	if !ok {
		t.Fatal("captureSession refused to build state")
	}
	if len(st.Tabs) != 2 {
		t.Fatalf("tabs = %+v, want both", st.Tabs)
	}
	if st.Tabs[0].CursorLine != 7 || st.Tabs[0].CursorCol != 3 || len(st.Tabs[0].Folds) != 1 {
		t.Fatalf("cold tab state = %+v, want the restored cursor and fold", st.Tabs[0])
	}
	if st.ActiveTab != 1 {
		t.Fatalf("active tab = %d, want 1", st.ActiveTab)
	}
}

func TestSessionTickWritesFile(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go")
	openFile(t, m, files[0])

	tick := m.scheduleSessionSave()
	if tick == nil {
		t.Fatal("scheduleSessionSave returned no command")
	}
	msg, ok := tick().(messages.SessionTickMsg)
	if !ok {
		t.Fatalf("tick produced %T, want SessionTickMsg", tick())
	}
	if _, write := m.Update(msg); write != nil {
		runAppCmds(t, m, write)
	}

	st, err := store.Load(m.sessionRoot)
	if err != nil {
		t.Fatalf("session file was not written: %v", err)
	}
	if len(st.Tabs) != 1 || st.Tabs[0].Path != "a.go" {
		t.Fatalf("written tabs = %+v, want a.go", st.Tabs)
	}
}

func TestStaleSessionTickDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	sessionTabFiles(t, root, "a.go")

	stale := m.scheduleSessionSave()
	if stale == nil {
		t.Fatal("scheduleSessionSave returned no command")
	}
	m.scheduleSessionSave() // supersedes the first tick

	msg := stale().(messages.SessionTickMsg)
	if _, write := m.Update(msg); write != nil {
		runAppCmds(t, m, write)
	}
	if _, err := os.Stat(store.Path(m.sessionRoot)); !os.IsNotExist(err) {
		t.Fatalf("a stale tick wrote a session file (stat err = %v)", err)
	}
}

func TestQuitCmdFlushesSession(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go")
	openFile(t, m, files[0])

	cmd := m.quitCmd()
	if cmd == nil {
		t.Fatal("quitCmd returned no command")
	}
	st, err := store.Load(m.sessionRoot)
	if err != nil {
		t.Fatalf("quit did not flush the session: %v", err)
	}
	if len(st.Tabs) != 1 || st.Tabs[0].Path != "a.go" {
		t.Fatalf("flushed tabs = %+v, want a.go", st.Tabs)
	}
}

func TestPersistDisabledWritesNothing(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.Session.Persist = false
	m, err := New(cfg, "", root, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	dir := t.TempDir()
	store := session.NewStore(dir)
	m.ConfigureSession(dir, cfg.Session, true)
	sessionTabFiles(t, root, "a.go")
	openFile(t, m, filepath.Join(root, "a.go"))

	if cmd := m.scheduleSessionSave(); cmd != nil {
		t.Fatal("persist=false should not arm a save")
	}
	m.flushSession()
	if _, err := os.Stat(store.Path(m.sessionRoot)); !os.IsNotExist(err) {
		t.Fatalf("persist=false wrote a session file (stat err = %v)", err)
	}
}

func TestSwitchWorkspaceFlushesOutgoingSession(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go")
	openFile(t, m, files[0])

	oldRoot := m.sessionRoot
	other := t.TempDir()
	m.switchWorkspace(other)

	st, err := store.Load(oldRoot)
	if err != nil {
		t.Fatalf("switching workspaces did not write the outgoing session: %v", err)
	}
	if len(st.Tabs) != 1 || st.Tabs[0].Path != "a.go" {
		t.Fatalf("outgoing session tabs = %+v, want a.go", st.Tabs)
	}

	resolved, err := session.ResolveRoot(other)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if m.sessionRoot != resolved {
		t.Fatalf("session re-keyed on %q, want %q", m.sessionRoot, resolved)
	}
}

func TestBufferCloseDropsPendingTabState(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	files := sessionTabFiles(t, root, "a.go", "b.go")
	writeSessionFile(t, m, store, session.State{
		Version:   session.Version,
		ActiveTab: 1,
		Tabs:      []session.Tab{{Path: "a.go"}, {Path: "b.go"}},
	})
	if cmd := m.restoreSession(); cmd == nil {
		t.Fatal("restoreSession returned no command")
	}
	bufID := m.openBuffers[files[0]]

	if _, cmd := m.Update(messages.BufferClosedMsg{BufferID: bufID}); cmd != nil {
		_ = cmd
	}
	if _, pending := m.pendingTabState[files[0]]; pending {
		t.Fatal("closing a cold tab must drop its pending state")
	}
}

// runAppCmds delivers the messages a command produces to the model and follows
// what those messages produce, so an async file load settles. The session
// debounce is shortened in TestMain, so following it costs nothing.
func runAppCmds(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	runAppCmdsDepth(t, m, cmd, 0)
}

func runAppCmdsDepth(t *testing.T, m *Model, cmd tea.Cmd, depth int) {
	t.Helper()
	if cmd == nil || depth > 12 {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, inner := range batch {
			runAppCmdsDepth(t, m, inner, depth)
		}
		return
	}
	_, next := m.Update(msg)
	runAppCmdsDepth(t, m, next, depth+1)
}

func TestWindowResizeClampsRestoredSidebarWidth(t *testing.T) {
	root := t.TempDir()
	m, store := newSessionApp(t, root, true)
	sessionTabFiles(t, root, "a.go")
	writeSessionFile(t, m, store, session.State{
		Version: session.Version,
		Tabs:    []session.Tab{{Path: "a.go"}},
		Sidebar: session.SidebarState{Visible: true, Width: 70},
	})
	if cmd := m.restoreSession(); cmd == nil {
		t.Fatalf("restoreSession returned no command")
	}
	if m.sidebarWidth != 70 {
		t.Fatalf("restored width = %d, want 70 before any resize", m.sidebarWidth)
	}

	// The terminal the session was saved on was wider than this one.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.sidebarWidth != 40 {
		t.Fatalf("sidebar width = %d, want it clamped to half of 80", m.sidebarWidth)
	}

	// On a terminal narrower than 2*minSidebarWidth the half-width cap wins,
	// exactly as it does for a drag.
	m.Update(tea.WindowSizeMsg{Width: 24, Height: 24})
	if m.sidebarWidth != 12 {
		t.Fatalf("sidebar width = %d, want it capped at half of 24", m.sidebarWidth)
	}
}

func TestFreshOpenWritesNoSessionUntilWorkspacePicked(t *testing.T) {
	cfg := config.Defaults()
	home := t.TempDir()
	m, err := New(cfg, "", home, "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	dir := t.TempDir()
	store := session.NewStore(dir)
	m.ConfigureSession(dir, cfg.Session, false)
	m.ShowWorkspacePrompt()

	// The bundle's fresh-open state points at $HOME and has no chosen root.
	if cmd := m.scheduleSessionSave(); cmd != nil {
		t.Fatal("fresh-open state should not arm a session save")
	}
	m.flushSession()
	if _, err := os.Stat(store.Path(m.sessionRoot)); !os.IsNotExist(err) {
		t.Fatalf("fresh-open state wrote a session for the home directory (stat err = %v)", err)
	}

	// Picking a folder makes the workspace real: the outgoing (prompt) state is
	// still not recorded, and the new root starts persisting.
	picked := t.TempDir()
	sessionTabFiles(t, picked, "a.go")
	m.switchWorkspace(picked)
	if cmd := m.scheduleSessionSave(); cmd == nil {
		t.Fatal("a chosen workspace should arm a session save")
	}
	m.flushSession()
	resolved, err := session.ResolveRoot(picked)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if _, err := store.Load(resolved); err != nil {
		t.Fatalf("chosen workspace did not persist a session: %v", err)
	}
	if _, err := os.Stat(store.Path(home)); !os.IsNotExist(err) {
		t.Fatalf("switching workspaces recorded the home directory (stat err = %v)", err)
	}
}

func TestWorkspacePickRestoresSavedSession(t *testing.T) {
	cfg := config.Defaults()
	m, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	dir := t.TempDir()
	store := session.NewStore(dir)
	m.ConfigureSession(dir, cfg.Session, true)
	m.ShowWorkspacePrompt()

	// The bundled app's fresh-open state has no real root, so nothing restores.
	if cmd := m.restoreSession(); cmd != nil {
		t.Fatal("a visible workspace prompt must not restore a session")
	}

	// Picking a folder with a saved session brings its tabs back.
	picked := t.TempDir()
	files := sessionTabFiles(t, picked, "a.go", "b.go")
	resolved, err := session.ResolveRoot(picked)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if err := store.Save(session.State{
		Version:   session.Version,
		Root:      resolved,
		ActiveTab: 1,
		Tabs:      []session.Tab{{Path: "a.go"}, {Path: "b.go", CursorLine: 2}},
	}); err != nil {
		t.Fatalf("seeding session: %v", err)
	}

	cmds := m.switchWorkspace(picked)
	if cmds == nil {
		t.Fatal("switchWorkspace returned no commands")
	}
	tabs := m.tabBar.Tabs()
	if len(tabs) != 2 || tabs[0].Path != files[0] || tabs[1].Path != files[1] {
		t.Fatalf("tabs after picking a folder = %+v, want both saved files", tabs)
	}
	if got := m.tabBar.ActiveIndex(); got != 1 {
		t.Fatalf("active tab = %d, want the saved active tab 1", got)
	}
}
