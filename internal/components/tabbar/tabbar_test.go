package tabbar

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestTabBar() Model {
	tm, _ := theme.NewManager("", "")
	return New(tm)
}

func TestCloseButtonAtX_DetectsCloseColumn(t *testing.T) {
	m := newTestTabBar()
	// Inject two tabs manually.
	m.tabs = []Tab{
		{BufferID: 1, Path: "/a/foo.go"},
		{BufferID: 2, Path: "/b/bar.go"},
	}
	m.active = 0

	// closeButtonAtX uses lipgloss.Width (display columns), not byte length.
	// label ends with " × " so × is at display width - 3.
	label0 := m.tabLabel(m.tabs[0])
	closeCol := lipgloss.Width(label0) - 3
	got := m.closeButtonAtX(closeCol)
	if got != 0 {
		t.Fatalf("expected tab index 0 at close col %d, got %d", closeCol, got)
	}
}

func TestCloseButtonAtX_NotOnLabel(t *testing.T) {
	m := newTestTabBar()
	m.tabs = []Tab{{BufferID: 1, Path: "/a/foo.go"}}
	m.active = 0

	// Clicking before the × should return -1
	got := m.closeButtonAtX(1) // inside text, not on ×
	if got != -1 {
		t.Fatalf("expected -1 for non-close column, got %d", got)
	}
}

func TestCloseButtonAtX_TrailingSpaceAlsoCloses(t *testing.T) {
	// The label ends with " × " so the × is at width-3 and the trailing space
	// is at width-2.  Clicking the trailing space should still trigger close —
	// it is visually part of the close zone and is only 1 cell away from the
	// × glyph itself.
	m := newTestTabBar()
	m.tabs = []Tab{{BufferID: 5, Path: "/a/foo.go"}}
	m.active = 0

	label := m.tabLabel(m.tabs[0])
	trailingSpaceCol := lipgloss.Width(label) - 2 // one past ×

	got := m.closeButtonAtX(trailingSpaceCol)
	if got != 0 {
		t.Fatalf("expected tab index 0 at trailing-space col %d, got %d", trailingSpaceCol, got)
	}
}

func TestCloseButtonAtX_BeforeXDoesNotClose(t *testing.T) {
	// The cell just before × is part of the filename area; clicking it should
	// not trigger close (the user is clicking the tab text, not the ×).
	m := newTestTabBar()
	m.tabs = []Tab{{BufferID: 6, Path: "/a/foo.go"}}
	m.active = 0

	label := m.tabLabel(m.tabs[0])
	beforeCloseCol := lipgloss.Width(label) - 4 // one before ×

	got := m.closeButtonAtX(beforeCloseCol)
	if got != -1 {
		t.Fatalf("expected -1 for cell before ×, got %d", got)
	}
}

func TestMiddleClick_EmitsCloseTabRequestMsg(t *testing.T) {
	m := newTestTabBar()
	m.tabs = []Tab{{BufferID: 7, Path: "/a/foo.go"}}
	m.active = 0

	_, cmd := m.handleMouseRelease(tea.MouseReleaseMsg{Button: tea.MouseMiddle, X: 1, Y: 0})
	if cmd == nil {
		t.Fatal("expected a cmd from middle-click")
	}
	msg := cmd()
	req, ok := msg.(messages.CloseTabRequestMsg)
	if !ok {
		t.Fatalf("expected CloseTabRequestMsg, got %T", msg)
	}
	if req.BufferID != 7 {
		t.Errorf("expected BufferID 7, got %d", req.BufferID)
	}
}

func TestCloseButtonLeftClick_EmitsCloseTabRequestMsg(t *testing.T) {
	m := newTestTabBar()
	m.tabs = []Tab{{BufferID: 3, Path: "/a/foo.go"}}
	m.active = 0

	label := m.tabLabel(m.tabs[0])
	closeCol := lipgloss.Width(label) - 3

	_, cmd := m.handleMouseRelease(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: closeCol, Y: 0})
	if cmd == nil {
		t.Fatal("expected a cmd from close button click")
	}
	msg := cmd()
	req, ok := msg.(messages.CloseTabRequestMsg)
	if !ok {
		t.Fatalf("expected CloseTabRequestMsg, got %T", msg)
	}
	if req.BufferID != 3 {
		t.Errorf("expected BufferID 3, got %d", req.BufferID)
	}
}
