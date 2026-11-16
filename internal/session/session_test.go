package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleState(root string) State {
	return State{
		Version:   Version,
		Root:      root,
		SavedAt:   1700000000,
		ActiveTab: 1,
		Tabs: []Tab{
			{Path: "main.go", CursorLine: 4, CursorCol: 2, ViewportTop: 1, ViewportLeft: 3, Folds: []int{2, 10}},
			{Path: "/outside/notes.md", CursorLine: 1, Preview: true},
		},
		Tree: TreeState{
			Expanded: []string{"internal", "internal/app"},
			Selected: "internal/app/app.go",
			Offset:   7,
		},
		Sidebar: SidebarState{Visible: true, Width: 42},
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	root := filepath.Join(dir, "repo")
	want := sampleState(root)

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Root != want.Root {
		t.Fatalf("root = %q, want %q", got.Root, want.Root)
	}
	if got.ActiveTab != want.ActiveTab {
		t.Fatalf("active tab = %d, want %d", got.ActiveTab, want.ActiveTab)
	}
	if len(got.Tabs) != len(want.Tabs) {
		t.Fatalf("tabs = %d, want %d", len(got.Tabs), len(want.Tabs))
	}
	for i, tab := range got.Tabs {
		w := want.Tabs[i]
		if tab.Path != w.Path || tab.CursorLine != w.CursorLine || tab.CursorCol != w.CursorCol ||
			tab.ViewportTop != w.ViewportTop || tab.ViewportLeft != w.ViewportLeft || tab.Preview != w.Preview {
			t.Fatalf("tab %d = %+v, want %+v", i, tab, w)
		}
		if len(tab.Folds) != len(w.Folds) {
			t.Fatalf("tab %d folds = %v, want %v", i, tab.Folds, w.Folds)
		}
		for j, line := range tab.Folds {
			if line != w.Folds[j] {
				t.Fatalf("tab %d fold %d = %d, want %d", i, j, line, w.Folds[j])
			}
		}
	}
	if got.Tree.Selected != want.Tree.Selected || got.Tree.Offset != want.Tree.Offset {
		t.Fatalf("tree = %+v, want %+v", got.Tree, want.Tree)
	}
	if len(got.Tree.Expanded) != len(want.Tree.Expanded) {
		t.Fatalf("expanded = %v, want %v", got.Tree.Expanded, want.Tree.Expanded)
	}
	if got.Sidebar != want.Sidebar {
		t.Fatalf("sidebar = %+v, want %+v", got.Sidebar, want.Sidebar)
	}

	// The write must leave no temp files behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != Key(root)+".json" {
		t.Fatalf("session dir entries = %v, want only the session file", names(entries))
	}
}

func TestStorePathIsPerRootNotPerBasename(t *testing.T) {
	store := NewStore(t.TempDir())
	a := filepath.Join(string(filepath.Separator), "work", "one", "api")
	b := filepath.Join(string(filepath.Separator), "work", "two", "api")

	if store.Path(a) == store.Path(b) {
		t.Fatalf("two roots named api share %q", store.Path(a))
	}
	if Key(a) == Key(b) {
		t.Fatal("keys must differ for different roots")
	}
	if got, want := filepath.Base(store.Path(a)), Key(a)+".json"; got != want {
		t.Fatalf("session file name = %q, want %q", got, want)
	}
}

func TestStoreLoadMissingCorruptAndWrongVersion(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	root := filepath.Join(dir, "repo")

	if _, err := store.Load(root); err == nil {
		t.Fatal("expected an error when no session file exists")
	}

	path := store.Path(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(root); err == nil {
		t.Fatal("expected an error for corrupt JSON")
	}

	old := sampleState(root)
	old.Version = Version - 1
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(root); err == nil {
		t.Fatal("expected an error for an unsupported version")
	}

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(root); err == nil {
		t.Fatal("expected an error for an empty file")
	}
}

func TestStoreDisabled(t *testing.T) {
	store := NewStore("")
	if got := store.Path("/repo"); got != "" {
		t.Fatalf("Path = %q, want empty", got)
	}
	if err := store.Save(State{Root: "/repo"}); err == nil {
		t.Fatal("expected an error from a disabled store")
	}
	if _, err := store.Load("/repo"); err == nil {
		t.Fatal("expected an error from a disabled store")
	}
}

func TestStorePrune(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	root := filepath.Join(dir, "repo")
	if err := store.Save(sampleState(root)); err != nil {
		t.Fatal(err)
	}

	oldPath := filepath.Join(dir, "old.json")
	if err := os.WriteFile(oldPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, stale, stale); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keepPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(keepPath, stale, stale); err != nil {
		t.Fatal(err)
	}

	if err := store.Prune(30 * 24 * time.Hour); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("stale session file was not pruned")
	}
	if _, err := os.Stat(store.Path(root)); err != nil {
		t.Fatalf("current session file was pruned: %v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("non-session file was pruned: %v", err)
	}
}

func TestPruneMissingDir(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "absent"))
	if err := store.Prune(time.Hour); err != nil {
		t.Fatalf("Prune on a missing dir: %v", err)
	}
}

func TestResolveRootFollowsSymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	direct, err := ResolveRoot(target)
	if err != nil {
		t.Fatalf("ResolveRoot(target): %v", err)
	}
	viaLink, err := ResolveRoot(link)
	if err != nil {
		t.Fatalf("ResolveRoot(link): %v", err)
	}
	if direct != viaLink {
		t.Fatalf("symlinked and direct roots resolve differently: %q vs %q", viaLink, direct)
	}
}

func TestResolveRootMissingDirFallsBackToAbs(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "gone")
	got, err := ResolveRoot(missing)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if !filepath.IsAbs(got) || filepath.Base(got) != "gone" {
		t.Fatalf("ResolveRoot = %q, want the absolute missing path", got)
	}
}

func TestRelPathAndAbsPath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	tests := []struct {
		name   string
		path   string
		want   string
		inside bool
	}{
		{"inside", filepath.Join(root, "internal", "app.go"), filepath.Join("internal", "app.go"), true},
		{"outside", filepath.Join(string(filepath.Separator), "tmp", "x.go"), filepath.Join(string(filepath.Separator), "tmp", "x.go"), false},
		{"sibling prefix", root + "-other", root + "-other", false},
		{"root itself", root, root, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelPath(root, tc.path); got != tc.want {
				t.Fatalf("RelPath = %q, want %q", got, tc.want)
			}
			if got := InsideRoot(root, tc.path); got != tc.inside {
				t.Fatalf("InsideRoot = %v, want %v", got, tc.inside)
			}
			if got := AbsPath(root, RelPath(root, tc.path)); got != tc.path {
				t.Fatalf("AbsPath(RelPath(path)) = %q, want %q", got, tc.path)
			}
		})
	}
}

func TestAbsPathKeepsAbsolute(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "outside", "notes.md")
	if got := AbsPath(filepath.Join(string(filepath.Separator), "repo"), abs); got != abs {
		t.Fatalf("AbsPath = %q, want %q", got, abs)
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
