package filetree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/workspace"
)

// multiTree builds two independent roots:
//
//	web/  index.ts  src/app.ts
//	api/  main.go
func multiTree(t *testing.T) (Model, string, string) {
	t.Helper()
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	for _, dir := range []string{filepath.Join(web, "src"), api} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(web, "index.ts"):      "x",
		filepath.Join(web, "src", "app.ts"): "x",
		filepath.Join(api, "main.go"):       "x",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewRoots(nil, config.Config{}, []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	})
	return m, web, api
}

func TestNewRootsRendersOneRowPerRoot(t *testing.T) {
	m, web, api := multiTree(t)

	names := make(map[string]string)
	for _, node := range m.flat {
		names[node.Path] = node.Name
	}
	if names[web] != "Frontend" || names[api] != "Backend" {
		t.Fatalf("root labels = %q/%q, want Frontend/Backend", names[web], names[api])
	}
	if m.RootPath() != web {
		t.Fatalf("RootPath = %q, want the primary root %q", m.RootPath(), web)
	}

	// Both subtrees are expanded, and each root's children are indented one
	// level so they read as belonging to it.
	webIndex := -1
	for i, node := range m.flat {
		if node.Path == web {
			webIndex = i
		}
	}
	if webIndex < 0 {
		t.Fatal("frontend root row missing")
	}
	if got := m.nodeDepth(m.flat[webIndex]); got != 0 {
		t.Fatalf("root depth = %d, want 0", got)
	}
	child := findNode(t, m, filepath.Join(web, "src"))
	if got := m.nodeDepth(child); got != 1 {
		t.Fatalf("child depth = %d, want 1 in a multi-root tree", got)
	}
}

func findNode(t *testing.T, m Model, path string) *TreeNode {
	t.Helper()
	for _, node := range m.flat {
		if node.Path == path {
			return node
		}
	}
	t.Fatalf("node %q not in flat list %v", path, flatPaths(m))
	return nil
}

func TestCollapsingOneRootKeepsTheOther(t *testing.T) {
	m, web, api := multiTree(t)
	m.focused = true

	webIdx := -1
	for i, node := range m.flat {
		if node.Path == web {
			webIdx = i
		}
	}
	updated, _ := m.activateNode(webIdx)
	m = updated

	if !m.isRootNode(findNode(t, m, api)) {
		t.Fatal("backend root row disappeared when frontend collapsed")
	}
	for _, node := range m.flat {
		if node.Path == filepath.Join(web, "src") {
			t.Fatal("frontend child still visible after collapsing its root")
		}
	}
	if findNode(t, m, filepath.Join(api, "main.go")).Name != "main.go" {
		t.Fatal("backend child missing")
	}
}

func TestGitStatusAppliesPerRoot(t *testing.T) {
	m, web, api := multiTree(t)
	m.gitStatuses = map[string]messages.GitStatus{
		filepath.Join(web, "index.ts"): messages.GitStatusModified,
		filepath.Join(api, "main.go"):  messages.GitStatusUntracked,
	}
	m.applyGitStatus(m.gitStatuses)

	if got := findNode(t, m, filepath.Join(web, "index.ts")).GitStatus; got != messages.GitStatusModified {
		t.Fatalf("web status = %v", got)
	}
	if got := findNode(t, m, filepath.Join(api, "main.go")).GitStatus; got != messages.GitStatusUntracked {
		t.Fatalf("api status = %v", got)
	}
	if got := findNode(t, m, web).GitStatus; got != messages.GitStatusModified {
		t.Fatalf("web root aggregate = %v, want modified", got)
	}
}

func TestMultiRootSessionRoundTrip(t *testing.T) {
	m, web, api := multiTree(t)

	// Expand src under the frontend and select a backend file.
	m.expandDirChain(filepath.Join(web, "src"))
	m.rebuildFlat()
	m.cursor = 0
	for i, node := range m.flat {
		if node.Path == filepath.Join(api, "main.go") {
			m.cursor = i
		}
	}
	expanded := m.ExpandedPaths()
	selected := m.SelectedPath()
	if len(expanded) != 1 {
		t.Fatalf("expanded = %v, want one entry", expanded)
	}
	if selected == "" {
		t.Fatal("expected a selected key")
	}

	restored := NewRoots(nil, config.Config{}, []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	})
	restored.RestoreState(expanded, selected, 0)

	if got := restored.ExpandedPaths(); len(got) != 1 || got[0] != expanded[0] {
		t.Fatalf("restored expanded = %v, want %v", got, expanded)
	}
	if got := restored.SelectedPath(); got != selected {
		t.Fatalf("restored selected = %q, want %q", got, selected)
	}
	if got := restored.flat[restored.cursor].Path; got != filepath.Join(api, "main.go") {
		t.Fatalf("restored cursor file = %q", got)
	}
	if !restored.roots[0].Children[0].Expanded {
		t.Fatalf("restored tree did not expand %q", filepath.Join(web, "src"))
	}
}

func TestPathForKeyRejectsEscapes(t *testing.T) {
	m, _, _ := multiTree(t)

	for _, key := range []string{"../escape", "99/x", "0/../escape", "", "."} {
		if path, ok := m.pathForKey(key); ok {
			t.Fatalf("pathForKey(%q) = %q, want rejection", key, path)
		}
	}
	if path, ok := m.pathForKey(filepath.Join("1", "main.go")); !ok {
		t.Fatalf("valid key rejected: %q", path)
	}
}

func TestRootIndexForPrefersDeepestRoot(t *testing.T) {
	base := t.TempDir()
	outer := filepath.Join(base, "repo")
	inner := filepath.Join(outer, "lib")
	m := NewRoots(nil, config.Config{}, []workspace.Root{
		{Name: "repo", Path: outer},
		{Name: "lib", Path: inner},
	})

	root, index, ok := m.rootIndexFor(filepath.Join(inner, "a.go"))
	if !ok || index != 1 || root.Name != "lib" {
		t.Fatalf("rootIndexFor = %+v, %d, %v; want lib at index 1", root, index, ok)
	}
}
