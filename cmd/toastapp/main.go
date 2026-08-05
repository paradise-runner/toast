// Command toastapp is a self-extracting launcher that bundles toast inside a
// libghostty-based terminal window (Ghostling), producing a standalone desktop
// app from the TUI. It embeds the built toast binary, the Ghostling terminal,
// and libghostty-vt as payloads; at runtime it extracts them to a temporary
// directory, writes a tiny shell wrapper that execs toast, and starts
// Ghostling with SHELL set to that wrapper. Ghostling then opens its window
// and runs toast inside its own pseudo-terminal.
//
// Build the payloads with scripts/build-libghostty-bundle.sh — the payload
// directory only contains a placeholder in the repository.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed payload/*
var payload embed.FS

const (
	ghostlingPayload = "payload/ghostling"
	toastPayload     = "payload/toast"
	dylibPayload     = "payload/libghostty-vt.dylib"
)

// version is the toast version, injected at build time by
// scripts/build-libghostty-bundle.sh (it mirrors cmd/toast's version).
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		var exitErr *exitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.code)
		}
		fmt.Fprintf(os.Stderr, "toast-libghostty: %v\n", err)
		os.Exit(1)
	}
}

type exitCodeError struct {
	code int
}

func (e *exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func run(toastArgs []string) error {
	tmpDir, err := os.MkdirTemp("", "toast-libghostty-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	ghostlingPath, err := extractExecutable(tmpDir, ghostlingPayload, "ghostling")
	if err != nil {
		return err
	}
	toastPath, err := extractExecutable(tmpDir, toastPayload, "toast")
	if err != nil {
		return err
	}
	// Ghostling links libghostty-vt via @rpath; the build script adds an
	// @executable_path rpath, so the dylib must sit next to the ghostling
	// binary for the bundle to run on machines other than the build box.
	if _, err := extractFile(tmpDir, dylibPayload, "libghostty-vt.dylib"); err != nil {
		return err
	}
	wrapperPath, err := writeToastWrapper(tmpDir, toastPath, toastArgs)
	if err != nil {
		return err
	}

	cmd := exec.Command(ghostlingPath)
	cmd.Env = append(os.Environ(),
		"SHELL="+wrapperPath,
		"TOAST_GHOSTTY_BUNDLE=1",
		"TOAST_VERSION="+version,
		"COLORTERM=truecolor",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir, _ = os.Getwd()

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &exitCodeError{code: exitErr.ExitCode()}
		}
		return fmt.Errorf("running bundled ghostling terminal: %w", err)
	}
	return nil
}

func extractExecutable(tmpDir, payloadPath, name string) (string, error) {
	path, err := extractFile(tmpDir, payloadPath, name)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return "", fmt.Errorf("making %s payload executable: %w", name, err)
	}
	return path, nil
}

func extractFile(tmpDir, payloadPath, name string) (string, error) {
	data, err := payload.ReadFile(payloadPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("missing %s payload; run scripts/build-libghostty-bundle.sh", filepath.Base(payloadPath))
		}
		return "", fmt.Errorf("reading %s payload: %w", filepath.Base(payloadPath), err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty %s payload; run scripts/build-libghostty-bundle.sh", filepath.Base(payloadPath))
	}
	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing %s payload: %w", name, err)
	}
	return path, nil
}

func writeToastWrapper(tmpDir, toastPath string, args []string) (string, error) {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("export COLORTERM=truecolor\n")
	b.WriteString("exec ")
	b.WriteString(shellQuote(toastPath))
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(shellQuote(arg))
	}
	b.WriteByte('\n')

	path := filepath.Join(tmpDir, "run-toast")
	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		return "", fmt.Errorf("writing toast wrapper: %w", err)
	}
	return path, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
