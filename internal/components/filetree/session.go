package filetree

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ExpandedPaths lists the expanded directories below the roots, sorted. A root
// row itself is implicit: every root is expanded on restore, so only directories
// below it are stored. In a multi-root workspace each entry is prefixed with the
// root's index ("1/pkg") so two roots with the same layout cannot collide; a
// single-root tree keeps the plain relative path.
func (m Model) ExpandedPaths() []string {
	var paths []string
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		for _, child := range n.Children {
			if !child.IsDir || !child.Expanded {
				continue
			}
			if rel, ok := m.relPath(child.Path); ok {
				paths = append(paths, rel)
			}
			walk(child)
		}
	}
	for _, root := range m.roots {
		walk(root)
	}
	sort.Strings(paths)
	return paths
}

// SelectedPath returns the selected row as a path relative to its root, or ""
// when the selection is a root row itself or the tree is empty.
func (m Model) SelectedPath() string {
	if m.cursor < 0 || m.cursor >= len(m.flat) {
		return ""
	}
	rel, ok := m.relPath(m.flat[m.cursor].Path)
	if !ok {
		return ""
	}
	return rel
}

// Offset returns the index of the first visible row.
func (m Model) Offset() int { return m.offset }

// RestoreState re-applies a saved tree state: it expands the given directories
// (loading each one on the way down), selects the saved row, and restores the
// scroll offset. Directories that no longer exist are dropped silently.
func (m *Model) RestoreState(expanded []string, selected string, offset int) {
	if len(m.roots) == 0 {
		return
	}
	for _, root := range m.roots {
		root.Expanded = true
	}
	for _, key := range expanded {
		target, ok := m.pathForKey(key)
		if !ok {
			continue
		}
		m.expandDirChain(target)
	}
	if m.gitStatuses != nil {
		m.applyGitStatus(m.gitStatuses)
	}
	m.rebuildFlat()

	m.cursor = 0
	if selected != "" {
		if target, ok := m.pathForKey(selected); ok {
			for i, node := range m.flat {
				if node.Path == target {
					m.cursor = i
					break
				}
			}
		}
	}
	m.offset = offset
	m.clampOffset()
}

// expandDirChain loads and expands target and every directory between it and
// its root, so a nested saved expansion can be rebuilt from a fresh tree. It
// reports false when a path component is missing or is no longer a directory.
func (m *Model) expandDirChain(target string) bool {
	root, _, ok := m.rootIndexFor(target)
	if !ok {
		return false
	}
	// Check the destination before expanding anything: a stale entry must not
	// leave its ancestors half-expanded.
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return false
	}
	rel, err := filepath.Rel(root.Path, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	node := root
	if !node.Loaded() {
		if err := node.LoadChildren(m.cfg.IgnoredPatterns); err != nil {
			return false
		}
	}
	node.Expanded = true
	if rel == "." {
		return true
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		var next *TreeNode
		for _, child := range node.Children {
			if child.Name == part {
				next = child
				break
			}
		}
		if next == nil || !next.IsDir {
			return false
		}
		if !next.Loaded() {
			if err := next.LoadChildren(m.cfg.IgnoredPatterns); err != nil {
				return false
			}
		}
		next.Expanded = true
		node = next
	}
	return true
}

// relPath renders path relative to its owning root, reporting false for a root
// itself and for paths outside every root. Multi-root workspaces prefix the
// entry with the root index so the key is unambiguous (see ExpandedPaths).
func (m Model) relPath(path string) (string, bool) {
	root, index, ok := m.rootIndexFor(path)
	if !ok {
		return "", false
	}
	rel, err := filepath.Rel(root.Path, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if len(m.roots) > 1 {
		return filepath.Join(strconv.Itoa(index), rel), true
	}
	return rel, true
}

// pathForKey is the inverse of relPath. It rejects keys that would escape the
// root, so a corrupt session cannot make Toast expand or select outside the
// workspace.
func (m Model) pathForKey(key string) (string, bool) {
	if key == "" || len(m.roots) == 0 {
		return "", false
	}
	root := m.roots[0]
	rel := key
	if len(m.roots) > 1 {
		indexPart, rest, ok := strings.Cut(key, string(filepath.Separator))
		if !ok {
			return "", false
		}
		index, err := strconv.Atoi(indexPart)
		if err != nil || index < 0 || index >= len(m.roots) {
			return "", false
		}
		root = m.roots[index]
		rel = rest
	}
	if rel == "" || rel == "." || filepath.IsAbs(rel) {
		return "", false
	}
	path := filepath.Join(root.Path, rel)
	if !pathInside(root.Path, path) || path == root.Path {
		return "", false
	}
	return path, true
}

// clampOffset keeps the scroll offset inside the flat list and, once the tree
// has a height, keeps the selected row on screen.
func (m *Model) clampOffset() {
	maxOffset := len(m.flat) - 1
	if m.height > 0 {
		if v := len(m.flat) - m.height; v < maxOffset {
			maxOffset = v
		}
	}
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.height <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}
