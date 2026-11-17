package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadResolvesRelativePathsAndNames(t *testing.T) {
	base := t.TempDir()
	wsPath := filepath.Join(base, "acme.code-workspace")
	writeFile(t, wsPath, `{
		"folders": [
			{"path": "frontend"},
			{"path": "shared/lib", "name": "Shared"},
			{"path": "/tmp/absolute-root"}
		],
		"settings": {"editor.fontSize": 13}
	}`)

	ws, err := Load(wsPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ws.File != wsPath {
		t.Fatalf("File = %q, want %q", ws.File, wsPath)
	}
	want := []Root{
		{Name: "frontend", Path: filepath.Join(base, "frontend")},
		{Name: "Shared", Path: filepath.Join(base, "shared", "lib")},
		{Name: "absolute-root", Path: "/tmp/absolute-root"},
	}
	if len(ws.Roots) != len(want) {
		t.Fatalf("got %d roots, want %d: %+v", len(ws.Roots), len(want), ws.Roots)
	}
	for i := range want {
		if ws.Roots[i] != want[i] {
			t.Fatalf("root %d = %+v, want %+v", i, ws.Roots[i], want[i])
		}
	}
	if got := ws.Name(); got != "acme" {
		t.Fatalf("Name() = %q, want acme", got)
	}
	if !ws.Multi() {
		t.Fatal("expected Multi() for three roots")
	}
}

func TestLoadToleratesCommentsAndTrailingCommas(t *testing.T) {
	base := t.TempDir()
	wsPath := filepath.Join(base, "proj.code-workspace")
	writeFile(t, wsPath, "\xef\xbb\xbf"+`// VS Code writes this header itself
{
	"folders": [
		// the frontend
		{"path": "web", "name": "Web",},
		{"path": "api" /* server */},
	],
	"settings": {
		"editor.words": ["http://example.com/a//b", "a,}b"],
	},
}`)

	ws, err := Load(wsPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ws.Roots) != 2 {
		t.Fatalf("got %d roots, want 2: %+v", len(ws.Roots), ws.Roots)
	}
	if ws.Roots[0].Name != "Web" || ws.Roots[0].Path != filepath.Join(base, "web") {
		t.Fatalf("unexpected first root: %+v", ws.Roots[0])
	}
	if ws.Roots[1].Name != "api" || ws.Roots[1].Path != filepath.Join(base, "api") {
		t.Fatalf("unexpected second root: %+v", ws.Roots[1])
	}
}

func TestLoadSkipsEmptyAndDuplicateFolders(t *testing.T) {
	base := t.TempDir()
	wsPath := filepath.Join(base, "proj.code-workspace")
	writeFile(t, wsPath, `{"folders": [
		{"path": ""},
		{"path": "."},
		{"path": "./"},
		{"path": "lib", "name": "first"},
		{"path": "./lib", "name": "second"}
	]}`)

	ws, err := Load(wsPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ws.Roots) != 2 {
		t.Fatalf("got %d roots, want 2: %+v", len(ws.Roots), ws.Roots)
	}
	if ws.Roots[0].Path != base || ws.Roots[0].Name != filepath.Base(base) {
		t.Fatalf("unexpected first root: %+v", ws.Roots[0])
	}
	if ws.Roots[1].Name != "first" {
		t.Fatalf("duplicate kept the later entry: %+v", ws.Roots[1])
	}
}

func TestLoadErrors(t *testing.T) {
	base := t.TempDir()
	tests := []struct {
		name    string
		content string
		wantSub string
	}{
		{"no folders", `{"settings": {}}`, "declares no folders"},
		{"empty folders", `{"folders": []}`, "declares no folders"},
		{"only empty paths", `{"folders": [{"path": ""}]}`, "no usable folders"},
		{"invalid json", `{"folders": [}`, "parsing workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(base, "ws.code-workspace")
			writeFile(t, path, tt.content)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantSub)
			}
		})
	}

	if _, err := Load(filepath.Join(base, "missing.code-workspace")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestSingle(t *testing.T) {
	ws := Single("/tmp/project")
	if ws.Multi() {
		t.Fatal("Single() must not be multi")
	}
	if ws.File != "" {
		t.Fatalf("File = %q, want empty", ws.File)
	}
	if ws.Name() != "project" {
		t.Fatalf("Name() = %q, want project", ws.Name())
	}
	if got := ws.SessionKey(); got != "/tmp/project" {
		t.Fatalf("SessionKey() = %q, want /tmp/project", got)
	}
	if got := ws.Dir(); got != "/tmp/project" {
		t.Fatalf("Dir() = %q, want /tmp/project", got)
	}
}

func TestRootForPicksDeepestRoot(t *testing.T) {
	ws := &Workspace{Roots: []Root{
		{Name: "repo", Path: "/work/repo"},
		{Name: "lib", Path: "/work/repo/lib"},
		{Name: "other", Path: "/work/other"},
	}}
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/work/repo/main.go", "repo", true},
		{"/work/repo/lib/util.go", "lib", true},
		{"/work/repo/lib", "lib", true},
		{"/work/repo/libx/util.go", "repo", true},
		{"/work/elsewhere/x.go", "", false},
		{"/work/repository/x.go", "", false},
	}
	for _, tt := range tests {
		root, ok := ws.RootFor(tt.path)
		if ok != tt.ok || root.Name != tt.want {
			t.Fatalf("RootFor(%q) = %+v, %v; want %q, %v", tt.path, root, ok, tt.want, tt.ok)
		}
	}
}

func TestRelative(t *testing.T) {
	single := Single("/work/repo")
	if got := single.Relative("/work/repo/src/a.go"); got != filepath.Join("src", "a.go") {
		t.Fatalf("single Relative = %q", got)
	}

	multi := &Workspace{Roots: []Root{
		{Name: "frontend", Path: "/work/web"},
		{Name: "api", Path: "/work/api"},
	}}
	if got := multi.Relative("/work/web/src/a.ts"); got != filepath.Join("frontend", "src", "a.ts") {
		t.Fatalf("multi Relative = %q", got)
	}
	if got := multi.Relative("/work/outside/a.go"); got != "/work/outside/a.go" {
		t.Fatalf("outside Relative = %q", got)
	}
}

func TestIsWorkspaceFile(t *testing.T) {
	tests := map[string]bool{
		"proj.code-workspace":           true,
		"/tmp/Proj.CODE-WORKSPACE":      true,
		"/tmp/project":                  false,
		"/tmp/code-workspace":           false,
		"/tmp/proj.code-workspace.json": false,
	}
	for path, want := range tests {
		if got := IsWorkspaceFile(path); got != want {
			t.Fatalf("IsWorkspaceFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestStripJSONCPreservesStrings(t *testing.T) {
	in := `{"a": "x // not a comment", "b": "/* also not */", "c": [1,2,],}`
	var parsed map[string]any
	if err := json.Unmarshal(stripJSONC([]byte(in)), &parsed); err != nil {
		t.Fatalf("stripped JSON is invalid: %v\n%s", err, stripJSONC([]byte(in)))
	}
	if parsed["a"] != "x // not a comment" || parsed["b"] != "/* also not */" {
		t.Fatalf("strings were altered: %+v", parsed)
	}
}
