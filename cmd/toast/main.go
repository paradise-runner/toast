package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/paradise-runner/toast/internal/app"
	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/migrate"
	"github.com/paradise-runner/toast/internal/session"
	"github.com/paradise-runner/toast/internal/workspace"
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

const migrateThemeUsage = `usage: toast migrate-theme <source> <path-to-theme.json> [output-dir]

Sources:
  vscode  A VSCode colour theme (.json)
  zed     A Zed theme file (.json); every variant it contains is converted

Without output-dir, themes are written to ~/.config/toast/themes/.
`

func runMigrateTheme(args []string) int {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, migrateThemeUsage)
		return 1
	}
	source, themePath := args[0], args[1]

	outDir := ""
	if len(args) >= 3 {
		outDir = args[2]
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		outDir = filepath.Join(home, ".config", "toast", "themes")
	}

	switch source {
	case "vscode":
		result, err := migrate.ConvertVSCode(themePath, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Theme %q written to %s\n", result.Name, result.OutputPath)
	case "zed":
		results, err := migrate.ConvertZed(themePath, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		for _, result := range results {
			fmt.Printf("Theme %q written to %s\n", result.Name, result.OutputPath)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown theme source %q\n\n%s", source, migrateThemeUsage)
		return 1
	}
	return 0
}

// splitLaunchArgs separates launch flags from positional arguments so that
// "toast --no-restore path" and "toast path --no-restore" behave the same.
func splitLaunchArgs(args []string) (positional []string, noRestore bool) {
	for _, arg := range args {
		if arg == "--no-restore" {
			noRestore = true
			continue
		}
		positional = append(positional, arg)
	}
	return positional, noRestore
}

func main() {
	ver := currentVersion()
	args, noRestore := splitLaunchArgs(os.Args[1:])
	if len(args) >= 1 {
		switch args[0] {
		case "-v", "--version":
			fmt.Println(ver)
			return
		case "--help", "-h":
			fmt.Printf(`toast %s - a terminal editor

Usage:
  toast [path]           Open a file, directory or .code-workspace file
                         (defaults to current directory)
  toast migrate-theme <source> <path> [out-dir]
                         Convert a VSCode or Zed theme to toast format

Options:
  -v, --version  Print version and exit
  -h, --help     Print this help and exit
  --no-restore   Start fresh instead of restoring the saved session
`, ver)
			return
		case "migrate-theme":
			os.Exit(runMigrateTheme(args[1:]))
		}
	}

	dir := "."
	var initialFile string
	var wsSpec *workspace.Workspace

	if len(args) >= 1 {
		arg := args[0]
		absArg, err := filepath.Abs(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if workspace.IsWorkspaceFile(absArg) {
			// A .code-workspace file defines several roots. Load it up front so a
			// bad file fails before a program starts, with a real error message.
			wsSpec, err = workspace.Load(absArg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		} else if info, err := os.Stat(absArg); err == nil && !info.IsDir() {
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config error: %v\n", err)
	}

	// A build with an embedded ripgrep (make build) extracts it once and points
	// search at it, so project-wide search works without rg on PATH. The desktop
	// bundle sets TOAST_RG_PATH itself, so this is a no-op there.
	if os.Getenv("TOAST_RG_PATH") == "" {
		if path := bundledRipgrep(); path != "" {
			os.Setenv("TOAST_RG_PATH", path)
		}
	}

	// The bundled desktop app (toast-libghostty) runs inside a terminal that
	// cannot report a meaningful system color scheme, so a \"system\" theme
	// would resolve to the terminal's raw black/white defaults. Default the
	// bundle to the built-in toast-dark theme instead.
	if os.Getenv("TOAST_GHOSTTY_BUNDLE") == "1" {
		cfg.Theme = "toast-dark"
	}

	// The bundled desktop app (toast-libghostty) launches with a working
	// directory chosen by LaunchServices (often "/"), which is never what the
	// user wants. With no path argument, open a blank editor instead: point
	// the workspace at the home directory and hide the file tree.
	bundledFreshOpen := os.Getenv("TOAST_GHOSTTY_BUNDLE") == "1" && len(args) == 0
	if bundledFreshOpen {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		}
		cfg.Sidebar.Visible = false
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	themeDir := filepath.Join(home, ".config", "toast", "themes")

	var model *app.Model
	if wsSpec != nil {
		model, err = app.NewWorkspace(cfg, themeDir, wsSpec, initialFile)
	} else {
		model, err = app.New(cfg, themeDir, absDir, initialFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Per-folder session state. The bundle's fresh-open state has no
	// user-chosen root yet (it starts on the home directory); the app neither
	// restores nor writes for it, and both directions start working once a
	// folder is picked.
	if stateDir, err := session.DefaultDir(); err == nil {
		model.ConfigureSession(stateDir, cfg.Session, !noRestore)
	}

	// In the bundle's fresh-open state show a Select Workspace button that
	// opens the native folder picker.
	if bundledFreshOpen {
		model.ShowWorkspacePrompt()
	}

	p := tea.NewProgram(model)
	model.SetLSPSend(p.Send)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	model.ShutdownLSP()
}
