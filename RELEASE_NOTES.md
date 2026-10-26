# Release Notes

## [0.14.0](https://github.com/paradise-runner/toast/compare/v0.13.0...v0.14.0) (2026-10-26)


### Features

* add markdown preview pane ([13687b3](https://github.com/paradise-runner/toast/commit/13687b3f107161588988b04041f80c41be5fa5d3))
* add terminal file icons to tree ([67922f4](https://github.com/paradise-runner/toast/commit/67922f44714d71575c4ae28ebc1028eca2469e9d))
* auto-save after inactivity (issue [#45](https://github.com/paradise-runner/toast/issues/45)) ([f25dd92](https://github.com/paradise-runner/toast/commit/f25dd92508995b2c7d8b9372f14397d760c1f241))
* bundle ripgrep so search works off PATH ([f407d67](https://github.com/paradise-runner/toast/commit/f407d674bffc194a5298ac9d5eec92f47fb9a896)), closes [#62](https://github.com/paradise-runner/toast/issues/62)
* drag/drop files and folders to move them in the sidebar ([#72](https://github.com/paradise-runner/toast/issues/72)) ([53b691a](https://github.com/paradise-runner/toast/commit/53b691a0e5595e21945ee5be8e6e69d12fee36df))
* **editor:** add visual row helpers for wrap mode ([957dd8b](https://github.com/paradise-runner/toast/commit/957dd8b02385cb029ef380237e5a885bd160dd90))
* **editor:** add wordWrapChunks and chunkContaining helpers ([3998d30](https://github.com/paradise-runner/toast/commit/3998d309027a45e9bb59ce9eaaf4cc56e666c50c))
* **editor:** add wrapMode field, set on markdown file load ([186ca28](https://github.com/paradise-runner/toast/commit/186ca2838fa8cec36384b58e3dc525c85c9f1e8e))
* **editor:** update clampViewport for wrap mode ([e02a40a](https://github.com/paradise-runner/toast/commit/e02a40a7d0fcaf09ddb1dfb87abb00652da76688))
* **editor:** update cursor up/down navigation for wrap mode ([e0894ce](https://github.com/paradise-runner/toast/commit/e0894ce3e04060c9610ae7a5a6977dd623a888a2))
* **editor:** update editor.go call sites for word-boundary wrap ([ea0eca7](https://github.com/paradise-runner/toast/commit/ea0eca79830fca1656257f5bea10839238e0b244))
* **editor:** update screenToBuffer for wrap mode mouse clicks ([acea7da](https://github.com/paradise-runner/toast/commit/acea7da46b4efd02ca693d623b4b80852b8b12d3))
* **editor:** update View to render wrapped lines for markdown ([d087c3d](https://github.com/paradise-runner/toast/commit/d087c3df34494fa2596b645b764fc9b1f4114bc5))
* **editor:** update wrap helpers to use word-boundary chunks ([c3ffc4f](https://github.com/paradise-runner/toast/commit/c3ffc4fa42d091ea1e101c0236f4fb059dbc0734))
* expand tree file icon coverage ([1b74437](https://github.com/paradise-runner/toast/commit/1b74437eed849a70531f93ba56cd8e77f75c4c80))
* open non-existent file as new buffer, create on save ([3414c50](https://github.com/paradise-runner/toast/commit/3414c508a292fa37425930d7ac6f9556008c1aa7))
* preserve unsaved changes across tab switches and defer save prompts to quit ([220e52f](https://github.com/paradise-runner/toast/commit/220e52f801f2c8efbc583b9041a2746e317d4f5b))
* sidebar drag/drop polish (copy fallback, auto-expand, auto-scroll, ghost) ([#73](https://github.com/paradise-runner/toast/issues/73)) ([d5f6eb8](https://github.com/paradise-runner/toast/commit/d5f6eb86bb11f3a48c9eca027875e86751877364))
* standalone macOS app bundle (libghostty) + release packaging + README/landing updates ([8ba8dce](https://github.com/paradise-runner/toast/commit/8ba8dcebe3256e439bdb91a14b098cdcfb0d55cb))
* zed theme migration and 32 new builtin themes ([#88](https://github.com/paradise-runner/toast/issues/88)) ([0e9aa03](https://github.com/paradise-runner/toast/commit/0e9aa03549c5e79d6bc9394e0389b7b051415d57))


### Fixes

* auto-save writes verbatim (no whitespace stripping) + grapheme-cluster display widths ([eee0129](https://github.com/paradise-runner/toast/commit/eee012913705833f37f57fd82671527c540d71be))
* bundle script falls back to an older macOS SDK when zig 0.15.2 cannot parse the default SDK ([2e0dcf8](https://github.com/paradise-runner/toast/commit/2e0dcf81e44852d4948c51a852a0f61da4a69b75))
* **ci:** pick an older Xcode SDK for zig, build the app bundle on arm64 only ([26fc9e6](https://github.com/paradise-runner/toast/commit/26fc9e6b1d34cb76abc459730764fb39c7b1dcd1))
* clamp cursor after save-time buffer replacement to prevent panic when editing multi-line text ([d103a6e](https://github.com/paradise-runner/toast/commit/d103a6e2bb3a0599b398d29e7eb9dc45615df436))
* compute bundle version outside the subshell so Info.plist sees it ([4cfd9d6](https://github.com/paradise-runner/toast/commit/4cfd9d67a90ce538a39a2b1fcfd1068504f84b05))
* **editor:** recover horizontal viewport and add wheel scrolling ([#69](https://github.com/paradise-runner/toast/issues/69)) ([7a24435](https://github.com/paradise-runner/toast/commit/7a24435a8b1e26c934ca029e140019f7dfd8fe39))
* keep cursor in place on watcher-triggered reloads ([#83](https://github.com/paradise-runner/toast/issues/83)) ([#84](https://github.com/paradise-runner/toast/issues/84)) ([b018385](https://github.com/paradise-runner/toast/commit/b018385cae9c583b1399f44497f090b8c78fe3d0))
* keep mouse selection drag tracking past viewport edges ([#77](https://github.com/paradise-runner/toast/issues/77)) ([cc05cc3](https://github.com/paradise-runner/toast/commit/cc05cc317be0223c8ef0e3ae737904c0142b0804))
* keep sidebar context menu clickable at the window edge ([#80](https://github.com/paradise-runner/toast/issues/80)) ([6426548](https://github.com/paradise-runner/toast/commit/64265486d6e9922fa2f977b73e854e81d63ebab8)), closes [#60](https://github.com/paradise-runner/toast/issues/60)
* prevent last buffer line from repeating in blank screen rows (wrap mode) ([abb4f9a](https://github.com/paradise-runner/toast/commit/abb4f9ac961b24c52269fac91c8935ca94f7d042))
* set pendingBufferID in binary file tests so messages aren't dropped ([f2cc80c](https://github.com/paradise-runner/toast/commit/f2cc80c131f949fc57a681fe1e750f2d044e53f7))


### Docs

* add markdown line wrap design spec ([c1b6941](https://github.com/paradise-runner/toast/commit/c1b6941a46eebebd09b2ec0ce19efd7748a90709))
* add markdown line wrap implementation plan ([85594ba](https://github.com/paradise-runner/toast/commit/85594bab487a1072369dd2ef48c5a6aeeef0559f))
* add word-boundary wrap design spec ([d9e9715](https://github.com/paradise-runner/toast/commit/d9e9715d54dffbba984de99a2ad127474247e111))
* add word-boundary wrap implementation plan ([69b7831](https://github.com/paradise-runner/toast/commit/69b78311990702d50e04fe7af70bcc16082ff6ff))

## [0.13.0](https://github.com/paradise-runner/toast/compare/v0.12.2...v0.13.0) (2026-10-26)

The theme release: 32 new builtin themes converted from the most popular Zed
theme extensions, the Zed theme importer they were generated with (#3, #17), and
a theme picker that scrolls instead of growing past the terminal.

### Themes
- 32 new builtins alongside `toast-dark`, `toast-light` and `system`: Catppuccin (`catppuccin-latte`, `-frappe`, `-macchiato`, `-mocha`), Tokyo Night (`tokyo-night`, `-storm`, `-moon`, `-light`), GitHub (`github-light`, `-light-high-contrast`, `-dark`, `-dark-high-contrast`, `-dark-dimmed`), Dracula (`dracula`, `-solid`, `-light-alucard`), Nord (`nord-dark`, `-darker`, `-light`), Kanagawa (`kanagawa`, `-wave`, `-dragon`, `-lotus`), Nightfox (`nightfox`, `dayfox`, `dawnfox`, `duskfox`, `nordfox`, `terafox`, `carbonfox`) and macOS Classic (`macos-classic-light`, `-dark`).
- Chosen by Zed extension download counts (Catppuccin 1.2M, macOS Classic 507k, Tokyo Night 463k, GitHub 294k, Nightfox 279k, Dracula 267k, Nord 153k, Kanagawa 109k). Colorblind/tritanopia, "No Italics" and "blurred" variants are left out, and the Nightfox ` - opaque` suffix is dropped.
- Every upstream theme is MIT licensed; repository, commit and the verbatim license text are recorded in `internal/theme/builtin/SOURCES.md` and `internal/theme/builtin/licenses/`, together with the regeneration steps.

### Theme importer
- `toast migrate-theme zed <file> [out-dir]` converts a Zed theme file. One Zed file usually holds a whole family, so every variant becomes its own Toast theme; an optional output directory keeps the source tree tidy when generating builtins.
- Translucent Zed surfaces (`#rrggbbaa`) are composited over the editor background instead of dropping the alpha channel, so a theme's `element.selected` stays the subtle highlight its author intended.
- Token names Zed spells differently resolve through fallback chains (`namespace` → Toast's `module`, `variable.special`/`preproc` → `builtin`), with `font_weight: 700` and `font_style: "italic"` carrying over as bold and italic.
- Search-highlight foregrounds are picked by WCAG contrast, so highlights that are darker than the editor background (a translucent tint in Nightfox and Kanagawa) stay readable instead of turning into invisible text.
- `toKebab` folds accents, so "Catppuccin Frappé" becomes `catppuccin-frappe` rather than `catppuccin-frapp`.
- Missing-token errors now name the source format ("VSCode" or "Zed"), sharing one report between both importers.

### Theme picker
- The list scrolls in a 12-row window with clickable `▲`/`▼` arrows, `pgup`/`pgdn`/`home`/`end`, and click mapping through the scroll offset. Previously the dialog grew with the theme count and clipped the action row on a 24-row terminal.
- The arrows scroll the viewport only: they never move the selection or preview a theme. The next selection key anchors the highlight onto the visible page (down to the top row, up to the bottom row) instead of bouncing the list back to the off-screen selection.
- The dialog's hit box now matches its rendered size — clicks used to arrive one column to the right of the drawn box — so the one-column arrows are reachable and confirm/cancel/folder hit exactly where they are drawn.
- Rows are measured with `lipgloss.Width` against the real inner width (the border is part of the configured width), and over-long custom theme names are ellipsised instead of wrapping the dialog.
- `Discover` sorts a family's base theme ahead of its variants (`dracula` before `dracula-solid`) and pins `system` first.

### Fixes
* keep cursor in place on watcher-triggered reloads ([#83](https://github.com/paradise-runner/toast/issues/83)) ([#84](https://github.com/paradise-runner/toast/issues/84)) ([b018385](https://github.com/paradise-runner/toast/commit/b018385cae9c583b1399f44497f090b8c78fe3d0))

### Docs & misc
- README lists the new themes, points at `SOURCES.md`, and documents both importers plus the default output directory.
- `TestAllBuiltinThemesLoad` fails if a shipped theme is missing any UI, syntax or git token the UI needs; `TestDimensionsMatchRender` and an end-to-end arrow-click test keep the picker's drawn geometry and hit-testing in sync.

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
