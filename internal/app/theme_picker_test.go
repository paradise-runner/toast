package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

// findArrow returns the screen cell where the picker draws one of its scroll
// arrows. The arrow always sits directly in front of the right border, which
// distinguishes it from other triangle glyphs in the UI (the file tree's folder
// chevrons). Reading the position back from the rendered view means the test
// fails if the drawn box and the hit-test geometry ever disagree.
func findArrow(t *testing.T, m Model, glyph string) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(m.View().Content, "\n") {
		// Columns are counted in runes: strings.Index would return a byte offset,
		// and every box-drawing character is three bytes wide.
		runes := []rune(stripSGR(line))
		for col, r := range runes {
			if string(r) == glyph && col+1 < len(runes) && runes[col+1] == '│' {
				return col, row
			}
		}
	}
	t.Fatalf("scroll arrow %q not found in the rendered view", glyph)
	return 0, 0
}

// stripSGR removes SGR escape sequences so rendered lines can be indexed by
// display column.
func stripSGR(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestThemePicker_ArrowClicksPageTheList clicks the scroll arrows at the cells
// they are actually drawn in. It is the end-to-end guard for the picker's
// geometry: app.go centres the dialog with Dimensions(), while the box is drawn
// by overlayCenter, so a mismatch makes clicks land next to the arrow and select
// the theme row underneath instead.
func TestThemePicker_ArrowClicksPageTheList(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_, _ = m.Update(messages.ThemePickerOpenMsg{})

	names := theme.Discover("")
	// Move the selection to the top of the list so it can page a full window down.
	if cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyHome}); cmd != nil {
		_, _ = m.Update(cmd())
	}
	if got := m.theme.Name(); got != names[0] {
		t.Fatalf("home selected %q, want %q", got, names[0])
	}
	start := 0
	// One window of theme rows; the dialog also draws 4 non-theme lines.
	pageRows := len(strings.Split(m.themePicker.Render(), "\n")) - 4
	if start+pageRows >= len(names) {
		t.Fatalf("need at least %d themes below %q to test paging (have %d)", pageRows, m.theme.Name(), len(names))
	}

	downX, downY := findArrow(t, *m, "▼")
	cmd := m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: downX, Y: downY})
	if cmd == nil {
		t.Fatalf("clicking the down arrow at (%d,%d) did nothing", downX, downY)
	}
	downMsg, ok := cmd().(messages.ThemeChangedMsg)
	if !ok {
		t.Fatalf("down arrow click produced %T, want ThemeChangedMsg", downMsg)
	}
	wantDown := names[start+pageRows]
	if downMsg.ThemeName != wantDown {
		t.Errorf("down arrow paged to %q, want %q", downMsg.ThemeName, wantDown)
	}

	// Apply the preview so the active theme follows the selection.
	_, _ = m.Update(downMsg)
	if m.theme.Name() != wantDown {
		t.Fatalf("active theme = %q, want %q", m.theme.Name(), wantDown)
	}

	upX, upY := findArrow(t, *m, "▲")
	cmd = m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: upX, Y: upY})
	if cmd == nil {
		t.Fatalf("clicking the up arrow at (%d,%d) did nothing", upX, upY)
	}
	upMsg, ok := cmd().(messages.ThemeChangedMsg)
	if !ok {
		t.Fatalf("up arrow click produced %T, want ThemeChangedMsg", upMsg)
	}
	if upMsg.ThemeName != names[start] {
		t.Errorf("up arrow paged to %q, want %q", upMsg.ThemeName, names[start])
	}
}
