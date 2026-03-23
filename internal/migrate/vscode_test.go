package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yourusername/toast/internal/theme"
)

// minimalVSCodeTheme returns a complete VSCode theme JSON that should convert
// successfully with all 54 Toast tokens resolved.
func minimalVSCodeTheme() map[string]interface{} {
	return map[string]interface{}{
		"name": "Test Theme",
		"type": "dark",
		"colors": map[string]string{
			"editor.background":                    "#1f1f28",
			"editor.foreground":                    "#dcd7ba",
			"editorCursor.foreground":              "#dcd7ba",
			"editor.selectionBackground":           "#223249",
			"editor.lineHighlightBackground":       "#2a2a37",
			"editorGroup.border":                   "#16161d",
			"tab.activeBackground":                 "#363646",
			"tab.activeForeground":                 "#dcd7ba",
			"tab.inactiveBackground":               "#1f1f28",
			"sideBar.foreground":                   "#dcd7ba",
			"sideBar.background":                   "#1f1f28",
			"list.activeSelectionBackground":       "#363646",
			"list.activeSelectionForeground":       "#dcd7ba",
			"statusBar.background":                 "#16161d",
			"statusBar.foreground":                 "#c8c093",
			"editorLineNumber.foreground":          "#54546d",
			"editorLineNumber.activeForeground":    "#957fb8",
			"editorError.foreground":               "#e82424",
			"editorWarning.foreground":             "#ff9e3b",
			"editorSuggestWidget.background":       "#223249",
			"editorSuggestWidget.selectedBackground": "#2d4f67",
			"editorHoverWidget.background":         "#1f1f28",
			"editorHoverWidget.border":             "#2a2a37",
			"editor.findMatchHighlightBackground":  "#2d4f67",
			"editor.findMatchBackground":           "#2d4f67",
			"editorGutter.addedBackground":         "#76946a",
			"editorGutter.modifiedBackground":      "#dca561",
			"editorGutter.deletedBackground":       "#c34043",
			"gitDecoration.untrackedResourceForeground": "#727169",
		},
		"tokenColors": []map[string]interface{}{
			{
				"scope":    []string{"comment", "punctuation.definition.comment"},
				"settings": map[string]string{"foreground": "#727169"},
			},
			{
				"scope":    []string{"variable", "string constant.other.placeholder"},
				"settings": map[string]string{"foreground": "#dcd7ba"},
			},
			{
				"scope":    []string{"keyword.control", "storage.type", "storage.modifier"},
				"settings": map[string]interface{}{"foreground": "#957fb8", "fontStyle": "bold"},
			},
			{
				"scope":    "keyword.operator",
				"settings": map[string]string{"foreground": "#c0a36e"},
			},
			{
				"scope":    []string{"punctuation", "meta.brace"},
				"settings": map[string]string{"foreground": "#9cabca"},
			},
			{
				"scope":    []string{"entity.name.tag"},
				"settings": map[string]string{"foreground": "#e6c384"},
			},
			{
				"scope":    []string{"entity.name.function", "support.function"},
				"settings": map[string]string{"foreground": "#7e9cd8"},
			},
			{
				"scope":    "constant.numeric",
				"settings": map[string]string{"foreground": "#d27e99"},
			},
			{
				"scope":    []string{"string", "punctuation.definition.string"},
				"settings": map[string]string{"foreground": "#98bb6c"},
			},
			{
				"scope":    []string{"entity.name", "support.type"},
				"settings": map[string]string{"foreground": "#7aa89f"},
			},
			{
				"scope":    []string{"constant.language", "support.constant"},
				"settings": map[string]string{"foreground": "#7fb4ca"},
			},
			{
				"scope":    []string{"entity.other.attribute-name"},
				"settings": map[string]string{"foreground": "#e6c384"},
			},
			{
				"scope":    "variable.other.property",
				"settings": map[string]string{"foreground": "#e6c384"},
			},
			{
				"scope":    []string{"entity.name.namespace", "entity.name.type.module"},
				"settings": map[string]string{"foreground": "#dcd7ba"},
			},
			{
				"scope":    "variable.language",
				"settings": map[string]string{"foreground": "#ff5d62"},
			},
		},
	}
}

func writeThemeJSON(t *testing.T, dir string, data map[string]interface{}) string {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test-theme.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConvertVSCode_FullTheme(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := writeThemeJSON(t, tmpDir, minimalVSCodeTheme())
	outDir := filepath.Join(tmpDir, "out")

	result, err := ConvertVSCode(inputPath, outDir)
	if err != nil {
		t.Fatalf("ConvertVSCode failed: %v", err)
	}

	if result.Name != "Test Theme" {
		t.Errorf("name = %q, want %q", result.Name, "Test Theme")
	}
	if filepath.Base(result.OutputPath) != "test-theme.json" {
		t.Errorf("output file = %q, want test-theme.json", filepath.Base(result.OutputPath))
	}

	// Read and parse the output.
	data, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	var th theme.Theme
	if err := json.Unmarshal(data, &th); err != nil {
		t.Fatalf("parsing output theme: %v", err)
	}

	if th.Variant != "dark" {
		t.Errorf("variant = %q, want dark", th.Variant)
	}

	// Check all 34 UI tokens exist.
	for _, m := range uiMappings {
		if _, ok := th.UI[m.ToastKey]; !ok {
			t.Errorf("missing UI token: %s", m.ToastKey)
		}
	}
	// Check all 15 syntax tokens exist.
	for _, m := range syntaxMappings {
		if _, ok := th.Syntax[m.ToastKey]; !ok {
			t.Errorf("missing syntax token: %s", m.ToastKey)
		}
	}
	// Check all 5 git tokens exist.
	for _, m := range gitMappings {
		if _, ok := th.Git[m.ToastKey]; !ok {
			t.Errorf("missing git token: %s", m.ToastKey)
		}
	}
}

func TestConvertVSCode_MissingRequiredColor(t *testing.T) {
	tmpDir := t.TempDir()
	data := minimalVSCodeTheme()
	colors := data["colors"].(map[string]string)
	delete(colors, "editor.background")

	inputPath := writeThemeJSON(t, tmpDir, data)
	outDir := filepath.Join(tmpDir, "out")

	_, err := ConvertVSCode(inputPath, outDir)
	if err == nil {
		t.Fatal("expected error for missing editor.background, got nil")
	}
	errMsg := err.Error()
	if !containsAll(errMsg, "background", "editor.background") {
		t.Errorf("error should mention background and editor.background, got: %s", errMsg)
	}
}

func TestConvertVSCode_FallbackUsed(t *testing.T) {
	tmpDir := t.TempDir()
	data := minimalVSCodeTheme()
	colors := data["colors"].(map[string]string)
	// tab_inactive_fg uses fallback sideBar.foreground when tab.inactiveForeground is absent.
	// sideBar.foreground is already present; tab.inactiveForeground was never set.
	// Just verify conversion succeeds.

	inputPath := writeThemeJSON(t, tmpDir, data)
	outDir := filepath.Join(tmpDir, "out")

	result, err := ConvertVSCode(inputPath, outDir)
	if err != nil {
		t.Fatalf("ConvertVSCode failed: %v", err)
	}

	raw, _ := os.ReadFile(result.OutputPath)
	var th theme.Theme
	json.Unmarshal(raw, &th)
	if th.UI["tab_inactive_fg"] != colors["sideBar.foreground"] {
		t.Errorf("tab_inactive_fg = %q, want sideBar.foreground %q", th.UI["tab_inactive_fg"], colors["sideBar.foreground"])
	}
}

func TestConvertVSCode_DeriveSearchFG(t *testing.T) {
	tmpDir := t.TempDir()
	data := minimalVSCodeTheme()
	inputPath := writeThemeJSON(t, tmpDir, data)
	outDir := filepath.Join(tmpDir, "out")

	result, err := ConvertVSCode(inputPath, outDir)
	if err != nil {
		t.Fatalf("ConvertVSCode failed: %v", err)
	}

	raw, _ := os.ReadFile(result.OutputPath)
	var th theme.Theme
	json.Unmarshal(raw, &th)

	bg := data["colors"].(map[string]string)["editor.background"]
	if th.UI["search_match_fg"] != bg {
		t.Errorf("search_match_fg = %q, want %q (editor.background)", th.UI["search_match_fg"], bg)
	}
	if th.UI["search_current_fg"] != bg {
		t.Errorf("search_current_fg = %q, want %q (editor.background)", th.UI["search_current_fg"], bg)
	}
}

func TestScopeMatching_ExactMatch(t *testing.T) {
	index := []scopeEntry{
		{scope: "keyword.control", fg: "#aaa", order: 0},
	}
	result := resolveSyntax(index, []string{"keyword.control"})
	if result == nil || result.fg != "#aaa" {
		t.Errorf("expected exact match, got %+v", result)
	}
}

func TestScopeMatching_PrefixMatch(t *testing.T) {
	index := []scopeEntry{
		{scope: "keyword", fg: "#bbb", order: 0},
	}
	result := resolveSyntax(index, []string{"keyword.control"})
	if result == nil || result.fg != "#bbb" {
		t.Errorf("expected prefix match, got %+v", result)
	}
}

func TestScopeMatching_MostSpecificWins(t *testing.T) {
	index := []scopeEntry{
		{scope: "keyword", fg: "#bbb", order: 0},
		{scope: "keyword.control", fg: "#ccc", order: 1},
	}
	result := resolveSyntax(index, []string{"keyword.control"})
	if result == nil || result.fg != "#ccc" {
		t.Errorf("expected most specific match #ccc, got %+v", result)
	}
}

func TestScopeMatching_NoFalsePrefix(t *testing.T) {
	index := []scopeEntry{
		{scope: "keyword", fg: "#aaa", order: 0},
	}
	// "keyword_other" should NOT match "keyword" (no dot separator).
	result := resolveSyntax(index, []string{"keyword_other"})
	if result != nil {
		t.Errorf("expected no match for keyword_other, got %+v", result)
	}
}

func TestColorNormalization(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"#1F1F28", "#1f1f28"},
		{"#1F1F2880", "#1f1f28"},
		{"#fff", "#ffffff"},
		{"#FFF", "#ffffff"},
		{"", ""},
		{"#abcdef", "#abcdef"},
	}
	for _, tt := range tests {
		got := normalizeColor(tt.in)
		if got != tt.want {
			t.Errorf("normalizeColor(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToKebab(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Kanagawa Wave", "kanagawa-wave"},
		{"Test Theme", "test-theme"},
		{"  Spaced  Out  ", "spaced-out"},
		{"ONE-two_THREE", "one-two-three"},
	}
	for _, tt := range tests {
		got := toKebab(tt.in)
		if got != tt.want {
			t.Errorf("toKebab(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestScopeMatching_FontStyle(t *testing.T) {
	index := []scopeEntry{
		{scope: "keyword.control", fg: "#957fb8", bold: true, italic: false, order: 0},
	}
	result := resolveSyntax(index, []string{"keyword.control"})
	if result == nil {
		t.Fatal("expected match")
	}
	if !result.bold {
		t.Error("expected bold=true")
	}
	if result.italic {
		t.Error("expected italic=false")
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
