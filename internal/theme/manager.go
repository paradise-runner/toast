package theme

import (
	"embed"
	"image/color"
	"os"
	"path/filepath"
	"strconv"

	"charm.land/lipgloss/v2"
)

//go:embed builtin/*.json
var builtinFS embed.FS

type Manager struct {
	name         string
	theme        *Theme
	syntaxStyles map[string]lipgloss.Style
	systemColors map[int]string
	systemBG     string
	systemFG     string
	systemCursor string
	systemDark   bool
}

func NewManager(name, themeDir string) (*Manager, error) {
	t, _ := loadTheme(name, themeDir)
	activeName := name
	if t == nil {
		t, _ = loadBuiltin("toast-dark")
		activeName = "toast-dark"
	}
	if t == nil {
		t = &Theme{
			UI:     make(map[string]string),
			Syntax: make(map[string]SyntaxStyle),
			Git:    make(map[string]string),
		}
		activeName = ""
	}
	m := &Manager{name: activeName, theme: t}
	m.buildSyntaxStyles()
	return m, nil
}

func (m *Manager) UI(key string) string       { return m.theme.UI[key] }
func (m *Manager) Git(key string) string      { return m.theme.Git[key] }
func (m *Manager) Variant() string            { return m.theme.Variant }
func (m *Manager) SyntaxFG(key string) string { return m.theme.Syntax[key].FG }
func (m *Manager) IsSystem() bool             { return m.name == systemThemeName }

func (m *Manager) SyntaxStyle(capture string) lipgloss.Style {
	if s, ok := m.syntaxStyles[capture]; ok {
		return s
	}
	fg := m.theme.UI["foreground"]
	if fg == "" {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
}

func (m *Manager) Reload(name, themeDir string) error {
	t, err := loadTheme(name, themeDir)
	if err != nil || t == nil {
		return err
	}
	m.name = name
	m.theme = t
	m.systemColors = nil
	m.systemBG = ""
	m.systemFG = ""
	m.systemCursor = ""
	m.systemDark = true
	m.buildSyntaxStyles()
	return nil
}

// ApplySystemBackground updates the synthetic system theme with the terminal's
// actual default background and switches light/dark mappings to match it.
func (m *Manager) ApplySystemBackground(c color.Color, isDark bool) {
	if !m.IsSystem() {
		return
	}
	m.systemBG = colorHex(c)
	m.systemDark = isDark
	m.rebuildSystemTheme()
}

func (m *Manager) applySystemPaletteColors() {
	for index, replacement := range m.systemColors {
		old := strconv.Itoa(index)
		replaceColors(m.theme.UI, old, replacement)
		replaceColors(m.theme.Git, old, replacement)
		for key, style := range m.theme.Syntax {
			if style.FG == old {
				style.FG = replacement
			}
			if style.BG == old {
				style.BG = replacement
			}
			m.theme.Syntax[key] = style
		}
	}
}

func (m *Manager) ApplySystemForeground(c color.Color) {
	if !m.IsSystem() {
		return
	}
	m.systemFG = colorHex(c)
	m.rebuildSystemTheme()
}

func (m *Manager) ApplySystemCursor(c color.Color) {
	if !m.IsSystem() {
		return
	}
	m.systemCursor = colorHex(c)
	m.rebuildSystemTheme()
}

// Name returns the load identifier of the active theme (e.g. "toast-dark"),
// as passed to NewManager or Reload. If a fallback was applied because the
// requested theme could not be loaded, the fallback identifier is returned.
// Returns "" if no valid theme could be loaded.
func (m *Manager) Name() string { return m.name }

// ListBuiltin returns the names of all embedded builtin themes.
func ListBuiltin() []string {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries)+1)
	names = append(names, systemThemeName)
	for _, e := range entries {
		n := e.Name()
		if filepath.Ext(n) == ".json" {
			names = append(names, n[:len(n)-5])
		}
	}
	return names
}

func (m *Manager) buildSyntaxStyles() {
	m.syntaxStyles = make(map[string]lipgloss.Style, len(m.theme.Syntax))
	for key, s := range m.theme.Syntax {
		style := lipgloss.NewStyle()
		if s.FG != "" {
			style = style.Foreground(lipgloss.Color(s.FG))
		}
		if s.BG != "" {
			style = style.Background(lipgloss.Color(s.BG))
		}
		if s.Bold {
			style = style.Bold(true)
		}
		if s.Italic {
			style = style.Italic(true)
		}
		m.syntaxStyles[key] = style
	}
}

func loadTheme(name, themeDir string) (*Theme, error) {
	if name == systemThemeName {
		return newSystemTheme(true), nil
	}
	if themeDir != "" {
		data, err := os.ReadFile(filepath.Join(themeDir, name+".json"))
		if err == nil {
			return parse(data)
		}
	}
	return loadBuiltin(name)
}

func loadBuiltin(name string) (*Theme, error) {
	data, err := builtinFS.ReadFile("builtin/" + name + ".json")
	if err != nil {
		return nil, err
	}
	return parse(data)
}
