// Package workspace describes the directories one Toast window is browsing.
//
// A workspace is either a single directory (a plain `toast .` launch) or the
// set of folders declared by a VS Code `.code-workspace` file, so one window
// can span a frontend, a backend and a shared library that live in separate
// repositories. Every component that used to take "the" root takes the root
// list instead and degrades to one entry for a single-folder launch.
package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Root is one directory in a workspace. Name is the label the file tree shows
// for it, which a `.code-workspace` file can set explicitly.
type Root struct {
	Name string
	Path string
}

// Workspace is the ordered set of roots one Toast window browses. File is the
// absolute path of the `.code-workspace` file that defined it, or "" when the
// workspace is a single directory launch.
type Workspace struct {
	File  string
	Roots []Root
}

// Single builds a one-root workspace for a plain directory launch.
func Single(dir string) *Workspace {
	return &Workspace{
		Roots: []Root{{
			Name: filepath.Base(dir),
			Path: filepath.Clean(dir),
		}},
	}
}

// IsWorkspaceFile reports whether path names a VS Code workspace file. The
// check is by extension only: it must work for a path that does not exist yet,
// and Load reports a missing file with a clear error anyway.
func IsWorkspaceFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".code-workspace")
}

// Load parses a `.code-workspace` file. Relative folder paths resolve against
// the workspace file's own directory, comments and trailing commas are
// tolerated the way VS Code tolerates them, and unknown keys (settings, launch,
// tasks) are ignored. Folders that are missing from disk are kept so a stale
// network mount does not silently change what the workspace means; the file
// tree reports them as empty and unreadable.
func Load(path string) (*Workspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving workspace file: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("reading workspace %s: %w", abs, err)
	}

	var parsed struct {
		Folders []struct {
			Path string `json:"path"`
			Name string `json:"name"`
		} `json:"folders"`
	}
	cleaned := stripJSONC(data)
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		return nil, fmt.Errorf("parsing workspace %s: %w", abs, err)
	}
	if len(parsed.Folders) == 0 {
		return nil, fmt.Errorf("workspace %s declares no folders", abs)
	}

	baseDir := filepath.Dir(abs)
	seen := make(map[string]bool, len(parsed.Folders))
	roots := make([]Root, 0, len(parsed.Folders))
	for i, folder := range parsed.Folders {
		folderPath := strings.TrimSpace(folder.Path)
		if folderPath == "" {
			continue
		}
		if !filepath.IsAbs(folderPath) {
			folderPath = filepath.Join(baseDir, folderPath)
		}
		folderPath = filepath.Clean(folderPath)
		if seen[folderPath] {
			continue
		}
		seen[folderPath] = true

		name := strings.TrimSpace(folder.Name)
		if name == "" {
			name = filepath.Base(folderPath)
		}
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = fmt.Sprintf("folder %d", i+1)
		}
		roots = append(roots, Root{Name: name, Path: folderPath})
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("workspace %s declares no usable folders", abs)
	}
	return &Workspace{File: abs, Roots: roots}, nil
}

// Name is the label for this workspace: the workspace file name without its
// extension, or the directory name for a single-folder launch.
func (w *Workspace) Name() string {
	if w == nil || len(w.Roots) == 0 {
		return ""
	}
	if w.File != "" {
		base := filepath.Base(w.File)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return w.Roots[0].Name
}

// Multi reports whether the workspace has more than one root. Components use it
// to decide whether a path needs a root label to be unambiguous.
func (w *Workspace) Multi() bool { return w != nil && len(w.Roots) > 1 }

// Dir is the directory that holds the workspace definition. It is the base for
// session state: one `.code-workspace` file gets one session regardless of
// which roots are open, and a single-folder launch keeps its directory.
func (w *Workspace) Dir() string {
	if w == nil || len(w.Roots) == 0 {
		return ""
	}
	if w.File != "" {
		return filepath.Dir(w.File)
	}
	return w.Roots[0].Path
}

// SessionKey is the stable identity a session is keyed on. A workspace file is
// its own identity, so moving the roots around inside it does not orphan the
// saved state.
func (w *Workspace) SessionKey() string {
	if w == nil || len(w.Roots) == 0 {
		return ""
	}
	if w.File != "" {
		return w.File
	}
	return w.Roots[0].Path
}

// Paths returns just the root directories, in declaration order.
func (w *Workspace) Paths() []string {
	if w == nil {
		return nil
	}
	paths := make([]string, 0, len(w.Roots))
	for _, root := range w.Roots {
		paths = append(paths, root.Path)
	}
	return paths
}

// RootFor returns the root that owns path. When roots nest, the deepest one
// wins, so a workspace that also lists a subfolder attributes files in it to the
// subfolder rather than to its parent.
func (w *Workspace) RootFor(path string) (Root, bool) {
	if w == nil || path == "" {
		return Root{}, false
	}
	path = filepath.Clean(path)
	var best Root
	found := false
	for _, root := range w.Roots {
		if !inside(root.Path, path) {
			continue
		}
		if !found || len(root.Path) > len(best.Path) {
			best = root
			found = true
		}
	}
	return best, found
}

// Relative renders path relative to its owning root for display. In a
// multi-root workspace the root name is prepended ("frontend/src/app.ts") so two
// roots cannot produce indistinguishable rows; a single-root workspace keeps
// the bare relative path.
func (w *Workspace) Relative(path string) string {
	root, ok := w.RootFor(path)
	if !ok {
		return path
	}
	rel, err := filepath.Rel(root.Path, path)
	if err != nil {
		return path
	}
	if w.Multi() {
		return filepath.Join(root.Name, rel)
	}
	return rel
}

// inside reports whether path is dir itself or lies below it.
func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// stripJSONC removes the parts of a VS Code JSON-with-comments file that
// encoding/json rejects: line and block comments, and trailing commas. String
// contents and escapes are preserved verbatim, so a URL in a "background" image
// setting survives. A UTF-8 BOM and the leading "//" header comment VS Code
// writes are both handled.
func stripJSONC(data []byte) []byte {
	data = trimBOM(data)
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			// Keep the newline that ended the comment.
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				if data[i] == '\n' {
					out = append(out, '\n')
				}
				i++
			}
			// Skip the closing "*/" (i+1 is '/', the loop advances past it).
			i++
		case c == ',':
			// A comma followed only by whitespace and then '}' or ']' is a
			// trailing comma; drop it.
			j := i + 1
			for j < len(data) && isSpace(data[j]) {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// trimBOM drops a leading UTF-8 byte-order mark, which some editors write and
// encoding/json rejects.
func trimBOM(data []byte) []byte {
	const bom = "\xef\xbb\xbf"
	if len(data) >= len(bom) && string(data[:len(bom)]) == bom {
		return data[len(bom):]
	}
	return data
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
