package main

import "testing"

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"":             "''",
		"plain":        "'plain'",
		"with space":   "'with space'",
		"save's file":  "'save'\\''s file'",
		"semi;colon":   "'semi;colon'",
		"dollar$value": "'dollar$value'",
	}
	for input, want := range tests {
		if got := shellQuote(input); got != want {
			t.Fatalf("shellQuote(%q) = %q, want %q", input, got, want)
		}
	}
}
