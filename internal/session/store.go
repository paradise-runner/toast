package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store reads and writes session files in one directory.
type Store struct {
	dir string
}

// NewStore returns a store rooted at dir. An empty dir yields a store that
// never reads or writes, which keeps tests and one-off runs off the real state
// directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// DefaultDir returns the directory that holds session files. It lives beside
// the config file so one location covers both.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving session dir: %w", err)
	}
	return filepath.Join(home, ".config", "toast", "sessions"), nil
}

// ResolveRoot normalizes a workspace root so a symlinked path and the path it
// points at share one session: absolute first, then symlinks resolved. A root
// that cannot be resolved (for example because it was deleted) falls back to
// the absolute path.
func ResolveRoot(root string) (string, error) {
	if root == "" {
		return "", nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving root: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// Key is the stable file-name component for a resolved root. It hashes the
// whole path, so two checkouts named "api" do not collide and the file name
// stays a fixed length whatever the path looks like.
func Key(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:16]
}

// Path returns the session file for root, or "" when the store is disabled.
func (s *Store) Path(root string) string {
	if s == nil || s.dir == "" || root == "" {
		return ""
	}
	return filepath.Join(s.dir, Key(root)+".json")
}

// Load reads the session saved for root. A missing, unreadable, corrupt, or
// differently-versioned file yields an error and no state; callers treat that
// as "no session" and carry on with a fresh start.
func (s *Store) Load(root string) (*State, error) {
	path := s.Path(root)
	if path == "" {
		return nil, fmt.Errorf("session store is disabled")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing session %s: %w", path, err)
	}
	if st.Version != Version {
		return nil, fmt.Errorf("session %s has unsupported version %d", path, st.Version)
	}
	return &st, nil
}

// Save writes st atomically: a temp file inside the state directory is renamed
// over the target, so an interrupted or concurrent write never leaves a
// half-written session behind.
func (s *Store) Save(st State) error {
	path := s.Path(st.Root)
	if path == "" {
		return fmt.Errorf("session store is disabled")
	}
	st.Version = Version
	if st.SavedAt == 0 {
		st.SavedAt = time.Now().Unix()
	}
	if st.Tabs == nil {
		st.Tabs = []Tab{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling session: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("creating session temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		cleanup()
		return fmt.Errorf("writing session: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing session temp file: %w", err)
	}
	// CreateTemp opens 0600, but be explicit: sessions record which files the
	// user had open.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting session permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing session: %w", err)
	}
	return nil
}

// Prune removes session files whose last write is older than maxAge, so roots
// that are never opened again do not accumulate forever.
func (s *Store) Prune(maxAge time.Duration) error {
	if s == nil || s.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading session dir: %w", err)
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, entry.Name())); err != nil {
			return fmt.Errorf("pruning session: %w", err)
		}
	}
	return nil
}
