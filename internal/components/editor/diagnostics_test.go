package editor

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

// underlinedChars extracts the runes rendered with an underline SGR sequence.
// Lipgloss emits one styled run at a time, e.g. "\x1b[4;58;2;166;227;161;4mq\x1b[m".
func underlinedChars(rendered string) string {
	re := regexp.MustCompile(`\x1b\[4(?:;.*?)?m(.)`)
	var sb strings.Builder
	for _, m := range re.FindAllStringSubmatch(rendered, -1) {
		sb.WriteString(m[1])
	}
	return sb.String()
}

func stripANSI(s string) string { return ansi.Strip(s) }

func TestRenderDiagnosticUnderline(t *testing.T) {
	m := newThemedTestModel(t, "teh quikc brown fox\n")
	updated, _ := m.Update(messages.DiagnosticsUpdatedMsg{
		Path: "test.txt",
		Diagnostics: []messages.Diagnostic{
			{Line: 0, Col: 0, EndLine: 0, EndCol: 3, Severity: 4, Message: "Did you mean 'the'?", Source: "harper"},
			{Line: 0, Col: 4, EndLine: 0, EndCol: 9, Severity: 4, Message: "Did you mean 'quick'?", Source: "harper"},
		},
	})
	m = updated.(Model)

	view := m.View()

	// Underlines must cover exactly the two misspelled words.
	if got := underlinedChars(view.Content); got != "tehquikc" {
		t.Fatalf("underlined chars = %q, want %q", got, "tehquikc")
	}
	// Text itself must be untouched.
	stripped := stripANSI(view.Content)
	if !strings.Contains(stripped, "teh quikc brown fox") {
		t.Fatalf("line text missing from render: %q", stripped)
	}
	// Severity-4 diagnostics use the theme's hint color (#a6e3a1 → 166;227;161).
	if !strings.Contains(view.Content, "166;227;161") {
		t.Fatalf("expected hint underline color in render, got: %q", view.Content)
	}
}

func TestRenderDiagnosticUnderlineRespectsUTF16Columns(t *testing.T) {
	// "wro\u0301ng emoji 😀 word": the combining acute is one UTF-16 unit,
	// the emoji is two, so "😀 word" spans UTF-16 columns 13..20. Spaces are
	// not underlined (lipgloss default), so only the word runes render it.
	m := newThemedTestModel(t, "wro\u0301ng emoji 😀 word\n")
	updated, _ := m.Update(messages.DiagnosticsUpdatedMsg{
		Path: "test.txt",
		Diagnostics: []messages.Diagnostic{
			{Line: 0, Col: 13, EndLine: 0, EndCol: 20, Severity: 4, Message: "spelling", Source: "harper"},
		},
	})
	m = updated.(Model)

	view := m.View()
	if got := underlinedChars(view.Content); got != "😀word" {
		t.Fatalf("underlined chars = %q, want %q", got, "😀word")
	}
}

func TestDiagRangesForLineClampsAndSkips(t *testing.T) {
	m := newTestModel("one\ntwo\n")
	m.diagnostics = []messages.Diagnostic{
		{Line: 0, Col: 0, EndLine: 0, EndCol: 3, Severity: 4},   // "one"
		{Line: 1, Col: 0, EndLine: 1, EndCol: 3, Severity: 4},   // "two"
		{Line: 5, Col: 0, EndLine: 5, EndCol: 3, Severity: 4},   // beyond buffer
		{Line: 0, Col: 0, EndLine: 2, EndCol: 2, Severity: 2},   // spans lines 0-2
		{Line: 0, Col: 10, EndLine: 0, EndCol: 20, Severity: 4}, // beyond line end
	}

	ranges := m.diagRangesForLine(0, 0, 3)
	if len(ranges) != 2 {
		t.Fatalf("line 0: got %d ranges, want 2: %#v", len(ranges), ranges)
	}
	// First diagnostic: bytes 0..3.
	if ranges[0].start != 0 || ranges[0].end != 3 {
		t.Fatalf("line 0 range 0 = [%d,%d), want [0,3)", ranges[0].start, ranges[0].end)
	}
	// Spans-lines diagnostic covers the whole visible slice.
	if ranges[1].start != 0 || ranges[1].end != 3 || ranges[1].severity != 2 {
		t.Fatalf("line 0 range 1 = [%d,%d) sev %d, want [0,3) sev 2", ranges[1].start, ranges[1].end, ranges[1].severity)
	}
	// The beyond-line-end diagnostic is zero-width on this line and skipped.
	// Clamping to a visible sub-slice: only "one" (bytes 0..3) plus the
	// spans-lines diagnostic intersect [1,2).
	ranges = m.diagRangesForLine(0, 1, 2)
	if len(ranges) != 2 {
		t.Fatalf("line 0 slice [1,2): got %d ranges, want 2: %#v", len(ranges), ranges)
	}
	if ranges[0].start != 1 || ranges[0].end != 2 {
		t.Fatalf("slice range = [%d,%d), want [1,2)", ranges[0].start, ranges[0].end)
	}
	if len(m.diagRangesForLine(3, 0, 3)) != 0 {
		t.Fatal("expected no ranges on line 3")
	}
}

func TestHoverShowsDiagnosticMessage(t *testing.T) {
	m := newThemedTestModel(t, "teh quikc brown fox\n")
	updated, _ := m.Update(messages.DiagnosticsUpdatedMsg{
		Path: "test.txt",
		Diagnostics: []messages.Diagnostic{
			{Line: 0, Col: 0, EndLine: 0, EndCol: 3, Severity: 4, Message: "Did you mean 'the'?", Source: "harper"},
			{Line: 0, Col: 4, EndLine: 0, EndCol: 9, Severity: 4, Message: "Did you mean 'quick'?", Source: "harper"},
		},
	})
	m = updated.(Model)

	// Hover over "quikc" (byte col 4..9, gutter is 6 wide).
	updated, _ = m.handleMouseMotion(tea.MouseMotionMsg{X: 6 + 4, Y: 0})
	m = updated.(Model)
	if !m.hover.visible {
		t.Fatal("expected hover tooltip over diagnostic")
	}
	if m.hover.contents != "harper: Did you mean 'quick'?" {
		t.Fatalf("hover contents = %q", m.hover.contents)
	}

	// The tooltip renders into the view.
	view := m.View()
	if !strings.Contains(stripANSI(view.Content), "Did you mean 'quick'?") {
		t.Fatalf("tooltip missing from view: %q", view.Content)
	}

	// Hovering a clean word hides the tooltip.
	updated, _ = m.handleMouseMotion(tea.MouseMotionMsg{X: 6 + 10, Y: 0}) // "brown"
	m = updated.(Model)
	if m.hover.visible {
		t.Fatal("expected tooltip hidden over clean text")
	}
}

func TestHoverShowsDiagnosticOnMultiLineSpan(t *testing.T) {
	m := newThemedTestModel(t, "one\ntwo\n")
	updated, _ := m.Update(messages.DiagnosticsUpdatedMsg{
		Path: "test.txt",
		Diagnostics: []messages.Diagnostic{
			{Line: 0, Col: 0, EndLine: 2, EndCol: 2, Severity: 2, Message: "spans lines", Source: "harper"},
		},
	})
	m = updated.(Model)

	updated, _ = m.handleMouseMotion(tea.MouseMotionMsg{X: 6, Y: 1}) // line 1, col 0
	m = updated.(Model)
	if !m.hover.visible || m.hover.contents != "harper: spans lines" {
		t.Fatalf("expected tooltip on spanned line, got visible=%v contents=%q", m.hover.visible, m.hover.contents)
	}
}

func TestDiagnosticAtRespectsUTF16Columns(t *testing.T) {
	m := newTestModel("wro\u0301ng emoji 😀 word\n")
	m.diagnostics = []messages.Diagnostic{
		{Line: 0, Col: 13, EndLine: 0, EndCol: 20, Severity: 4, Message: "spelling", Source: "harper"},
	}
	// "😀 word" starts at byte 14 (two UTF-16 units for the emoji).
	if diag := m.diagnosticAt(0, 14); diag == nil || diag.Message != "spelling" {
		t.Fatalf("expected diagnostic at byte 14, got %#v", diag)
	}
	if diag := m.diagnosticAt(0, 13); diag != nil { // the space before the emoji
		t.Fatalf("expected no diagnostic at byte 13, got %#v", diag)
	}
	if diag := m.diagnosticAt(0, 23); diag != nil { // past the range end
		t.Fatalf("expected no diagnostic at byte 23, got %#v", diag)
	}
	if diag := m.diagnosticAt(1, 0); diag != nil { // other line
		t.Fatalf("expected no diagnostic on line 1, got %#v", diag)
	}
}

func TestHoverTooltipUsesThemedBorderBackground(t *testing.T) {
	m := newThemedTestModel(t, "teh quikc\n")
	updated, _ := m.Update(messages.DiagnosticsUpdatedMsg{
		Path: "test.txt",
		Diagnostics: []messages.Diagnostic{
			{Line: 0, Col: 0, EndLine: 0, EndCol: 3, Severity: 4, Message: "Did you mean 'the'?", Source: "harper"},
		},
	})
	m = updated.(Model)

	updated, _ = m.handleMouseMotion(tea.MouseMotionMsg{X: 6, Y: 0})
	m = updated.(Model)
	if !m.hover.visible {
		t.Fatal("expected hover tooltip")
	}

	tooltip := renderHover(m.hover, 6, 0, m.viewHeight, m.theme)
	if tooltip == "" {
		t.Fatal("expected rendered tooltip")
	}
	lines := strings.Split(tooltip, "\n")
	// The whole tooltip — border ring and interior — carries the themed hover
	// background (#313244 → 49;50;68 in toast-dark), and the text uses the
	// hover foreground. Corner cells are square; that is an accepted
	// trade-off of filling the ring.
	topBorder := lines[0]
	if !strings.Contains(topBorder, "48;2;49;50;68") {
		t.Fatalf("border ring lost the hover background: %q", topBorder)
	}
	if !strings.Contains(lines[1], "48;2;49;50;68") || !strings.Contains(lines[1], "205;214;244") {
		t.Fatalf("tooltip interior not using hover colors: %q", lines[1])
	}
}

func TestHoverTooltipBoxIsUniformAndTruncatesWithEllipsis(t *testing.T) {
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme setup: %v", err)
	}
	long := "Did you mean to spell `quikc` this way? Suggestions: quick, quake, quark, quirk"
	rendered := renderHover(hoverState{contents: long, visible: true}, 0, 0, 40, tm)

	lines := strings.Split(stripANSI(rendered), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 box lines, got %d: %q", len(lines), lines)
	}
	// All three lines — top border, content, bottom border — must have the
	// same display width (lipgloss MaxWidth previously left the borders
	// wider than the content and chopped the right border).
	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = lipgloss.Width(line)
		if widths[i] != widths[0] {
			t.Fatalf("box lines have different widths: %v", widths)
		}
	}
	// Long content is truncated with an ellipsis, keeping the box intact.
	if !strings.Contains(lines[1], "…") {
		t.Fatalf("expected truncated content with ellipsis, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[1], "│ ") || !strings.HasSuffix(lines[1], " │") {
		t.Fatalf("content line lost its borders: %q", lines[1])
	}
}
