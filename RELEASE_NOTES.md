# Release Notes

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
