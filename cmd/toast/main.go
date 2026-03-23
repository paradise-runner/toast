package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/app"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/migrate"
)

// findGitRoot walks up the directory tree from path looking for a .git entry
// (directory or file, to support worktrees). It returns the directory containing
// .git and true if found, or ("", false) if no git repo is found.
func findGitRoot(path string) (string, bool) {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .git
			return "", false
		}
		dir = parent
	}
}

func runMigrateTheme(args []string) int {
	if len(args) < 2 || args[0] != "vscode" {
		fmt.Fprintf(os.Stderr, "usage: toast migrate-theme vscode <path-to-theme.json>\n")
		return 1
	}
	themePath := args[1]
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	outDir := filepath.Join(home, ".config", "toast", "themes")
	result, err := migrate.ConvertVSCode(themePath, outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Theme %q written to %s\n", result.Name, result.OutputPath)
	return 0
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "migrate-theme" {
		os.Exit(runMigrateTheme(os.Args[2:]))
	}

	dir := "."
	var initialFile string

	if len(os.Args) >= 2 {
		arg := os.Args[1]
		absArg, err := filepath.Abs(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		info, err := os.Stat(absArg)
		if err == nil && !info.IsDir() {
			// Argument is a file: find git root, fall back to parent dir
			initialFile = absArg
			if root, ok := findGitRoot(absArg); ok {
				dir = root
			} else {
				dir = filepath.Dir(absArg)
			}
		} else {
			dir = absArg
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config error: %v\n", err)
	}

	home, _ := os.UserHomeDir()
	themeDir := filepath.Join(home, ".config", "toast", "themes")

	model, err := app.New(cfg, themeDir, absDir, initialFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model)
	model.SetLSPSend(p.Send)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	model.ShutdownLSP()
}
