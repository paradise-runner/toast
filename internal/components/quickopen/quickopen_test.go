package quickopen

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/workspace"
)

func writeFiles(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadFilesSpansRoots(t *testing.T) {
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	writeFiles(t, web, "src/app.ts", "index.ts", ".hidden.ts", "node_modules/dep.ts")
	writeFiles(t, api, "main.go")

	m := NewRoots(nil, []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	}, []string{"node_modules"})

	opened, cmd := m.Open()
	if cmd == nil {
		t.Fatal("expected a load command")
	}
	opened, _ = opened.Update(cmd())

	got := make(map[string]string)
	for _, file := range opened.files {
		got[file.RelativePath] = file.Path
	}
	want := map[string]string{
		filepath.Join("Frontend", "src", "app.ts"): filepath.Join(web, "src", "app.ts"),
		filepath.Join("Frontend", "index.ts"):      filepath.Join(web, "index.ts"),
		filepath.Join("Backend", "main.go"):        filepath.Join(api, "main.go"),
	}
	if len(got) != len(want) {
		t.Fatalf("loaded %v, want %v", got, want)
	}
	for rel, path := range want {
		if got[rel] != path {
			t.Fatalf("loaded[%q] = %q, want %q (all: %v)", rel, got[rel], path, got)
		}
	}

	// Typing filters across roots and keeps the absolute path for opening.
	opened, _ = opened.Update(tea.KeyPressMsg{Text: "a"})
	opened, _ = opened.Update(tea.KeyPressMsg{Text: "p"})
	opened, _ = opened.Update(tea.KeyPressMsg{Text: "p"})
	if len(opened.results) != 1 {
		t.Fatalf("results = %v, want the frontend app.ts", opened.results)
	}
	if opened.results[0].Path != filepath.Join(web, "src", "app.ts") {
		t.Fatalf("result path = %q", opened.results[0].Path)
	}
}

func TestLoadFilesDeduplicatesNestedRoots(t *testing.T) {
	base := t.TempDir()
	outer := filepath.Join(base, "repo")
	inner := filepath.Join(outer, "lib")
	writeFiles(t, outer, "main.go")
	writeFiles(t, inner, "util.go")

	m := NewRoots(nil, []workspace.Root{
		{Name: "repo", Path: outer},
		{Name: "lib", Path: inner},
	}, nil)
	_, cmd := m.Open()
	msg := cmd().(LoadFilesMsg)

	seen := make(map[string]int)
	for _, file := range msg.Files {
		seen[file.Path]++
	}
	if seen[filepath.Join(inner, "util.go")] != 1 {
		t.Fatalf("nested file counted %d times, want once: %v", seen[filepath.Join(inner, "util.go")], msg.Files)
	}
}

func TestSingleRootKeepsBareRelativePaths(t *testing.T) {
	base := t.TempDir()
	writeFiles(t, base, "pkg/main.go")

	m := New(nil, base, nil)
	_, cmd := m.Open()
	msg := cmd().(LoadFilesMsg)

	if len(msg.Files) != 1 {
		t.Fatalf("files = %v", msg.Files)
	}
	if msg.Files[0].RelativePath != filepath.Join("pkg", "main.go") {
		t.Fatalf("RelativePath = %q, want pkg/main.go", msg.Files[0].RelativePath)
	}
}
