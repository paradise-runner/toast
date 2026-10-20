# Release Notes

## [0.12.2](https://github.com/paradise-runner/toast/compare/v0.12.1...v0.12.2) (2026-10-19)


### Fixes

* keep sidebar context menu clickable at the window edge ([#80](https://github.com/paradise-runner/toast/issues/80)) ([6426548](https://github.com/paradise-runner/toast/commit/64265486d6e9922fa2f977b73e854e81d63ebab8)), closes [#60](https://github.com/paradise-runner/toast/issues/60)

## [0.12.1](https://github.com/paradise-runner/toast/compare/v0.12.0...v0.12.1) (2026-10-19)


### Fixes

* keep mouse selection drag tracking past viewport edges ([#77](https://github.com/paradise-runner/toast/issues/77)) ([cc05cc3](https://github.com/paradise-runner/toast/commit/cc05cc317be0223c8ef0e3ae737904c0142b0804))

## v0.10.0 — 2026-10-16

Fixes horizontal scrolling getting stuck after scrolling right on a long line: shorter lines rendered blank and clicks mapped to line ends, with no way back except moving the caret to column 0. Also adds horizontal wheel / shift+wheel scrolling in non-wrap mode, clamped to the longest visible line, and hides the caret instead of pinning it to the gutter when scrolled off-screen.

### Fixes
- `clampViewport` now compares raw byte offsets instead of display columns, so a cursor line scrolled entirely off-screen snaps back to the line start; otherwise the viewport scrolls the minimum needed to reveal the cursor. Regression tests in `hscroll_test.go` cover short/blank line recovery, wheel and shift+wheel scrolling, clamping, wrap-mode no-op, and caret hiding.
- Horizontal wheel (`MouseWheelLeft`/`Right`) and shift+wheel scroll the viewport without moving the caret in non-wrap mode, clamped to the longest visible line so it cannot drift into empty space.

Addresses #68.

## v0.9.0 — 2026-08-29

The command palette (issue #26) — `Ctrl+Shift+P` fuzzy search over every editor action — with keybinding hints, live toggle-state pills, MRU ordering, a rotating-tips footer, and a status-bar confirmation flash.

### Command palette
- `Ctrl+Shift+P` opens a fuzzy-search palette over every editor action: type to filter by label or category, arrows to navigate, Enter to run, Escape to dismiss. The selected command's description shows beneath the list, and a rotating tips footer cycles on each open.
- Rows show each command's keybinding hint (right-aligned) and a live `●on` / `○off` pill for toggle commands (e.g. sidebar, markdown preview), tracked through the settings state at open time.
- Results are ordered most-recently-used-first on an empty query, with fuzzy score, usage, then label as tie-breakers once you type.
- The overlay gets an accent-glow border in the theme's `find_replace_border` accent (purple in the builtin themes), shifts to the accent color when the palette locks onto a single result, and the page behind it is faint-dimmed; the footer tips render in a new `muted` secondary-text token for legibility.
- Choosing a command flashes a branded confirmation in the status bar (`⌄ Saved main.go`, ~1.5 s), and when a command has a keybinding you could have pressed instead, a coach nudge shows it (`⌄ next time: ctrl+s`).
- App-level key dispatch was refactored into `runAction` keyed by `config.Action*` IDs — keybinding cases delegate to it with no behavior change — and a shared fuzzy matcher was extracted into `internal/fuzzy` so quick-open and the palette score identically. The palette also gains a Theme Picker action.

### Docs & misc
- README: command palette feature bullet, `Ctrl+Shift+P` in the keybinding table, and `command_palette` / `theme_picker` added to the available-actions list.

---

## v0.8.2 — 2026-08-29

Auto-save now writes files verbatim — intentional trailing spaces (e.g. markdown hard line breaks) survive background saves, and only an explicit ctrl+s applies the configured trim — and cursor positioning correctly handles multi-cell emoji. The README also gains a features/trade-offs comparison table.

### Fixes
- Auto-save writes the buffer **verbatim** (issue #56): background saves no longer strip trailing whitespace or force a final newline, so intentional trailing spaces survive auto-save. Only an explicit ctrl+s (or the settings toggle) applies the configured `trim_trailing_whitespace_on_save`. Regression tests cover trailing spaces on disk and mid-file edits.
- Cursor, layout, and wrapping math now use per-grapheme-cluster display widths, fixing the off-by-one cursor column after wide emoji (e.g. ⚠️ renders 2 cells) and misaligned wrapped lines. Regression tests cover emoji/VS16 sequences.

### Docs & misc
- README adds a features/trade-offs comparison table (toast vs vim/emacs/vscode/zed): no AI features, no telemetry, built-in file tree, mouse support — toast is the only one that checks every box.
- New `docs/experimental/side-by-side-editors.md` design & learnings doc for the in-progress side-by-side editors feature (issue #49).
- LICENSE updated with the maintainer's pledge: if toast ever gains irrelevant features (AI agents, AI chat, telemetry) that don't make it a better text editor, you can come kick the maintainer's ass.

---

## v0.8.1 — 2026-08-06

Fixes a crash (panic) when editing multi-line text: auto-save's trailing-whitespace trim replaced the buffer without moving the cursor, so the next backspace sliced past the end of a now-shorter line.

### Stability
- Fixed the `slice bounds out of range` panic on backspace after an auto-save trimmed trailing whitespace (e.g. deleting spaces at the end of a line that then became empty). The cursor is now clamped back inside the buffer when save-time whitespace trimming or JSON reformatting shortens a line.
- Line-slicing edit/navigation operations (backspace, word-delete, arrow/word movement, forward-delete, completion insert) now defensively clamp a stale cursor column before reading the line, so any out-of-bounds cursor state can never panic the editor again.
- Regression tests cover backspace after a whitespace-trimming save (empty line and mid-line cases) plus a deliberately corrupted cursor column.

---

## v0.8.0 — 2026-08-05

Toast now ships as a **standalone macOS app**: the TUI bundled inside a libghostty terminal window, so you can run toast as a native desktop application — dock icon, menu bar, and all — without a terminal or Homebrew.

### Standalone macOS app
- Every release now includes `toast-app-darwin-arm64.zip` (Apple Silicon) and `toast-app-darwin-amd64.zip` (Intel): download, unzip, drag **Toast.app** into Applications.
- Native macOS menu bar (Toast / File / Edit / View) with Cmd key equivalents and `NSOpenPanel` file/folder pickers, an About dialog with the version and a GitHub link, and a **Select Workspace** button for choosing a folder when the app opens fresh.
- Opening a file or folder re-roots the whole workspace — file tree, breadcrumbs, quick-open, search, git status, and LSP all follow.
- ⚠️ Experimental: the app wraps toast in a bundled terminal and has more rough edges than the TUI — the terminal version (Homebrew or release binary) remains the primary install.
- Built by `scripts/build-libghostty-bundle.sh` (`make app`); see `docs/experimental/libghostty-bundle.md` for how it works and its limitations.

---

## v0.7.0 — 2026-08-04

Toast now saves your work automatically: by default, dirty files are written to disk shortly after you stop typing, so an unexpected quit can never lose edits again.

### Auto-save
- Enabled by default: after 300ms of inactivity — the debounce resets with every keystroke — the file you're editing is written to disk, along with any other dirty open tabs.
- Configurable in `~/.config/toast/config.json`: `editor.auto_save` selects `"auto"` or `"manual"` (manual restores the classic ctrl+s workflow), and `editor.auto_save_delay_ms` tunes the inactivity window.
- Both options are also live-adjustable from the Settings dialog (`Ctrl+,` → Editor): an Auto Save auto/manual cycle and an Auto Save Delay preset cycle (100ms–10s).
- Auto-save uses the same pipeline as manual save, so git status stays fresh and the file watcher never mistakes toast's own writes for external changes. (issue #45)

---

## v0.6.0 — 2026-08-04

The new settings dialog: open it from the statusbar gear button or `Ctrl+,` to browse and tweak toast's configuration from inside the editor.

### Settings
- Two-pane dialog: a group list on the left (Appearance, etc.) and the selected group's settings on the right. Navigate with `Tab`/`←`/`→` to switch panes, `↑`/`↓` to move the selection, `h`/`l` (or `◄`/`►`) to adjust stepper values, `Enter` to toggle switches, and `Esc` to close.
- Full mouse support: hovering a group or setting moves the selection, clicking activates it — toggles flip, and stepper/cycle controls respond to their arrows.
- Changes apply live: every control reads and writes the same config object the editor uses, so toggling an option or picking a theme takes effect immediately (the theme picker is now a group inside the dialog).
- New `settings_*` theme tokens (bg, fg, selected, separator, border) for the dialog, added to the built-in dark/light/system themes with a fallback for older user themes.
- Opened from the statusbar gear button or `Ctrl+,`; settings persist to disk on change.
- Tests cover keyboard navigation, toggles, steppers, mouse hit-testing, and the new theme tokens.

---

## v0.5.0 — 2026-08-03

Right-clicking a file or folder in the sidebar now reveals it in your OS file manager, so you can find it in Finder (macOS), your file manager (Linux), or Explorer (Windows) without leaving the editor.

### File tree
- New **View in Finder** / **View in Folder** / **View in Explorer** context menu item (label matches the OS). Selecting it reveals the clicked file or folder in the file manager: `open -R` on macOS, `explorer /select,` on Windows, and the freedesktop `FileManager1` D-Bus interface (with an `xdg-open` fallback) on Linux. Right-clicking empty sidebar space reveals the directory under the cursor. (issue #50)

---

## v0.4.3 — 2026-08-03

A small UX patch: the sidebar context menu (New File / New Folder / Delete) now highlights the item under the mouse as you hover.

### File tree
- The context menu's selection highlight follows the mouse: hovering an item moves the highlight there, on top of the existing keyboard navigation (up/down). Hover is tracked across the whole menu box, including when the menu extends past the sidebar's right edge. (issue #51)

---

## v0.4.2 — 2026-08-03

A patch release fixing an escape-key trap in the sidebar's file/folder creation flow.

### File tree
- Escape now always dismisses the sidebar context menu, inline file/folder creation row, and delete confirmation dialog, regardless of which component has keyboard focus. Previously, right-clicking to start a creation and then clicking into the editor left the creation UI stuck open with Escape going to the editor instead. (issue #48)

---

## v0.4.1 — 2026-08-03

A patch release fixing paste consistency on macOS.

### Editor
- Fixed Ctrl+V and Cmd+V pasting different text when content was copied outside toast: in-app paste now reads the real system clipboard (`pbpaste` on macOS, `wl-paste`/`xclip`/`xsel` on Linux, PowerShell on Windows) instead of only toast's internal fallback, so both shortcuts always paste the same content. The internal store is kept as a fallback when no system clipboard tool is readable, with tests covering system-first paste, fallback, and hermetic editor tests.

---

## v0.4.0 — 2026-08-03

A small polish release.

### Landing page 🌈
- Redesigned the landing page as a full marketing site with rainbow branding: hero with a real demo GIF in a terminal frame, tech marquee, bento feature grid, keyboard shortcuts section, and install tabs with copy buttons. Colors derive from the toast-logo rainbow palette.
- Fixed the reported binary size on the page.

### Editor
- Added bottom padding to the editor so content doesn't sit flush against the bottom edge, with tests covering the change and line wrapping.

---

*Keep this file updated with a `## vX.Y.Z` section for each release — the pre-push hook (prek) refuses to push a tag without matching notes here.*
