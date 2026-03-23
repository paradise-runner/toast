# AGENTS.md

Guidelines for agentic coding tools (Copilot, Cursor, Claude, etc.) working in this repository.

## Module

```
github.com/paradise-runner/toast
```

## Build & Run

```bash
make build          # go build -o bin/toast ./cmd/toast
make run            # go run ./cmd/toast .
make clean          # rm -rf bin/
```

## Testing

```bash
make test                                                    # go test ./...
go test ./internal/components/editor/                        # single package
go test ./internal/components/editor/ -run TestMouseDrag      # single test (substring match)
go test -v ./...                                             # verbose
go test -race ./...                                          # race detector
```

## Linting

No dedicated linter config. Use the standard tools:

```bash
go vet ./...
gofmt -l .
goimports -w .
```

## Architecture

Entry point: `cmd/toast/main.go`. Top-level bubbletea model: `internal/app/app.go`.

```
internal/
  app/            # top-level model: layout, focus, mouse routing
  buffer/         # EditBuffer (rope-backed): edit.go, rope.go
  clipboard/      # clipboard abstraction
  components/
    breadcrumbs/  # path breadcrumb above editor
    closedialog/  # unsaved-changes confirmation dialog
    editor/       # text editor (editor.go, 1600+ lines)
    filetree/     # sidebar file tree
    search/       # search panel
    statusbar/    # bottom status bar
    tabbar/       # open-file tab bar
    themepicker/  # theme selection overlay
  config/         # Config struct, Defaults(), Load()
  git/            # git status / diff helpers
  lsp/            # Language Server Protocol client
  messages/       # all inter-component message types
  syntax/         # tree-sitter syntax highlighting
  theme/          # theme manager
  watcher/        # fsnotify file watcher
```

## bubbletea v2

This project uses **bubbletea v2** (`charm.land/bubbletea/v2`) and **lipgloss v2**
(`charm.land/lipgloss/v2`). **Do not** import the v1 paths
(`github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`).

Key v2 differences:

- `View()` returns `tea.View`, not `string`. Use `tea.NewView(content)`.
  Set `AltScreen` and `MouseMode` as fields on the returned `tea.View`.
- Key events: `tea.KeyPressMsg`. No named key constants; use `msg.String()`
  (e.g. `"ctrl+c"`, `"enter"`).
- Modifiers: `msg.Mod.Contains(tea.ModCtrl)`, `msg.Mod.Contains(tea.ModSuper)`.
- Mouse types: `tea.MouseClickMsg`, `tea.MouseReleaseMsg`,
  `tea.MouseMotionMsg`, `tea.MouseWheelMsg`.

## Component Pattern

```go
// Value receivers for Init, Update, View.
func (m Model) Init() (Model, tea.Cmd)
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() tea.View   // top-level only; inner components return string

// Pointer receivers only for helpers that mutate state outside the update loop.
func (m *Model) reset() { ... }
```

Mouse coordinates: the app model subtracts `sidebarWidth` from X and
`tabBarHeight + breadcrumbHeight` from Y before forwarding to child components.

## Inter-Component Messages

Defined in `internal/messages/messages.go`. Every type uses a `*Msg` suffix
(e.g. `FileSelectedMsg`, `BufferOpenedMsg`). Add new message types there, not
in component packages.

## Cursor & Buffer Conventions

- `cursorPos{line, col}` is **zero-based**. `col` is a **byte offset** (not rune/display).
- `buffer.EditBuffer` is rope-backed. Use `Insert`, `Delete`, `Undo`, `Redo`;
  do not manipulate the underlying rope directly.
- `gutterWidth = len(strconv.Itoa(lineCount)) + 5` (digits + space + diff-bar + fold-indicator + space before content).
- `editor.screenToBuffer(x, y)` converts adjusted screen coords to `(line, col)`.

## Import Grouping

Three groups separated by blank lines: stdlib, external, internal.

```go
import (
    "fmt"
    "strings"

    tea "charm.land/bubbletea/v2"
    "charm.land/lipgloss/v2"

    "github.com/yourusername/toast/internal/buffer"
    "github.com/yourusername/toast/internal/messages"
)
```

`goimports` enforces this. Run `goimports -w .` before committing.

## Code Style

- **Constants**: typed `const` blocks with `iota`.
- **Section comments**: Unicode-dash rulers — `// ── Rendering ──────────────`.
- **Error handling**: wrap with `fmt.Errorf("context: %w", err)`, propagate up.
  Non-fatal warnings go to stderr: `fmt.Fprintf(os.Stderr, ...)`.
- **Theme colors**: access via `m.theme.UI("key")` or `m.theme.Git("key")`.
  Never hardcode color values.
- **No global mutable state**. All state lives in model structs passed by value.
- **Naming**: exported types are `PascalCase`; unexported helpers `camelCase`.
  Constructors are named `New` (e.g. `editor.New()`). Message types end in `Msg`.
- **Package comments**: placed on the `package` line or as a doc comment above it
  (see `editor.go` for an example).

## Test Style

- Test files use the **same package** (e.g. `package editor`, not `editor_test`).
- Provide a `newTestModel(content string) Model` helper for unit tests.
- Use `t.Fatalf` (not `t.Errorf`) when failure would cause panics or misleading output.
- Prefer table-driven tests for many input/output combinations.
- Do **not** start a real bubbletea program; call `Update` and helpers directly.
- Use `t.TempDir()` for tests that need filesystem paths.
