package clipboard

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// internal holds the last text copied from within Toast. It is a fallback
// used only when no system clipboard is readable.
var internal string

// pasteFunc reads the current system clipboard contents. ok=false means no
// system clipboard is readable (tool missing, failed, or timed out).
// Overridable via SetPasteFunc for tests.
var pasteFunc = readSystemClipboard

// Copy writes text to the clipboard using OSC 52 escape sequences.
// Also stores in internal clipboard as fallback.
func Copy(text string) {
	internal = text
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\x07", encoded)
}

// Paste returns the system clipboard contents, falling back to the internal
// clipboard only when no system clipboard is readable.
//
// Reading the real system clipboard keeps Ctrl+V and Cmd+V consistent on
// macOS: Cmd+V arrives as a terminal bracketed paste (tea.PasteMsg) carrying
// the system clipboard, while Ctrl+V is handled in-app. Previously Ctrl+V
// returned only the internal fallback, so the two shortcuts could paste
// different text (e.g. stale or empty content after copying in another app).
func Paste() string {
	text, ok := pasteFunc()
	if ok {
		if text != "" {
			internal = text
		}
		return text
	}
	return internal
}

// SetPasteFunc overrides the system-clipboard reader used by Paste. Pass nil
// to restore the platform default. Tests use this to keep clipboard access
// hermetic and side-effect free.
func SetPasteFunc(fn func() (string, bool)) {
	if fn == nil {
		pasteFunc = readSystemClipboard
		return
	}
	pasteFunc = fn
}

// readSystemClipboard shells out to the platform's clipboard tool. ok=true
// even when the clipboard is empty, as long as the tool ran successfully.
func readSystemClipboard() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		return runClipboardTool("pbpaste")
	case "windows":
		return runClipboardTool("powershell.exe", "-NoProfile", "-Command", "Get-Clipboard -Raw")
	default:
		// Wayland first, then X11.
		if text, ok := runClipboardTool("wl-paste", "--no-newline"); ok {
			return text, true
		}
		if text, ok := runClipboardTool("xclip", "-selection", "clipboard", "-o"); ok {
			return text, true
		}
		return runClipboardTool("xsel", "--clipboard", "--output")
	}
}

// runClipboardTool runs a clipboard helper with a short timeout so a missing
// or hung tool (e.g. over SSH) degrades to the internal fallback.
func runClipboardTool(name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
