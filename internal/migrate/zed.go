package migrate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourusername/toast/internal/theme"
)

// Zed theme files bundle a whole family in one document: a single "One.json"
// holds both One Dark and One Light. Every entry of "themes" is converted into
// its own Toast theme file.
//
// Compared to VSCode themes the mapping is much closer: Zed's style keys are
// flat dotted names (editor.background, tab.active_background, status_bar.
// background) and its syntax palette uses the same token names Toast uses,
// with two exceptions: Zed calls Toast's "module" token "namespace", and has
// no direct "builtin" token (the closest are "variable.special" and
// "preproc").
type zedThemeFile struct {
	Name   string     `json:"name"`
	Author string     `json:"author"`
	Themes []zedTheme `json:"themes"`
}

// zedTheme is a single variant (dark, light, high contrast, ...) inside a Zed
// theme file. Style values are decoded lazily because they may be colors,
// nulls, numbers, arrays (players, accents) or objects (syntax).
type zedTheme struct {
	Name       string                     `json:"name"`
	Appearance string                     `json:"appearance"`
	Style      map[string]json.RawMessage `json:"style"`
}

// zedMapping resolves one Toast token from Zed style keys, tried in order.
//
// Overlay marks background-role tokens. Zed writes many of them with an alpha
// channel (#2f343ebf) that is meant to be composited over the surface below,
// so those values are blended over the editor background instead of having the
// alpha silently dropped.
type zedMapping struct {
	ToastKey string
	Keys     []string
	Overlay  bool
	Derive   func(colors map[string]string) string
}

// syntaxZedMapping resolves one Toast syntax token from Zed syntax token
// names, tried in order.
type syntaxZedMapping struct {
	ToastKey string
	Keys     []string
}

// uiZedMappings maps every Toast UI token onto Zed style keys. Chains end in
// editor.foreground only as a last resort, for themes that omit the muted/
// semantic colors entirely (a few community ports do).
var uiZedMappings = []zedMapping{
	{ToastKey: "background", Keys: []string{"editor.background"}},
	{ToastKey: "foreground", Keys: []string{"editor.foreground"}},
	// Zed does not theme the caret separately; it is drawn in the editor
	// foreground.
	{ToastKey: "cursor", Keys: []string{"editor.foreground"}},
	{
		ToastKey: "selection",
		Keys:     []string{"element.selection_background", "editor.document_highlight.read_background"},
		Overlay:  true,
		// Most Zed themes leave text selection to the editor's own default, so
		// derive one from the foreground the same way Toast's system theme does.
		Derive: func(colors map[string]string) string {
			return mixHex(colors["editor.background"], colors["editor.foreground"], 0.25)
		},
	},
	{ToastKey: "line_highlight", Keys: []string{"editor.active_line.background"}, Overlay: true},
	{ToastKey: "border", Keys: []string{"border"}},

	{ToastKey: "tab_active_bg", Keys: []string{"tab.active_background"}, Overlay: true},
	{ToastKey: "tab_active_fg", Keys: []string{"text", "editor.foreground"}},
	{ToastKey: "tab_inactive_bg", Keys: []string{"tab.inactive_background", "tab_bar.background"}, Overlay: true},
	{ToastKey: "tab_inactive_fg", Keys: []string{"text.muted", "editor.foreground"}},

	{ToastKey: "sidebar_bg", Keys: []string{"panel.background", "surface.background"}, Overlay: true},
	{ToastKey: "sidebar_fg", Keys: []string{"text", "editor.foreground"}},
	{ToastKey: "sidebar_icon_fg", Keys: []string{"icon", "text.accent", "text.muted", "editor.foreground"}},
	{ToastKey: "sidebar_selected_bg", Keys: []string{"element.selected", "ghost_element.selected"}, Overlay: true},
	{ToastKey: "sidebar_selected_fg", Keys: []string{"text", "editor.foreground"}},

	{ToastKey: "statusbar_bg", Keys: []string{"status_bar.background"}, Overlay: true},
	{ToastKey: "statusbar_fg", Keys: []string{"text.muted", "text", "editor.foreground"}},

	{ToastKey: "breadcrumbs_fg", Keys: []string{"text.muted", "text", "editor.foreground"}},
	{ToastKey: "breadcrumbs_active_fg", Keys: []string{"text", "editor.foreground"}},

	{ToastKey: "gutter_fg", Keys: []string{"editor.line_number"}},
	{ToastKey: "gutter_active_fg", Keys: []string{"editor.active_line_number"}},

	{ToastKey: "muted", Keys: []string{"text.muted", "text", "editor.foreground"}},

	{
		ToastKey: "diagnostic_error",
		Keys:     []string{"error", "error.border", "deleted", "terminal.ansi.red", "editor.foreground"},
	},
	{
		ToastKey: "diagnostic_warning",
		Keys:     []string{"warning", "warning.border", "modified", "terminal.ansi.yellow", "editor.foreground"},
	},
	{
		ToastKey: "diagnostic_info",
		Keys:     []string{"info", "text.accent", "terminal.ansi.blue", "editor.foreground"},
	},
	{
		ToastKey: "diagnostic_hint",
		Keys:     []string{"hint", "success", "info", "terminal.ansi.green", "editor.foreground"},
	},

	{ToastKey: "completion_bg", Keys: []string{"elevated_surface.background", "surface.background"}, Overlay: true},
	{ToastKey: "completion_fg", Keys: []string{"text", "editor.foreground"}},
	{ToastKey: "completion_selected", Keys: []string{"element.selected", "ghost_element.selected"}, Overlay: true},

	{ToastKey: "settings_bg", Keys: []string{"elevated_surface.background", "surface.background"}, Overlay: true},
	{ToastKey: "settings_fg", Keys: []string{"text", "editor.foreground"}},
	{ToastKey: "settings_selected", Keys: []string{"element.selected", "ghost_element.selected"}, Overlay: true},
	{ToastKey: "settings_separator", Keys: []string{"border.variant", "border"}},
	{ToastKey: "settings_border", Keys: []string{"border.focused", "text.accent", "border"}},

	// element.hover is a button highlight in Zed and is often a saturated
	// opaque colour; popovers (hover docs, dialogs) sit on the elevated surface.
	{ToastKey: "hover_bg", Keys: []string{"elevated_surface.background", "surface.background", "element.hover"}, Overlay: true},
	{ToastKey: "hover_fg", Keys: []string{"text", "editor.foreground"}},
	// Zed's focused border is the accent used around focused panes and popups;
	// fall back to muted text so the frame stays visible on flat themes.
	{ToastKey: "hover_border", Keys: []string{"border.focused", "text.muted", "border.variant"}},
	{ToastKey: "find_replace_border", Keys: []string{"text.accent", "border.focused", "border"}},

	{
		ToastKey: "search_match_bg",
		Keys:     []string{"search.match_background", "editor.document_highlight.read_background", "element.selected"},
		Overlay:  true,
	},
	{
		ToastKey: "search_current_bg",
		Keys: []string{
			"search.active_match_background",
			"search.match_background",
			"editor.document_highlight.read_background",
			"element.selected",
		},
		Overlay: true,
	},
	// search_*_fg is not mapped here: it depends on the resolved highlight
	// background and is chosen for contrast at the end of convertZedTheme.
}

var gitZedMappings = []zedMapping{
	{ToastKey: "added", Keys: []string{"created", "version_control.added", "success", "terminal.ansi.green"}},
	{ToastKey: "modified", Keys: []string{"modified", "version_control.modified", "warning", "terminal.ansi.yellow"}},
	{ToastKey: "deleted", Keys: []string{"deleted", "version_control.deleted", "error", "terminal.ansi.red"}},
	{ToastKey: "untracked", Keys: []string{"version_control.untracked", "ignored", "text.muted", "terminal.ansi.bright_black"}},
	{ToastKey: "conflict", Keys: []string{"conflict", "version_control.conflict", "warning", "terminal.ansi.yellow"}},
	{ToastKey: "ignored", Keys: []string{"ignored", "text.muted", "terminal.ansi.bright_black"}},
}

// syntaxZedMappings maps Toast syntax tokens onto Zed syntax token names.
var syntaxZedMappings = []syntaxZedMapping{
	{ToastKey: "keyword", Keys: []string{"keyword", "preproc"}},
	{ToastKey: "string", Keys: []string{"string", "string.special"}},
	{ToastKey: "number", Keys: []string{"number"}},
	{ToastKey: "comment", Keys: []string{"comment", "comment.doc"}},
	{ToastKey: "function", Keys: []string{"function", "constructor", "keyword"}},
	{ToastKey: "type", Keys: []string{"type", "enum", "keyword"}},
	{ToastKey: "variable", Keys: []string{"variable", "variable.parameter", "property"}},
	{ToastKey: "constant", Keys: []string{"constant", "boolean", "number"}},
	{ToastKey: "operator", Keys: []string{"operator", "keyword"}},
	{
		ToastKey: "punctuation",
		Keys:     []string{"punctuation", "punctuation.delimiter", "punctuation.bracket", "operator", "keyword"},
	},
	{ToastKey: "tag", Keys: []string{"tag", "attribute"}},
	{ToastKey: "attribute", Keys: []string{"attribute", "property"}},
	{ToastKey: "property", Keys: []string{"property", "variable", "attribute"}},
	{ToastKey: "module", Keys: []string{"namespace", "type", "enum", "keyword"}},
	{ToastKey: "builtin", Keys: []string{"variable.special", "preproc", "embedded", "function", "keyword"}},
}

// ConvertZed reads a Zed theme JSON file and converts every variant it
// contains into its own Toast theme file inside outputDir. The returned slice
// holds one result per variant, in file order.
func ConvertZed(inputPath, outputDir string) ([]ConvertResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading Zed theme: %w", err)
	}

	var file zedThemeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing Zed theme JSON: %w", err)
	}
	if len(file.Themes) == 0 {
		return nil, fmt.Errorf("Zed theme has no \"themes\" array")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating themes directory: %w", err)
	}

	results := make([]ConvertResult, 0, len(file.Themes))
	seen := make(map[string]string, len(file.Themes))
	for _, z := range file.Themes {
		t, err := convertZedTheme(z)
		if err != nil {
			return nil, err
		}
		fileName := toKebab(t.Name) + ".json"
		if prev, ok := seen[fileName]; ok {
			return nil, fmt.Errorf(
				"Zed theme variants %q and %q both map to %s", prev, t.Name, fileName,
			)
		}
		seen[fileName] = t.Name

		out, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshalling theme: %w", err)
		}
		outPath := filepath.Join(outputDir, fileName)
		if err := os.WriteFile(outPath, append(out, '\n'), 0o644); err != nil {
			return nil, fmt.Errorf("writing theme file: %w", err)
		}
		results = append(results, ConvertResult{Name: t.Name, OutputPath: outPath})
	}

	return results, nil
}

// convertZedTheme converts a single Zed theme variant into a Toast theme.
func convertZedTheme(z zedTheme) (*theme.Theme, error) {
	if z.Name == "" {
		return nil, fmt.Errorf("Zed theme variant has no \"name\" field")
	}
	colors := z.colors()
	if colors["editor.background"] == "" {
		return nil, fmt.Errorf("Zed theme %q has no editor.background", z.Name)
	}
	if toKebab(z.Name) == "" {
		return nil, fmt.Errorf("Zed theme %q has no usable file name", z.Name)
	}

	var missing []missingToken
	background := normalizeColor(colors["editor.background"])

	ui := make(map[string]string, len(uiZedMappings))
	for _, m := range uiZedMappings {
		val := resolveZedColor(colors, m, background)
		if val == "" {
			missing = append(missing, missingToken{
				Category: "UI",
				Token:    m.ToastKey,
				Searched: describeZedSearch(m.Keys),
			})
			continue
		}
		ui[m.ToastKey] = val
	}

	tokens := z.syntax()
	syntax := make(map[string]theme.SyntaxStyle, len(syntaxZedMappings))
	for _, sm := range syntaxZedMappings {
		style, ok := resolveZedSyntax(tokens, sm)
		if !ok {
			missing = append(missing, missingToken{
				Category: "Syntax",
				Token:    sm.ToastKey,
				Searched: describeZedSearch(sm.Keys) + ` in "syntax"`,
			})
			continue
		}
		syntax[sm.ToastKey] = style
	}

	git := make(map[string]string, len(gitZedMappings))
	for _, m := range gitZedMappings {
		val := resolveZedColor(colors, m, background)
		if val == "" {
			missing = append(missing, missingToken{
				Category: "Git",
				Token:    m.ToastKey,
				Searched: describeZedSearch(m.Keys),
			})
			continue
		}
		git[m.ToastKey] = val
	}

	if len(missing) > 0 {
		return nil, formatMissingError("Zed", missing)
	}

	// Zed highlights are often translucent, so the composited match colour can
	// be brighter or darker than the editor background. Darker highlights (a
	// translucent tint over a dark theme) need the foreground as text colour.
	ui["search_match_fg"] = readableOn(ui["search_match_bg"], ui["background"], ui["foreground"])
	ui["search_current_fg"] = readableOn(ui["search_current_bg"], ui["background"], ui["foreground"])

	variant := "dark"
	if z.Appearance == "light" {
		variant = "light"
	}

	return &theme.Theme{
		Name:    z.Name,
		Variant: variant,
		UI:      ui,
		Syntax:  syntax,
		Git:     git,
	}, nil
}

// colors flattens a Zed style object into color strings. Values that are not
// non-null strings (arrays such as "players", numbers, nested "syntax") are
// dropped.
func (z zedTheme) colors() map[string]string {
	out := make(map[string]string, len(z.Style))
	for key, raw := range z.Style {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			out[key] = s
		}
	}
	return out
}

// zedToken is one entry of a Zed theme's "syntax" object. Zed stores the font
// style as a string ("italic") and the weight as a number (400, 700, ...).
type zedToken struct {
	Color      string          `json:"color"`
	FontStyle  json.RawMessage `json:"font_style"`
	FontWeight json.RawMessage `json:"font_weight"`
}

// syntax parses the "syntax" object into Toast syntax styles, dropping tokens
// without a color.
func (z zedTheme) syntax() map[string]theme.SyntaxStyle {
	raw, ok := z.Style["syntax"]
	if !ok {
		return nil
	}
	var tokens map[string]zedToken
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return nil
	}
	out := make(map[string]theme.SyntaxStyle, len(tokens))
	for key, tok := range tokens {
		if tok.Color == "" {
			continue
		}
		out[key] = theme.SyntaxStyle{
			FG:     normalizeColor(tok.Color),
			Bold:   zedWeightIsBold(tok.FontWeight),
			Italic: zedStyleIsItalic(tok.FontStyle),
		}
	}
	return out
}

// zedWeightIsBold reports whether a "font_weight" value asks for bold. Zed
// uses numeric CSS weights; fall back to a string comparison for ports that
// write "bold".
func zedWeightIsBold(raw json.RawMessage) bool {
	var weight int
	if err := json.Unmarshal(raw, &weight); err == nil {
		return weight >= 600
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(s) {
		case "bold", "bolder", "700", "800", "900":
			return true
		}
	}
	return false
}

// zedStyleIsItalic reports whether a "font_style" value asks for italics.
func zedStyleIsItalic(raw json.RawMessage) bool {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(s), "italic")
}

// resolveZedColor returns the first available Zed color for m, normalized for
// Toast. Overlay mappings are composited over bg. Returns "" when nothing
// resolves.
func resolveZedColor(colors map[string]string, m zedMapping, bg string) string {
	for _, key := range m.Keys {
		val, ok := colors[key]
		if !ok || val == "" {
			continue
		}
		if m.Overlay {
			return compositeOver(val, bg)
		}
		return normalizeColor(val)
	}
	if m.Derive != nil {
		if val := m.Derive(colors); val != "" {
			if m.Overlay {
				return compositeOver(val, bg)
			}
			return normalizeColor(val)
		}
	}
	return ""
}

// resolveZedSyntax finds the first Zed syntax token in m.Keys that carries a
// color.
func resolveZedSyntax(tokens map[string]theme.SyntaxStyle, m syntaxZedMapping) (theme.SyntaxStyle, bool) {
	for _, key := range m.Keys {
		if style, ok := tokens[key]; ok && style.FG != "" {
			return style, true
		}
	}
	return theme.SyntaxStyle{}, false
}

// describeZedSearch renders the style keys a mapping looked for.
func describeZedSearch(keys []string) string {
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = fmt.Sprintf("%q", key)
	}
	return strings.Join(quoted, " or ") + " in style object"
}

// compositeOver flattens a hex color carrying an alpha channel onto bg. Zed
// writes eight-digit #rrggbbaa values for translucent surfaces. Colors without
// alpha are returned normalized, so this is safe for opaque surfaces too.
func compositeOver(c, bg string) string {
	r, g, b, a, ok := parseZedColor(c)
	if !ok {
		return normalizeColor(c)
	}
	if a == 0xff {
		return normalizeColor(c)
	}
	br, bgG, bb, _, okBG := parseZedColor(bg)
	if !okBG {
		return normalizeColor(c)
	}
	return fmt.Sprintf(
		"#%02x%02x%02x",
		alphaBlend(r, br, a),
		alphaBlend(g, bgG, a),
		alphaBlend(b, bb, a),
	)
}

// readableOn returns whichever of dark or light reads better as text on bg.
func readableOn(bg, dark, light string) string {
	if bg == "" {
		return dark
	}
	darkRatio := contrastRatio(dark, bg)
	lightRatio := contrastRatio(light, bg)
	if lightRatio > darkRatio {
		return light
	}
	return dark
}

// contrastRatio returns the WCAG contrast ratio between two opaque hex
// colours. Unknown colours fall back to 1 (no preference).
func contrastRatio(a, b string) float64 {
	la, okA := relativeLuminance(a)
	lb, okB := relativeLuminance(b)
	if !okA || !okB {
		return 1
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relativeLuminance computes the WCAG relative luminance of an opaque hex
// colour.
func relativeLuminance(c string) (float64, bool) {
	r, g, b, _, ok := parseZedColor(c)
	if !ok {
		return 0, false
	}
	linear := func(v uint8) float64 {
		s := float64(v) / 0xff
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b), true
}

// alphaBlend composites fg over bg at alpha a.
func alphaBlend(fg, bg, a uint8) uint8 {
	return uint8((float64(fg)*float64(a) + float64(bg)*float64(0xff-a)) / 0xff)
}

// mixHex blends two opaque hex colors, amount being the weight of b.
func mixHex(a, b string, amount float64) string {
	ar, ag, ab, _, okA := parseZedColor(a)
	br, bg, bb, _, okB := parseZedColor(b)
	if !okA || !okB {
		return ""
	}
	channel := func(x, y uint8) uint8 {
		return uint8(float64(x)*(1-amount) + float64(y)*amount)
	}
	return fmt.Sprintf("#%02x%02x%02x", channel(ar, br), channel(ag, bg), channel(ab, bb))
}

// parseZedColor parses #rgb, #rgba, #rrggbb and #rrggbbaa color strings. A
// missing alpha channel is reported as opaque.
func parseZedColor(s string) (r, g, b, a uint8, ok bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '#' {
		return 0, 0, 0, 0, false
	}
	hex := strings.ToLower(s[1:])
	expand := func(c byte) uint8 {
		v, err := parseHexNibble(c)
		if err != nil {
			return 0
		}
		return v * 0x11
	}
	pair := func(i int) (uint8, bool) {
		v, err := parseHexByte(hex[i : i+2])
		return v, err == nil
	}
	switch len(hex) {
	case 3, 4:
		for i := range len(hex) {
			if _, err := parseHexNibble(hex[i]); err != nil {
				return 0, 0, 0, 0, false
			}
		}
		r, g, b, a = expand(hex[0]), expand(hex[1]), expand(hex[2]), 0xff
		if len(hex) == 4 {
			a = expand(hex[3])
		}
		return r, g, b, a, true
	case 6, 8:
		var okR, okG, okB bool
		r, okR = pair(0)
		g, okG = pair(2)
		b, okB = pair(4)
		if !okR || !okG || !okB {
			return 0, 0, 0, 0, false
		}
		a = 0xff
		if len(hex) == 8 {
			var okA bool
			a, okA = pair(6)
			if !okA {
				return 0, 0, 0, 0, false
			}
		}
		return r, g, b, a, true
	}
	return 0, 0, 0, 0, false
}

func parseHexNibble(c byte) (uint8, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	}
	return 0, fmt.Errorf("invalid hex digit %q", c)
}

func parseHexByte(s string) (uint8, error) {
	hi, err := parseHexNibble(s[0])
	if err != nil {
		return 0, err
	}
	lo, err := parseHexNibble(s[1])
	if err != nil {
		return 0, err
	}
	return hi<<4 | lo, nil
}
