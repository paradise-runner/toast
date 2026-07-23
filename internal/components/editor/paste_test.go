package editor

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNormalizePasteText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lf unchanged", "a\nb\nc", "a\nb\nc"},
		{"crlf", "a\r\nb\r\nc", "a\nb\nc"},
		{"lone cr", "a\rb\rc", "a\nb\nc"},
		{"mixed", "a\r\nb\rc\nd", "a\nb\nc\nd"},
		{"no line endings", "abc", "abc"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePasteText(tt.input); got != tt.want {
				t.Fatalf("normalizePasteText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPasteMsg_LineEndingNormalization(t *testing.T) {
	// VTE-based Linux terminals deliver bracketed-paste newlines as CR.
	// The pasted text must be split into buffer lines with no CR left behind.
	content := "{\r  \"keybindings\": {\r    \"save\": [\"ctrl+s\"],\r  }\r}"
	want := "{\n  \"keybindings\": {\n    \"save\": [\"ctrl+s\"],\n  }\n}"

	m := newThemedTestModel(t, "")
	m.focused = true
	model, _ := m.Update(tea.PasteMsg{Content: content})
	m = model.(Model)

	if got := m.buf.String(); got != want {
		t.Fatalf("buffer content = %q, want %q", got, want)
	}
	if strings.ContainsRune(m.buf.String(), '\r') {
		t.Fatal("buffer contains CR character after paste")
	}
	if lc := m.buf.LineCount(); lc != 5 {
		t.Fatalf("LineCount = %d, want 5", lc)
	}
	// Cursor lands at the end of the pasted text.
	if m.cursor.line != 4 || m.cursor.col != 1 {
		t.Fatalf("cursor = %+v, want {line:4 col:1}", m.cursor)
	}
	// Rendered view must not contain raw CR bytes.
	if out := model.(Model).View().Content; strings.ContainsRune(out, '\r') {
		t.Fatal("rendered view contains CR character")
	}
}

func TestPasteMsg_CRLF(t *testing.T) {
	m := newThemedTestModel(t, "")
	m.focused = true
	model, _ := m.Update(tea.PasteMsg{Content: "a\r\nb\r\nc"})
	m = model.(Model)

	if got := m.buf.String(); got != "a\nb\nc" {
		t.Fatalf("buffer content = %q, want %q", got, "a\nb\nc")
	}
	if m.cursor.line != 2 || m.cursor.col != 1 {
		t.Fatalf("cursor = %+v, want {line:2 col:1}", m.cursor)
	}
}
