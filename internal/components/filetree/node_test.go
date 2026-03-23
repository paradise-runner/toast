package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bubbletea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/theme"
)

func TestLoadChildren(t *testing.T) {
	dir := t.TempDir()

	// Create 2 files and 1 subdir
	if err := os.WriteFile(filepath.Join(dir, "file1.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	node := &TreeNode{Path: dir, IsDir: true}
	if err := node.LoadChildren(nil); err != nil {
		t.Fatalf("LoadChildren error: %v", err)
	}

	if len(node.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(node.Children))
	}
}

func TestIgnoredPatternsExcluded(t *testing.T) {
	dir := t.TempDir()

	// Create main.go, .git/, node_modules/
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}

	node := &TreeNode{Path: dir, IsDir: true}
	if err := node.LoadChildren([]string{".git", "node_modules"}); err != nil {
		t.Fatalf("LoadChildren error: %v", err)
	}

	for _, child := range node.Children {
		if child.Name == ".git" || child.Name == "node_modules" {
			t.Errorf("expected %q to be excluded but it was present", child.Name)
		}
	}

	if len(node.Children) != 1 {
		t.Errorf("expected 1 child (main.go), got %d", len(node.Children))
	}
}

func TestDirsBeforeFiles(t *testing.T) {
	dir := t.TempDir()

	// Create z_file.go and a_dir/
	if err := os.WriteFile(filepath.Join(dir, "z_file.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a_dir"), 0755); err != nil {
		t.Fatal(err)
	}

	node := &TreeNode{Path: dir, IsDir: true}
	if err := node.LoadChildren(nil); err != nil {
		t.Fatalf("LoadChildren error: %v", err)
	}

	if len(node.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(node.Children))
	}

	if !node.Children[0].IsDir {
		t.Errorf("expected first child to be a directory, got file %q", node.Children[0].Name)
	}
	if node.Children[1].IsDir {
		t.Errorf("expected second child to be a file, got directory %q", node.Children[1].Name)
	}
}

// ── Rendering regression tests ────────────────────────────────────────────────

func TestView_LightTheme_AllLinesHaveBackground(t *testing.T) {
	// Regression test: non-cursor file tree lines had trailing unstyled spaces
	// (the git-icon padding) that showed the terminal's default dark background
	// on light themes.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tm, err := theme.NewManager("toast-light", "../../../internal/theme/builtin")
	if err != nil {
		t.Fatalf("failed to load theme: %v", err)
	}
	cfg := config.Config{}
	m := New(tm, cfg, dir)
	m, _ = m.Update(bubbletea.WindowSizeMsg{Width: 30, Height: 5})

	view := m.View().Content
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if hasUnstyledSpacesAfterReset(line) {
			t.Errorf("FileTree light theme: line %d has unstyled spaces after ANSI reset.\nLine: %q", i, line)
		}
	}
}

func TestView_LightTheme_LinesAreFullWidth(t *testing.T) {
	// Regression test: non-cursor filetree rows were shorter than m.width,
	// causing lipgloss.JoinHorizontal to pad them with unstyled (dark) spaces.
	// Every rendered line must be exactly m.width visual columns wide.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.go"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tm, err := theme.NewManager("toast-light", "../../../internal/theme/builtin")
	if err != nil {
		t.Fatalf("failed to load theme: %v", err)
	}
	cfg := config.Config{}
	const width = 30
	m := New(tm, cfg, dir)
	m, _ = m.Update(bubbletea.WindowSizeMsg{Width: width, Height: 5})

	view := m.View().Content
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != width {
			t.Errorf("FileTree line %d: visual width = %d, want %d\nLine: %q", i, w, width, line)
		}
	}
}

// hasUnstyledSpacesAfterReset returns true if the ANSI string s contains any
// space that appears after an SGR reset (\x1b[m) without an intervening
// background-color SGR sequence (one containing "48;").
func hasUnstyledSpacesAfterReset(s string) bool {
	hasBG := true // assume background is active at start of a line
	i := 0
	for i < len(s) {
		if s[i] != '\x1b' {
			if s[i] == ' ' && !hasBG {
				return true
			}
			i++
			continue
		}
		if i+1 >= len(s) || s[i+1] != '[' {
			i++
			continue
		}
		end := i + 2
		for end < len(s) && s[end] != 'm' {
			end++
		}
		seq := s[i : end+1]
		if seq == "\x1b[m" {
			hasBG = false
		} else if strings.Contains(seq, "48;") {
			hasBG = true
		}
		i = end + 1
	}
	return false
}
