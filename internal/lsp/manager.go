package lsp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
func (m *Manager) EnsureServer(language string) {
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
	command, found := m.resolveCommand(language, lspCmd)
	if !found {
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
			m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerNotFound, Message: fmt.Sprintf("%s was not found", lspCmd.Command)})
		}
		return
	}
	m.starting[language] = true
	m.mu.Unlock()

	m.startClient(language, command, lspCmd)
}

func (m *Manager) startClient(language, command string, lspCmd config.LSPCmd) {
	m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerStarting})
	client, err := NewClient(language, command, lspCmd.Args, m.send)
	if err == nil {
		err = client.Initialize(URIFromPath(m.rootDir))
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
	installDir := m.installDir(language)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		m.installFailed(language, fmt.Errorf("create install directory: %w", err))
		return
	}
	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0o755); err != nil {
		m.installFailed(language, fmt.Errorf("create binary directory: %w", err))
		return
	}

	command := m.expand(language, install.Command)
	args := make([]string, len(install.Args))
	for i, arg := range install.Args {
		args[i] = m.expand(language, arg)
	}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	for key, value := range install.Env {
		cmd.Env = append(cmd.Env, key+"="+m.expand(language, value))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
		m.installFailed(language, fmt.Errorf("install %s: %w", install.Name, err))
		return
	}

	m.mu.Lock()
	m.installing[language] = false
	m.prompted[language] = false
	m.mu.Unlock()
	m.send(messages.LSPInstallStatusMsg{Language: language, Status: messages.LSPInstallSucceeded})
	m.EnsureServer(language)
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
	m.EnsureServer(language)
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
func (m *Manager) Completion(bufferID int, path, language string, line, col int) {
	if c := m.getClient(language); c != nil {
		c.Completion(bufferID, path, line, m.protocolCol(path, line, col))
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

func (m *Manager) expand(language, value string) string {
	home, _ := os.UserHomeDir()
	return strings.NewReplacer(
		"{install_dir}", m.installDir(language),
		"{install_root}", m.installRoot,
		"{root_dir}", m.rootDir,
		"{home}", home,
	).Replace(value)
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
