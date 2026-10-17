//go:build embed_rg

package main

import (
	"os"
	"testing"
)

func TestBundledRipgrep(t *testing.T) {
	// Keep the extraction inside a temp dir rather than the real user cache.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	path := bundledRipgrep()
	if path == "" {
		t.Fatal("bundledRipgrep returned an empty path")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat extracted ripgrep: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("extracted ripgrep is empty")
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("extracted ripgrep is not executable: %v", info.Mode())
	}
}
