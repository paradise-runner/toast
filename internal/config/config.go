package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Theme           string            `json:"theme"`
	Editor          EditorConfig      `json:"editor"`
	Sidebar         SidebarConfig     `json:"sidebar"`
	LSP             map[string]LSPCmd `json:"lsp"`
	Search          SearchConfig      `json:"search"`
	IgnoredPatterns []string          `json:"ignored_patterns"`
}

type EditorConfig struct {
	TabWidth                     int  `json:"tab_width"`
	WordWrap                     bool `json:"word_wrap"`
	ShowWhitespace               bool `json:"show_whitespace"`
	AutoIndent                   bool `json:"auto_indent"`
	TrimTrailingWhitespaceOnSave bool `json:"trim_trailing_whitespace_on_save"`
	InsertFinalNewlineOnSave     bool `json:"insert_final_newline_on_save"`
}

type SidebarConfig struct {
	Visible       bool `json:"visible"`
	Width         int  `json:"width"`
	ConfirmDelete bool `json:"confirm_delete"`
}

type LSPCmd struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type SearchConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func Defaults() Config {
	return Config{
		Theme: "toast-dark",
		Editor: EditorConfig{
			TabWidth: 4, AutoIndent: true,
			TrimTrailingWhitespaceOnSave: true, InsertFinalNewlineOnSave: true,
		},
		Sidebar: SidebarConfig{Visible: true, Width: 30, ConfirmDelete: true},
		LSP: map[string]LSPCmd{
			"go":         {Command: "gopls", Args: []string{"serve"}},
			"python":     {Command: "pyright-langserver", Args: []string{"--stdio"}},
			"typescript": {Command: "typescript-language-server", Args: []string{"--stdio"}},
			"rust":       {Command: "rust-analyzer"},
		},
		Search:          SearchConfig{Command: "rg", Args: []string{"--json"}},
		IgnoredPatterns: []string{".git", "node_modules", "__pycache__", ".DS_Store"},
	}
}

// Save writes cfg as JSON to path, creating parent directories as needed.
func Save(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// DefaultPath returns the default path for the config file.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving config path: %w", err)
	}
	return filepath.Join(home, ".config", "toast", "config.json"), nil
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Defaults(), nil
	}
	return LoadFrom(filepath.Join(home, ".config", "toast", "config.json"))
}

func LoadFrom(path string) (Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
