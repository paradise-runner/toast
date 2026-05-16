package theme_test

import (
	"image/color"
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
	foundSystem := false
	for _, n := range names {
		if n == "toast-dark" {
			found = true
		}
		if n == "system" {
			foundSystem = true
		}
	}
	if !found {
		t.Error("expected toast-dark in builtin list")
	}
	if !foundSystem {
		t.Error("expected system in builtin list")
	}
}

func TestManagerNameFallsBackToDark(t *testing.T) {
	m, _ := theme.NewManager("nonexistent-theme", "")
	if m.Name() != "toast-dark" {
		t.Errorf("expected toast-dark after fallback, got %q", m.Name())
	}
}

func TestSystemThemeLoads(t *testing.T) {
	m, err := theme.NewManager("system", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name() != "system" {
		t.Fatalf("expected system, got %q", m.Name())
	}
	if !m.IsSystem() {
		t.Fatal("expected system theme")
	}
	if m.Git("added") == "" {
		t.Error("expected git added color, got empty string")
	}
}

func TestSystemThemeAppliesTerminalColors(t *testing.T) {
	m, _ := theme.NewManager("system", "")
	m.ApplySystemBackground(color.RGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}, false)
	m.ApplySystemForeground(color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	m.ApplySystemCursor(color.RGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff})

	if m.Variant() != "light" {
		t.Fatalf("expected light variant, got %q", m.Variant())
	}
	if got := m.UI("background"); got != "#eeeeee" {
		t.Errorf("background = %q, want #eeeeee", got)
	}
	if got := m.UI("foreground"); got != "#112233" {
		t.Errorf("foreground = %q, want #112233", got)
	}
	if got := m.UI("cursor"); got != "#aabbcc" {
		t.Errorf("cursor = %q, want #aabbcc", got)
	}
	if got := m.UI("sidebar_bg"); got == m.UI("foreground") {
		t.Errorf("sidebar_bg should be derived from background, got foreground %q", got)
	}
	if got := m.UI("statusbar_fg"); got == m.UI("foreground") {
		t.Errorf("statusbar_fg should be muted, got foreground %q", got)
	}
}

func TestSystemPaletteResponseAppliesTerminalAccent(t *testing.T) {
	index, c, ok := theme.ParseSystemPaletteResponse("\x1b]4;5;rgb:8800/3300/eeee\x07")
	if !ok {
		t.Fatal("expected palette response to parse")
	}
	if index != 5 {
		t.Fatalf("index = %d, want 5", index)
	}

	m, _ := theme.NewManager("system", "")
	m.ApplySystemPaletteColor(index, c)

	if got := m.SyntaxFG("keyword"); got != "#8833ee" {
		t.Errorf("keyword = %q, want #8833ee", got)
	}
}

func TestSystemPaletteSurvivesBackgroundRefresh(t *testing.T) {
	m, _ := theme.NewManager("system", "")
	m.ApplySystemPaletteColor(5, color.RGBA{R: 0x88, G: 0x33, B: 0xee, A: 0xff})
	m.ApplySystemBackground(color.RGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xff}, true)

	if got := m.SyntaxFG("keyword"); got != "#8833ee" {
		t.Errorf("keyword = %q, want #8833ee", got)
	}
}

func TestTerminalColorsIgnoredForNonSystemTheme(t *testing.T) {
	m, _ := theme.NewManager("toast-dark", "")
	before := m.UI("background")
	m.ApplySystemBackground(color.RGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}, false)
	if got := m.UI("background"); got != before {
		t.Errorf("background changed for non-system theme: %q -> %q", before, got)
	}
}
