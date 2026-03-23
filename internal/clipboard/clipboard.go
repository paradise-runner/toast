package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
)

var internal string

// Copy writes text to the clipboard using OSC 52 escape sequences.
// Also stores in internal clipboard as fallback.
func Copy(text string) {
	internal = text
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(os.Stdout, "\x1b]52;c;%s\x07", encoded)
}

// Paste returns the internal clipboard contents.
// OSC 52 read is not universally supported; we rely on internal clipboard
// for paste-within-Toast and terminal paste events for cross-app paste.
func Paste() string {
	return internal
}
