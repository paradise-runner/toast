package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/components/breadcrumbs"
	"github.com/yourusername/toast/internal/components/filetree"
	"github.com/yourusername/toast/internal/components/quickopen"
	"github.com/yourusername/toast/internal/components/search"
	"github.com/yourusername/toast/internal/messages"
)

// toastOSCNumber is the OSC number used for toast-specific commands pushed by
// the native app host (the bundled ghostling terminal's menu bar).
const toastOSCNumber = "1337"

type toastOSC struct {
	command string
	arg     string
}

// parseToastOSC extracts a toast command from an unknown OSC sequence. The
// host pushes commands of the form "\x1b]1337;toast-open-file;/abs/path\x1b\\"
// into the pty; ultraviolet surfaces them here as uv.UnknownOscEvent.
func parseToastOSC(raw string) (toastOSC, bool) {
	raw = strings.TrimPrefix(raw, "\x1b]")
	raw = strings.TrimPrefix(raw, "\x9d")
	raw = strings.TrimSuffix(raw, "\x07")
	raw = strings.TrimSuffix(raw, "\x1b\\")
	raw = strings.TrimSuffix(raw, "\x9c")

	parts := strings.SplitN(raw, ";", 3)
	if len(parts) < 2 || parts[0] != toastOSCNumber {
		return toastOSC{}, false
	}
	osc := toastOSC{command: parts[1]}
	if len(parts) == 3 {
		osc.arg = parts[2]
	}
	return osc, true
}

// requestWorkspacePickerCmd writes the escape sequence that asks the bundled
// ghostling terminal to open its native folder picker (NSOpenPanel). Ghostling
// scans pty output for this marker, strips it, and opens the panel; the chosen
// folder comes back as a toast-open-folder OSC command.
func requestWorkspacePickerCmd() tea.Cmd {
	return func() tea.Msg {
		fmt.Fprint(os.Stdout, "\x1b]1337;toast-request-open-folder\x07")
		return nil
	}
}

// handleToastOSC processes a toast OSC command, mutating the model as needed
// and returning a command to run (or nil).
func (m *Model) handleToastOSC(raw string) tea.Cmd {
	osc, ok := parseToastOSC(raw)
	if !ok {
		return nil
	}
	switch osc.command {
	case "toast-open-file":
		path := m.resolveWorkspacePath(osc.arg)
		if path == "" {
			return nil
		}
		return func() tea.Msg { return messages.FileSelectedMsg{Path: path} }

	case "toast-open-folder":
		path := m.resolveWorkspacePath(osc.arg)
		if path == "" {
			return nil
		}
		return m.switchWorkspace(path)

	case "toast-action":
		return m.menuAction(osc.arg)

	default:
		return nil
	}
}

// resolveWorkspacePath makes an OSC-supplied path absolute relative to the
// current workspace root when needed. Empty input yields "".
func (m *Model) resolveWorkspacePath(path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.rootDir, path)
	}
	return path
}

// menuAction triggers a TUI action from the native menu bar. The action is
// expressed as the key press that normally drives it, so user keybindings and
// component logic stay in a single place; the menu's key equivalents are
// intercepted by macOS and never reach the terminal.
func (m *Model) menuAction(action string) tea.Cmd {
	var msg tea.KeyPressMsg
	switch action {
	case "save":
		msg = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "undo":
		msg = tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
	case "redo":
		msg = tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
	case "cut":
		msg = tea.KeyPressMsg{Code: 'x', Mod: tea.ModSuper}
	case "copy":
		msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper}
	case "paste":
		msg = tea.KeyPressMsg{Code: 'v', Mod: tea.ModSuper}
	case "select-all":
		msg = tea.KeyPressMsg{Code: 'a', Mod: tea.ModSuper}
	case "toggle-sidebar":
		msg = tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
	case "quick-open":
		msg = tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "find":
		msg = tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
	case "close-tab":
		msg = tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
	case "quit":
		msg = tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl}
	case "search":
		msg = tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl | tea.ModShift}
	default:
		return nil
	}
	return m.handleKey(msg)
}

// switchWorkspace points the whole workspace at a new root directory: the
// file tree, breadcrumbs, quick-open, search, git status, and LSP all follow.
// Open buffers are kept (their files are absolute paths and remain editable).
func (m *Model) switchWorkspace(dir string) tea.Cmd {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	// Re-root the workspace components.
	m.rootDir = absDir
	m.workspacePromptVisible = false
	m.fileTree = filetree.New(m.theme, m.cfg, absDir)
	m.breadcrumb = breadcrumbs.New(m.theme, absDir)
	m.quickOpen = quickopen.New(m.theme, absDir, m.cfg.IgnoredPatterns)
	m.search = search.New(m.theme, absDir, m.cfg.Search)
	m.sidebarVisible = true
	m.focus = FocusEditor

	// Restart LSP and the file watcher for the new root.
	m.restartServices()

	var cmds []tea.Cmd
	cmds = append(cmds, m.runGitStatus())
	cmds = append(cmds, m.resizeComponents()...)
	return tea.Batch(cmds...)
}
