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

// topVisibleTheme reads the theme name drawn in the first list row.
func topVisibleTheme(t *testing.T, m Model) string {
	t.Helper()
	lines := strings.Split(stripSGR(m.themePicker.Render()), "\n")
	if len(lines) < 2 {
		t.Fatal("picker rendered no rows")
	}
	row := strings.TrimSuffix(lines[1], "│")
	row = strings.TrimPrefix(row, "│")
	// The first row carries the up arrow when the list is scrolled.
	row = strings.TrimSuffix(strings.TrimSpace(row), "▲")
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "●")
	return strings.TrimSpace(row)
}

// TestThemePicker_ArrowClicksScrollWithoutSelecting clicks the scroll arrows at
// the cells they are drawn in. Two things must hold at once: the list scrolls,
// and the selection and active theme stay exactly where they were. The click
// coordinates come from the rendered view, so the drawn box and the hit-test
// geometry can never drift apart unnoticed.
func TestThemePicker_ArrowClicksScrollWithoutSelecting(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_, _ = m.Update(messages.ThemePickerOpenMsg{})

	names := theme.Discover("")
	if len(names) <= 12 {
		t.Fatalf("need more themes than fit in the picker, have %d", len(names))
	}
	// Normalise the window to the top of the list so a page down and a page up
	// are both full pages (a page down near the end is clamped short).
	if cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyHome}); cmd != nil {
		_, _ = m.Update(cmd())
	}
	active := m.theme.Name()
	before := topVisibleTheme(t, *m)

	downX, downY := findArrow(t, *m, "▼")
	cmd := m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: downX, Y: downY})
	if cmd != nil {
		t.Fatalf("clicking the down arrow at (%d,%d) produced %T; scrolling must not preview a theme", downX, downY, cmd())
	}
	if got := m.theme.Name(); got != active {
		t.Errorf("active theme changed from %q to %q on a scroll click", active, got)
	}
	after := topVisibleTheme(t, *m)
	if after == before {
		t.Errorf("down arrow did not scroll: first visible row is still %q", after)
	}

	// The arrow has moved off the first row, but ▲ is now drawn higher up and
	// a page up returns the list to exactly where it started.
	upX, upY := findArrow(t, *m, "▲")
	cmd = m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: upX, Y: upY})
	if cmd != nil {
		t.Fatalf("clicking the up arrow at (%d,%d) produced %T; scrolling must not preview a theme", upX, upY, cmd())
	}
	if got := m.theme.Name(); got != active {
		t.Errorf("active theme changed from %q to %q on a scroll click", active, got)
	}
	if got := topVisibleTheme(t, *m); got != before {
		t.Errorf("up arrow did not restore the list: first visible row is %q, want %q", got, before)
	}
}

// TestThemePicker_ClickingARowStillSelects makes sure the arrow column only
// scrolls on the boundary rows: any other row click selects the theme under the
// cursor, after arbitrary scrolling.
func TestThemePicker_ClickingARowStillSelects(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_, _ = m.Update(messages.ThemePickerOpenMsg{})

	// Scroll down one page without selecting anything.
	downX, downY := findArrow(t, *m, "▼")
	if cmd := m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: downX, Y: downY}); cmd != nil {
		_, _ = m.Update(cmd())
	}

	// Click a row below the first; the theme under the cursor becomes active.
	// Row i of the visible list sits at screen Y = startY + i + 1, and downY is
	// the last list row, so pick index 5 (the sixth row).
	const rowIndex = 5
	rows := len(visibleRowNames(t, *m))
	want := visibleRowNames(t, *m)[rowIndex]
	clickY := downY - (rows - 1 - rowIndex)
	// X must stay inside the box: the dialog is centred, so screen column 20 is
	// the file tree, and a click there closes the picker as a click-outside.
	cmd := m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: downX - 10, Y: clickY})
	if cmd == nil {
		t.Fatal("expected a preview command from the row click")
	}
	_, _ = m.Update(cmd())
	if got := m.theme.Name(); got != want {
		t.Errorf("active theme = %q, want %q", got, want)
	}
}

// visibleRowNames lists the theme names drawn in the picker's list rows.
func visibleRowNames(t *testing.T, m Model) []string {
	t.Helper()
	lines := strings.Split(stripSGR(m.themePicker.Render()), "\n")
	var names []string
	for _, line := range lines[1 : len(lines)-3] {
		row := strings.TrimSuffix(line, "│")
		row = strings.TrimPrefix(row, "│")
		row = strings.TrimSuffix(strings.TrimSpace(row), "▲")
		row = strings.TrimSuffix(strings.TrimSpace(row), "▼")
		row = strings.TrimSpace(row)
		row = strings.TrimPrefix(row, "●")
		names = append(names, strings.TrimSpace(row))
	}
	return names
}

// TestThemePicker_KeyAfterScrollStaysOnPage: after scrolling with the arrows, an
// arrow key must anchor the selection onto the visible page instead of jumping
// the list back to the row the user scrolled away from.
func TestThemePicker_KeyAfterScrollStaysOnPage(t *testing.T) {
	m := newTestApp(t, t.TempDir())
	_, _ = m.Update(messages.ThemePickerOpenMsg{})

	// Start at the top, then scroll to the end of the list twice, leaving the
	// selection far above the window.
	if cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyHome}); cmd != nil {
		_, _ = m.Update(cmd())
	}
	for range 2 {
		downX, downY := findArrow(t, *m, "▼")
		if cmd := m.handleMouseClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: downX, Y: downY}); cmd != nil {
			t.Fatalf("scrolling previewed a theme: %#v", cmd())
		}
	}
	topBefore := topVisibleTheme(t, *m)
	if topBefore != "nightfox" {
		t.Fatalf("expected the list scrolled to the end, top row is %q", topBefore)
	}

	cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd == nil {
		t.Fatal("expected the down key to preview the anchored row")
	}
	_, _ = m.Update(cmd())

	if got := m.theme.Name(); got != topBefore {
		t.Errorf("down key after scrolling selected %q, want the first visible row %q", got, topBefore)
	}
	if got := topVisibleTheme(t, *m); got != topBefore {
		t.Errorf("the list jumped: top row %q, want %q", got, topBefore)
	}
}
