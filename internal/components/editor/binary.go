package editor

const binaryScanBytes = 8192

// IsBinary reports whether data appears to be binary content.
// It scans up to the first binaryScanBytes bytes for a null byte (0x00),
// using the same heuristic as git diff and less(1).
func IsBinary(data []byte) bool {
	n := len(data)
	if n > binaryScanBytes {
		n = binaryScanBytes
	}
	for _, b := range data[:n] {
		if b == 0x00 {
			return true
		}
	}
	return false
}
