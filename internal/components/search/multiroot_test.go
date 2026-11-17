package search

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/workspace"
)

func TestDisplayPathSingleRootIsBare(t *testing.T) {
	m := New(nil, "/work/web", config.SearchConfig{})
	if got := m.displayPath("/work/web/src/app.ts"); got != filepath.Join("src", "app.ts") {
		t.Fatalf("displayPath = %q", got)
	}
}

func TestDisplayPathMultiRootIsPrefixed(t *testing.T) {
	m := NewRoots(nil, []workspace.Root{
		{Name: "Frontend", Path: "/work/web"},
		{Name: "Backend", Path: "/work/api"},
	}, config.SearchConfig{})

	if got := m.displayPath("/work/web/src/app.ts"); got != filepath.Join("Frontend", "src", "app.ts") {
		t.Fatalf("displayPath = %q", got)
	}
	if got := m.displayPath("/work/api/main.go"); got != filepath.Join("Backend", "main.go") {
		t.Fatalf("displayPath = %q", got)
	}
}

func TestRunSearchRootsMergesResults(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}
	base := t.TempDir()
	web := filepath.Join(base, "web")
	api := filepath.Join(base, "api")
	for _, dir := range []string{filepath.Join(web, "src"), api} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(web, "src", "app.ts"): "const needle = 1\n",
		filepath.Join(api, "main.go"):       "var needle = 2\n",
		filepath.Join(api, "other.go"):      "no match here\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	roots := []workspace.Root{
		{Name: "Frontend", Path: web},
		{Name: "Backend", Path: api},
	}
	msg := runSearchRoots("needle", roots, "rg", []string{"--json"})()
	batch, ok := msg.(batchMsg)
	if !ok {
		t.Fatalf("expected batchMsg, got %T: %v", msg, msg)
	}
	if batch.totalMatches != 2 || batch.totalFiles != 2 {
		t.Fatalf("got %d matches in %d files, want 2 in 2", batch.totalMatches, batch.totalFiles)
	}

	paths := make(map[string]bool)
	for _, r := range batch.results {
		paths[r.Path] = true
	}
	if !paths[filepath.Join(web, "src", "app.ts")] || !paths[filepath.Join(api, "main.go")] {
		t.Fatalf("results missing a root: %v", paths)
	}
}

func TestRunSearchRootsDeduplicatesNestedRoots(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}
	base := t.TempDir()
	outer := filepath.Join(base, "repo")
	inner := filepath.Join(outer, "lib")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(outer, "main.go"):   "needle\n",
		filepath.Join(inner, "util.go"):   "needle\n",
		filepath.Join(inner, "helper.go"): "needle\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	roots := []workspace.Root{
		{Name: "repo", Path: outer},
		{Name: "lib", Path: inner},
	}
	msg := runSearchRoots("needle", roots, "rg", []string{"--json"})()
	batch, ok := msg.(batchMsg)
	if !ok {
		t.Fatalf("expected batchMsg, got %T: %v", msg, msg)
	}
	if len(batch.results) != 3 {
		t.Fatalf("got %d results, want 3 (one per file): %+v", len(batch.results), batch.results)
	}
	if batch.totalFiles != 3 {
		t.Fatalf("totalFiles = %d, want 3", batch.totalFiles)
	}
}
