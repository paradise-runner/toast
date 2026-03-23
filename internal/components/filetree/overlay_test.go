package filetree

import (
	"strings"
	"testing"
)

func TestOverlayAt_BasicPosition(t *testing.T) {
	base := "AAAAAAAAAA\nBBBBBBBBBB\nCCCCCCCCCC"
	overlay := "XX\nYY"
	result := overlayAt(base, overlay, 2, 1, 10, 3)
	lines := strings.Split(result, "\n")
	// overlay at x=2, y=1: row 1 gets "BB"+"XX"+"BBBBBB"
	if want := "BBXXBBBBBB"; lines[1] != want {
		t.Errorf("row 1: want %q, got %q", want, lines[1])
	}
	// row 2 gets "CC"+"YY"+"CCCCCC"
	if want := "CCYYCCCCCC"; lines[2] != want {
		t.Errorf("row 2: want %q, got %q", want, lines[2])
	}
}

func TestOverlayAt_ClampsRight(t *testing.T) {
	base := "AAAAAAAAAA\nBBBBBBBBBB"
	overlay := "XXXX" // 4 wide
	// x=8 would overflow 10-wide canvas; x clamped to 10-4=6
	// before = "AAAAAA" (6 A's), ol = "XXXX" (4), after = "" (6+4==10)
	result := overlayAt(base, overlay, 8, 0, 10, 2)
	lines := strings.Split(result, "\n")
	if want := "AAAAAAXXXX"; lines[0] != want {
		t.Errorf("row 0: want %q, got %q", want, lines[0])
	}
}

func TestOverlayAt_ClampsDown(t *testing.T) {
	base := "AAAAAAAAAA\nBBBBBBBBBB"
	overlay := "XX\nYY\nZZ" // 3 tall
	// y=1 with 3-tall overlay overflows 2-row canvas; y clamped to 2-3=-1 → 0
	// row 0 gets "XX"+"AAAAAAAA", row 1 gets "YY"+"BBBBBBBB"; "ZZ" is cut off
	result := overlayAt(base, overlay, 0, 1, 10, 2)
	lines := strings.Split(result, "\n")
	if want := "XXAAAAAAAA"; lines[0] != want {
		t.Errorf("row 0: want %q, got %q", want, lines[0])
	}
	if want := "YYBBBBBBBB"; lines[1] != want {
		t.Errorf("row 1: want %q, got %q", want, lines[1])
	}
}

func TestOverlayAt_OverlayWiderThanCanvas(t *testing.T) {
	base := "AAAAAAAAAA"              // 10 wide
	overlay := "XXXXXXXXXXXXXXXXXXXX" // 20 wide
	// x=0, availWidth=10; overlay truncated to 10 X's; after="" (x+ow==10==blWidth)
	result := overlayAt(base, overlay, 0, 0, 10, 1)
	lines := strings.Split(result, "\n")
	if want := "XXXXXXXXXX"; lines[0] != want {
		t.Errorf("row 0: want %q, got %q", want, lines[0])
	}
}

func TestOverlayAt_NegativeCoordsClamped(t *testing.T) {
	base := "AAAAAAAAAA\nBBBBBBBBBB"
	overlay := "XX"
	// x=-5, ow=2: x+ow=-3 < 10 so no right-clamp; then x=-5 clamped to 0
	// row 0: before="", ol="XX", after="AAAAAAAA" (8 A's from position 2)
	result := overlayAt(base, overlay, -5, 0, 10, 2)
	lines := strings.Split(result, "\n")
	if want := "XXAAAAAAAA"; lines[0] != want {
		t.Errorf("row 0: want %q, got %q", want, lines[0])
	}
}
