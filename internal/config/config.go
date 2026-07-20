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
	Visible       bool           `json:"visible"`
	Width         int            `json:"width"`
	ConfirmDelete bool           `json:"confirm_delete"`
	FileIcons     FileIconConfig `json:"file_icons"`
}

type FileIconConfig struct {
	Enabled   bool   `json:"enabled"`
	ColorMode string `json:"color_mode"`
}

type LSPCmd struct {
	Command        string      `json:"command"`
	Args           []string    `json:"args"`
	Extensions     []string    `json:"extensions,omitempty"`
	LanguageID     string      `json:"language_id,omitempty"`
	ManagedCommand string      `json:"managed_command,omitempty"`
	Install        *LSPInstall `json:"install,omitempty"`
}

// LSPInstall describes an opt-in language-server installation. Placeholders
// such as {install_dir} are expanded by the LSP manager at runtime.
type LSPInstall struct {
	Name    string            `json:"name,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
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
		Sidebar: SidebarConfig{
			Visible:       true,
			Width:         30,
			ConfirmDelete: true,
			FileIcons:     FileIconConfig{Enabled: true, ColorMode: "accent"},
		},
		LSP: map[string]LSPCmd{
			"go": {
				Command: "gopls", Args: []string{"serve"}, Extensions: []string{".go"},
				ManagedCommand: "{install_dir}/bin/gopls",
				Install:        &LSPInstall{Name: "gopls", Command: "go", Args: []string{"install", "golang.org/x/tools/gopls@latest"}, Env: map[string]string{"GOBIN": "{install_dir}/bin"}},
			},
			"python": {
				Command: "pyright-langserver", Args: []string{"--stdio"}, Extensions: []string{".py", ".pyi"},
				ManagedCommand: "{install_dir}/node_modules/.bin/pyright-langserver",
				Install:        &LSPInstall{Name: "Pyright", Command: "npm", Args: []string{"install", "--prefix", "{install_dir}", "pyright"}},
			},
			"typescript": {
				Command: "typescript-language-server", Args: []string{"--stdio"}, Extensions: []string{".ts", ".tsx", ".mts", ".cts"},
				ManagedCommand: "{install_dir}/node_modules/.bin/typescript-language-server",
				Install:        &LSPInstall{Name: "TypeScript Language Server", Command: "npm", Args: []string{"install", "--prefix", "{install_dir}", "typescript-language-server", "typescript"}},
			},
			"javascript": {
				Command: "typescript-language-server", Args: []string{"--stdio"}, Extensions: []string{".js", ".jsx", ".mjs", ".cjs"},
				ManagedCommand: "{install_dir}/node_modules/.bin/typescript-language-server",
				Install:        &LSPInstall{Name: "TypeScript Language Server", Command: "npm", Args: []string{"install", "--prefix", "{install_dir}", "typescript-language-server", "typescript"}},
			},
			"rust": {
				Command: "rust-analyzer", Extensions: []string{".rs"}, ManagedCommand: "{home}/.cargo/bin/rust-analyzer",
				Install: &LSPInstall{Name: "rust-analyzer", Command: "rustup", Args: []string{"component", "add", "rust-analyzer"}},
			},
			"terraform": {
				Command: "terraform-ls", Args: []string{"serve"}, Extensions: []string{".tf", ".tfvars"},
				LanguageID: "terraform",
				ManagedCommand: "{install_dir}/bin/terraform-ls",
				Install:        &LSPInstall{Name: "Terraform Language Server", Command: "go", Args: []string{"install", "github.com/hashicorp/terraform-ls@latest"}, Env: map[string]string{"GOBIN": "{install_dir}/bin"}},
			},
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
	// A present lsp object replaces the defaults. This makes "lsp": {} an
	// explicit opt-out while omitting lsp continues to use managed defaults.
	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(data, &topLevel); err == nil {
		if _, present := topLevel["lsp"]; present {
			cfg.LSP = make(map[string]LSPCmd)
		}
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Defaults(), fmt.Errorf("parsing config: %w", err)
	}
	cfg.normalize()
	return cfg, nil
}

func (c *Config) normalize() {
	switch c.Sidebar.FileIcons.ColorMode {
	case "", "accent", "semantic", "none":
	default:
		c.Sidebar.FileIcons.ColorMode = "accent"
	}
	if c.Sidebar.FileIcons.ColorMode == "" {
		c.Sidebar.FileIcons.ColorMode = "accent"
	}

	// Preserve managed metadata when loading the older command/args-only shape.
	// A custom command remains custom and is never assigned a default installer.
	defaults := Defaults()
	for language, server := range c.LSP {
		builtIn, ok := defaults.LSP[language]
		if !ok {
			continue
		}
		if len(server.Extensions) == 0 {
			server.Extensions = builtIn.Extensions
		}
		if server.Command == builtIn.Command {
			if server.ManagedCommand == "" {
				server.ManagedCommand = builtIn.ManagedCommand
			}
			if server.Install == nil {
				server.Install = builtIn.Install
			}
		}
		c.LSP[language] = server
	}
}
