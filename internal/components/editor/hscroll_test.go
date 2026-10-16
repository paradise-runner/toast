package editor

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// newHScrollModel builds a non-wrap editor with a single very long line so the
// horizontal viewport has room to scroll. viewWidth=40, gutterWidth=4.
func newHScrollModel(content string) Model {
	m := newTestModel(content)
	m.viewWidth = 40
	m.viewHeight = 5
	m.recomputeGutterWidth()
	return m
}

// TestClampViewport_RecoversShortLineAfterHorizontalScroll is the regression
// test for the blank-line bug: scrolling right on a long line must not leave
// shorter lines (including the cursor's) rendering empty.
func TestClampViewport_RecoversShortLineAfterHorizontalScroll(t *testing.T) {
	long := strings.Repeat("x", 200)
	m := newThemedTestModel(t, "short1\n"+long+"\nshort2\nend\n")

	// Put the cursor at the end of the long line so the viewport scrolls right.
	m.cursor = cursorPos{line: 1, col: len(long)}
	m.preferredCol = m.cursor.col
	m.clampViewport()
	if m.viewportLeft == 0 {
		t.Fatal("expected the viewport to scroll right on the long line")
	}

	// Move down onto the short line. The viewport must snap back so the line
	// is readable instead of staying scrolled past its end.
	m.cursor = cursorPos{line: 2, col: m.lineContentLen(2)}
	m.clampViewport()
	if m.viewportLeft != 0 {
		t.Fatalf("viewportLeft = %d, want 0 after moving to a short line", m.viewportLeft)
	}
	if out := m.View().Content; !strings.Contains(out, "short2") {
		t.Fatalf("short line missing from view after horizontal scroll:\n%q", out)
	}
}

func TestClampViewport_RecoversFromBlankCursorLine(t *testing.T) {
	long := strings.Repeat("x", 200)
	m := newThemedTestModel(t, long+"\n\nshort\n")

	m.cursor = cursorPos{line: 0, col: len(long)}
	m.preferredCol = m.cursor.col
	m.clampViewport()
	if m.viewportLeft == 0 {
		t.Fatal("expected the viewport to scroll right on the long line")
	}

	// A blank cursor line must not leave the viewport scrolled past its end.
	m.cursor = cursorPos{line: 1, col: 0}
	m.clampViewport()
	if m.viewportLeft != 0 {
		t.Fatalf("viewportLeft = %d, want 0 after moving to a blank line", m.viewportLeft)
	}
	if out := m.View().Content; !strings.Contains(out, "short") {
		t.Fatalf("short line missing from view after horizontal scroll:\n%q", out)
	}
}

func TestMouseWheelHorizontalScrollsViewport(t *testing.T) {
	m := newHScrollModel(strings.Repeat("abcdefgh", 40) + "\n")

	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelRight})
	m = updated.(Model)
	if m.viewportLeft != horizontalScrollStep {
		t.Fatalf("viewportLeft after wheel right = %d, want %d", m.viewportLeft, horizontalScrollStep)
	}
	if m.cursor != (cursorPos{}) {
		t.Fatalf("horizontal wheel moved the cursor: %+v", m.cursor)
	}

	updated, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelLeft})
	m = updated.(Model)
	if m.viewportLeft != 0 {
		t.Fatalf("viewportLeft after wheel left = %d, want 0", m.viewportLeft)
	}
}

func TestShiftWheelScrollsHorizontally(t *testing.T) {
	m := newHScrollModel(strings.Repeat("abcdefgh", 40) + "\n")

	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, Mod: tea.ModShift})
	m = updated.(Model)
	if m.viewportLeft != horizontalScrollStep {
		t.Fatalf("viewportLeft after shift+wheel down = %d, want %d", m.viewportLeft, horizontalScrollStep)
	}
	if m.cursor.line != 0 {
		t.Fatalf("shift+wheel moved the cursor: %+v", m.cursor)
	}

	updated, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, Mod: tea.ModShift})
	m = updated.(Model)
	if m.viewportLeft != 0 {
		t.Fatalf("viewportLeft after shift+wheel up = %d, want 0", m.viewportLeft)
	}
}

func TestScrollHorizontalClampsAtLongestVisibleLine(t *testing.T) {
	long := strings.Repeat("a", 50)
	m := newHScrollModel(long + "\n")

	for i := 0; i < 50; i++ {
		updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelRight})
		m = updated.(Model)
	}
	// At least the last character of the longest visible line stays on screen.
	if want := len(long) - 1; m.viewportLeft != want {
		t.Fatalf("viewportLeft = %d, want %d", m.viewportLeft, want)
	}
}

func TestHorizontalWheelNoopInWrapMode(t *testing.T) {
	m := newWrapModel("hello\nworld\n")

	for _, button := range []tea.MouseButton{tea.MouseWheelRight, tea.MouseWheelLeft} {
		updated, _ := m.Update(tea.MouseWheelMsg{Button: button})
		m = updated.(Model)
		if m.viewportLeft != 0 {
			t.Fatalf("wrap mode scrolled horizontally on %v: viewportLeft = %d", button, m.viewportLeft)
		}
	}
}

func TestPlainWheelStillMovesCursor(t *testing.T) {
	m := newHScrollModel("a\nb\nc\n")

	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.cursor.line != 1 {
		t.Fatalf("wheel down cursor line = %d, want 1", m.cursor.line)
	}
}

func TestShiftWheelMovesCursorInWrapMode(t *testing.T) {
	m := newWrapModel("hello\nworld\n")

	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, Mod: tea.ModShift})
	m = updated.(Model)
	if m.cursor.line != 1 {
		t.Fatalf("shift+wheel in wrap mode: cursor line = %d, want 1", m.cursor.line)
	}
}

// TestCursorHiddenWhenScrolledOffScreen guards against drawing a caret pinned
// to the gutter while the text it belongs to is scrolled out of view.
func TestCursorHiddenWhenScrolledOffScreen(t *testing.T) {
	m := newThemedTestModel(t, strings.Repeat("x", 200)+"\n")

	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelRight})
		m = updated.(Model)
	}
	if m.viewportLeft == 0 {
		t.Fatal("expected the viewport to scroll right")
	}
	if v := m.View(); v.Cursor != nil {
		t.Fatalf("cursor visible after being scrolled off-screen: %+v", v.Cursor)
	}

	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelLeft})
		m = updated.(Model)
	}
	if v := m.View(); v.Cursor == nil {
		t.Fatal("cursor should be visible again after scrolling back")
	}
}
