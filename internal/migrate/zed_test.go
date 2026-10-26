package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourusername/toast/internal/theme"
)

// minimalZedTheme returns a complete Zed theme document with a single dark
// variant that should convert successfully with every Toast token resolved.
func minimalZedTheme() map[string]interface{} {
	return map[string]interface{}{
		"name":   "Test Family",
		"author": "Tests",
		"themes": []interface{}{
			map[string]interface{}{
				"name":       "Test Zed",
				"appearance": "dark",
				"style": map[string]interface{}{
					"editor.background":              "#1f1f28ff",
					"editor.foreground":              "#dcd7baff",
					"border":                         "#16161dff",
					"border.variant":                 "#2a2a37ff",
					"border.focused":                 nil,
					"editor.active_line.background":  "#2a2a37ff",
					"editor.line_number":             "#54546dff",
					"editor.active_line_number":      "#957fb8ff",
					"tab.active_background":          "#363646ff",
					"tab.inactive_background":        "#1f1f28ff",
					"tab_bar.background":             "#1f1f28ff",
					"panel.background":               "#1f1f28ff",
					"surface.background":             "#1f1f28ff",
					"elevated_surface.background":    "#223249ff",
					"element.selected":               "#363646ff",
					"element.hover":                  "#2d4f67ff",
					"text":                           "#dcd7baff",
					"text.muted":                     "#727169ff",
					"text.accent":                    "#7e9cd8ff",
					"icon":                           "#957fb8ff",
					"status_bar.background":          "#16161dff",
					"error":                          "#e82424ff",
					"warning":                        "#ff9e3bff",
					"info":                           "#7e9cd8ff",
					"hint":                           "#7aa89fff",
					"created":                        "#76946aff",
					"modified":                       "#dca561ff",
					"deleted":                        "#c34043ff",
					"conflict":                       "#ff9e3bff",
					"ignored":                        "#727169ff",
					"search.match_background":        "#2d4f67ff",
					"search.active_match_background": "#957fb8ff",
					"terminal.ansi.red":              "#e82424ff",
					"terminal.ansi.green":            "#76946aff",
					"terminal.ansi.yellow":           "#dca561ff",
					"terminal.ansi.blue":             "#7e9cd8ff",
					"terminal.ansi.bright_black":     "#727169ff",
					"syntax": map[string]interface{}{
						"keyword":          map[string]interface{}{"color": "#957fb8ff", "font_weight": 700},
						"string":           map[string]interface{}{"color": "#98bb6cff"},
						"number":           map[string]interface{}{"color": "#d27e99ff"},
						"comment":          map[string]interface{}{"color": "#727169ff", "font_style": "italic"},
						"function":         map[string]interface{}{"color": "#7e9cd8ff"},
						"type":             map[string]interface{}{"color": "#7aa89fff"},
						"variable":         map[string]interface{}{"color": "#dcd7baff"},
						"constant":         map[string]interface{}{"color": "#ffa066ff"},
						"operator":         map[string]interface{}{"color": "#c0a36eff"},
						"punctuation":      map[string]interface{}{"color": "#9cabbeff"},
						"tag":              map[string]interface{}{"color": "#957fb8ff"},
						"attribute":        map[string]interface{}{"color": "#ffa066ff"},
						"property":         map[string]interface{}{"color": "#7e9cd8ff"},
						"namespace":        map[string]interface{}{"color": "#e46876ff"},
						"variable.special": map[string]interface{}{"color": "#e46876ff"},
					},
				},
			},
		},
	}
}

// writeZedTheme writes doc as JSON into a temp dir and returns its path.
func writeZedTheme(t *testing.T, doc interface{}) string {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal theme: %v", err)
	}
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	return path
}

// readConverted loads a converted Toast theme.
func readConverted(t *testing.T, path string) *theme.Theme {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read converted theme: %v", err)
	}
	var th theme.Theme
	if err := json.Unmarshal(data, &th); err != nil {
		t.Fatalf("unmarshal converted theme: %v", err)
	}
	return &th
}

func TestConvertZed_FullTheme(t *testing.T) {
	input := writeZedTheme(t, minimalZedTheme())
	outDir := t.TempDir()

	results, err := ConvertZed(input, outDir)
	if err != nil {
		t.Fatalf("ConvertZed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "Test Zed" {
		t.Errorf("name = %q, want Test Zed", results[0].Name)
	}
	if want := filepath.Join(outDir, "test-zed.json"); results[0].OutputPath != want {
		t.Errorf("output = %q, want %q", results[0].OutputPath, want)
	}

	th := readConverted(t, results[0].OutputPath)
	if th.Name != "Test Zed" || th.Variant != "dark" {
		t.Errorf("theme = %q/%q, want Test Zed/dark", th.Name, th.Variant)
	}
	// Every Toast UI token should be populated, including the derived ones.
	for _, m := range uiZedMappings {
		if th.UI[m.ToastKey] == "" {
			t.Errorf("ui token %q empty", m.ToastKey)
		}
	}
	for _, m := range gitZedMappings {
		if th.Git[m.ToastKey] == "" {
			t.Errorf("git token %q empty", m.ToastKey)
		}
	}
	for _, m := range syntaxZedMappings {
		if th.Syntax[m.ToastKey].FG == "" {
			t.Errorf("syntax token %q empty", m.ToastKey)
		}
	}
	if got := th.UI["background"]; got != "#1f1f28" {
		t.Errorf("background = %q, want #1f1f28 (alpha stripped)", got)
	}
	// Zed draws the caret in the editor foreground.
	if got := th.UI["cursor"]; got != "#dcd7ba" {
		t.Errorf("cursor = %q, want editor.foreground #dcd7ba", got)
	}
	// border.focused is null, so the accent is used for the dialog frame.
	if got := th.UI["settings_border"]; got != "#7e9cd8" {
		t.Errorf("settings_border = %q, want text.accent #7e9cd8", got)
	}
	if got := th.Syntax["module"].FG; got != "#e46876" {
		t.Errorf("module = %q, want namespace colour #e46876", got)
	}
	if !th.Syntax["keyword"].Bold {
		t.Error("keyword should be bold from font_weight 700")
	}
	if !th.Syntax["comment"].Italic {
		t.Error("comment should be italic from font_style")
	}
}

func TestConvertZed_MultipleVariants(t *testing.T) {
	doc := minimalZedTheme()
	themes := doc["themes"].([]interface{})
	light := minimalZedTheme()["themes"].([]interface{})[0].(map[string]interface{})
	light["name"] = "Test Zed Light"
	light["appearance"] = "light"
	doc["themes"] = append(themes, light)

	input := writeZedTheme(t, doc)
	results, err := ConvertZed(input, t.TempDir())
	if err != nil {
		t.Fatalf("ConvertZed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	dark := readConverted(t, results[0].OutputPath)
	lightTheme := readConverted(t, results[1].OutputPath)
	if dark.Variant != "dark" || lightTheme.Variant != "light" {
		t.Errorf("variants = %q/%q, want dark/light", dark.Variant, lightTheme.Variant)
	}
	if !strings.HasSuffix(lightTheme.Name, " Light") {
		t.Errorf("unexpected light name %q", lightTheme.Name)
	}
}

func TestConvertZed_DuplicateFileNames(t *testing.T) {
	doc := minimalZedTheme()
	themes := doc["themes"].([]interface{})
	doc["themes"] = append(themes, themes[0])
	input := writeZedTheme(t, doc)

	_, err := ConvertZed(input, t.TempDir())
	if err == nil {
		t.Fatal("expected error for duplicate variant names")
	}
	if !strings.Contains(err.Error(), "both map to") {
		t.Errorf("error = %v, want duplicate name error", err)
	}
}

func TestConvertZed_MissingRequiredColor(t *testing.T) {
	doc := minimalZedTheme()
	style := doc["themes"].([]interface{})[0].(map[string]interface{})["style"].(map[string]interface{})
	delete(style, "editor.background")
	input := writeZedTheme(t, doc)

	_, err := ConvertZed(input, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing editor.background")
	}
	if !strings.Contains(err.Error(), "editor.background") {
		t.Errorf("error = %v, want it to name editor.background", err)
	}
}

func TestConvertZed_MissingSyntaxToken(t *testing.T) {
	doc := minimalZedTheme()
	style := doc["themes"].([]interface{})[0].(map[string]interface{})["style"].(map[string]interface{})
	syntax := style["syntax"].(map[string]interface{})
	for _, key := range syntaxZedMappings[len(syntaxZedMappings)-1].Keys {
		delete(syntax, key)
	}
	input := writeZedTheme(t, doc)

	_, err := ConvertZed(input, t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing syntax tokens")
	}
	if !strings.Contains(err.Error(), "builtin") {
		t.Errorf("error = %v, want it to name the builtin token", err)
	}
}

func TestConvertZed_NotAZedTheme(t *testing.T) {
	doc := map[string]interface{}{
		"name":   "A VSCode theme",
		"colors": map[string]string{"editor.background": "#000000"},
	}
	input := writeZedTheme(t, doc)

	_, err := ConvertZed(input, t.TempDir())
	if err == nil {
		t.Fatal("expected error for a document without a themes array")
	}
	if !strings.Contains(err.Error(), `no "themes" array`) {
		t.Errorf("error = %v, want a missing themes array error", err)
	}
}

func TestConvertZed_NullAndNonColorValuesIgnored(t *testing.T) {
	doc := minimalZedTheme()
	style := doc["themes"].([]interface{})[0].(map[string]interface{})["style"].(map[string]interface{})
	style["panel.background"] = nil
	style["players"] = []interface{}{map[string]interface{}{"cursor": "#ff0000ff"}}
	style["some.number"] = 12
	input := writeZedTheme(t, doc)

	results, err := ConvertZed(input, t.TempDir())
	if err != nil {
		t.Fatalf("ConvertZed: %v", err)
	}
	th := readConverted(t, results[0].OutputPath)
	// panel.background is null → sidebar_bg falls back to surface.background.
	if got := th.UI["sidebar_bg"]; got != "#1f1f28" {
		t.Errorf("sidebar_bg = %q, want surface.background fallback", got)
	}
}

func TestCompositeOver(t *testing.T) {
	tests := []struct {
		name string
		c    string
		bg   string
		want string
	}{
		{name: "opaque is unchanged", c: "#1f1f28ff", bg: "#000000", want: "#1f1f28"},
		{name: "no alpha is unchanged", c: "#1f1f28", bg: "#000000", want: "#1f1f28"},
		{name: "alpha 0 is the background", c: "#ffffff00", bg: "#112233", want: "#112233"},
		{name: "half alpha blends", c: "#ffffff80", bg: "#000000", want: "#808080"},
		{name: "translucent over colour", c: "#74ade866", bg: "#282c33", want: "#465f7b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compositeOver(tt.c, tt.bg); got != tt.want {
				t.Errorf("compositeOver(%q, %q) = %q, want %q", tt.c, tt.bg, got, tt.want)
			}
		})
	}
}

func TestParseZedColor(t *testing.T) {
	tests := []struct {
		in    string
		r     uint8
		g     uint8
		b     uint8
		a     uint8
		valid bool
	}{
		{in: "#fff", r: 0xff, g: 0xff, b: 0xff, a: 0xff, valid: true},
		{in: "#f008", r: 0xff, g: 0x00, b: 0x00, a: 0x88, valid: true},
		{in: "#1f1f28", r: 0x1f, g: 0x1f, b: 0x28, a: 0xff, valid: true},
		{in: "#1F1F2880", r: 0x1f, g: 0x1f, b: 0x28, a: 0x80, valid: true},
		{in: "red", valid: false},
		{in: "#12345", valid: false},
		{in: "#ggghhh", valid: false},
	}
	for _, tt := range tests {
		r, g, b, a, ok := parseZedColor(tt.in)
		if ok != tt.valid {
			t.Fatalf("parseZedColor(%q) ok = %v, want %v", tt.in, ok, tt.valid)
		}
		if !tt.valid {
			continue
		}
		if r != tt.r || g != tt.g || b != tt.b || a != tt.a {
			t.Errorf(
				"parseZedColor(%q) = %02x%02x%02x%02x, want %02x%02x%02x%02x",
				tt.in, r, g, b, a, tt.r, tt.g, tt.b, tt.a,
			)
		}
	}
}

func TestZedFontStyle(t *testing.T) {
	tests := []struct {
		name   string
		style  string
		weight interface{}
		bold   bool
		italic bool
	}{
		{name: "weight 700", weight: 700, bold: true},
		{name: "weight 600", weight: 600, bold: true},
		{name: "weight 400", weight: 400, bold: false},
		{name: "string bold", weight: "bold", bold: true},
		{name: "italic", style: "italic", italic: true},
		{name: "null weight", weight: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := json.Marshal(tt.weight)
			if err != nil {
				t.Fatalf("marshal weight: %v", err)
			}
			style := "null"
			if tt.style != "" {
				encoded, err := json.Marshal(tt.style)
				if err != nil {
					t.Fatalf("marshal style: %v", err)
				}
				style = string(encoded)
			}
			if got := zedWeightIsBold(weight); got != tt.bold {
				t.Errorf("zedWeightIsBold(%s) = %v, want %v", weight, got, tt.bold)
			}
			if got := zedStyleIsItalic(json.RawMessage(style)); got != tt.italic {
				t.Errorf("zedStyleIsItalic(%s) = %v, want %v", style, got, tt.italic)
			}
		})
	}
}

func TestZedSyntaxFallbackChain(t *testing.T) {
	tokens := map[string]theme.SyntaxStyle{
		"type": {FG: "#abcdef"},
	}
	style, ok := resolveZedSyntax(tokens, syntaxZedMapping{ToastKey: "module", Keys: []string{"namespace", "type"}})
	if !ok {
		t.Fatal("expected the chain to fall back to type")
	}
	if style.FG != "#abcdef" {
		t.Errorf("chain resolved %q, want #abcdef", style.FG)
	}
	if _, ok := resolveZedSyntax(tokens, syntaxZedMapping{Keys: []string{"namespace"}}); ok {
		t.Error("expected no resolution for a single missing token")
	}
}

func TestReadableOn(t *testing.T) {
	// A bright highlight (the VSCode convention) needs dark text.
	if got := readableOn("#f9e2af", "#1e1e2e", "#cdd6f4"); got != "#1e1e2e" {
		t.Errorf("readableOn(bright) = %q, want the dark editor background", got)
	}
	// A highlight only slightly lighter than a dark background needs the
	// foreground, otherwise the matched text would be invisible.
	if got := readableOn("#202d3d", "#192330", "#cdcecf"); got != "#cdcecf" {
		t.Errorf("readableOn(dark) = %q, want the foreground", got)
	}
	if got := readableOn("", "#192330", "#cdcecf"); got != "#192330" {
		t.Errorf("readableOn(empty) = %q, want the first fallback", got)
	}
}

func TestContrastRatio(t *testing.T) {
	// White on black is the maximum 21:1.
	if got := contrastRatio("#ffffff", "#000000"); got < 20.9 || got > 21.1 {
		t.Errorf("contrastRatio(white, black) = %v, want about 21", got)
	}
	if got := contrastRatio("#ffffff", "#ffffff"); got != 1 {
		t.Errorf("contrastRatio(same) = %v, want 1", got)
	}
	if got := contrastRatio("not-a-colour", "#000000"); got != 1 {
		t.Errorf("contrastRatio(invalid) = %v, want 1", got)
	}
}
