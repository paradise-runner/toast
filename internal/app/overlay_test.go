package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestOverlayCenter_PreservesBaseBackground is a regression test: overlayCenter
// was using a hand-rolled ansiSkip for the "after" region which did not
// correctly restore ANSI state after the overlay, leaving unstyled (dark)
// spaces on rows flanking the dialog.
func TestOverlayCenter_PreservesBaseBackground(t *testing.T) {
	// Build a base "screen" that is 40 columns wide and 7 rows tall, all with
	// a light background color (simulating the editor area on a light theme).
	const (
		bgColor = "#e6e9ef"
		width   = 40
		height  = 7
	)
	baseStyle := lipgloss.NewStyle().Background(lipgloss.Color(bgColor))
	var baseRows []string
	for i := 0; i < height; i++ {
		baseRows = append(baseRows, baseStyle.Width(width).Render(""))
	}
	base := strings.Join(baseRows, "\n")

	// Build a small overlay (10 wide, 3 tall) that has its own background.
	overlayBG := lipgloss.Color("#bcc0cc")
	overlayStyle := lipgloss.NewStyle().Background(overlayBG).Width(10)
	var overlayRows []string
	for i := 0; i < 3; i++ {
		overlayRows = append(overlayRows, overlayStyle.Render("item"))
	}
	overlay := strings.Join(overlayRows, "\n")

	result := overlayCenter(base, overlay, width, height)

	// Every cell in the result must have a background color. We check that no
	// line contains a plain space after an SGR reset with no intervening BG.
	resultLines := strings.Split(result, "\n")
	for i, line := range resultLines {
		if hasUnstyledSpaces(line) {
			t.Errorf("overlayCenter: line %d has unstyled spaces after ANSI reset.\nLine: %q", i, line)
		}
	}
}

// hasUnstyledSpaces returns true if s contains a space character that appears
// after an SGR full-reset (\x1b[m) without an intervening background-set sequence.
func hasUnstyledSpaces(s string) bool {
	hasBG := true
	i := 0
	for i < len(s) {
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
