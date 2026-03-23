package editor

import (
	"bytes"
	"testing"
)

func TestIsBinary_EmptySlice(t *testing.T) {
	if IsBinary([]byte{}) {
		t.Fatal("expected false for empty slice")
	}
}

func TestIsBinary_PlainText(t *testing.T) {
	if IsBinary([]byte("hello world\n")) {
		t.Fatal("expected false for plain text")
	}
}

func TestIsBinary_UTF8(t *testing.T) {
	if IsBinary([]byte("Héllo wörld\n")) {
		t.Fatal("expected false for valid UTF-8 with no nulls")
	}
}

func TestIsBinary_SingleNullByte(t *testing.T) {
	if !IsBinary([]byte{0x00}) {
		t.Fatal("expected true for single null byte")
	}
}

func TestIsBinary_NullAtLastScannedByte(t *testing.T) {
	// Build binaryScanBytes-byte slice with null at index binaryScanBytes-1 (last byte inside scan window)
	data := bytes.Repeat([]byte("a"), binaryScanBytes)
	data[binaryScanBytes-1] = 0x00
	if !IsBinary(data) {
		t.Fatal("expected true: null at index binaryScanBytes-1 is within the scan window")
	}
}

func TestIsBinary_NullBeyondScanWindow(t *testing.T) {
	// Build (binaryScanBytes+1)-byte slice with null at index binaryScanBytes (first byte outside scan window)
	data := bytes.Repeat([]byte("a"), binaryScanBytes+1)
	data[binaryScanBytes] = 0x00
	if IsBinary(data) {
		t.Fatal("expected false: null at index binaryScanBytes is beyond the scan window")
	}
}

func TestIsBinary_NilSlice(t *testing.T) {
	if IsBinary(nil) {
		t.Fatal("expected false for nil slice")
	}
}

func TestIsBinary_PNGMagicBytes(t *testing.T) {
	// PNG header: \x89 P N G \r \n \x1a \n — contains no null, but real PNGs
	// have nulls shortly after. Use a minimal synthetic PNG-like header with null.
	data := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	if !IsBinary(data) {
		t.Fatal("expected true: PNG-like data with null byte")
	}
}
