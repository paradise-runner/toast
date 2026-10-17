package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// ── Cross-device move fallback ────────────────────────────────────────────────

func TestMovePathWith_CrossDeviceCopiesThenRemoves(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	mustWrite(t, src)
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	renameCalled := false
	err := movePathWith(src, dst, func(string, string) error {
		renameCalled = true
		return syscall.EXDEV
	})
	if err != nil {
		t.Fatalf("movePathWith: %v", err)
	}
	if !renameCalled {
		t.Fatal("expected the rename function to be tried first")
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("destination content = %q, want %q", data, "hello")
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be gone, stat err = %v", err)
	}
}

func TestMovePathWith_NonEXDEVErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	mustWrite(t, src)

	wantErr := os.ErrPermission
	err := movePathWith(src, dst, func(string, string) error { return wantErr })
	if err != wantErr {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should be untouched on a non-EXDEV failure: %v", err)
	}
}

func TestCopyPath_DirectoryTree(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "b.txt"), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")

	if err := copyPath(src, dst); err != nil {
		t.Fatalf("copyPath: %v", err)
	}
	for _, tc := range []struct{ rel, want string }{
		{"a.txt", "a"},
		{filepath.Join("nested", "b.txt"), "b"},
	} {
		got, err := os.ReadFile(filepath.Join(dst, tc.rel))
		if err != nil {
			t.Fatalf("read %s: %v", tc.rel, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s = %q, want %q", tc.rel, got, tc.want)
		}
	}
	info, err := os.Stat(filepath.Join(dst, "nested", "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("nested file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestCopyPath_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	mustWrite(t, target)
	src := filepath.Join(dir, "link")
	if err := os.Symlink(target, src); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	dst := filepath.Join(dir, "link-copy")

	if err := copyPath(src, dst); err != nil {
		t.Fatalf("copyPath: %v", err)
	}
	got, err := os.Readlink(dst)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if got != target {
		t.Errorf("symlink target = %q, want %q", got, target)
	}
}

// ── Hover-to-expand ───────────────────────────────────────────────────────────

func TestDrag_HoverExpandsCollapsedDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(subDir, "inner.txt"))
	mustWrite(t, filepath.Join(dir, "a.txt"))

	m := newTestModel(t, dir)
	// flat: [root, sub, a.txt] — sub starts collapsed.

	m, _ = m.Update(leftPress(2, 2))  // press a.txt
	m, _ = m.Update(leftMotion(2, 1)) // hover collapsed sub
	if m.dragExpandPending != subDir {
		t.Fatalf("dragExpandPending = %q, want %q", m.dragExpandPending, subDir)
	}

	m, _ = m.Update(dragExpandTickMsg{path: subDir})

	if !m.flat[1].Expanded {
		t.Error("expected sub to be expanded after the dwell")
	}
	if m.dragExpandPending != "" {
		t.Errorf("dragExpandPending = %q, want empty", m.dragExpandPending)
	}
	// The expanded child is now visible in the flat list.
	var found bool
	for _, n := range m.flat {
		if n.Path == filepath.Join(subDir, "inner.txt") {
			found = true
		}
	}
	if !found {
		t.Error("expanded directory's child not present in flat list")
	}
	// Expanding inserted a row below the target, shifting the dragged file's
	// index; the drag must still point at the same node.
	if m.drag.fromIdx < 0 || m.drag.fromIdx >= len(m.flat) || m.flat[m.drag.fromIdx] != m.drag.from {
		t.Errorf("drag.fromIdx = %d no longer points at the dragged node", m.drag.fromIdx)
	}
}

func TestDrag_HoverExpandStaleTickIgnored(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "a.txt"))
	m := newTestModel(t, dir)

	m, _ = m.Update(leftPress(2, 2))
	m, _ = m.Update(leftMotion(2, 1))
	// A timer armed for a different directory must not expand anything.
	m, _ = m.Update(dragExpandTickMsg{path: filepath.Join(dir, "other")})
	if m.flat[1].Expanded {
		t.Error("stale expand timer must not expand the hovered directory")
	}
}

// ── Edge auto-scroll ──────────────────────────────────────────────────────────

func TestDrag_EdgeScrollsViewport(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		mustWrite(t, filepath.Join(dir, "file"+string(rune('a'+i))+".txt"))
	}
	m := newTestModel(t, dir) // height 10, 21 rows total

	m, _ = m.Update(leftPress(2, 1))
	m, _ = m.Update(leftMotion(2, m.height)) // past the bottom edge
	if m.drag.edge != 1 {
		t.Fatalf("drag.edge = %d, want 1", m.drag.edge)
	}
	if !m.dragScrolling {
		t.Fatal("expected the auto-scroll loop to be running")
	}

	start := m.offset
	m, _ = m.Update(dragScrollTickMsg{})
	m, _ = m.Update(dragScrollTickMsg{})
	if m.offset <= start {
		t.Errorf("offset did not advance: start=%d now=%d", start, m.offset)
	}
}

func TestDrag_EdgeScrollStopsWhenInside(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		mustWrite(t, filepath.Join(dir, "file"+string(rune('a'+i))+".txt"))
	}
	m := newTestModel(t, dir)

	m, _ = m.Update(leftPress(2, 1))
	m, _ = m.Update(leftMotion(2, m.height))
	m, _ = m.Update(dragScrollTickMsg{})
	// Pointer returns inside the viewport.
	m, _ = m.Update(leftMotion(2, 1))
	m, _ = m.Update(dragScrollTickMsg{})
	if m.dragScrolling {
		t.Error("auto-scroll loop should have stopped once the pointer was inside")
	}
}

// ── Drag ghost label ──────────────────────────────────────────────────────────

func TestDragOverlay_ShowsSourceAndDestination(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "a.txt"))
	m := newTestModel(t, dir)

	if _, _, _, ok := m.DragOverlay(); ok {
		t.Fatal("overlay must be hidden before a drag starts")
	}

	m, _ = m.Update(leftPress(2, 2))
	m, _ = m.Update(leftMotion(2, 1))

	rendered, x, y, ok := m.DragOverlay()
	if !ok {
		t.Fatal("expected the drag overlay to be visible")
	}
	stripped := stripANSI(rendered)
	if !strings.Contains(stripped, "a.txt") {
		t.Errorf("overlay %q missing source name", stripped)
	}
	if !strings.Contains(stripped, "→") || !strings.Contains(stripped, "sub/") {
		t.Errorf("overlay %q missing destination", stripped)
	}
	if x <= 2 || y <= 1 {
		t.Errorf("overlay anchor = (%d,%d), want offset from the pointer", x, y)
	}

	m, _ = m.Update(leftRelease(2, 1))
	if _, _, _, ok := m.DragOverlay(); ok {
		t.Error("overlay must disappear after the drop")
	}
}

// stripANSI removes escape sequences so tests can assert on visible text.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == 0x1b:
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
