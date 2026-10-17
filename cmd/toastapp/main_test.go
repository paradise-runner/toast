package main

import (
	"strings"
	"testing"
)

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

func TestBundleEnvRipgrepPayload(t *testing.T) {
	// The rg payload is absent until the bundle is built. With it, TOAST_RG_PATH
	// must point at the extracted binary; without it, the env must not
	// advertise a non-existent ripgrep.
	env := bundleEnv(t.TempDir(), "/tmp/run-toast")

	joined := strings.Join(env, "\n")
	for _, want := range []string{
		"SHELL=/tmp/run-toast",
		"TOAST_GHOSTTY_BUNDLE=1",
		"COLORTERM=truecolor",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("env missing %q:\n%s", want, joined)
		}
	}

	_, payloadErr := payload.ReadFile(rgPayload)
	hasRG := strings.Contains(joined, "TOAST_RG_PATH=")
	if payloadErr == nil && !hasRG {
		t.Fatalf("rg payload exists but TOAST_RG_PATH is missing:\n%s", joined)
	}
	if payloadErr != nil && hasRG {
		t.Fatalf("no rg payload but TOAST_RG_PATH is set:\n%s", joined)
	}
}
