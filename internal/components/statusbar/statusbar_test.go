package statusbar

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func newTestModel() Model {
	tm, _ := theme.NewManager("toast-dark", "")
	m := New(tm)
	// Give it a width so button positions are computable
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 1})
	return updated.(Model)
}

func TestThemeButton_ClickEmitsOpenMsg(t *testing.T) {
	m := newTestModel()
	// The theme button is on the right side of the statusbar.
	// After rendering, themeButtonX should be set.
	if m.themeButtonX < 0 {
		t.Skip("theme button not positioned (width too small)")
	}
	click := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.themeButtonX,
		Y:      0,
	}
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected a cmd from clicking theme button")
	}
	result := cmd()
	if _, ok := result.(messages.ThemePickerOpenMsg); !ok {
		t.Errorf("expected ThemePickerOpenMsg, got %T", result)
	}
}

func TestThemeButton_ClickRightEdge(t *testing.T) {
	m := newTestModel()
	if m.themeButtonX < 0 {
		t.Skip("button not positioned")
	}
	click := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.themeButtonX + themeButtonWidth - 1,
		Y:      0,
	}
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected cmd at right edge of button")
	}
	result := cmd()
	if _, ok := result.(messages.ThemePickerOpenMsg); !ok {
		t.Errorf("expected ThemePickerOpenMsg, got %T", result)
	}
}

func TestThemeButton_ClickOutside_NoMsg(t *testing.T) {
	m := newTestModel()
	if m.themeButtonX < 0 {
		t.Skip("button not positioned")
	}
	// Click one past right edge — should not fire
	click := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.themeButtonX + themeButtonWidth,
		Y:      0,
	}
	_, cmd := m.Update(click)
	if cmd != nil {
		result := cmd()
		if _, ok := result.(messages.ThemePickerOpenMsg); ok {
			t.Error("should NOT emit ThemePickerOpenMsg for click outside button")
		}
	}
}

func TestThemeButton_ClickWithBranch(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(messages.GitStatusUpdatedMsg{Branch: "main"})
	m = updated.(Model)
	if m.themeButtonX < 0 {
		t.Skip("button not positioned")
	}
	click := tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      m.themeButtonX,
		Y:      0,
	}
	_, cmd := m.Update(click)
	if cmd == nil {
		t.Fatal("expected cmd when branch is set")
	}
	result := cmd()
	if _, ok := result.(messages.ThemePickerOpenMsg); !ok {
		t.Errorf("expected ThemePickerOpenMsg, got %T", result)
	}
}

func TestView_LightTheme_PaddingHasBackground(t *testing.T) {
	// Regression test: on the light theme the middle padding between left/right
	// sections was plain unstyled spaces that showed the terminal's default
	// (dark) background instead of the statusbar background color.
	tm, _ := theme.NewManager("toast-light", "../../../internal/theme/builtin")
	m := New(tm)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 1})
	m = updated.(Model)

	view := m.View().Content

	// After an ANSI reset (\x1b[m), any space character must be preceded by a
	// new background-color sequence before it appears. If plain spaces follow a
	// reset, the terminal's default (dark) background shows through on light themes.
	if hasUnstyledSpacesAfterReset(view) {
		t.Errorf("StatusBar light theme view has spaces without background color after reset.\nView: %q", view)
	}
}

// hasUnstyledSpacesAfterReset returns true if the ANSI string s contains any
// space that appears after an SGR reset (\x1b[m) without an intervening
// background-color SGR sequence (one containing "48;").
func hasUnstyledSpacesAfterReset(s string) bool {
	hasBG := true // assume background active at start
	i := 0
	for i < len(s) {
		if s[i] != '\x1b' {
			if s[i] == ' ' && !hasBG {
				return true
			}
			i++
			continue
		}
		// Parse escape sequence: \x1b[...m
		if i+1 >= len(s) || s[i+1] != '[' {
			i++
			continue
		}
		end := i + 2
		for end < len(s) && s[end] != 'm' {
			end++
		}
		seq := s[i : end+1]
		switch seq {
		case "\x1b[m":
			hasBG = false
		default:
			if strings.Contains(seq, "48;") {
				hasBG = true
			}
		}
		i = end + 1
	}
	return false
}
