package themepicker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestModel() Model {
	tm, _ := theme.NewManager("toast-dark", "")
	m := New(tm, "", "toast-dark")
	m, _ = m.Init()
	return m
}

func TestInit_IncludesBuiltins(t *testing.T) {
	m := newTestModel()
	if len(m.themes) == 0 {
		t.Fatal("expected themes list to be non-empty after Init")
	}
	found := false
	for _, n := range m.themes {
		if n == "toast-dark" {
			found = true
		}
	}
	if !found {
		t.Error("expected toast-dark in theme list")
	}
}

func TestInit_SelectsActiveTheme(t *testing.T) {
	m := newTestModel()
	if m.themes[m.selected] != "toast-dark" {
		t.Errorf("expected selected to point at toast-dark, got %q", m.themes[m.selected])
	}
}

func TestArrowDown_MovesSelection(t *testing.T) {
	m := newTestModel()
	// Force selected to 0
	m.selected = 0
	msg := keyMsg("down")
	m, cmd := m.Update(msg)
	if m.selected != 1 {
		t.Errorf("expected selected=1, got %d", m.selected)
	}
	if cmd == nil {
		t.Error("expected a ThemeChangedMsg cmd")
	}
	result := cmd()
	if tc, ok := result.(messages.ThemeChangedMsg); !ok {
		t.Errorf("expected ThemeChangedMsg, got %T", result)
	} else if tc.ThemeName != m.themes[1] {
		t.Errorf("expected theme %q, got %q", m.themes[1], tc.ThemeName)
	}
}

func TestArrowUp_MovesSelection(t *testing.T) {
	m := newTestModel()
	m.selected = 1
	msg := keyMsg("up")
	m, _ = m.Update(msg)
	if m.selected != 0 {
		t.Errorf("expected selected=0, got %d", m.selected)
	}
}

func TestArrowDown_WrapsAtEnd(t *testing.T) {
	m := newTestModel()
	m.selected = len(m.themes) - 1
	m, _ = m.Update(keyMsg("down"))
	if m.selected != 0 {
		t.Errorf("expected wrap to 0, got %d", m.selected)
	}
}

func TestArrowUp_WrapsAtStart(t *testing.T) {
	m := newTestModel()
	m.selected = 0
	m, _ = m.Update(keyMsg("up"))
	if m.selected != len(m.themes)-1 {
		t.Errorf("expected wrap to %d, got %d", len(m.themes)-1, m.selected)
	}
}

func TestEnter_EmitsClosedWithSelected(t *testing.T) {
	m := newTestModel()
	m.selected = 1
	_, cmd := m.Update(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected cmd on enter")
	}
	result := cmd()
	tc, ok := result.(messages.ThemePickerClosedMsg)
	if !ok {
		t.Fatalf("expected ThemePickerClosedMsg, got %T", result)
	}
	if tc.ThemeName != m.themes[1] {
		t.Errorf("expected %q, got %q", m.themes[1], tc.ThemeName)
	}
}

func TestEsc_EmitsClosedWithOriginal(t *testing.T) {
	m := newTestModel()
	m.selected = 1
	_, cmd := m.Update(keyMsg("escape"))
	if cmd == nil {
		t.Fatal("expected cmd on escape")
	}
	result := cmd()
	tc, ok := result.(messages.ThemePickerClosedMsg)
	if !ok {
		t.Fatalf("expected ThemePickerClosedMsg, got %T", result)
	}
	// escape should revert to the original active theme
	if tc.ThemeName != "toast-dark" {
		t.Errorf("expected original toast-dark, got %q", tc.ThemeName)
	}
}

func TestRender_NotEmpty(t *testing.T) {
	m := newTestModel()
	out := m.Render()
	if out == "" {
		t.Error("expected non-empty render output")
	}
}

func TestJ_MovesSelectionDown(t *testing.T) {
	m := newTestModel()
	m.selected = 0
	updated, cmd := m.Update(keyMsg("j"))
	m = updated
	if m.selected != 1 {
		t.Errorf("expected selected=1, got %d", m.selected)
	}
	if cmd == nil {
		t.Error("expected ThemeChangedMsg cmd")
	}
}

func TestK_MovesSelectionUp(t *testing.T) {
	m := newTestModel()
	m.selected = 1
	updated, _ := m.Update(keyMsg("k"))
	m = updated
	if m.selected != 0 {
		t.Errorf("expected selected=0, got %d", m.selected)
	}
}

func TestO_EmitsCmd(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(keyMsg("o"))
	if cmd == nil {
		t.Error("expected non-nil cmd for 'o' key")
	}
	// Don't call cmd() — it would spawn a process
}

func TestDiscoverThemes_UserThemesDir(t *testing.T) {
	dir := t.TempDir()
	// Write a custom theme file
	if err := os.WriteFile(filepath.Join(dir, "my-theme.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := discoverThemes(dir)
	found := false
	for _, n := range names {
		if n == "my-theme" {
			found = true
		}
	}
	if !found {
		t.Error("expected my-theme in discovered list")
	}
	// Verify no .json suffix in returned names
	for _, n := range names {
		if strings.Contains(n, ".json") {
			t.Errorf("expected no .json suffix, got %q", n)
		}
	}
}

func TestClick_ThemeRowPreviewsOnly(t *testing.T) {
	m := newTestModel()
	// Y=2 → row index 1 (second theme). Should emit ThemeChangedMsg but NOT close.
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 2}
	updated, cmd := m.Update(click)
	m = updated
	if cmd == nil {
		t.Fatal("expected cmd from click")
	}
	if m.selected != 1 {
		t.Errorf("expected selected=1 after click on row Y=2, got %d", m.selected)
	}
	result := cmd()
	tc, ok := result.(messages.ThemeChangedMsg)
	if !ok {
		t.Fatalf("expected ThemeChangedMsg, got %T", result)
	}
	if tc.ThemeName != m.themes[1] {
		t.Errorf("expected %q, got %q", m.themes[1], tc.ThemeName)
	}
}

func TestClick_ConfirmButtonClosesWithSelected(t *testing.T) {
	m := newTestModel()
	m.selected = 1
	// Click within the confirm button region: contentX = confirmBtnStart+1, Y = action row
	actionY := len(m.themes) + 2
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: confirmBtnStart + 2, Y: actionY}
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected cmd from confirm button click")
	}
	result := cmd()
	closed, ok := result.(messages.ThemePickerClosedMsg)
	if !ok {
		t.Fatalf("expected ThemePickerClosedMsg, got %T", result)
	}
	if closed.ThemeName != m.themes[1] {
		t.Errorf("expected %q, got %q", m.themes[1], closed.ThemeName)
	}
}

func TestClick_CancelButtonRevertsAndCloses(t *testing.T) {
	m := newTestModel()
	m.selected = 1 // navigated away from toast-dark
	actionY := len(m.themes) + 2
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: cancelBtnStart + 2, Y: actionY}
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected cmd from cancel button click")
	}
	result := cmd()
	closed, ok := result.(messages.ThemePickerClosedMsg)
	if !ok {
		t.Fatalf("expected ThemePickerClosedMsg, got %T", result)
	}
	if closed.ThemeName != "toast-dark" {
		t.Errorf("expected original toast-dark, got %q", closed.ThemeName)
	}
}

func TestClick_SeparatorRowNoOp(t *testing.T) {
	m := newTestModel()
	sepY := len(m.themes) + 1
	click := tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: sepY}
	_, cmd := m.Update(click)
	if cmd != nil {
		t.Error("expected no cmd when clicking separator row")
	}
}

// keyMsg builds a tea.KeyPressMsg for testing using named rune constants.
func keyMsg(key string) tea.KeyPressMsg {
	codes := map[string]rune{
		"up":     tea.KeyUp,
		"down":   tea.KeyDown,
		"enter":  tea.KeyEnter,
		"escape": tea.KeyEscape,
	}
	if code, ok := codes[key]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	// single char keys like "o", "k", "j"
	r := []rune(key)[0]
	return tea.KeyPressMsg{Code: r}
}
