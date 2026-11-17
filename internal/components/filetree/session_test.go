package filetree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
)

// sessionTree builds a tree over a small nested layout:
//
//	root/
//	  a/
//	    b/
//	      deep.go
//	    file.go
//	  top.go
func sessionTree(t *testing.T) (Model, string) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"a/b"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"a/file.go", "a/b/deep.go", "top.go"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return New(nil, config.Config{}, root), root
}

func TestExpandedPathsExcludesRoot(t *testing.T) {
	m, _ := sessionTree(t)

	if got := m.ExpandedPaths(); len(got) != 0 {
		t.Fatalf("fresh tree expanded = %v, want none below the root", got)
	}
	if !m.roots[0].Expanded {
		t.Fatal("the root row must stay expanded")
	}
}

func TestRestoreStateRebuildsNestedExpansion(t *testing.T) {
	m, _ := sessionTree(t)

	m.RestoreState([]string{filepath.Join("a", "b"), "a"}, "", 0)

	expanded := m.ExpandedPaths()
	want := []string{"a", filepath.Join("a", "b")}
	if len(expanded) != len(want) {
		t.Fatalf("expanded = %v, want %v", expanded, want)
	}
	for i, rel := range expanded {
		if rel != want[i] {
			t.Fatalf("expanded[%d] = %q, want %q", i, rel, want[i])
		}
	}

	// The nested file must be reachable in the flat list, which only holds rows
	// whose ancestors are expanded.
	var found bool
	for _, node := range m.flat {
		if node.Path == filepath.Join(m.rootDir, "a", "b", "deep.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("deep.go is not visible after restoring nested expansion: %v", flatPaths(m))
	}
}

func TestRestoreStateSelectsSavedRowAndOffset(t *testing.T) {
	m, root := sessionTree(t)
	m.height = 3

	m.RestoreState([]string{"a"}, filepath.Join("a", "file.go"), 0)

	want := filepath.Join(root, "a", "file.go")
	if got := m.flat[m.cursor].Path; got != want {
		t.Fatalf("selected = %q, want %q", got, want)
	}
	if got := m.SelectedPath(); got != filepath.Join("a", "file.go") {
		t.Fatalf("SelectedPath = %q, want a/file.go", got)
	}
}

func TestRestoreStateClampsOffset(t *testing.T) {
	m, _ := sessionTree(t)
	m.height = 2

	m.RestoreState([]string{"a"}, "", 99)
	if m.offset > len(m.flat)-1 {
		t.Fatalf("offset = %d, want it clamped into %d rows", m.offset, len(m.flat))
	}

	m.RestoreState([]string{"a"}, "", -5)
	if m.offset != 0 {
		t.Fatalf("offset = %d, want 0 for a negative saved offset", m.offset)
	}
}

func TestRestoreStateDropsMissingDirectories(t *testing.T) {
	m, root := sessionTree(t)

	m.RestoreState([]string{"gone", filepath.Join("a", "missing"), "a"}, filepath.Join("gone", "x.go"), 0)

	expanded := m.ExpandedPaths()
	if len(expanded) != 1 || expanded[0] != "a" {
		t.Fatalf("expanded = %v, want only \"a\"", expanded)
	}
	if got := m.flat[m.cursor].Path; got != root {
		t.Fatalf("selection = %q, want the root row when the saved row is gone", got)
	}
}

func TestRestoreStateDoesNotExpandFiles(t *testing.T) {
	m, _ := sessionTree(t)

	m.RestoreState([]string{"top.go", filepath.Join("a", "file.go")}, "", 0)

	if got := m.ExpandedPaths(); len(got) != 0 {
		t.Fatalf("expanded = %v, want files to be ignored", got)
	}
}

func TestSelectedPathOfRootRow(t *testing.T) {
	m, _ := sessionTree(t)
	m.cursor = 0
	if got := m.SelectedPath(); got != "" {
		t.Fatalf("SelectedPath = %q, want empty for the root row", got)
	}
}

func TestExpandCollapseEmitsSessionDirty(t *testing.T) {
	m, _ := sessionTree(t)

	// Row 1 is the "a" directory in the fresh tree.
	updated, cmd := m.activateNode(1)
	if cmd == nil {
		t.Fatal("expanding a directory should emit a message")
	}
	if got := updated.ExpandedPaths(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("expanded = %v, want [a]", got)
	}
	if _, ok := cmd().(messages.SessionDirtyMsg); !ok {
		t.Fatalf("expanding a directory emitted %T, want SessionDirtyMsg", cmd())
	}

	updated, cmd = updated.activateNode(1)
	if cmd == nil {
		t.Fatal("collapsing a directory should emit a message")
	}
	if _, ok := cmd().(messages.SessionDirtyMsg); !ok {
		t.Fatalf("collapsing a directory emitted %T, want SessionDirtyMsg", cmd())
	}
	if got := updated.ExpandedPaths(); len(got) != 0 {
		t.Fatalf("expanded = %v after collapse, want none", got)
	}
}

func flatPaths(m Model) []string {
	out := make([]string, 0, len(m.flat))
	for _, node := range m.flat {
		out = append(out, node.Path)
	}
	return out
}
