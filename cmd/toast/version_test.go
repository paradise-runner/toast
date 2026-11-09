package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	revision := func(rev string, modified bool) []debug.BuildSetting {
		dirty := "false"
		if modified {
			dirty = "true"
		}
		return []debug.BuildSetting{
			{Key: "vcs.revision", Value: rev},
			{Key: "vcs.modified", Value: dirty},
		}
	}

	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		want     string
	}{
		{
			name:     "injected release tag wins over build info",
			injected: "v0.14.1",
			info:     &debug.BuildInfo{Main: debug.Module{Version: "v0.13.0"}},
			want:     "v0.14.1",
		},
		{
			name:     "injected release tag alone",
			injected: "v0.14.1",
			info:     nil,
			want:     "v0.14.1",
		},
		{
			name: "go install module version",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.14.1"}},
			want: "v0.14.1",
		},
		{
			name: "pseudo version from a commit install",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.14.2-0.20261109010203-abcdef123456"}},
			want: "v0.14.2-0.20261109010203-abcdef123456",
		},
		{
			name: "working copy with clean revision",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: revision("abcdef1234567890", false),
			},
			want: "devel (abcdef1)",
		},
		{
			name: "working copy with dirty revision",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: revision("abcdef1234567890", true),
			},
			want: "devel (abcdef1, dirty)",
		},
		{
			name: "working copy without vcs stamping",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			want: "devel",
		},
		{
			name: "revision shorter than the reported length",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: revision("abc", false),
			},
			want: "devel (abc)",
		},
		{
			name: "empty module version",
			info: &debug.BuildInfo{},
			want: "devel",
		},
		{
			name: "no build info",
			info: nil,
			want: "devel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.injected, tt.info); got != tt.want {
				t.Fatalf("resolveVersion(%q, %+v) = %q, want %q", tt.injected, tt.info, got, tt.want)
			}
		})
	}
}

// TestCurrentVersionIsNotHardcoded guards the bug that made `go install
// <module>@<version>` report an unrelated old release: a release string baked
// into the source is never correct for a binary the toolchain stamped itself.
func TestCurrentVersionIsNotHardcoded(t *testing.T) {
	got := currentVersion()
	if got == "" {
		t.Fatal("currentVersion() is empty")
	}
	if got == "v0.1.11" {
		t.Fatalf("currentVersion() = %q, the hardcoded release string", got)
	}
}
