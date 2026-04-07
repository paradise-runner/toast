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

var version = "v0.0.7"

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
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Println(version)
			return
		case "--help", "-h":
			fmt.Printf(`toast %s - a terminal editor

Usage:
  toast [path]           Open a file or directory (defaults to current directory)
  toast migrate-theme vscode <path>  Convert a VSCode theme to toast format

Options:
  -v, --version  Print version and exit
  -h, --help     Print this help and exit
`, version)
			return
		case "migrate-theme":
			os.Exit(runMigrateTheme(os.Args[2:]))
		}
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
			// Argument is an existing file: find git root, fall back to parent dir
			initialFile = absArg
			if root, ok := findGitRoot(absArg); ok {
				dir = root
			} else {
				dir = filepath.Dir(absArg)
			}
		} else if err != nil {
			// Argument doesn't exist — treat as a new file if its parent dir exists
			if _, parentErr := os.Stat(filepath.Dir(absArg)); parentErr == nil {
				initialFile = absArg
				if root, ok := findGitRoot(absArg); ok {
					dir = root
				} else {
					dir = filepath.Dir(absArg)
				}
			} else {
				// Parent dir missing too — fall back to treating as a (missing) directory
				dir = absArg
			}
		} else {
			// Argument is an existing directory
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
