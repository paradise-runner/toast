package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/session"
	"github.com/paradise-runner/toast/internal/workspace"
)

func newWorkspaceModel(t *testing.T, roots []workspace.Root) *Model {
	t.Helper()
	m, err := NewWorkspace(config.Defaults(), "", &workspace.Workspace{Roots: roots}, "")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewWorkspaceSetsPrimaryRootAndBrowsesAll(t *testing.T) {
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	touchFile(t, filepath.Join(web, "index.ts"))
	touchFile(t, filepath.Join(api, "main.go"))

	m := newWorkspaceModel(t, []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	})

	if m.rootDir != web {
		t.Fatalf("rootDir = %q, want primary root %q", m.rootDir, web)
	}
	if m.fileTree.RootPath() != web {
		t.Fatalf("file tree primary root = %q", m.fileTree.RootPath())
	}
	view := m.fileTree.View().Content
	if !strings.Contains(view, "Frontend") || !strings.Contains(view, "Backend") {
		t.Fatalf("file tree missing a root row:\n%s", view)
	}

	// Opening a file in the second root makes the breadcrumbs lead with its
	// label, so the trail is unambiguous across roots.
	_, cmd := m.Update(messages.FileSelectedMsg{Path: filepath.Join(api, "main.go")})
	runAppCmd(t, m, cmd, 0)
	breadcrumbsView := m.breadcrumb.View().Content
	if !strings.Contains(breadcrumbsView, "Backend") || !strings.Contains(breadcrumbsView, "main.go") {
		t.Fatalf("breadcrumbs = %q, want Backend / main.go", breadcrumbsView)
	}
}

func TestSwitchWorkspaceOpensCodeWorkspaceFile(t *testing.T) {
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	touchFile(t, filepath.Join(web, "index.ts"))
	touchFile(t, filepath.Join(api, "main.go"))
	wsFile := filepath.Join(base, "acme.code-workspace")
	if err := os.WriteFile(wsFile, []byte(`{
		"folders": [
			{"path": "web", "name": "Frontend"},
			{"path": "api", "name": "Backend"},
		],
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestApp(t, t.TempDir())
	updated, cmd := m.Update(uv.UnknownOscEvent("\x1b]1337;toast-open-folder;" + wsFile + "\x1b\\"))
	if cmd == nil {
		t.Fatal("expected a command from toast-open-folder")
	}
	got := updated.(*Model)
	if got.ws == nil || got.ws.File != wsFile {
		t.Fatalf("ws = %+v, want the workspace file %q", got.ws, wsFile)
	}
	if got.rootDir != web {
		t.Fatalf("rootDir = %q, want %q", got.rootDir, web)
	}
	if got.fileTree.RootPath() != web {
		t.Fatalf("file tree primary root = %q", got.fileTree.RootPath())
	}
	view := got.fileTree.View().Content
	if !strings.Contains(view, "Frontend") || !strings.Contains(view, "Backend") {
		t.Fatalf("file tree missing a root row:\n%s", view)
	}

	// A non-workspace file is refused, leaving the current workspace intact.
	other := filepath.Join(base, "notes.txt")
	touchFile(t, other)
	updated, cmd = got.Update(uv.UnknownOscEvent("\x1b]1337;toast-open-folder;" + other + "\x1b\\"))
	if cmd != nil {
		t.Fatal("expected no command for a plain file")
	}
	if updated.(*Model).ws.File != wsFile {
		t.Fatal("plain file changed the workspace")
	}
}

func TestRunGitStatusMergesWorkspaceRoots(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	for _, dir := range []string{web, api} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v\n%s", dir, err, out)
		}
	}
	touchFile(t, filepath.Join(web, "index.ts"))
	touchFile(t, filepath.Join(api, "main.go"))

	m := newWorkspaceModel(t, []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	})
	cmd := m.runGitStatus()
	if cmd == nil {
		t.Fatal("expected a git status command")
	}
	msg, ok := cmd().(messages.GitStatusUpdatedMsg)
	if !ok {
		t.Fatalf("expected GitStatusUpdatedMsg, got %T", cmd())
	}
	if msg.FileStatuses[filepath.Join(web, "index.ts")] != messages.GitStatusUntracked {
		t.Fatalf("web file status missing: %v", msg.FileStatuses)
	}
	if msg.FileStatuses[filepath.Join(api, "main.go")] != messages.GitStatusUntracked {
		t.Fatalf("api file status missing: %v", msg.FileStatuses)
	}
	if msg.Branch == "" {
		t.Fatal("expected a branch label")
	}
}

func TestSessionKeyedOnWorkspaceFile(t *testing.T) {
	base := t.TempDir()
	web := filepath.Join(base, "web")
	touchFile(t, filepath.Join(web, "index.ts"))
	wsFile := filepath.Join(base, "acme.code-workspace")
	if err := os.WriteFile(wsFile, []byte(`{"folders": [{"path": "web"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Load(wsFile)
	if err != nil {
		t.Fatalf("workspace.Load: %v", err)
	}

	m, err := NewWorkspace(config.Defaults(), "", ws, "")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	m.ConfigureSession(t.TempDir(), config.Defaults().Session, true)

	resolved, err := session.ResolveRoot(wsFile)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if m.sessionRoot != resolved {
		t.Fatalf("sessionRoot = %q, want %q", m.sessionRoot, resolved)
	}
}

func TestWorkspaceRootsFallsBackToPrimary(t *testing.T) {
	m := &Model{rootDir: "/tmp/one"}
	if got := m.workspaceRoots(); len(got) != 1 || got[0] != "/tmp/one" {
		t.Fatalf("workspaceRoots = %v", got)
	}
}

func TestWorkspaceSessionRestoresTabInSecondRoot(t *testing.T) {
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	touchFile(t, filepath.Join(web, "index.ts"))
	touchFile(t, filepath.Join(api, "main.go"))
	wsFile := filepath.Join(base, "acme.code-workspace")
	if err := os.WriteFile(wsFile, []byte(`{"folders": [{"path": "web"}, {"path": "api"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := workspace.Load(wsFile)
	if err != nil {
		t.Fatalf("workspace.Load: %v", err)
	}
	cfg := config.Defaults()
	stateDir := t.TempDir()

	m, err := NewWorkspace(cfg, "", ws, "")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	m.ConfigureSession(stateDir, cfg.Session, true)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = cmd
	_, cmd = m.Update(messages.FileSelectedMsg{Path: filepath.Join(api, "main.go")})
	runAppCmd(t, m, cmd, 0)

	st, ok := m.captureSession()
	if !ok {
		t.Fatal("captureSession refused")
	}
	if err := m.sessionStore.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	restored, err := NewWorkspace(cfg, "", ws, "")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	restored.ConfigureSession(stateDir, cfg.Session, true)
	restoreCmd := restored.restoreSession()
	if restoreCmd == nil {
		t.Fatal("restoreSession returned no command")
	}
	tabs := restored.tabBar.Tabs()
	if len(tabs) != 1 || tabs[0].Path != filepath.Join(api, "main.go") {
		t.Fatalf("restored tabs = %+v, want the backend file", tabs)
	}
	runAppCmd(t, restored, restoreCmd, 0)
	if got := restored.editor.Path(); got != filepath.Join(api, "main.go") {
		t.Fatalf("editor path = %q, want the backend file", got)
	}
}
