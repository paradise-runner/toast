package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// writeEmbeddedBinary extracts data to a per-user cache directory and returns
// the path to the executable. It returns "" when the binary cannot be written,
// so callers fall back to looking the tool up on PATH.
//
// The file name is derived from the content hash, so upgrading the embedded
// binary writes a new file and leaves the old one in place.
func writeEmbeddedBinary(data []byte, name string) string {
	if len(data) == 0 {
		return ""
	}

	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	sum := sha256.Sum256(data)
	fileName := fmt.Sprintf("%s-%s", name, hex.EncodeToString(sum[:8]))
	dir := filepath.Join(base, "toast", "bin")
	path := filepath.Join(dir, fileName)

	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() == int64(len(data)) {
		if cached, err := os.ReadFile(path); err == nil && sha256.Sum256(cached) == sum {
			return path
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	tmp, err := os.CreateTemp(dir, fileName+".tmp-*")
	if err != nil {
		return ""
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return ""
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return ""
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return ""
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return ""
	}
	return path
}
