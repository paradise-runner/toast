package findreplace

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

func TestMouseClickTogglesMatchCase(t *testing.T) {
	m := New(nil).Open("")

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      contentOffsetX + 1,
		Y:      contentOffsetY + optionsRow,
	})
	m = updated

	if !m.MatchCase() {
		t.Fatal("expected match-case to be enabled")
	}
	if cmd == nil {
		t.Fatal("expected query changed command")
	}
	msg := cmd()
	if _, ok := msg.(messages.FindReplaceQueryChangedMsg); !ok {
		t.Fatalf("expected FindReplaceQueryChangedMsg, got %T", msg)
	}
}

func TestMouseClickTogglesWholeWord(t *testing.T) {
	m := New(nil).Open("")
	wordX := strings.Index(optionsLine(false, false), "Whole word")
	if wordX < 0 {
		t.Fatal("test setup: Whole word label not found")
	}

	updated, cmd := m.Update(tea.MouseClickMsg{
		Button: tea.MouseLeft,
		X:      contentOffsetX + wordX,
		Y:      contentOffsetY + optionsRow,
	})
	m = updated

	if !m.WholeWord() {
		t.Fatal("expected whole-word to be enabled")
	}
	if cmd == nil {
		t.Fatal("expected query changed command")
	}
	msg, ok := cmd().(messages.FindReplaceQueryChangedMsg)
	if !ok {
		t.Fatalf("expected FindReplaceQueryChangedMsg, got %T", msg)
	}
	if !msg.WholeWord {
		t.Fatalf("expected WholeWord=true in message, got %+v", msg)
	}
}

func TestEscapeKeyCodeClosesOverlay(t *testing.T) {
	m := New(nil).Open("")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated

	if m.IsOpen() {
		t.Fatal("expected overlay to be closed")
	}
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd()
	if _, ok := msg.(messages.FindReplaceCloseMsg); !ok {
		t.Fatalf("expected FindReplaceCloseMsg, got %T", msg)
	}
}

func TestViewPaintsDialogWhitespace(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	m := New(tm).Open("te")
	m.width = 90
	m.SetMatchStatus(10, 21)

	for i, line := range strings.Split(m.View(), "\n") {
		if hasUnstyledSpaces(line) {
			t.Fatalf("line %d has unstyled spaces after ANSI reset: %q", i, line)
		}
	}
}

func hasUnstyledSpaces(s string) bool {
	hasBG := true
	for i := 0; i < len(s); {
		if s[i] != '\x1b' {
			if s[i] == ' ' && !hasBG {
				return true
			}
			i++
			continue
		}
		if i+1 >= len(s) || s[i+1] != '[' {
			i++
			continue
		}
		end := i + 2
		for end < len(s) && s[end] != 'm' {
			end++
		}
		if end >= len(s) {
			break
		}
		seq := s[i : end+1]
		if seq == "\x1b[m" {
			hasBG = false
		} else if strings.Contains(seq, "48;") {
			hasBG = true
		}
		i = end + 1
	}
	return false
}
