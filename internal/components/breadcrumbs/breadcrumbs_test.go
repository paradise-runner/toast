package breadcrumbs

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/theme"
	"github.com/paradise-runner/toast/internal/workspace"
)

func newModel(t *testing.T, roots []workspace.Root, path string) Model {
	t.Helper()
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme.NewManager: %v", err)
	}
	var m Model
	if len(roots) == 1 {
		m = New(tm, roots[0].Path)
	} else {
		m = NewRoots(tm, roots)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = updated.(Model)
	updated, _ = m.Update(messages.ActiveBufferChangedMsg{Path: path})
	return updated.(Model)
}

func TestSingleRootShowsRelativePath(t *testing.T) {
	m := newModel(t, []workspace.Root{{Name: "web", Path: "/work/web"}}, "/work/web/src/app.ts")
	view := m.View().Content
	if !strings.Contains(view, "src") || !strings.Contains(view, "app.ts") {
		t.Fatalf("breadcrumbs missing path: %q", view)
	}
	if strings.Contains(view, "web") {
		t.Fatalf("single-root breadcrumbs should not include the root label: %q", view)
	}
}

func TestMultiRootLeadsWithRootName(t *testing.T) {
	m := newModel(t, []workspace.Root{
		{Name: "Frontend", Path: "/work/web"},
		{Name: "Backend", Path: "/work/api"},
	}, "/work/web/src/app.ts")
	view := m.View().Content
	if !strings.Contains(view, "Frontend") {
		t.Fatalf("breadcrumbs missing root label: %q", view)
	}
	idxRoot := strings.Index(view, "Frontend")
	idxSrc := strings.Index(view, "src")
	if idxRoot < 0 || idxSrc < 0 || idxRoot > idxSrc {
		t.Fatalf("expected root before path: %q", view)
	}
}

func TestPathOutsideRootsFallsBackToAbsolute(t *testing.T) {
	m := newModel(t, []workspace.Root{
		{Name: "Frontend", Path: "/work/web"},
		{Name: "Backend", Path: "/work/api"},
	}, "/elsewhere/notes.md")
	view := m.View().Content
	if !strings.Contains(view, "elsewhere") {
		t.Fatalf("expected absolute fallback, got %q", view)
	}
}

func TestDisplayPathRelative(t *testing.T) {
	m := New(nil, "/work/web")
	if got := m.displayPath("/work/web/src/app.ts"); got != filepath.Join("src", "app.ts") {
		t.Fatalf("displayPath = %q", got)
	}
}
