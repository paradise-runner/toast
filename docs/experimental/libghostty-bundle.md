# Experimental libghostty Bundle

Toast is a TUI, but it can ship as a standalone desktop application by
bundling it with a libghostty-based terminal window. This mirrors the
packaging spike from Terminal Empire (`scripts/build-libghostty-bundle.sh` in
that repo).

## Shape of the Bundle

`libghostty-vt` is a terminal emulation library, not a complete terminal
window. A usable app bundle still needs a window, renderer, PTY, and
child-process launcher. Those pieces come from
[Ghostling](https://github.com/ghostty-org/ghostling), the upstream minimal C
example that embeds `libghostty-vt` with Raylib for windowing.

The build produces one distributable executable:

1. Build the regular toast TUI as a payload binary.
2. Build Ghostling, which pulls in Ghostty/libghostty through its own CMake
   flow (requires Zig 0.15.2, provisioned automatically).
3. Apply the patches in `patches/` — Raylib's default `Esc` quit is disabled so
   `Esc` reaches toast (it closes dialogs), and the terminal glyph atlas is
   extended so toast's box-drawing, arrows, check marks, middle-dot
   separators, the trigram settings icon, and quadrant spinner blocks render
   instead of `?`.
4. Copy the toast binary, Ghostling, and `libghostty-vt.dylib` into
   `cmd/toastapp/payload/`.
5. Build `cmd/toastapp`, a small self-extracting launcher.
6. Write `dist/toast-libghostty`.

At runtime, the launcher extracts Ghostling, the dylib, and toast to a
temporary directory, writes a tiny shell wrapper, sets `SHELL` to that
wrapper, and starts Ghostling. Ghostling opens its window and runs toast
inside its own PTY, so toast behaves exactly as it does in a real terminal
(mouse tracking, alt screen, truecolor).

The launcher sets `TOAST_GHOSTTY_BUNDLE=1`. Toast reads this to:

- Default to the built-in `toast-dark` theme. The bundled terminal cannot
  report a meaningful system color scheme, so a `system` theme would resolve
  to the terminal's raw black/white defaults.
- Substitute UI glyphs the embedded JetBrains Mono font lacks or renders
  badly: `≡` for the ⚙ settings button and quadrant-block frames for the
  braille LSP spinner. (Glyphs JetBrains Mono does contain, like the
  Select Workspace hint's `·` separators, are baked into the atlas by the
  glyph patch.)
- Open a blank editor when launched with no path argument. LaunchServices
  sets the app's working directory to `/`, which is never the directory the
  user intended; instead the workspace points at the home directory with the
  file tree hidden. Pass a path to open a specific directory/file.
- Show a Select Workspace button in that fresh state (see below).

## Select Workspace Button

When toast opens with no file and no explicit workspace (the bundle's
fresh-open state), the editor area shows a centered Select Workspace button
styled with the editor's own background. Pressing Enter or clicking the
button writes a marker sequence to the PTY; ghostling intercepts it and opens
the native `NSOpenPanel` folder picker. Choosing a folder re-roots the whole
workspace (file tree, breadcrumbs, quick-open, search, git status, LSP) and
shows the file tree. Clicks elsewhere in the blank editor do nothing, and Esc
dismisses the button. This gives two ways to choose a location: the button
in the GUI, and File > Open File… / Open Folder… in the menu bar.

## Native Menu Bar

The bundled app installs a native macOS menu bar (Toast / File / Edit /
View) with standard Cmd key equivalents (Cmd+O, Cmd+Shift+O, Cmd+S, Cmd+Z,
Cmd+X/C/V/A, Cmd+F, Cmd+P, Cmd+B, Cmd+W, Cmd+Q). Items whose key equivalents
are intercepted by macOS never reach the terminal; each menu action forwards
its command to toast through a custom OSC protocol written to the PTY:

- `\x1b]1337;toast-action;<name>\x1b\\` — trigger an editor action (save,
  undo, redo, cut, copy, paste, select-all, toggle-sidebar, quick-open,
  find, close-tab, quit, search)
- `\x1b]1337;toast-open-file;<path>\x1b\\` — open a file
- `\x1b]1337;toast-open-folder;<path>\x1b\\` — open a folder as the workspace

File > Open File… / Open Folder… present native `NSOpenPanel` pickers that
open at the user's home directory. Toast parses these commands in
`internal/app` (`handleToastOSC`); opening a
folder re-roots the file tree, breadcrumbs, quick-open, search, git status,
and LSP. The OSC payload is queued and flushed by ghostling's render loop —
while a modal panel is up the render loop is paused, so a direct PTY write
can hit a full kernel buffer and drop.

The Toast menu includes an About dialog (Toast menu > About Toast) showing
the current version (read from `TOAST_VERSION`, injected into the launcher at
build time from `cmd/toast`'s `version`) and a link to the GitHub page.

Quitting (Ctrl+Q, Cmd+Q, or the Toast menu > Quit Toast) exits the whole
app: toast quits, ghostling detects the child exit, and — in bundle mode —
closes its window, which ends the launcher.

The reverse direction (toast → ghostling) uses a marker sequence
`\x1b]1337;toast-request-open-folder\x07` written to the PTY by toast's
Select Workspace button; ghostling scans the pty output for it, strips it,
and opens the folder picker (the same `toast_request_open_folder` used by
the File menu).

## Build

Requirements: Go, Git, CMake, Ninja, curl, tar. Zig 0.15.2 is downloaded to
`build/` unless `ZIG` points at a working 0.15.2 binary. On macOS, if the
active Xcode SDK is too new for Zig's tbd parser, the script falls back to the
CommandLineTools SDK automatically.

```bash
make app
```

or directly:

```bash
scripts/build-libghostty-bundle.sh
```

The output is:

```bash
dist/toast-libghostty
```

On macOS the script additionally assembles **`dist/Toast.app`** — a proper
application bundle (dock icon, `Info.plist`, version) wrapping a copy of the
launcher, so the app can be dragged into `/Applications`.

Launcher arguments are forwarded to the embedded editor:

```bash
dist/toast-libghostty ~/src/foo.go
```

Set `GHOSTLING_REF` to build a different Ghostling revision (the default is
pinned to the commit the patches are tested against), `TOAST_BUILD_DIR` /
`TOAST_DIST_DIR` to relocate the build, `TOAST_VERSION` to override the
version baked into the launcher and `Info.plist` (the CI release workflow
passes the git tag), or `ZIG` / `TOAST_ZIG_VERSION` to control the Zig
toolchain.

## Limitations

- This is a single distributable binary, but not a single in-memory process.
  It extracts payload executables to a temporary directory before launching.
- The wrapper relies on Ghostling's current `SHELL` behavior for choosing the
  PTY child command.
- The bundle applies a small Ghostling patch to disable Raylib's default
  `Esc` exit key.
- Ghostling is Unix-oriented today; Windows support is future work.
- The bundle build fetches upstream code and is intentionally not wired into
  `go build ./...` (the payload directory only holds a placeholder in the
  repository).
- Code signing and notarization are not handled here. The release workflow
  ships `Toast.app` zips unsigned; macOS Gatekeeper requires the
  right-click → Open workaround on first launch.
