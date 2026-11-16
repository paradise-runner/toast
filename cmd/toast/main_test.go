package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGitRoot_FindsRepoRoot(t *testing.T) {
	// Create a temp directory tree: root/.git, root/sub/file.go
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	filePath := filepath.Join(subDir, "file.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, ok := findGitRoot(filePath)
	if !ok {
		t.Fatal("expected findGitRoot to return ok=true, got false")
	}
	if got != dir {
		t.Fatalf("expected root %q, got %q", dir, got)
	}
}

func TestFindGitRoot_NoRepo_ReturnsFalse(t *testing.T) {
	// Create a temp directory with no .git anywhere in the tree
	dir := t.TempDir()
	filePath := filepath.Join(dir, "orphan.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, ok := findGitRoot(filePath)
	if ok {
		t.Fatal("expected findGitRoot to return ok=false for path with no .git ancestor")
	}
}

func TestFindGitRoot_GitFileInRoot(t *testing.T) {
	// .git can be a file (git worktrees), not just a directory
	dir := t.TempDir()
	gitFile := filepath.Join(dir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: ../.git/worktrees/foo"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, ok := findGitRoot(filePath)
	if !ok {
		t.Fatal("expected findGitRoot to return ok=true for worktree .git file")
	}
	if got != dir {
		t.Fatalf("expected root %q, got %q", dir, got)
	}
}

func TestSplitLaunchArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantPos       []string
		wantNoRestore bool
	}{
		{name: "no args"},
		{name: "path only", args: []string{"path/to/dir"}, wantPos: []string{"path/to/dir"}},
		{name: "flag before path", args: []string{"--no-restore", "repo"}, wantPos: []string{"repo"}, wantNoRestore: true},
		{name: "flag after path", args: []string{"repo", "--no-restore"}, wantPos: []string{"repo"}, wantNoRestore: true},
		{name: "flag alone", args: []string{"--no-restore"}, wantNoRestore: true},
		{name: "version with flag", args: []string{"--no-restore", "-v"}, wantPos: []string{"-v"}, wantNoRestore: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos, noRestore := splitLaunchArgs(tc.args)
			if noRestore != tc.wantNoRestore {
				t.Fatalf("noRestore = %v, want %v", noRestore, tc.wantNoRestore)
			}
			if len(pos) != len(tc.wantPos) {
				t.Fatalf("positional = %v, want %v", pos, tc.wantPos)
			}
			for i, arg := range pos {
				if arg != tc.wantPos[i] {
					t.Fatalf("positional[%d] = %q, want %q", i, arg, tc.wantPos[i])
				}
			}
		})
	}
}
