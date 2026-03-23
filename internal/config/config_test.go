package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourusername/toast/internal/config"
)

func TestDefaultsWhenNoFile(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Theme != "toast-dark" {
		t.Errorf("expected default theme 'toast-dark', got %q", cfg.Theme)
	}
	if cfg.Editor.TabWidth != 4 {
		t.Errorf("expected default tab width 4, got %d", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.AutoIndent {
		t.Error("expected auto_indent true by default")
	}
	if cfg.Sidebar.Width != 30 {
		t.Errorf("expected default sidebar width 30, got %d", cfg.Sidebar.Width)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{"theme": "my-theme", "editor": {"tab_width": 2}}`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != "my-theme" {
		t.Errorf("expected theme 'my-theme', got %q", cfg.Theme)
	}
	if cfg.Editor.TabWidth != 2 {
		t.Errorf("expected tab width 2, got %d", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.AutoIndent {
		t.Error("expected auto_indent to default to true even when not in file")
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.json")
	cfg := config.Defaults()
	cfg.Theme = "toast-light"
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loaded.Theme != "toast-light" {
		t.Errorf("expected toast-light, got %q", loaded.Theme)
	}
	if loaded.Editor.TabWidth != cfg.Editor.TabWidth {
		t.Errorf("expected tab_width %d, got %d", cfg.Editor.TabWidth, loaded.Editor.TabWidth)
	}
}

func TestInvalidJSONFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(cfgPath)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if cfg.Theme != "toast-dark" {
		t.Errorf("expected fallback to default theme, got %q", cfg.Theme)
	}
}
