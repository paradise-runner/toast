package filetree

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/theme"
)

const testDialogW = 80
const testDialogH = 24

func newTestDeleteDialog(path string, isDir bool) *DeleteConfirmDialog {
	tm, _ := theme.NewManager("toast-dark", "")
	return newDeleteConfirmDialog(path, isDir, tm)
}

func TestDeleteDialog_Render_ContainsFilename(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	out := d.Render(testDialogW, testDialogH)
	if !strings.Contains(out, "foo.go") {
		t.Errorf("expected filename 'foo.go' in dialog render, got: %q", out)
	}
	if !strings.Contains(out, "This cannot be undone") {
		t.Error("expected warning text in dialog render")
	}
}

func TestDeleteDialog_Render_DirText(t *testing.T) {
	d := newTestDeleteDialog("/tmp/mypkg", true)
	out := d.Render(testDialogW, testDialogH)
	if !strings.Contains(out, "mypkg") {
		t.Errorf("expected dir name 'mypkg' in dialog render, got: %q", out)
	}
	if !strings.Contains(out, "all its contents") {
		t.Error("expected 'all its contents' in dir dialog render")
	}
}

func TestDeleteDialog_Render_CheckboxUncheckedByDefault(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	out := d.Render(testDialogW, testDialogH)
	if !strings.Contains(out, "[ ]") {
		t.Errorf("expected unchecked checkbox '[ ]' in render, got: %q", out)
	}
}

func TestDeleteDialog_Render_StoresGeometry(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.Render(testDialogW, testDialogH)
	if d.dialogW == 0 || d.dialogH == 0 {
		t.Error("expected Render to store non-zero dialogW/dialogH")
	}
	// originX should be centered
	expectedX := (testDialogW - d.dialogW) / 2
	if d.originX != expectedX {
		t.Errorf("expected originX=%d, got %d", expectedX, d.originX)
	}
}

func TestDeleteDialog_HandleKey_EscCancels(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	confirm, cancel, _ := d.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if confirm || !cancel {
		t.Errorf("expected cancel=true on Esc, got confirm=%v cancel=%v", confirm, cancel)
	}
}

func TestDeleteDialog_HandleKey_EnterOnConfirmConfirms(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	// focus starts at 0 = Confirm
	confirm, cancel, _ := d.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !confirm || cancel {
		t.Errorf("expected confirm=true on Enter with focus=0, got confirm=%v cancel=%v", confirm, cancel)
	}
}

func TestDeleteDialog_HandleKey_EnterOnCancelCancels(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.focus = 1 // Cancel
	confirm, cancel, _ := d.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if confirm || !cancel {
		t.Errorf("expected cancel=true on Enter with focus=1, got confirm=%v cancel=%v", confirm, cancel)
	}
}

func TestDeleteDialog_HandleKey_SpaceTogglesCheckbox(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.focus = 0 // Confirm — space still toggles
	_, _, toggle := d.HandleKey(tea.KeyPressMsg{Code: ' '})
	if !toggle {
		t.Error("expected space to toggle checkbox regardless of focus")
	}
}

func TestDeleteDialog_HandleKey_TabAdvancesFocus(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	// 0 → 1
	d.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if d.focus != 1 {
		t.Errorf("expected focus=1 after Tab, got %d", d.focus)
	}
	// 1 → 2
	d.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if d.focus != 2 {
		t.Errorf("expected focus=2 after Tab, got %d", d.focus)
	}
	// 2 → 0
	d.HandleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if d.focus != 0 {
		t.Errorf("expected focus=0 after Tab wrap, got %d", d.focus)
	}
}

func TestDeleteDialog_HandleClick_HitsConfirm(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.Render(testDialogW, testDialogH) // populate geometry
	// Confirm button is on the last content row of the box.
	// We just need a click within the confirm button's column range.
	// Simulate a click at the confirm button area. We'll test that it returns confirm=true.
	// The confirm button is left-of-center on the button row (row originY+dialogH-2).
	btnRow := d.originY + d.dialogH - 2
	confirmCol := d.originX + 6 // roughly center of "[Confirm]"
	confirm, cancel, _ := d.HandleClick(confirmCol, btnRow)
	if !confirm || cancel {
		t.Errorf("expected confirm=true from click on Confirm button, got confirm=%v cancel=%v", confirm, cancel)
	}
}

func TestDeleteDialog_HandleClick_HitsCancel(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.Render(testDialogW, testDialogH)
	btnRow := d.originY + d.dialogH - 2
	// Cancel is roughly at col originX + dialogW - 12
	cancelCol := d.originX + d.dialogW - 8
	confirm, cancel, _ := d.HandleClick(cancelCol, btnRow)
	if confirm || !cancel {
		t.Errorf("expected cancel=true from click on Cancel button, got confirm=%v cancel=%v", confirm, cancel)
	}
}

func TestDeleteDialog_HandleClick_MissesOutside(t *testing.T) {
	d := newTestDeleteDialog("/tmp/foo.go", false)
	d.Render(testDialogW, testDialogH)
	confirm, cancel, toggle := d.HandleClick(0, 0)
	if confirm || cancel || toggle {
		t.Errorf("expected no action for click outside dialog, got confirm=%v cancel=%v toggle=%v", confirm, cancel, toggle)
	}
}
