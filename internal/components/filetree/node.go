package filetree

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/yourusername/toast/internal/messages"
)

type TreeNode struct {
	Name      string
	Path      string
	IsDir     bool
	Children  []*TreeNode // nil = not loaded; [] = loaded, empty
	Expanded  bool
	GitStatus messages.GitStatus
}

func (n *TreeNode) Loaded() bool { return n.Children != nil }

func (n *TreeNode) LoadChildren(ignoredPatterns []string) error {
	entries, err := os.ReadDir(n.Path)
	if err != nil {
		return err
	}
	var children []*TreeNode
	for _, e := range entries {
		if isIgnored(e.Name(), ignoredPatterns) {
			continue
		}
		children = append(children, &TreeNode{
			Name:  e.Name(),
			Path:  filepath.Join(n.Path, e.Name()),
			IsDir: e.IsDir(),
		})
	}
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].IsDir != children[j].IsDir {
			return children[i].IsDir
		}
		return children[i].Name < children[j].Name
	})
	n.Children = children
	return nil
}

func isIgnored(name string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
	}
	return false
}
