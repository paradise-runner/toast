package theme

import (
	"embed"
	"os"
	"path/filepath"

	"charm.land/lipgloss/v2"
)

//go:embed builtin/*.json
var builtinFS embed.FS

type Manager struct {
	name         string
	theme        *Theme
	syntaxStyles map[string]lipgloss.Style
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

func (m *Manager) UI(key string) string  { return m.theme.UI[key] }
func (m *Manager) Git(key string) string { return m.theme.Git[key] }

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
	m.buildSyntaxStyles()
	return nil
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
	names := make([]string, 0, len(entries))
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
