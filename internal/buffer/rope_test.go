package buffer_test

import (
	"strings"
	"testing"

	"github.com/yourusername/toast/internal/buffer"
)

func TestNewRopeFromString(t *testing.T) {
	r := buffer.NewRope("hello\nworld\n")
	if r.Len() != 12 {
		t.Errorf("expected len 12, got %d", r.Len())
	}
	if r.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", r.LineCount())
	}
}

func TestRopeString(t *testing.T) {
	input := "hello\nworld\n"
	r := buffer.NewRope(input)
	if r.String() != input {
		t.Errorf("expected %q, got %q", input, r.String())
	}
}

func TestRopeInsertAtStart(t *testing.T) {
	r := buffer.NewRope("world")
	r.Insert(0, "hello ")
	if r.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", r.String())
	}
}

func TestRopeInsertAtEnd(t *testing.T) {
	r := buffer.NewRope("hello")
	r.Insert(5, " world")
	if r.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", r.String())
	}
}

func TestRopeInsertMiddle(t *testing.T) {
	r := buffer.NewRope("helloworld")
	r.Insert(5, " ")
	if r.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", r.String())
	}
}

func TestRopeDeleteRange(t *testing.T) {
	r := buffer.NewRope("hello world")
	r.Delete(5, 6)
	if r.String() != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", r.String())
	}
}

func TestRopeDeleteAll(t *testing.T) {
	r := buffer.NewRope("hello")
	r.Delete(0, 5)
	if r.String() != "" {
		t.Errorf("expected empty, got %q", r.String())
	}
	if r.Len() != 0 {
		t.Errorf("expected len 0, got %d", r.Len())
	}
}

func TestRopeLineAt(t *testing.T) {
	r := buffer.NewRope("line1\nline2\nline3\n")
	tests := []struct {
		line     int
		expected string
	}{
		{0, "line1\n"},
		{1, "line2\n"},
		{2, "line3\n"},
	}
	for _, tt := range tests {
		got := r.LineAt(tt.line)
		if got != tt.expected {
			t.Errorf("line %d: expected %q, got %q", tt.line, tt.expected, got)
		}
	}
}

func TestRopeLineCount(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"no newline", 1},
		{"one\n", 1},
		{"one\ntwo\n", 2},
		{"one\ntwo\nthree", 3},
	}
	for _, tt := range tests {
		r := buffer.NewRope(tt.input)
		if r.LineCount() != tt.expected {
			t.Errorf("input %q: expected %d lines, got %d", tt.input, tt.expected, r.LineCount())
		}
	}
}

func TestRopeSlice(t *testing.T) {
	r := buffer.NewRope("hello world")
	got := r.Slice(6, 11)
	if got != "world" {
		t.Errorf("expected 'world', got %q", got)
	}
}

func TestRopeLenAfterInsert(t *testing.T) {
	r := buffer.NewRope("hello")
	r.Insert(5, " world")
	if r.Len() != 11 {
		t.Errorf("expected len 11, got %d", r.Len())
	}
}

func TestRopeLenAfterDelete(t *testing.T) {
	r := buffer.NewRope("hello world")
	r.Delete(5, 6)
	if r.Len() != 10 {
		t.Errorf("expected len 10, got %d", r.Len())
	}
}

func TestRopeLargeInserts(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10000; i++ {
		b.WriteString("line\n")
	}
	r := buffer.NewRope(b.String())
	if r.LineCount() != 10000 {
		t.Errorf("expected 10000 lines, got %d", r.LineCount())
	}
}

func TestRopeOffsetForLine(t *testing.T) {
	r := buffer.NewRope("abc\ndef\nghi\n")
	if off := r.OffsetForLine(0); off != 0 {
		t.Errorf("line 0 offset: expected 0, got %d", off)
	}
	if off := r.OffsetForLine(1); off != 4 {
		t.Errorf("line 1 offset: expected 4, got %d", off)
	}
	if off := r.OffsetForLine(2); off != 8 {
		t.Errorf("line 2 offset: expected 8, got %d", off)
	}
}

func TestRopeLineColForOffset(t *testing.T) {
	r := buffer.NewRope("abc\ndef\n")
	line, col := r.LineColForOffset(5)
	if line != 1 || col != 1 {
		t.Errorf("offset 5: expected line=1 col=1, got line=%d col=%d", line, col)
	}
}
