<div align="center">

<h1>
<img width="200" alt="prek" src="toast-logo.png" />

# toast

</h1>
</div>


A lightweight, in-terminal IDE for quick file edits. Toast runs entirely in your terminal with a familiar editor feel — file tree, tabs, syntax highlighting, LSP support, and git status — without the overhead of a full GUI.

<img src="toast-example.png" alt="toast logo" width="800" style="border-radius: 12px; display: block; margin: 20px 0;">


## Features

- **Multi-tab editing** with unsaved-changes indicators
- **Syntax highlighting** via tree-sitter (Go, Python, JavaScript, TypeScript, Rust, CSS, HTML, YAML, Bash, Markdown)
- **LSP integration** — completions, hover docs, and go-to-definition out of the box
- **File tree sidebar** with git status, create/rename/delete, and file watching
- **Project-wide search** powered by `rg` (ripgrep)
- **Rope-backed buffer** with full undo/redo
- **Theme system** — built-in dark and light themes, plus a VSCode theme importer
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

Requires Go 1.25+.

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
```

## Keybindings

| Key | Action |
|-----|--------|
| `Ctrl+Q` | Quit |
| `Ctrl+S` / `Cmd+S` | Save |
| `Ctrl+N` / `Cmd+N` | New file |
| `Ctrl+W` / `Cmd+W` | Close tab |
| `Ctrl+Tab` | Next tab |
| `Ctrl+Shift+Tab` | Previous tab |
| `Ctrl+B` | Toggle sidebar |
| `Ctrl+Shift+F` | Search |
| `Ctrl+G` / `Cmd+L` | Go to line |
| `Ctrl+Z` / `Cmd+Z` | Undo |
| `Ctrl+Y` / `Ctrl+Shift+Z` | Redo |
| `Ctrl+Space` | Trigger completion |
| `Ctrl+Shift+K` | Show hover |
| `F12` | Go to definition |

## Configuration

Toast reads `~/.config/toast/config.json` on startup. Missing keys fall back to defaults.

```json
{
  "theme": "toast-dark",
  "editor": {
    "tab_width": 4,
    "word_wrap": false,
    "show_whitespace": false,
    "auto_indent": true,
    "trim_trailing_whitespace_on_save": true,
    "insert_final_newline_on_save": true
  },
  "sidebar": {
    "visible": true,
    "width": 30,
    "confirm_delete": true
  },
  "lsp": {
    "go":         { "command": "gopls",                       "args": ["serve"] },
    "python":     { "command": "pyright-langserver",          "args": ["--stdio"] },
    "typescript": { "command": "typescript-language-server",  "args": ["--stdio"] },
    "rust":       { "command": "rust-analyzer",               "args": [] }
  },
  "search": {
    "command": "rg",
    "args": ["--json"]
  },
  "ignored_patterns": [".git", "node_modules", "__pycache__", ".DS_Store"]
}
```

### Themes

Built-in themes: `toast-dark`, `toast-light`. Custom themes live in `~/.config/toast/themes/`.

**Import a VSCode theme:**

```bash
toast migrate-theme vscode path/to/theme.json
# writes ~/.config/toast/themes/<theme-name>.json
```

Then set `"theme": "<theme-name>"` in your config.

## Development

```bash
make build    # compile
make run      # go run ./cmd/toast .
make test     # go test ./...
```
