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
