// Package session persists per-root editor session state so that a relaunch in
// the same folder starts where the last one left off: open tabs with their
// cursors, viewport and folds, the expanded file tree, and the sidebar
// geometry.
//
// A session is keyed on the resolved workspace root rather than on its
// basename, so two checkouts that happen to be called "api" never share state,
// and a symlinked path and the directory it points at resolve to one session.
package session

import (
	"path/filepath"
	"strings"
)

// Version is the on-disk schema version. A file written by a different version
// is ignored rather than migrated; the next save rewrites it.
const Version = 1

// State is everything Toast restores for one workspace root.
type State struct {
	Version   int          `json:"version"`
	Root      string       `json:"root"`
	SavedAt   int64        `json:"saved_at"` // unix seconds
	Tabs      []Tab        `json:"tabs"`
	ActiveTab int          `json:"active_tab"`
	Tree      TreeState    `json:"tree"`
	Sidebar   SidebarState `json:"sidebar"`
}

// Tab is one restored buffer: the file plus the editing state that belongs to
// it. Path is relative to the session root when the file lives inside it, and
// absolute otherwise.
type Tab struct {
	Path         string `json:"path"`
	CursorLine   int    `json:"cursor_line"`
	CursorCol    int    `json:"cursor_col"`
	ViewportTop  int    `json:"viewport_top"`
	ViewportLeft int    `json:"viewport_left"`
	// Folds holds the zero-based header lines of collapsed regions. Only the
	// start is stored because the end is re-derived from the file on restore,
	// so an edit inside a folded block cannot resurrect a stale hidden range.
	Folds []int `json:"folds,omitempty"`
	// Preview is the per-tab Markdown preview flag.
	Preview bool `json:"preview,omitempty"`
}

// TreeState is the sidebar file tree: which directories below the root were
// expanded, the selected row, and the scroll offset.
type TreeState struct {
	Expanded []string `json:"expanded,omitempty"`
	Selected string   `json:"selected,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// SidebarState is the sidebar geometry. It overrides the config default while a
// session exists, so a resize survives the process.
type SidebarState struct {
	Visible bool `json:"visible"`
	Width   int  `json:"width"`
}

// RelPath returns path relative to root when it lies inside root, and path
// unchanged otherwise. Sessions store relative paths so a checkout keeps its
// tabs after the directory above it is renamed or moved.
func RelPath(root, path string) string {
	if root == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// AbsPath is the inverse of RelPath: it resolves a stored path against the
// session root. Absolute paths are returned unchanged.
func AbsPath(root, path string) string {
	if root == "" || path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// InsideRoot reports whether path is root itself or lies below it.
func InsideRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
