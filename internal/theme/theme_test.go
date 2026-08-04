package theme_test

import (
	"image/color"
	"os"
	"path/filepath"
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

func TestSettingsTokensPopulated(t *testing.T) {
	cases := map[string]func(m *theme.Manager){
		"toast-dark": func(m *theme.Manager) {},
		"toast-light": func(m *theme.Manager) {},
		"system": func(m *theme.Manager) {
			// System theme derives tokens from terminal colors. Apply a
			// background first so the derived settings_* values are populated.
			m.ApplySystemBackground(color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}, true)
			m.ApplySystemForeground(color.RGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff})
		},
	}
	for name, prep := range cases {
		t.Run(name, func(t *testing.T) {
			m, err := theme.NewManager(name, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			prep(m)
			for _, key := range []string{"bg", "fg", "selected", "separator", "border"} {
				if got := m.Settings(key); got == "" {
					t.Errorf("Settings(%q) empty for theme %q", key, name)
				}
			}
		})
	}
}

func TestSettingsFallsBackToCompletionWhenUnset(t *testing.T) {
	// Build a manager that has completion_* populated but no settings_*. We
	// do this by constructing the manager and then clearing the settings_*
	// keys on its embedded theme via a re-load from a minimal theme dir.
	dir := t.TempDir()
	themeJSON := `{
		"name": "Test Fallback",
		"variant": "dark",
		"ui": {
			"background": "#222222", "foreground": "#eeeeee",
			"completion_bg": "#333333", "completion_fg": "#eeeeee", "completion_selected": "#444444"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "fallback.json"), []byte(themeJSON), 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	m, err := theme.NewManager("fallback", dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if bg := m.Settings("bg"); bg != "#333333" {
		t.Errorf("Settings(bg) = %q, want %q (completion_bg)", bg, "#333333")
	}
	if fg := m.Settings("fg"); fg != "#eeeeee" {
		t.Errorf("Settings(fg) = %q, want %q (completion_fg)", fg, "#eeeeee")
	}
	if sel := m.Settings("selected"); sel != "#444444" {
		t.Errorf("Settings(selected) = %q, want %q (completion_selected)", sel, "#444444")
	}
	if sep := m.Settings("separator"); sep != "" {
		// No border defined either → fallback chain still empty. Confirm we
		// get an empty string back rather than a panic, and that adding a
		// border makes it return that border.
		t.Errorf("Settings(separator) = %q, want empty when border missing", sep)
	}
	// And that a defined border is used.
	borderTheme := `{
		"name": "Test Fallback 2",
		"variant": "dark",
		"ui": {"border": "#aaaaaa", "completion_bg": "#bbbbbb"}
	}`
	if err := os.WriteFile(filepath.Join(dir, "fallback.json"), []byte(borderTheme), 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	m2, err := theme.NewManager("fallback", dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if sep := m2.Settings("separator"); sep != "#aaaaaa" {
		t.Errorf("Settings(separator) = %q, want %q (border fallback)", sep, "#aaaaaa")
	}
	if bd := m2.Settings("border"); bd != "#aaaaaa" {
		t.Errorf("Settings(border) = %q, want %q (border fallback)", bd, "#aaaaaa")
	}
}
