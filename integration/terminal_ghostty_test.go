package integration

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const terminalIntegrationEnv = "TOAST_RUN_TERMINAL_INTEGRATION"

func TestGhosttyTmuxTerminalSmoke(t *testing.T) {
	if os.Getenv(terminalIntegrationEnv) != "1" {
		t.Skipf("set %s=1 to run the Ghostty/tmux integration test", terminalIntegrationEnv)
	}
	if runtime.GOOS != "darwin" {
		t.Skip("Ghostty screenshot integration is currently macOS-only")
	}

	openPath := requireCommand(t, "open")
	tmuxPath := requireCommand(t, "tmux")
	screencapturePath := requireCommand(t, "screencapture")
	osascriptPath, _ := exec.LookPath("osascript")
	clangPath, _ := exec.LookPath("clang")

	ghosttyApp := os.Getenv("TOAST_GHOSTTY_APP")
	if ghosttyApp == "" {
		ghosttyApp = "/Applications/Ghostty.app"
	}
	if _, err := os.Stat(ghosttyApp); err != nil {
		t.Skipf("Ghostty app not found at %s; set TOAST_GHOSTTY_APP to override", ghosttyApp)
	}

	repoRoot := repoRoot(t)
	artifacts := artifactDir(t)
	t.Logf("terminal integration artifacts: %s", artifacts)
	windowFinderPath := buildWindowFinder(t, artifacts, clangPath)

	binaryPath := filepath.Join(artifacts, "toast-it")
	run(t, repoRoot, "go", "build", "-o", binaryPath, "./cmd/toast")

	homeDir := filepath.Join(artifacts, "home")
	fixtureDir := filepath.Join(artifacts, "fixture")
	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "toast"), 0o755); err != nil {
		t.Fatalf("creating isolated home: %v", err)
	}
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("creating fixture dir: %v", err)
	}

	id := uniqueID()
	needle := "toastneedle" + id
	editMarker := "typed" + id
	filePath := filepath.Join(fixtureDir, "sample-"+id+".md")
	fileContent := "# Toast Integration\n\n" + needle + "\n"
	if err := os.WriteFile(filePath, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	socketName := "toastit" + id
	sessionName := "toastit"
	targetPane := sessionName + ":0.0"
	tmux := tmuxRunner{path: tmuxPath, socketName: socketName}
	tmux.must(t, "new-session", "-d", "-s", sessionName, "-x", "100", "-y", "34", "-c", fixtureDir, "/bin/sh")
	tmux.must(t, "set-option", "-t", sessionName, "status", "off")
	t.Cleanup(func() {
		_ = tmux.run("kill-server")
	})

	title := "toast integration " + id
	launchGhostty(t, openPath, ghosttyApp, tmuxPath, socketName, sessionName, title, fixtureDir)
	waitForTmuxClient(t, tmux, sessionName, 15*time.Second)

	command := fmt.Sprintf("HOME=%s %s %s", shellQuote(homeDir), shellQuote(binaryPath), shellQuote(filePath))
	tmux.must(t, "send-keys", "-t", targetPane, command, "Enter")
	waitForPane(t, tmux, targetPane, 15*time.Second, filepath.Base(filePath), needle)

	openPane := paneCapture(t, tmux, targetPane)
	writeArtifact(t, filepath.Join(artifacts, "01-opened-pane.txt"), []byte(openPane))
	captureScreenshot(t, screencapturePath, windowFinderPath, osascriptPath, title, filepath.Join(artifacts, "01-opened.png"))

	tmux.must(t, "send-keys", "-t", targetPane, editMarker)
	tmux.must(t, "send-keys", "-t", targetPane, "C-s")
	waitForFile(t, filePath, editMarker, 5*time.Second)

	tmux.must(t, "send-keys", "-t", targetPane, "C-f")
	waitForPane(t, tmux, targetPane, 5*time.Second, "Find / Replace in File", "Find")
	tmux.must(t, "send-keys", "-t", targetPane, needle)
	waitForPane(t, tmux, targetPane, 5*time.Second, "Find / Replace in File", needle, "1/1")

	findPane := paneCapture(t, tmux, targetPane)
	writeArtifact(t, filepath.Join(artifacts, "02-find-pane.txt"), []byte(findPane))
	captureScreenshot(t, screencapturePath, windowFinderPath, osascriptPath, title, filepath.Join(artifacts, "02-find.png"))

	tmux.must(t, "send-keys", "-t", targetPane, "Escape")
	tmux.must(t, "send-keys", "-t", targetPane, "C-q")
}

type tmuxRunner struct {
	path       string
	socketName string
}

func (r tmuxRunner) args(args ...string) []string {
	return append([]string{"-L", r.socketName}, args...)
}

func (r tmuxRunner) run(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.path, r.args(args...)...)
	return cmd.Run()
}

func (r tmuxRunner) must(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.path, r.args(args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (r tmuxRunner) output(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.path, r.args(args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func launchGhostty(t *testing.T, openPath, ghosttyApp, tmuxPath, socketName, sessionName, title, workingDir string) {
	t.Helper()
	initialCommand := strings.Join([]string{
		shellQuote(tmuxPath),
		"-L", shellQuote(socketName),
		"attach-session",
		"-t", shellQuote(sessionName),
	}, " ")

	args := []string{
		"-na", ghosttyApp,
		"--args",
		"--title=" + title,
		"--working-directory=" + workingDir,
		"--initial-command=" + initialCommand,
		"--window-width=100",
		"--window-height=34",
		"--window-save-state=never",
		"--window-inherit-working-directory=false",
		"--tab-inherit-working-directory=false",
		"--window-inherit-font-size=false",
		"--confirm-close-surface=false",
		"--quit-after-last-window-closed=true",
		"--shell-integration=none",
		"--macos-applescript=true",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, openPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launching Ghostty failed: %v\n%s", err, out)
	}
}

func waitForTmuxClient(t *testing.T, tmux tmuxRunner, session string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := tmuxOutput(tmux, "list-clients", "-t", session, "-F", "#{client_name}")
		if err == nil && strings.TrimSpace(out) != "" {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for Ghostty to attach to tmux session %q", session)
}

func waitForPane(t *testing.T, tmux tmuxRunner, target string, timeout time.Duration, want ...string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		out, err := tmuxOutput(tmux, "capture-pane", "-p", "-t", target)
		if err == nil {
			last = out
			if containsAll(out, want) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pane to contain %q\nlast capture:\n%s", want, last)
}

func paneCapture(t *testing.T, tmux tmuxRunner, target string) string {
	t.Helper()
	return tmux.output(t, "capture-pane", "-p", "-t", target)
}

func waitForFile(t *testing.T, path, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last []byte
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			last = data
			if bytes.Contains(data, []byte(want)) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to contain %q\nlast content:\n%s", path, want, last)
}

func tmuxOutput(tmux tmuxRunner, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tmux.path, tmux.args(args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func captureScreenshot(t *testing.T, screencapturePath, windowFinderPath, osascriptPath, title, path string) {
	t.Helper()
	if windowFinderPath != "" {
		windowID, err := ghosttyWindowIDFromCoreGraphics(windowFinderPath, title)
		if err == nil && strings.TrimSpace(windowID) != "" {
			if err := screencaptureWindow(screencapturePath, strings.TrimSpace(windowID), path); err == nil {
				validatePNG(t, path)
				return
			} else {
				t.Logf("window-specific screencapture failed for window id %q: %v", strings.TrimSpace(windowID), err)
			}
		} else if err != nil {
			t.Logf("CoreGraphics window lookup failed for %q: %v", title, err)
		}
	}
	if osascriptPath != "" {
		windowID, err := ghosttyWindowID(osascriptPath, title)
		if err == nil && strings.TrimSpace(windowID) != "" {
			if err := screencaptureWindow(screencapturePath, strings.TrimSpace(windowID), path); err == nil {
				validatePNG(t, path)
				return
			} else {
				t.Logf("window-specific screencapture failed for window id %q: %v", strings.TrimSpace(windowID), err)
			}
		} else if err != nil {
			t.Logf("could not resolve Ghostty window id for %q: %v", title, err)
		}
	}
	if err := screencaptureDisplay(screencapturePath, path); err != nil {
		t.Fatalf("screencapture failed: %v", err)
	}
	validatePNG(t, path)
}

func buildWindowFinder(t *testing.T, artifacts, clangPath string) string {
	t.Helper()
	if clangPath == "" {
		t.Log("clang not found; screenshots will fall back to AppleScript or full-display capture")
		return ""
	}

	sourcePath := filepath.Join(artifacts, "ghostty-window-id.c")
	binaryPath := filepath.Join(artifacts, "ghostty-window-id")
	if err := os.WriteFile(sourcePath, []byte(windowFinderSource), 0o644); err != nil {
		t.Fatalf("writing window finder source: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, clangPath,
		"-framework", "ApplicationServices",
		"-framework", "CoreFoundation",
		"-o", binaryPath,
		sourcePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("compiling CoreGraphics window finder failed; falling back: %v\n%s", err, out)
		return ""
	}
	return binaryPath
}

func ghosttyWindowIDFromCoreGraphics(windowFinderPath, title string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, windowFinderPath, title)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func ghosttyWindowID(osascriptPath, title string) (string, error) {
	script := `
on run argv
  set targetTitle to item 1 of argv
  tell application "System Events"
    repeat with proc in (processes whose bundle identifier is "com.mitchellh.ghostty")
      repeat with w in windows of proc
        set windowTitle to ""
        try
          set windowTitle to name of w as text
        end try
        if windowTitle contains targetTitle then
          try
            return id of w as text
          end try
        end if
      end repeat
    end repeat
  end tell
  return ""
end run
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, osascriptPath, "-e", script, title)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func screencaptureWindow(screencapturePath, windowID, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, screencapturePath, "-x", "-o", "-t", "png", "-l", windowID, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func screencaptureDisplay(screencapturePath, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, screencapturePath, "-x", "-m", "-t", "png", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func validatePNG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening screenshot %s: %v", path, err)
	}
	defer file.Close()

	cfg, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("validating screenshot %s as PNG: %v", path, err)
	}
	if cfg.Width < 100 || cfg.Height < 100 {
		t.Fatalf("screenshot %s is unexpectedly small: %dx%d", path, cfg.Width, cfg.Height)
	}
}

func requireCommand(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not found in PATH", name)
	}
	return path
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolving test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func artifactDir(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("TOAST_TERMINAL_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating artifact dir %s: %v", dir, err)
		}
		return dir
	}
	dir, err := os.MkdirTemp("", "toast-terminal-integration-*")
	if err != nil {
		t.Fatalf("creating artifact dir: %v", err)
	}
	return dir
}

func writeArtifact(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing artifact %s: %v", path, err)
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func uniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

const windowFinderSource = `
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>
#include <stdio.h>

static int cfstring_contains_cstr(CFStringRef value, const char *needle) {
	if (value == NULL || needle == NULL) {
		return 0;
	}
	CFStringRef needleValue = CFStringCreateWithCString(NULL, needle, kCFStringEncodingUTF8);
	if (needleValue == NULL) {
		return 0;
	}
	CFRange found = CFStringFind(value, needleValue, kCFCompareCaseInsensitive);
	CFRelease(needleValue);
	return found.location != kCFNotFound;
}

int main(int argc, char **argv) {
	if (argc != 2) {
		return 2;
	}

	CFArrayRef windows = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (windows == NULL) {
		return 1;
	}

	CFIndex count = CFArrayGetCount(windows);
	for (CFIndex i = 0; i < count; i++) {
		CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);
		CFStringRef owner = (CFStringRef)CFDictionaryGetValue(window, kCGWindowOwnerName);
		CFStringRef name = (CFStringRef)CFDictionaryGetValue(window, kCGWindowName);
		CFNumberRef number = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowNumber);

		if (number == NULL || !cfstring_contains_cstr(owner, "ghostty") || !cfstring_contains_cstr(name, argv[1])) {
			continue;
		}

		uint32_t windowID = 0;
		if (CFNumberGetValue(number, kCFNumberSInt32Type, &windowID)) {
			printf("%u\n", windowID);
			CFRelease(windows);
			return 0;
		}
	}

	CFRelease(windows);
	return 0;
}
`
