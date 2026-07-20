
<div align="center">

<h1>
<img width="200" alt="prek" src="toast-logo.png" />

# toast

</h1>
</div>


A lightweight, in-terminal IDE for quick file edits. Toast runs entirely in your terminal with a familiar editor feel: file tree, tabs, syntax highlighting, LSP support, project search, and git status, without the overhead of a full GUI.

> ⚠️ This project is in _early development_, you may encounter bugs. ⚠️

<img src="toast-demo.gif" alt="toast logo" width="800" style="border-radius: 12px; display: block; margin: 20px 0;">


## Features

- **Multi-tab editing** with unsaved-changes indicators, mouse-close buttons, and quit confirmation
- **Syntax highlighting** via tree-sitter (Go, Python, JavaScript, TypeScript, Rust, CSS, HTML, YAML, Bash, Markdown)
- **Managed language servers** — Toast offers to install missing servers for Go, Rust, Python, JavaScript, and TypeScript, with an extensible config for other languages
- **Go to definition** — hold `Ctrl` and hover to underline symbols with a target, then `Ctrl`-click to jump to the exact definition
- **File tree sidebar** with git status, ignored-file dimming, create/delete actions, file watching, and draggable resizing
- **Project-wide search** powered by `rg` (ripgrep)
- **In-file find/replace** with next/previous navigation, match-case, and whole-word options
- **Go to line** overlay
- **Markdown preview** for `.md`, `.markdown`, and `.mdx` files
- **External file watching** that silently reloads clean buffers
- **Rope-backed buffer** with full undo/redo
- **Theme system** — built-in `system` (derived from terminal colors at runtime), `toast-dark`, and `toast-light`, plus a VSCode theme importer
- **Binary file guard** to avoid dumping binary content into the editor
- **Configurable** via `~/.config/toast/config.json`

## Installation

**Homebrew (macOS)**

```bash
brew install paradise-runner/tap/toast
```

**Download a release**

Grab a zip for your platform from the [releases page](https://github.com/paradise-runner/toast/releases), unzip it, and place the binary on your `$PATH`:

```bash
# example for Apple Silicon
curl -Lo toast.zip https://github.com/paradise-runner/toast/releases/latest/download/toast-darwin-arm64.zip
unzip toast.zip
install -m755 toast-darwin-arm64 /usr/local/bin/toast
```

**Build from source**

Requires Go 1.25.2+.

```bash
git clone https://github.com/paradise-runner/toast
cd toast
make build
# binary written to bin/toast
```

## Usage

```bash
toast               # open current directory
toast path/to/dir   # open a specific directory
toast path/to/file  # open a file (auto-detects git root)
toast new/file.go   # open a new file buffer if the parent directory exists
toast --help
toast --version
```

`rg` is required for project search. When a built-in language server is missing, Toast shows an install prompt in the lower-right corner. Managed installs use the language's standard toolchain (`go`, `npm`, or `rustup`) and only run after you accept the prompt. Toast also uses compatible servers already on your `$PATH`.

## Keybindings

| Key | Action |
|-----|--------|
| `Ctrl+Q` | Quit |
| `Ctrl+S` / `Cmd+S` | Save |
| `Ctrl+W` / `Cmd+W` | Close tab |
| `Ctrl+P` | Quick-open file search (fuzzy) |
| `Ctrl+Alt+Right` | Next tab |
| `Ctrl+Alt+Left` | Previous tab |
| `Ctrl+B` | Toggle sidebar |
| `Ctrl+Shift+E` | Toggle focus between editor and file tree |
| `Ctrl+Shift+F` | Search |
| `Ctrl+F` / `Cmd+F` | Find and replace in the current file |
| `Ctrl+G` / `Cmd+L` | Go to line |
| `Ctrl+Shift+M` | Toggle Markdown preview |
| `Ctrl+Z` / `Cmd+Z` | Undo |
| `Ctrl+Y` / `Ctrl+Shift+Z` / `Cmd+Y` / `Cmd+Shift+Z` | Redo |
| `Ctrl+Space` / `Cmd+Space` | Trigger completion |
| `Ctrl+Shift+K` | Show hover |
| `Ctrl`+hover / `Ctrl`-click | Check for and follow a definition |
| `F12` | Go to the definition at the cursor |

## Configuration

Toast reads `~/.config/toast/config.json` on startup. Missing keys fall back to defaults.

```json
{
  "theme": "toast-dark",
  "editor": {
    "tab_width": 4,
    "auto_indent": true,
    "trim_trailing_whitespace_on_save": true,
    "insert_final_newline_on_save": true
  },
  "sidebar": {
    "visible": true,
    "width": 30,
    "confirm_delete": true,
    "file_icons": {
      "enabled": true,
      "color_mode": "accent"
    }
  },
  "ignored_patterns": [".git", "node_modules", "__pycache__", ".DS_Store"]
}
```

Omit `lsp` to use Toast's managed defaults for Go, Rust, Python, JavaScript, and TypeScript; set `"lsp": {}` to disable language servers. Each entry is extension-driven, so other languages can be added without changing Toast. A custom server already installed on `$PATH` only needs a command and its filename suffixes:

```json
{
  "lsp": {
    "zig": {
      "command": "zls",
      "args": [],
      "extensions": [".zig"]
    }
  }
}
```

For an opt-in managed custom server, add `managed_command` (the installed executable path) and an `install` recipe. Recipes support `{install_dir}`, `{install_root}`, `{root_dir}`, and `{home}` placeholders:

```json
{
  "lsp": {
    "example": {
      "command": "example-language-server",
      "args": ["--stdio"],
      "extensions": [".example"],
      "managed_command": "{install_dir}/bin/example-language-server",
      "install": {
        "name": "Example Language Server",
        "command": "example-package-manager",
        "args": ["install", "--bin-dir", "{install_dir}/bin", "example-language-server"],
        "env": {}
      }
    }
  }
}
```

### Themes

Built-in themes: `system`, `toast-dark`, `toast-light`. Custom themes live in `~/.config/toast/themes/`.

**Import a VSCode theme:**

```bash
toast migrate-theme vscode path/to/theme.json
# writes ~/.config/toast/themes/<theme-name>.json
```

Then set `"theme": "<theme-name>"` in your config.

## Feedback & Issues

Found a bug or have a feature request? We'd love to hear from you! Please open an [issue on GitHub](https://github.com/paradise-runner/toast/issues) with as much detail as possible. Your feedback helps make Toast better.

## Development

```bash
make build             # compile
make run               # go run ./cmd/toast .
make test              # go test ./...
make test-integration  # run opt-in Ghostty/tmux terminal integration tests
make test-integration-update  # refresh golden screenshots
```
