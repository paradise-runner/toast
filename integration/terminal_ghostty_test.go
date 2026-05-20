package integration

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/toast/internal/config"
)

const (
	terminalIntegrationEnv = "TOAST_RUN_TERMINAL_INTEGRATION"
	updateGoldensEnv       = "TOAST_UPDATE_GOLDENS"
	goldenDirName          = "ghostty"
	fixtureWorkspaceDir    = "/private/tmp/toast-fixture"
	screenshotInset        = 8
	// tmux send-keys bypasses Ghostty's real keyboard handling, so we inject
	// Kitty CSI-u sequences directly for Ctrl+Shift+<letter> shortcuts.
	kittyCtrlShiftMod = 6
)

type terminalHarness struct {
	repoRoot          string
	artifacts         string
	binaryPath        string
	homeDir           string
	fixtureDir        string
	screencapturePath string
	windowFinderPath  string
	osascriptPath     string
	title             string
	targetPane        string
	tmux              tmuxRunner
}

func TestGhosttyTmuxOpenFileFromFileTree(t *testing.T) {
	h := newTerminalHarness(t)

	filePath := filepath.Join(h.fixtureDir, "alpha.txt")
	fileContent := "opened from tree\n"
	if err := os.WriteFile(filePath, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	h.launchToast(t, h.fixtureDir)
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, filepath.Base(h.fixtureDir), filepath.Base(filePath))

	h.tmux.sendCtrlShiftLetter(t, h.targetPane, 'e')
	h.tmux.sendKeys(t, h.targetPane, "Down", "Enter")
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, filepath.Base(filePath), strings.TrimSpace(fileContent))

	treePane := paneCapture(t, h.tmux, h.targetPane)
	writeArtifact(t, filepath.Join(h.artifacts, "tree-open-pane.txt"), []byte(treePane))
	treeScreenshotPath := filepath.Join(h.artifacts, "03-tree-open.png")
	captureScreenshot(t, h.screencapturePath, h.windowFinderPath, h.osascriptPath, h.title, treeScreenshotPath)
	assertGoldenScreenshot(t, h.repoRoot, h.artifacts, "03-tree-open", treeScreenshotPath)

	h.quit(t)
}

func TestGhosttyTmuxProjectSearchOpensResult(t *testing.T) {
	h := newTerminalHarness(t)

	needle := "journeysearchneedle"
	matchPath := filepath.Join(h.fixtureDir, "match.txt")
	matchContent := "alpha\n" + needle + "\nomega\n"
	if err := os.WriteFile(matchPath, []byte(matchContent), 0o644); err != nil {
		t.Fatalf("writing search fixture file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(h.fixtureDir, "other.txt"), []byte("no match here\n"), 0o644); err != nil {
		t.Fatalf("writing non-match fixture file: %v", err)
	}

	h.launchToast(t, h.fixtureDir)
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, filepath.Base(h.fixtureDir), filepath.Base(matchPath))

	h.tmux.sendCtrlShiftLetter(t, h.targetPane, 'f')
	waitForPane(t, h.tmux, h.targetPane, 5*time.Second, "Search", "Search...")

	h.tmux.sendLiteral(t, h.targetPane, needle)
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, "Search", "match.txt:2", needle)

	searchPane := paneCapture(t, h.tmux, h.targetPane)
	writeArtifact(t, filepath.Join(h.artifacts, "search-result-pane.txt"), []byte(searchPane))
	searchScreenshotPath := filepath.Join(h.artifacts, "04-search-result.png")
	captureScreenshot(t, h.screencapturePath, h.windowFinderPath, h.osascriptPath, h.title, searchScreenshotPath)
	assertGoldenScreenshot(t, h.repoRoot, h.artifacts, "04-search-result", searchScreenshotPath)

	h.tmux.sendKeys(t, h.targetPane, "Enter")
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, filepath.Base(matchPath), needle)

	h.quit(t)
}

func TestGhosttyTmuxTerminalSmoke(t *testing.T) {
	h := newTerminalHarness(t)
	needle := "toastneedle"
	editMarker := "typed "
	filePath := filepath.Join(h.fixtureDir, "sample.md")
	fileContent := "# Toast Integration\n\n" + needle + "\n"
	if err := os.WriteFile(filePath, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	h.launchToast(t, filePath)
	waitForPane(t, h.tmux, h.targetPane, 15*time.Second, filepath.Base(filePath), needle)

	openPane := paneCapture(t, h.tmux, h.targetPane)
	writeArtifact(t, filepath.Join(h.artifacts, "01-opened-pane.txt"), []byte(openPane))
	openScreenshotPath := filepath.Join(h.artifacts, "01-opened.png")
	captureScreenshot(t, h.screencapturePath, h.windowFinderPath, h.osascriptPath, h.title, openScreenshotPath)
	assertGoldenScreenshot(t, h.repoRoot, h.artifacts, "01-opened", openScreenshotPath)

	h.tmux.sendKeys(t, h.targetPane, editMarker, "C-s")
	waitForFile(t, filePath, editMarker, 5*time.Second)

	h.tmux.sendKeys(t, h.targetPane, "C-f")
	waitForPane(t, h.tmux, h.targetPane, 5*time.Second, "Find / Replace in File", "Find")
	h.tmux.sendKeys(t, h.targetPane, needle)
	waitForPane(t, h.tmux, h.targetPane, 5*time.Second, "Find / Replace in File", needle, "1/1")

	findPane := paneCapture(t, h.tmux, h.targetPane)
	writeArtifact(t, filepath.Join(h.artifacts, "02-find-pane.txt"), []byte(findPane))
	findScreenshotPath := filepath.Join(h.artifacts, "02-find.png")
	captureScreenshot(t, h.screencapturePath, h.windowFinderPath, h.osascriptPath, h.title, findScreenshotPath)
	assertGoldenScreenshot(t, h.repoRoot, h.artifacts, "02-find", findScreenshotPath)

	h.quit(t)
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

func (r tmuxRunner) sendKeys(t *testing.T, target string, keys ...string) {
	t.Helper()
	args := []string{"send-keys", "-t", target}
	args = append(args, keys...)
	r.must(t, args...)
}

func (r tmuxRunner) sendLiteral(t *testing.T, target, value string) {
	t.Helper()
	r.must(t, "send-keys", "-l", "-t", target, value)
}

func (r tmuxRunner) sendCtrlShiftLetter(t *testing.T, target string, letter rune) {
	t.Helper()
	r.sendLiteral(t, target, fmt.Sprintf("\x1b[%d;%du", letter, kittyCtrlShiftMod))
}

func newTerminalHarness(t *testing.T) terminalHarness {
	t.Helper()
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
	fixtureDir := fixtureWorkspaceDir
	writeToastConfig(t, homeDir)
	writeGhosttyConfig(t, artifacts)
	resetFixtureDir(t, fixtureDir)

	socketName := "toastit" + uniqueID()
	sessionName := "toastit"
	targetPane := sessionName + ":0.0"
	tmux := tmuxRunner{path: tmuxPath, socketName: socketName}
	tmux.must(t, "new-session", "-d", "-s", sessionName, "-x", "100", "-y", "34", "-c", fixtureDir, "/bin/zsh", "-i")
	tmux.must(t, "set-option", "-t", sessionName, "status", "off")
	t.Cleanup(func() {
		_ = tmux.run("kill-server")
	})

	title := "toast integration " + uniqueID()
	ghosttyConfigPath := filepath.Join(artifacts, "ghostty.config")
	launchGhostty(t, openPath, ghosttyApp, ghosttyConfigPath, tmuxPath, socketName, sessionName, title, fixtureDir)
	waitForTmuxClient(t, tmux, sessionName, 15*time.Second)

	return terminalHarness{
		repoRoot:          repoRoot,
		artifacts:         artifacts,
		binaryPath:        binaryPath,
		homeDir:           homeDir,
		fixtureDir:        fixtureDir,
		screencapturePath: screencapturePath,
		windowFinderPath:  windowFinderPath,
		osascriptPath:     osascriptPath,
		title:             title,
		targetPane:        targetPane,
		tmux:              tmux,
	}
}

func (h terminalHarness) launchToast(t *testing.T, args ...string) {
	t.Helper()
	h.tmux.sendKeys(t, h.targetPane, h.toastCommand(args...), "Enter")
}

func (h terminalHarness) launchToastTracked(t *testing.T, exitMarker string, args ...string) {
	t.Helper()
	command := h.toastCommand(args...)
	command += "; printf '%s\\n' " + shellQuote(exitMarker)
	h.tmux.sendKeys(t, h.targetPane, command, "Enter")
}

func (h terminalHarness) quit(t *testing.T) {
	t.Helper()
	h.tmux.sendKeys(t, h.targetPane, "Escape", "C-q")
}

func (h terminalHarness) toastCommand(args ...string) string {
	command := fmt.Sprintf("HOME=%s %s", shellQuote(h.homeDir), shellQuote(h.binaryPath))
	for _, arg := range args {
		command += " " + shellQuote(arg)
	}
	return command
}

func launchGhostty(t *testing.T, openPath, ghosttyApp, ghosttyConfigPath, tmuxPath, socketName, sessionName, title, workingDir string) {
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
		"--config-default-files=false",
		"--config-file=" + ghosttyConfigPath,
		"--title=" + title,
		"--working-directory=" + workingDir,
		"--initial-command=" + initialCommand,
		"--window-width=100",
		"--window-height=34",
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

func assertGoldenScreenshot(t *testing.T, repoRoot, artifacts, name, actualPath string) {
	t.Helper()
	goldenDir := filepath.Join(repoRoot, "integration", "testdata", goldenDirName)
	goldenPath := filepath.Join(goldenDir, name+".png")
	if os.Getenv(updateGoldensEnv) == "1" {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("creating golden dir %s: %v", goldenDir, err)
		}
		copyFile(t, actualPath, goldenPath)
		t.Logf("updated golden screenshot: %s", goldenPath)
		return
	}

	expected, err := decodePNG(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden screenshot missing: %s (rerun with %s=1 to create it)", goldenPath, updateGoldensEnv)
		}
		t.Fatalf("reading golden screenshot %s: %v", goldenPath, err)
	}
	actual, err := decodePNG(actualPath)
	if err != nil {
		t.Fatalf("reading actual screenshot %s: %v", actualPath, err)
	}

	expectedRawBounds := expected.Bounds()
	actualRawBounds := actual.Bounds()
	expected, actual = normalizeForComparison(t, name, expected, actual)

	expectedBounds := expected.Bounds()
	diffPixels, diffImage := diffImages(expected, actual)
	if diffPixels == 0 {
		return
	}

	diffPath := filepath.Join(artifacts, name+".diff.png")
	writePNG(t, diffPath, diffImage)
	totalPixels := expectedBounds.Dx() * expectedBounds.Dy()
	t.Fatalf("golden screenshot mismatch for %s: %d/%d pixels differ after normalization; expected_raw=%v actual_raw=%v actual=%s diff=%s", name, diffPixels, totalPixels, expectedRawBounds, actualRawBounds, actualPath, diffPath)
}

func decodePNG(path string) (*image.NRGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	src, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst, nil
}

func normalizeForComparison(t *testing.T, name string, expected, actual *image.NRGBA) (*image.NRGBA, *image.NRGBA) {
	t.Helper()
	expected = insetCrop(t, name, "expected", expected, screenshotInset)
	actual = insetCrop(t, name, "actual", actual, screenshotInset)

	width := minInt(expected.Bounds().Dx(), actual.Bounds().Dx())
	height := minInt(expected.Bounds().Dy(), actual.Bounds().Dy())
	if width < 1 || height < 1 {
		t.Fatalf("normalized screenshot for %s is too small: expected=%v actual=%v", name, expected.Bounds(), actual.Bounds())
	}
	return cropToSize(expected, width, height), cropToSize(actual, width, height)
}

func insetCrop(t *testing.T, name, label string, img *image.NRGBA, inset int) *image.NRGBA {
	t.Helper()
	bounds := img.Bounds()
	if bounds.Dx() <= inset*2 || bounds.Dy() <= inset*2 {
		t.Fatalf("%s screenshot for %s is too small to crop by %d pixels: %v", label, name, inset, bounds)
	}
	rect := image.Rect(bounds.Min.X+inset, bounds.Min.Y+inset, bounds.Max.X-inset, bounds.Max.Y-inset)
	return cropImage(img, rect)
}

func cropToSize(img *image.NRGBA, width, height int) *image.NRGBA {
	bounds := img.Bounds()
	rect := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+width, bounds.Min.Y+height)
	return cropImage(img, rect)
}

func cropImage(img *image.NRGBA, rect image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, rect.Min, draw.Src)
	return dst
}

func diffImages(expected, actual *image.NRGBA) (int, *image.NRGBA) {
	bounds := expected.Bounds()
	diff := image.NewNRGBA(bounds)
	diffPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			expectedPixel := expected.NRGBAAt(x, y)
			actualPixel := actual.NRGBAAt(x, y)
			if expectedPixel == actualPixel {
				diff.SetNRGBA(x, y, dimPixel(expectedPixel))
				continue
			}
			diffPixels++
			diff.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 255, A: 255})
		}
	}
	return diffPixels, diff
}

func dimPixel(pixel color.NRGBA) color.NRGBA {
	return color.NRGBA{
		R: pixel.R / 3,
		G: pixel.G / 3,
		B: pixel.B / 3,
		A: 255,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating png %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("writing png %s: %v", path, err)
	}
}

func copyFile(t *testing.T, srcPath, dstPath string) {
	t.Helper()
	src, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("opening %s: %v", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		t.Fatalf("creating %s: %v", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		t.Fatalf("copying %s to %s: %v", srcPath, dstPath, err)
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

func resetFixtureDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("resetting fixture dir %s: %v", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating fixture dir %s: %v", dir, err)
	}
}

func writeGhosttyConfig(t *testing.T, artifacts string) {
	t.Helper()
	configPath := filepath.Join(artifacts, "ghostty.config")
	content := strings.Join([]string{
		"font-family = Menlo",
		"font-size = 13",
		"cursor-style = block",
		"cursor-style-blink = false",
		"background-opacity = 1",
		"window-decoration = auto",
		"window-padding-x = 2",
		"window-padding-y = 2",
		"window-position-x = 80",
		"window-position-y = 80",
		"window-save-state = never",
		"window-inherit-working-directory = false",
		"tab-inherit-working-directory = false",
		"window-inherit-font-size = false",
		"confirm-close-surface = false",
		"quit-after-last-window-closed = true",
		"shell-integration = none",
		"macos-titlebar-style = hidden",
		"macos-window-shadow = false",
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing ghostty config %s: %v", configPath, err)
	}
}

func writeToastConfig(t *testing.T, homeDir string) {
	t.Helper()
	cfg := config.Config{
		Theme: "toast-dark",
		Editor: config.EditorConfig{
			TabWidth:                     4,
			WordWrap:                     false,
			ShowWhitespace:               false,
			AutoIndent:                   true,
			TrimTrailingWhitespaceOnSave: true,
			InsertFinalNewlineOnSave:     true,
		},
		Sidebar: config.SidebarConfig{
			Visible:       true,
			Width:         30,
			ConfirmDelete: true,
		},
		LSP: map[string]config.LSPCmd{},
		Search: config.SearchConfig{
			Command: "rg",
			Args:    []string{"--json"},
		},
		IgnoredPatterns: []string{".git", "node_modules", "__pycache__", ".DS_Store"},
	}
	configPath := filepath.Join(homeDir, ".config", "toast", "config.json")
	if err := config.Save(cfg, configPath); err != nil {
		t.Fatalf("writing toast config %s: %v", configPath, err)
	}
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
