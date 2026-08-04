package clipboard

import "testing"

func TestPaste_PrefersSystemClipboard(t *testing.T) {
	internal = "stale-internal"
	SetPasteFunc(func() (string, bool) { return "from-system", true })
	defer SetPasteFunc(nil)

	if got := Paste(); got != "from-system" {
		t.Fatalf("Paste() = %q, want %q", got, "from-system")
	}
	if internal != "from-system" {
		t.Fatalf("internal cache = %q, want %q", internal, "from-system")
	}
}

func TestPaste_FallsBackToInternal(t *testing.T) {
	internal = "from-internal"
	SetPasteFunc(func() (string, bool) { return "", false })
	defer SetPasteFunc(nil)

	if got := Paste(); got != "from-internal" {
		t.Fatalf("Paste() = %q, want %q", got, "from-internal")
	}
}

func TestPaste_EmptySystemClipboardWins(t *testing.T) {
	internal = "stale-internal"
	SetPasteFunc(func() (string, bool) { return "", true })
	defer SetPasteFunc(nil)

	if got := Paste(); got != "" {
		t.Fatalf("Paste() = %q, want empty", got)
	}
}
