package settings

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

// newTestModel creates a settings dialog with default config and builtin themes.
func newTestModel() Model {
	tm, _ := theme.NewManager("toast-dark", "")
	return New(tm, "", config.Defaults())
}

func keyMsg(key string) tea.KeyPressMsg {
	codes := map[string]rune{
		"up":    tea.KeyUp,
		"down":  tea.KeyDown,
		"left":  tea.KeyLeft,
		"right": tea.KeyRight,
		"enter": tea.KeyEnter,
		"esc":   tea.KeyEscape,
		"tab":   tea.KeyTab,
		"space": tea.KeySpace,
	}
	if code, ok := codes[key]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	return tea.KeyPressMsg{Code: []rune(key)[0]}
}

// changedConfig runs cmd and extracts the SettingsChangedMsg config.
func changedConfig(t *testing.T, cmd tea.Cmd) config.Config {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected a command, got nil")
	}
	msg := cmd()
	sm, ok := msg.(messages.SettingsChangedMsg)
	if !ok {
		t.Fatalf("expected SettingsChangedMsg, got %T", msg)
	}
	return sm.Config
}

// moveRight focuses the right pane on a fresh model.
func moveRight(m Model) Model {
	m, _ = m.Update(keyMsg("tab"))
	return m
}

func TestInitialState(t *testing.T) {
	m := newTestModel()
	if len(m.groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(m.groups))
	}
	if m.activeGroup != 0 || m.cursor != 0 || !m.focusLeft {
		t.Fatalf("unexpected initial state: group=%d cursor=%d focusLeft=%v", m.activeGroup, m.cursor, m.focusLeft)
	}
}

func TestTabSwitchesPane(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(keyMsg("tab"))
	if m.focusLeft {
		t.Fatal("expected focus in right pane after tab")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if !m.focusLeft {
		t.Fatal("expected focus back in left pane after shift+tab")
	}
}

func TestArrowKeysSwitchPanes(t *testing.T) {
	m := newTestModel()
	// Right arrow crosses into the settings pane without changing any value.
	m, _ = m.Update(keyMsg("right"))
	if m.focusLeft {
		t.Fatal("expected right arrow to focus right pane")
	}
	if m.cfg.Editor.TabWidth != 4 {
		t.Fatalf("expected right arrow not to adjust values, tab width = %d", m.cfg.Editor.TabWidth)
	}
	// Left arrow returns to the group pane, also without adjusting.
	m, _ = m.Update(keyMsg("left"))
	if !m.focusLeft {
		t.Fatal("expected left arrow to focus left pane")
	}
	// Left in the left pane and right in the right pane are no-ops.
	m, _ = m.Update(keyMsg("left"))
	if !m.focusLeft {
		t.Fatal("expected left in left pane to be a no-op")
	}
	m = moveRight(m)
	m, _ = m.Update(keyMsg("right"))
	if m.focusLeft {
		t.Fatal("expected right in right pane to be a no-op")
	}
}

func TestGroupNavigation(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(keyMsg("down"))
	if m.activeGroup != 1 {
		t.Fatalf("expected group 1, got %d", m.activeGroup)
	}
	// Up from the first group wraps to the last.
	m.activeGroup = 0
	m, _ = m.Update(keyMsg("up"))
	if m.activeGroup != len(m.groups)-1 {
		t.Fatalf("expected wrap to last group, got %d", m.activeGroup)
	}
}

func TestCursorClampedOnGroupChange(t *testing.T) {
	m := newTestModel()
	m = moveRight(m)
	// Move cursor to the last Editor setting (index 4 of 5).
	for i := 0; i < 4; i++ {
		m, _ = m.Update(keyMsg("down"))
	}
	if m.cursor != 4 {
		t.Fatalf("expected cursor 4, got %d", m.cursor)
	}
	// Switch to Appearance (1 setting) via the left pane: cursor must clamp.
	m, _ = m.Update(keyMsg("tab"))
	m, _ = m.Update(keyMsg("down")) // Appearance
	if m.cursor != 0 {
		t.Fatalf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestEnterOnGroupMovesToSettings(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(keyMsg("down")) // Appearance
	m, _ = m.Update(keyMsg("enter"))
	if m.focusLeft {
		t.Fatal("expected focus in right pane after enter on group")
	}
	if m.activeGroup != 1 || m.cursor != 0 {
		t.Fatalf("expected group 1 cursor 0, got %d/%d", m.activeGroup, m.cursor)
	}
}

func TestToggleSetting(t *testing.T) {
	m := newTestModel()
	m = moveRight(m)
	m, _ = m.Update(keyMsg("down")) // cursor 1: Auto Indent (default true)
	if !m.cfg.Editor.AutoIndent {
		t.Fatal("precondition: AutoIndent should default to true")
	}

	m, cmd := m.Update(keyMsg("enter"))
	if m.cfg.Editor.AutoIndent {
		t.Fatal("expected AutoIndent toggled off")
	}
	if changedConfig(t, cmd).Editor.AutoIndent {
		t.Fatal("expected SettingsChangedMsg with AutoIndent off")
	}

	// Space toggles too.
	m, _ = m.Update(keyMsg("space"))
	if !m.cfg.Editor.AutoIndent {
		t.Fatal("expected AutoIndent toggled back on")
	}
}

func TestStepperAdjustsAndClamps(t *testing.T) {
	m := newTestModel()
	m = moveRight(m) // cursor 0: Tab Width (default 4, min 1, max 16)

	m, cmd := m.Update(keyMsg("l"))
	if m.cfg.Editor.TabWidth != 5 {
		t.Fatalf("expected tab width 5, got %d", m.cfg.Editor.TabWidth)
	}
	if changedConfig(t, cmd).Editor.TabWidth != 5 {
		t.Fatal("expected SettingsChangedMsg with tab width 5")
	}

	m, _ = m.Update(keyMsg("h"))
	if m.cfg.Editor.TabWidth != 4 {
		t.Fatalf("expected tab width 4, got %d", m.cfg.Editor.TabWidth)
	}

	// Clamp at min: hammer left past the minimum.
	for i := 0; i < 25; i++ {
		m, _ = m.Update(keyMsg("h"))
	}
	if m.cfg.Editor.TabWidth != 1 {
		t.Fatalf("expected tab width clamped to 1, got %d", m.cfg.Editor.TabWidth)
	}
	// No change at the boundary: no command emitted.
	_, cmd = m.Update(keyMsg("h"))
	if cmd != nil {
		t.Fatal("expected nil cmd when stepper is already at minimum")
	}

	// Clamp at max.
	for i := 0; i < 40; i++ {
		m, _ = m.Update(keyMsg("l"))
	}
	if m.cfg.Editor.TabWidth != 16 {
		t.Fatalf("expected tab width clamped to 16, got %d", m.cfg.Editor.TabWidth)
	}
}

func TestThemeCycleWraps(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(keyMsg("down")) // Appearance
	m = moveRight(m)
	themes := m.activeSettings()[0].options
	if len(themes) < 2 {
		t.Skip("not enough themes to test cycling")
	}
	start := m.cfg.Theme

	// Find the start position in the options list.
	startIdx := -1
	for i, n := range themes {
		if n == start {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		t.Fatalf("active theme %q not in options", start)
	}

	m, cmd := m.Update(keyMsg("l"))
	want := themes[(startIdx+1)%len(themes)]
	if m.cfg.Theme != want {
		t.Fatalf("expected theme %q, got %q", want, m.cfg.Theme)
	}
	_ = changedConfig(t, cmd)

	// Cycling forward len(themes) times returns to the start.
	for i := 0; i < len(themes)-1; i++ {
		m, _ = m.Update(keyMsg("l"))
	}
	if m.cfg.Theme != start {
		t.Fatalf("expected wrap back to %q, got %q", start, m.cfg.Theme)
	}

	// Left wraps backwards to the previous theme.
	m, _ = m.Update(keyMsg("h"))
	wantPrev := themes[(startIdx-1+len(themes))%len(themes)]
	if m.cfg.Theme != wantPrev {
		t.Fatalf("expected %q after left, got %q", wantPrev, m.cfg.Theme)
	}
}

func TestEscCloses(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("expected close command on esc")
	}
	if _, ok := cmd().(messages.SettingsClosedMsg); !ok {
		t.Fatalf("expected SettingsClosedMsg, got %T", cmd())
	}
}

// ── Mouse ────────────────────────────────────────────────────────────────────

func hoverMsg(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y}
}

func clickMsg(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y}
}

func TestHoverLeftChangesGroup(t *testing.T) {
	m := newTestModel()
	// Row 1 (Appearance) in the left pane; dialog-local coords include the border.
	m, _ = m.Update(hoverMsg(3, firstRow+1))
	if m.activeGroup != 1 {
		t.Fatalf("expected hover to select group 1, got %d", m.activeGroup)
	}
	if !m.focusLeft {
		t.Fatal("expected focus in left pane after hovering it")
	}
	// Hovering outside the rows changes nothing.
	m.activeGroup = 0
	m, _ = m.Update(hoverMsg(3, footerY))
	if m.activeGroup != 0 {
		t.Fatalf("expected group unchanged, got %d", m.activeGroup)
	}
}

func TestHoverRightChangesCursor(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(hoverMsg(leftWidth+2, firstRow+2)) // row 2 of the right pane
	if m.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", m.cursor)
	}
	if m.focusLeft {
		t.Fatal("expected focus in right pane after hovering it")
	}
}

func TestClickGroupSelects(t *testing.T) {
	m := newTestModel()
	m, _ = m.Update(clickMsg(2, firstRow+2)) // Sidebar
	if m.activeGroup != 2 {
		t.Fatalf("expected group 2, got %d", m.activeGroup)
	}
}

func TestClickToggleSetting(t *testing.T) {
	m := newTestModel()
	if !m.cfg.Editor.AutoIndent {
		t.Fatal("precondition: AutoIndent should default to true")
	}
	// Click anywhere on the Auto Indent row (row 1) in the right pane.
	m, cmd := m.Update(clickMsg(leftWidth+2, firstRow+1))
	if m.cfg.Editor.AutoIndent {
		t.Fatal("expected click to toggle AutoIndent off")
	}
	if changedConfig(t, cmd).Editor.AutoIndent {
		t.Fatal("expected SettingsChangedMsg with AutoIndent off")
	}
}

// stepperArrowXs returns the dialog-local X positions of the ◄ and ► arrows
// for the Tab Width setting, mirroring handleClick's hit-test math.
func stepperArrowXs(m Model) (left, right int) {
	ctrl := controlString(m.groups[0].settings[0], &m.cfg)
	n := utf8.RuneCountInString(ctrl)
	leftPad := controlWidth - n
	rightStart := leftWidth + (dialogWidth - leftWidth - controlWidth) + leftPad
	return rightStart + 1, rightStart + n // +1 for the left border column
}

func TestClickStepperArrows(t *testing.T) {
	m := newTestModel()
	arrowLeft, arrowRight := stepperArrowXs(m)

	m, cmd := m.Update(clickMsg(arrowRight, firstRow))
	if m.cfg.Editor.TabWidth != 5 {
		t.Fatalf("expected ► click to increment to 5, got %d", m.cfg.Editor.TabWidth)
	}
	_ = changedConfig(t, cmd)

	m, cmd = m.Update(clickMsg(arrowLeft, firstRow))
	if m.cfg.Editor.TabWidth != 4 {
		t.Fatalf("expected ◄ click to decrement to 4, got %d", m.cfg.Editor.TabWidth)
	}
	_ = changedConfig(t, cmd)

	// Clicking the middle of the control selects but does not change the value.
	m, cmd = m.Update(clickMsg(arrowLeft+2, firstRow))
	if m.cfg.Editor.TabWidth != 4 {
		t.Fatalf("expected no change from middle click, got %d", m.cfg.Editor.TabWidth)
	}
	if cmd != nil {
		t.Fatal("expected nil cmd for non-arrow click")
	}
	if m.focusLeft || m.cursor != 0 {
		t.Fatal("expected middle click to select row 0 in right pane")
	}
}

func TestClickOutsideRowsIgnored(t *testing.T) {
	m := newTestModel()
	m, cmd := m.Update(clickMsg(5, headerY))
	if cmd != nil {
		t.Fatal("expected nil cmd for click on header")
	}
	if m.activeGroup != 0 || !m.focusLeft {
		t.Fatal("expected state unchanged by header click")
	}
}

func TestDimensionsMatchRender(t *testing.T) {
	m := newTestModel()
	w, h := m.Dimensions()
	rendered := m.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) != h {
		t.Fatalf("expected %d lines, got %d", h, len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != w {
			t.Fatalf("line %d: expected width %d, got %d", i, w, got)
		}
	}
}

func TestAutoSaveCycleTogglesMode(t *testing.T) {
	m := newTestModel()
	if m.cfg.Editor.AutoSave != "auto" {
		t.Fatalf("expected default auto_save 'auto', got %q", m.cfg.Editor.AutoSave)
	}
	// Editor group rows: ... 4 Bottom Padding, 5 Auto Save, 6 Auto Save Delay.
	m = moveRight(m)
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyMsg("down"))
	}
	// Cursor 5: Auto Save. 'l' cycles auto -> manual.
	m, cmd := m.Update(keyMsg("l"))
	if m.cfg.Editor.AutoSave != "manual" {
		t.Fatalf("expected auto_save 'manual' after cycle, got %q", m.cfg.Editor.AutoSave)
	}
	if changedConfig(t, cmd).Editor.AutoSave != "manual" {
		t.Fatal("expected SettingsChangedMsg with auto_save 'manual'")
	}
	// Cycles back to auto.
	m, _ = m.Update(keyMsg("l"))
	if m.cfg.Editor.AutoSave != "auto" {
		t.Fatalf("expected auto_save to wrap back to 'auto', got %q", m.cfg.Editor.AutoSave)
	}
}

func TestAutoSaveDelayCycleSetsMs(t *testing.T) {
	m := newTestModel()
	if m.cfg.Editor.AutoSaveDelayMs != 300 {
		t.Fatalf("expected default delay 300, got %d", m.cfg.Editor.AutoSaveDelayMs)
	}
	m = moveRight(m)
	for i := 0; i < 6; i++ {
		m, _ = m.Update(keyMsg("down"))
	}
	// Cursor 6: Auto Save Delay. 300ms -> 500ms.
	m, cmd := m.Update(keyMsg("l"))
	if m.cfg.Editor.AutoSaveDelayMs != 500 {
		t.Fatalf("expected delay 500 after cycle, got %d", m.cfg.Editor.AutoSaveDelayMs)
	}
	if changedConfig(t, cmd).Editor.AutoSaveDelayMs != 500 {
		t.Fatal("expected SettingsChangedMsg with delay 500")
	}
	// 500ms -> 1s.
	m, _ = m.Update(keyMsg("l"))
	if m.cfg.Editor.AutoSaveDelayMs != 1000 {
		t.Fatalf("expected delay 1000 after second cycle, got %d", m.cfg.Editor.AutoSaveDelayMs)
	}
	// 1s -> 2s.
	m, _ = m.Update(keyMsg("l"))
	if m.cfg.Editor.AutoSaveDelayMs != 2000 {
		t.Fatalf("expected delay 2000 after third cycle, got %d", m.cfg.Editor.AutoSaveDelayMs)
	}
	// Cycle backwards past the first option wraps to the last (10s).
	for i := 0; i < 6; i++ {
		m, _ = m.Update(keyMsg("h"))
	}
	if m.cfg.Editor.AutoSaveDelayMs != 10000 {
		t.Fatalf("expected delay to wrap to 10000, got %d", m.cfg.Editor.AutoSaveDelayMs)
	}
}

func TestAutoSaveDelayLabelRoundTrip(t *testing.T) {
	for _, label := range autoSaveDelayOptions {
		if got := autoSaveDelayLabel(autoSaveDelayMS(label)); got != label {
			t.Fatalf("round trip %q -> %d -> %q", label, autoSaveDelayMS(label), got)
		}
	}
	if autoSaveDelayLabel(1234) != "300ms" {
		t.Fatalf("unexpected label for non-preset value: %q", autoSaveDelayLabel(1234))
	}
	if autoSaveDelayMS("7s") != 7000 {
		t.Fatalf("expected 7s -> 7000, got %d", autoSaveDelayMS("7s"))
	}
	if autoSaveDelayMS("nonsense") != 300 {
		t.Fatalf("expected fallback 300 for nonsense label, got %d", autoSaveDelayMS("nonsense"))
	}
}
