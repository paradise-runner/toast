package filetree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExpandedPaths lists the expanded directories below the root as paths relative
// to the root, sorted. The root itself is implicit: it is always expanded and is
// represented by the absence of an entry.
func (m Model) ExpandedPaths() []string {
	if m.root == nil {
		return nil
	}
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
	walk(m.root)
	sort.Strings(paths)
	return paths
}

// SelectedPath returns the selected row as a path relative to the root, or ""
// when the selection is the root row itself or the tree is empty.
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
	if m.root == nil {
		return
	}
	m.root.Expanded = true
	for _, rel := range expanded {
		m.expandDirChain(filepath.Join(m.rootDir, rel))
	}
	if m.gitStatuses != nil {
		m.applyGitStatus(m.gitStatuses)
	}
	m.rebuildFlat()

	m.cursor = 0
	if selected != "" {
		target := filepath.Join(m.rootDir, selected)
		for i, node := range m.flat {
			if node.Path == target {
				m.cursor = i
				break
			}
		}
	}
	m.offset = offset
	m.clampOffset()
}

// expandDirChain loads and expands target and every directory between it and
// the root, so a nested saved expansion can be rebuilt from a fresh tree. It
// reports false when a path component is missing or is no longer a directory.
func (m *Model) expandDirChain(target string) bool {
	rel, ok := m.relPath(target)
	if !ok {
		return false
	}
	// Check the destination before expanding anything: a stale entry must not
	// leave its ancestors half-expanded.
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return false
	}
	node := m.root
	if !node.Loaded() {
		if err := node.LoadChildren(m.cfg.IgnoredPatterns); err != nil {
			return false
		}
	}
	node.Expanded = true
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

// relPath renders path relative to the tree root, reporting false for the root
// itself and for paths outside it.
func (m Model) relPath(path string) (string, bool) {
	if m.rootDir == "" {
		return "", false
	}
	rel, err := filepath.Rel(m.rootDir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
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
