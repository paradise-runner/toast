# Release Notes

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
