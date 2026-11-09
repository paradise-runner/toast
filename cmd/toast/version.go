package main

import (
	"fmt"
	"runtime/debug"
)

// devVersion is reported when neither the ldflags nor the build info name a
// version, which is the case for a plain `go build` without VCS stamping.
const devVersion = "devel"

// revisionLen is the number of commit characters reported for a working-copy
// build: enough to identify the commit, short enough to keep `--version` tidy.
const revisionLen = 7

// version is set at build time for release builds:
//
//	go build -ldflags "-X main.version=v0.14.1" ./cmd/toast
//
// It is deliberately empty otherwise. Hard-coding a release string here made
// every build that sets no ldflags — notably `go install <module>@<version>`,
// which stamps the version into the binary's build info instead — report the
// version of a long-since-released tag it had nothing to do with.
var version = ""

// currentVersion reports the version of this build.
func currentVersion() string {
	info, _ := debug.ReadBuildInfo()
	return resolveVersion(version, info)
}

// resolveVersion picks the most specific version available: the ldflags-injected
// release tag, then the module version that `go install <module>@<version>`
// records, then the commit of a working-copy build, and finally devVersion.
func resolveVersion(injected string, info *debug.BuildInfo) string {
	if injected != "" {
		return injected
	}
	if info == nil {
		return devVersion
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	rev, dirty := vcsState(info.Settings)
	switch {
	case rev == "":
		return devVersion
	case dirty:
		return fmt.Sprintf("%s (%s, dirty)", devVersion, rev)
	default:
		return fmt.Sprintf("%s (%s)", devVersion, rev)
	}
}

// vcsState returns the commit stamped into the build and whether the working
// tree had uncommitted changes when it was built.
func vcsState(settings []debug.BuildSetting) (rev string, dirty bool) {
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
			if len(rev) > revisionLen {
				rev = rev[:revisionLen]
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return rev, dirty
}
