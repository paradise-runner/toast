package theme

import (
	"bytes"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const systemThemeName = "system"
const systemPaletteSize = 16

func newSystemTheme(isDark bool) *Theme {
	variant := "light"
	if isDark {
		variant = "dark"
	}
	return &Theme{
		Name:    "System",
		Variant: variant,
		UI: map[string]string{
			"background": "", "foreground": "", "cursor": "",
			"selection": "", "line_highlight": "", "border": "",
			"tab_active_bg": "", "tab_active_fg": "",
			"tab_inactive_bg": "", "tab_inactive_fg": "",
			"sidebar_bg": "", "sidebar_fg": "",
			"sidebar_selected_bg": "", "sidebar_selected_fg": "",
			"statusbar_bg": "", "statusbar_fg": "",
			"breadcrumbs_fg": "", "breadcrumbs_active_fg": "",
			"gutter_fg": "", "gutter_active_fg": "",
			"diagnostic_error": "1", "diagnostic_warning": "3",
			"diagnostic_info": "4", "diagnostic_hint": "2",
			"completion_bg": "", "completion_fg": "", "completion_selected": "",
			"hover_bg": "", "hover_fg": "", "hover_border": "",
			"search_match_bg": "3", "search_match_fg": "0",
			"search_current_bg": "11", "search_current_fg": "0",
		},
		Syntax: systemSyntax(),
		Git:    systemGit(),
	}
}

func (m *Manager) rebuildSystemTheme() {
	m.theme = newSystemTheme(m.systemDark)
	m.applySystemBaseColors()
	m.applySystemPaletteColors()
	m.buildSyntaxStyles()
}

func (m *Manager) applySystemBaseColors() {
	bg := m.systemBG
	fg := m.systemFG
	if bg == "" {
		if m.systemDark {
			bg = "#000000"
		} else {
			bg = "#ffffff"
		}
	}
	if fg == "" {
		if m.systemDark {
			fg = "#ffffff"
		} else {
			fg = "#000000"
		}
	}

	surface := blendHex(bg, fg, 0.035)
	surface2 := blendHex(bg, fg, 0.07)
	selected := blendHex(bg, fg, 0.16)
	selection := blendHex(bg, fg, 0.22)
	border := blendHex(bg, fg, 0.20)
	muted := blendHex(bg, fg, 0.52)
	mutedStrong := blendHex(bg, fg, 0.72)

	m.theme.UI["background"] = bg
	m.theme.UI["foreground"] = fg
	m.theme.UI["cursor"] = firstNonEmpty(m.systemCursor, fg)
	m.theme.UI["selection"] = selection
	m.theme.UI["line_highlight"] = surface2
	m.theme.UI["border"] = border
	m.theme.UI["tab_active_bg"] = bg
	m.theme.UI["tab_active_fg"] = fg
	m.theme.UI["tab_inactive_bg"] = surface
	m.theme.UI["tab_inactive_fg"] = muted
	m.theme.UI["sidebar_bg"] = surface
	m.theme.UI["sidebar_fg"] = mutedStrong
	m.theme.UI["sidebar_selected_bg"] = selected
	m.theme.UI["sidebar_selected_fg"] = fg
	m.theme.UI["statusbar_bg"] = surface
	m.theme.UI["statusbar_fg"] = mutedStrong
	m.theme.UI["breadcrumbs_fg"] = muted
	m.theme.UI["breadcrumbs_active_fg"] = fg
	m.theme.UI["gutter_fg"] = muted
	m.theme.UI["gutter_active_fg"] = mutedStrong
	m.theme.UI["completion_bg"] = surface2
	m.theme.UI["completion_fg"] = fg
	m.theme.UI["completion_selected"] = selected
	m.theme.UI["hover_bg"] = surface2
	m.theme.UI["hover_fg"] = fg
	m.theme.UI["hover_border"] = border
}

func systemSyntax() map[string]SyntaxStyle {
	return map[string]SyntaxStyle{
		"keyword":     {FG: "5", Bold: true},
		"string":      {FG: "2"},
		"number":      {FG: "3"},
		"comment":     {FG: "8", Italic: true},
		"function":    {FG: "4"},
		"type":        {FG: "3"},
		"variable":    {},
		"constant":    {FG: "3"},
		"operator":    {FG: "6"},
		"punctuation": {FG: "8"},
		"tag":         {FG: "5"},
		"attribute":   {FG: "3"},
		"property":    {FG: "4"},
		"module":      {FG: "1"},
		"builtin":     {FG: "1"},
	}
}

func systemGit() map[string]string {
	return map[string]string{
		"added":     "2",
		"modified":  "3",
		"deleted":   "1",
		"untracked": "8",
		"conflict":  "9",
	}
}

func SystemPaletteQuery() string {
	var b bytes.Buffer
	for i := range systemPaletteSize {
		fmt.Fprintf(&b, "\x1b]4;%d;?\x07", i)
	}
	return b.String()
}

func ParseSystemPaletteResponse(raw string) (index int, c color.Color, ok bool) {
	raw = strings.TrimPrefix(raw, "\x1b]")
	raw = strings.TrimPrefix(raw, "\x9d")
	raw = strings.TrimSuffix(raw, "\x07")
	raw = strings.TrimSuffix(raw, "\x1b\\")
	raw = strings.TrimSuffix(raw, "\x9c")

	parts := strings.Split(raw, ";")
	if len(parts) != 3 || parts[0] != "4" {
		return 0, nil, false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= systemPaletteSize {
		return 0, nil, false
	}
	c = ansi.XParseColor(parts[2])
	if colorHex(c) == "" {
		return 0, nil, false
	}
	return index, c, true
}

func (m *Manager) ApplySystemPaletteColor(index int, c color.Color) {
	if !m.IsSystem() || index < 0 || index >= systemPaletteSize {
		return
	}
	replacement := colorHex(c)
	if replacement == "" {
		return
	}
	if m.systemColors == nil {
		m.systemColors = make(map[int]string, systemPaletteSize)
	}
	m.systemColors[index] = replacement
	m.applySystemPaletteColors()
	m.buildSyntaxStyles()
}

func replaceColors(colors map[string]string, old, replacement string) {
	for key, value := range colors {
		if value == old {
			colors[key] = replacement
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func blendHex(a, b string, amount float64) string {
	ar, ag, ab, okA := parseHexColor(a)
	br, bg, bb, okB := parseHexColor(b)
	if !okA || !okB {
		return a
	}
	return fmt.Sprintf(
		"#%02x%02x%02x",
		blendChannel(ar, br, amount),
		blendChannel(ag, bg, amount),
		blendChannel(ab, bb, amount),
	)
}

func blendChannel(a, b uint8, amount float64) uint8 {
	return uint8(float64(a)*(1-amount) + float64(b)*amount)
}

func parseHexColor(s string) (r, g, b uint8, ok bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, false
	}
	ri, err := strconv.ParseUint(s[1:3], 16, 8)
	if err != nil {
		return 0, 0, 0, false
	}
	gi, err := strconv.ParseUint(s[3:5], 16, 8)
	if err != nil {
		return 0, 0, 0, false
	}
	bi, err := strconv.ParseUint(s[5:7], 16, 8)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(ri), uint8(gi), uint8(bi), true
}

func colorHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}
