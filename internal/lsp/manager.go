package lsp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

type openDocument struct {
	path       string
	language   string
	languageID string
	text       string
	version    int
}

// Manager owns one Client per language, starts them lazily, offers opt-in
// managed installation, and bridges results to the Bubble Tea program.
type Manager struct {
	cfg         config.Config
	rootDir     string
	installRoot string
	send        func(tea.Msg)
	mu          sync.Mutex
	clients     map[string]*Client
	documents   map[string]openDocument
	starting    map[string]bool
	installing  map[string]bool
	prompted    map[string]bool
}

// NewManager creates a Manager. send is the Bubble Tea program's Send function.
func NewManager(cfg config.Config, rootDir string, send func(tea.Msg)) *Manager {
	return &Manager{
		cfg:         cfg,
		rootDir:     rootDir,
		installRoot: defaultInstallRoot(),
		send:        send,
		clients:     make(map[string]*Client),
		documents:   make(map[string]openDocument),
		starting:    make(map[string]bool),
		installing:  make(map[string]bool),
		prompted:    make(map[string]bool),
	}
}

// EnsureServer starts the language server if available. When a configured
// server is missing and has an install recipe, it asks the app to prompt once.
// path is the document that triggered the start; its project root is used to
// anchor the language server when the language declares RootMarkers.
func (m *Manager) EnsureServer(language, path string) {
	m.mu.Lock()
	if m.clients[language] != nil || m.starting[language] || m.installing[language] {
		m.mu.Unlock()
		return
	}
	lspCmd, ok := m.cfg.LSP[language]
	if !ok {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// The checks below are not run under the lock: the probe can start an
	// interpreter, and must not block other languages from starting.
	command, found := m.resolveCommand(language, lspCmd)
	if !found {
		m.offerInstall(language, lspCmd, fmt.Sprintf("%s was not found", lspCmd.Command))
		return
	}
	if !m.probeAvailable(language, lspCmd) {
		// The command resolved but the server is not actually installed (e.g.
		// Rscript without the languageserver package).
		m.offerInstall(language, lspCmd, fmt.Sprintf("%s is not installed for %s", lspCmd.Command, language))
		return
	}
	if missing, ok := m.missingRequirement(language, lspCmd); ok {
		m.send(messages.LSPServerStatusMsg{
			Language: language,
			Status:   messages.LSPServerNotFound,
			Message:  fmt.Sprintf("%s requires %s on PATH", lspCmd.Command, missing),
		})
		return
	}

	m.mu.Lock()
	// Another call may have started this server while the checks above ran.
	if m.clients[language] != nil || m.starting[language] || m.installing[language] {
		m.mu.Unlock()
		return
	}
	m.starting[language] = true
	m.mu.Unlock()

	m.startClient(language, command, lspCmd, path)
}

// offerInstall asks the app to run the configured managed install, at most once
// per language. When there is no install recipe, or the prompt was already
// offered, the server is reported as unavailable with reason instead.
func (m *Manager) offerInstall(language string, lspCmd config.LSPCmd, reason string) {
	m.mu.Lock()
	if lspCmd.Install != nil && !m.prompted[language] {
		m.prompted[language] = true
		name := lspCmd.Install.Name
		if name == "" {
			name = lspCmd.Command
		}
		m.mu.Unlock()
		m.send(messages.LSPInstallPromptMsg{Language: language, Name: name})
		return
	}
	m.mu.Unlock()
	if lspCmd.Install == nil {
		m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerNotFound, Message: reason})
	}
}

// probeAvailable reports whether the configured server is actually usable. A
// nil probe means the resolved command is proof enough, which is the case for
// every server installed as an executable.
func (m *Manager) probeAvailable(language string, cmd config.LSPCmd) bool {
	if cmd.Probe == nil {
		return true
	}
	// EnsureServer runs off the UI goroutine, so a probe that has to start an
	// interpreter is safe. The timeout only bounds a hung interpreter.
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	probe := exec.CommandContext(ctx, m.expand(language, cmd.Probe.Command), m.expandArgs(language, cmd.Probe.Args)...)
	return probe.Run() == nil
}

// missingRequirement reports the first entry of cmd.Requires that is not
// resolvable on PATH.
func (m *Manager) missingRequirement(language string, cmd config.LSPCmd) (string, bool) {
	for _, name := range cmd.Requires {
		if name == "" {
			continue
		}
		if _, err := exec.LookPath(m.expand(language, name)); err != nil {
			return name, true
		}
	}
	return "", false
}

func (m *Manager) startClient(language, command string, lspCmd config.LSPCmd, path string) {
	m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerStarting})
	client, err := NewClient(language, command, lspCmd.Args, m.send)
	if err == nil {
		err = client.Initialize(URIFromPath(m.rootDirFor(language, path)))
	}
	if err != nil {
		m.mu.Lock()
		m.starting[language] = false
		m.mu.Unlock()
		m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerCrashed, Message: err.Error()})
		return
	}

	m.mu.Lock()
	m.clients[language] = client
	m.starting[language] = false
	var documents []openDocument
	for _, doc := range m.documents {
		if doc.language == language {
			documents = append(documents, doc)
		}
	}
	m.mu.Unlock()

	for _, doc := range documents {
		_ = client.DidOpen(doc.path, doc.languageID, doc.text)
	}
	m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerReady})
}

// maxInstallOutput caps how much installer output is shown in a failure
// message. Installers (cargo, npm, …) can print megabytes on failure, and a
// giant message would stall the install prompt's renderer.
const maxInstallOutput = 4000

// probeTimeout bounds a language-server availability probe. Probes start an
// interpreter (e.g. Rscript), so they are given room to boot, but a hung
// interpreter must not leave EnsureServer blocked forever.
const probeTimeout = 10 * time.Second

// Install performs the configured opt-in managed installation, then starts the
// server and replays any documents that were opened while it was unavailable.
func (m *Manager) Install(language string) {
	m.mu.Lock()
	lspCmd, ok := m.cfg.LSP[language]
	if !ok || lspCmd.Install == nil || m.installing[language] {
		m.mu.Unlock()
		return
	}
	m.installing[language] = true
	install := *lspCmd.Install
	m.mu.Unlock()

	m.send(messages.LSPInstallStatusMsg{Language: language, Status: messages.LSPInstallRunning})

	var err error
	if install.Download != nil {
		err = m.installFromDownload(language, install)
	} else {
		err = m.runInstallCommand(language, install)
	}
	if err != nil {
		m.installFailed(language, fmt.Errorf("install %s: %w", install.Name, err))
		return
	}

	m.mu.Lock()
	m.installing[language] = false
	m.prompted[language] = false
	m.mu.Unlock()
	m.send(messages.LSPInstallStatusMsg{Language: language, Status: messages.LSPInstallSucceeded})
	m.EnsureServer(language, m.firstDocumentPath(language))
}

// firstDocumentPath returns the path of any open document for language, used
// to resolve a project root when the server starts after an install.
func (m *Manager) firstDocumentPath(language string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, doc := range m.documents {
		if doc.language == language {
			return doc.path
		}
	}
	return ""
}

// runInstallCommand executes a toolchain-based install recipe (e.g. `go
// install`, `npm install --prefix …`) and returns a descriptive error,
// including a capped tail of the installer's output, on failure.
func (m *Manager) runInstallCommand(language string, install config.LSPInstall) error {
	installDir := m.installDir(language)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0o755); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}

	command := m.expand(language, install.Command)
	args := m.expandArgs(language, install.Args)
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	for key, value := range install.Env {
		cmd.Env = append(cmd.Env, key+"="+m.expand(language, value))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if len(detail) > maxInstallOutput {
			detail = "…" + detail[len(detail)-maxInstallOutput:]
		}
		if detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

// installFromDownload downloads a prebuilt language-server archive and,
// depending on the recipe, either installs a single executable into
// {install_dir}/bin or extracts the whole archive to {install_dir}/server.
// A GitHub-style {target} placeholder selects the platform's asset.
func (m *Manager) installFromDownload(language string, install config.LSPInstall) error {
	installDir := m.installDir(language)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create binary directory: %w", err)
	}

	archivePath, err := m.downloadArchive(language, install.Download.URL, installDir)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	// A distribution made of many files (jdtls: bin/, plugins/, config_*) is
	// extracted whole; ManagedCommand then points inside {install_dir}/server.
	if install.Download.Extract {
		serverDir := filepath.Join(installDir, "server")
		if err := os.RemoveAll(serverDir); err != nil {
			return fmt.Errorf("clear %s: %w", serverDir, err)
		}
		if err := extractArchive(archivePath, serverDir); err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
		return nil
	}

	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	entries, err := readArchiveEntries(archive)
	if err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}
	exe, err := pickExecutable(entries)
	if err != nil {
		return err
	}
	binPath := filepath.Join(binDir, exe.name)
	if err := os.WriteFile(binPath, exe.data, 0o755); err != nil {
		return fmt.Errorf("write %s: %w", binPath, err)
	}
	return nil
}

// downloadArchive fetches rawURL (after placeholder expansion) into
// installDir/download.archive and returns its path.
func (m *Manager) downloadArchive(language, rawURL, installDir string) (string, error) {
	url := m.expand(language, rawURL)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	archivePath := filepath.Join(installDir, "download.archive")
	out, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(archivePath)
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	if err := out.Close(); err != nil {
		os.Remove(archivePath)
		return "", fmt.Errorf("close archive: %w", err)
	}
	return archivePath, nil
}

// extractArchive extracts a .tar.gz archive into destDir, preserving the
// archive's directory layout and file modes. Used for language-server
// distributions that consist of many files rather than one executable.
func extractArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	destDir = filepath.Clean(destDir)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := archiveTarget(destDir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode).Perm()
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Only allow relative links that stay inside destDir.
			if filepath.IsAbs(hdr.Linkname) {
				return fmt.Errorf("unsafe absolute symlink %s -> %s", hdr.Name, hdr.Linkname)
			}
			if _, err := archiveTarget(destDir, filepath.Join(filepath.Dir(hdr.Name), hdr.Linkname)); err != nil {
				return fmt.Errorf("unsafe symlink %s -> %s", hdr.Name, hdr.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// Skip hard links, devices, FIFOs and other entry types.
		}
	}
	return nil
}

// archiveTarget joins an archive entry name onto destDir, rejecting entries
// that escape it (the "zip slip" path traversal).
func archiveTarget(destDir, name string) (string, error) {
	target := filepath.Join(destDir, filepath.FromSlash(name))
	if target == destDir {
		return target, nil
	}
	if !strings.HasPrefix(target, destDir+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return target, nil
}

// downloadEntry is one regular file found in a downloaded archive.
type downloadEntry struct {
	name  string // base name of the file
	depth int    // path depth inside the archive
	data  []byte
}

// readArchiveEntries reads the regular files of a .tar.gz archive.
func readArchiveEntries(archive []byte) ([]downloadEntry, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var entries []downloadEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		entries = append(entries, downloadEntry{
			name:  filepath.Base(hdr.Name),
			depth: strings.Count(hdr.Name, "/"),
			data:  content,
		})
	}
	return entries, nil
}

// pickExecutable chooses the binary to install from an extracted archive,
// preferring files at the archive root and tolerating a single wrapping
// directory. Archives with several candidate executables are rejected.
func pickExecutable(entries []downloadEntry) (downloadEntry, error) {
	if len(entries) == 0 {
		return downloadEntry{}, fmt.Errorf("archive contains no executable file")
	}
	minDepth := entries[0].depth
	for _, e := range entries[1:] {
		if e.depth < minDepth {
			minDepth = e.depth
		}
	}
	var shallowest []downloadEntry
	for _, e := range entries {
		if e.depth == minDepth {
			shallowest = append(shallowest, e)
		}
	}
	if len(shallowest) > 1 {
		names := make([]string, len(shallowest))
		for i, e := range shallowest {
			names[i] = e.name
		}
		return downloadEntry{}, fmt.Errorf("archive has multiple executables: %s", strings.Join(names, ", "))
	}
	return shallowest[0], nil
}

func (m *Manager) installFailed(language string, err error) {
	m.mu.Lock()
	m.installing[language] = false
	m.mu.Unlock()
	m.send(messages.LSPInstallStatusMsg{Language: language, Status: messages.LSPInstallFailed, Message: err.Error()})
}

// DidOpen records a document and starts or notifies its language server.
func (m *Manager) DidOpen(path, language, text string) {
	languageID := languageIDForPath(path, language, m.cfg.LSP[language])
	m.mu.Lock()
	m.documents[path] = openDocument{path: path, language: language, languageID: languageID, text: text, version: 1}
	client := m.clients[language]
	m.mu.Unlock()
	if client != nil {
		_ = client.DidOpen(path, languageID, text)
		return
	}
	// No running server yet: the document is already recorded above, so start
	// the server asynchronously. EnsureServer blocks on Initialize (a
	// request/response round-trip that can hang on a slow server), so it must
	// never run on the UI goroutine.
	go m.EnsureServer(language, path)
}

func languageIDForPath(path, language string, cmd config.LSPCmd) string {
	if cmd.LanguageID != "" {
		return cmd.LanguageID
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tsx":
		return "typescriptreact"
	case ".jsx":
		return "javascriptreact"
	case ".txt":
		// harper-ls parses plain text with its prose parser rather than Markdown.
		return "plaintext"
	default:
		return language
	}
}

// DidChange stores and sends a full-document update.
func (m *Manager) DidChange(path, language, text string) {
	m.mu.Lock()
	doc, ok := m.documents[path]
	if !ok {
		m.mu.Unlock()
		m.DidOpen(path, language, text)
		return
	}
	doc.version++
	doc.text = text
	m.documents[path] = doc
	client := m.clients[language]
	m.mu.Unlock()
	if client != nil {
		_ = client.DidChange(path, doc.version, []TextDocumentContentChangeEvent{{Text: text}})
	}
}

// DidClose forgets and closes a document on the appropriate server.
func (m *Manager) DidClose(path, language string) {
	m.mu.Lock()
	delete(m.documents, path)
	client := m.clients[language]
	m.mu.Unlock()
	if client != nil {
		_ = client.DidClose(path)
	}
}

// Completion requests completion items at the given position.
func (m *Manager) Completion(bufferID, generation int, path, language string, line, col int) {
	if c := m.getClient(language); c != nil {
		c.Completion(bufferID, generation, path, line, col, m.protocolCol(path, line, col))
	}
}

// Hover requests hover information at the given position.
func (m *Manager) Hover(bufferID int, path, language string, line, col int) {
	if c := m.getClient(language); c != nil {
		c.Hover(bufferID, path, line, m.protocolCol(path, line, col))
	}
}

// Definition requests the definition location for a symbol.
func (m *Manager) Definition(bufferID int, path, language string, line, col int, navigate bool) {
	if c := m.getClient(language); c != nil {
		c.Definition(bufferID, path, line, col, m.protocolCol(path, line, col), navigate)
		return
	}
	m.send(messages.DefinitionResultMsg{BufferID: bufferID, SourceLine: line, SourceCol: col, Navigate: navigate})
}

// ShutdownAll sends shutdown+exit to every running language server.
func (m *Manager) ShutdownAll() {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()
	for _, c := range clients {
		_ = c.Shutdown()
	}
}

func (m *Manager) getClient(language string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[language]
}

func (m *Manager) protocolCol(path string, line, byteCol int) int {
	m.mu.Lock()
	doc := m.documents[path]
	m.mu.Unlock()
	return byteColToUTF16(lineText(doc.text, line), byteCol)
}

func (m *Manager) resolveCommand(language string, cmd config.LSPCmd) (string, bool) {
	if path, err := exec.LookPath(m.expand(language, cmd.Command)); err == nil {
		return path, true
	}
	if cmd.ManagedCommand != "" {
		if path, err := exec.LookPath(m.expand(language, cmd.ManagedCommand)); err == nil {
			return path, true
		}
	}
	return "", false
}

func (m *Manager) installDir(language string) string {
	return filepath.Join(m.installRoot, safeName(language))
}

// rootDirFor resolves the language server's workspace root for a document.
// When the language declares RootMarkers, the nearest ancestor of path that
// contains one of them wins (so Gradle/Maven subprojects anchor at the build
// root); otherwise the manager's workspace directory is used.
func (m *Manager) rootDirFor(language, path string) string {
	markers := m.cfg.LSP[language].RootMarkers
	if len(markers) == 0 || path == "" {
		return m.rootDir
	}
	if root, ok := projectRoot(filepath.Dir(path), markers); ok {
		return root
	}
	return m.rootDir
}

// projectRoot walks up from dir and returns the first directory containing
// one of markers. It reports false when no marker is found.
func projectRoot(dir string, markers []string) (string, bool) {
	if dir == "" {
		return "", false
	}
	dir = filepath.Clean(dir)
	for {
		for _, marker := range markers {
			if marker == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func (m *Manager) expand(language, value string) string {
	home, _ := os.UserHomeDir()
	return strings.NewReplacer(
		"{install_dir}", m.installDir(language),
		"{install_root}", m.installRoot,
		"{root_dir}", m.rootDir,
		"{home}", home,
		"{target}", rustTargetTriple(),
	).Replace(value)
}

// expandArgs expands placeholders in every argument.
func (m *Manager) expandArgs(language string, values []string) []string {
	args := make([]string, len(values))
	for i, value := range values {
		args[i] = m.expand(language, value)
	}
	return args
}

// rustTargetTriple returns the Rust target triple for the current platform,
// used to download prebuilt language-server binaries from GitHub releases.
// Unsupported platforms return "" and the download then fails with a clear
// HTTP 404.
func rustTargetTriple() string {
	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "aarch64-apple-darwin"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return "x86_64-apple-darwin"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return "aarch64-unknown-linux-gnu"
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "x86_64-unknown-linux-gnu"
	default:
		return ""
	}
}

func defaultInstallRoot() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "toast", "lsp")
	}
	return filepath.Join(os.TempDir(), "toast", "lsp")
}

func safeName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, value)
}

// LanguageForPathConfig returns a configured LSP language for path. Custom
// entries can support any language by declaring filename suffixes/extensions.
func LanguageForPathConfig(path string, configured map[string]config.LSPCmd) string {
	lowerPath := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	languages := make([]string, 0, len(configured))
	for language := range configured {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	for _, language := range languages {
		for _, extension := range configured[language].Extensions {
			extension = strings.ToLower(extension)
			if extension != "" && (strings.HasSuffix(lowerPath, extension) || base == extension) {
				return language
			}
		}
	}
	return LanguageForPath(path)
}

// LanguageForPath returns the built-in LSP language for a common file path.
func LanguageForPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".py"), strings.HasSuffix(lower, ".pyi"):
		return "python"
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"), strings.HasSuffix(lower, ".mts"), strings.HasSuffix(lower, ".cts"):
		return "typescript"
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".jsx"), strings.HasSuffix(lower, ".mjs"), strings.HasSuffix(lower, ".cjs"):
		return "javascript"
	case strings.HasSuffix(lower, ".rs"):
		return "rust"
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"), strings.HasSuffix(lower, ".mdx"), strings.HasSuffix(lower, ".txt"):
		return "markdown"
	case strings.HasSuffix(lower, ".tf"), strings.HasSuffix(lower, ".tfvars"):
		return "terraform"
	default:
		return ""
	}
}

func lineText(text string, line int) string {
	if line < 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return ""
	}
	return lines[line]
}

func byteColToUTF16(line string, col int) int {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	for col > 0 && col < len(line) && !utf8.RuneStart(line[col]) {
		col--
	}
	return len(utf16.Encode([]rune(line[:col])))
}
