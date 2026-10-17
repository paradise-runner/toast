package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEmbeddedBinaryEmpty(t *testing.T) {
	if got := writeEmbeddedBinary(nil, "rg"); got != "" {
		t.Fatalf("writeEmbeddedBinary(nil) = %q, want empty", got)
	}
}

func TestWriteEmbeddedBinary(t *testing.T) {
	// Keep the extraction inside a temp dir rather than the real user cache.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	data := []byte("#!/bin/sh\necho rg\n")
	path := writeEmbeddedBinary(data, "rg")
	if path == "" {
		t.Fatal("writeEmbeddedBinary returned an empty path")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("extracted content = %q, want %q", got, data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat extracted binary: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("extracted binary is not executable: %v", info.Mode())
	}

	// A second call should reuse the cached file.
	if again := writeEmbeddedBinary(data, "rg"); again != path {
		t.Fatalf("second call = %q, want cached %q", again, path)
	}
}

func TestWriteEmbeddedBinaryDifferentContent(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	first := writeEmbeddedBinary([]byte("one"), "rg")
	second := writeEmbeddedBinary([]byte("two"), "rg")
	if first == "" || second == "" {
		t.Fatalf("expected paths, got %q and %q", first, second)
	}
	if first == second {
		t.Fatalf("different content produced the same path %q", first)
	}
	if filepath.Dir(first) != filepath.Dir(second) {
		t.Fatalf("paths should share a directory: %q vs %q", first, second)
	}
}
