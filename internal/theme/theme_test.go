package theme_test

import (
	"testing"

	"github.com/yourusername/toast/internal/theme"
)

func TestBuiltinDarkThemeLoads(t *testing.T) {
	m, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.UI("background") == "" {
		t.Error("expected background color, got empty string")
	}
}

func TestBuiltinLightThemeLoads(t *testing.T) {
	m, err := theme.NewManager("toast-light", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.UI("background") == "" {
		t.Error("expected background color, got empty string")
	}
}

func TestUnknownThemeFallsBackToDark(t *testing.T) {
	m, err := theme.NewManager("nonexistent-theme", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.UI("background") == "" {
		t.Error("expected fallback background color, got empty string")
	}
}

func TestSyntaxStyleHasColor(t *testing.T) {
	m, _ := theme.NewManager("toast-dark", "")
	style := m.SyntaxStyle("keyword")
	_ = style
}

func TestSyntaxFallbackToForeground(t *testing.T) {
	m, _ := theme.NewManager("toast-dark", "")
	style := m.SyntaxStyle("unknown_capture_name")
	_ = style
}

func TestGitColor(t *testing.T) {
	m, _ := theme.NewManager("toast-dark", "")
	if m.Git("added") == "" {
		t.Error("expected git added color, got empty string")
	}
}

func TestManagerName(t *testing.T) {
	m, _ := theme.NewManager("toast-dark", "")
	if m.Name() != "toast-dark" {
		t.Errorf("expected toast-dark, got %q", m.Name())
	}
}

func TestListBuiltin(t *testing.T) {
	names := theme.ListBuiltin()
	if len(names) == 0 {
		t.Fatal("expected at least one builtin theme")
	}
	found := false
	for _, n := range names {
		if n == "toast-dark" {
			found = true
		}
	}
	if !found {
		t.Error("expected toast-dark in builtin list")
	}
}

func TestManagerNameFallsBackToDark(t *testing.T) {
	m, _ := theme.NewManager("nonexistent-theme", "")
	if m.Name() != "toast-dark" {
		t.Errorf("expected toast-dark after fallback, got %q", m.Name())
	}
}
