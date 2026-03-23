package lsp

import (
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

// Manager owns one Client per language, starts them lazily, and bridges
// them to the Bubbletea program via the send callback.
type Manager struct {
	cfg     config.Config
	rootDir string
	send    func(tea.Msg)
	mu      sync.Mutex
	clients map[string]*Client
}

// NewManager creates a Manager. send is the Bubbletea program's Send function.
func NewManager(cfg config.Config, rootDir string, send func(tea.Msg)) *Manager {
	return &Manager{
		cfg:     cfg,
		rootDir: rootDir,
		send:    send,
		clients: make(map[string]*Client),
	}
}

// EnsureServer starts the language server for the given language if it is not
// already running. It is safe to call from any goroutine.
func (m *Manager) EnsureServer(language string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.clients[language]; ok {
		return
	}

	lspCmd, ok := m.cfg.LSP[language]
	if !ok {
		return
	}

	m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerStarting})

	client, err := NewClient(language, lspCmd.Command, lspCmd.Args, m.send)
	if err != nil {
		m.send(messages.LSPServerStatusMsg{
			Language: language,
			Status:   messages.LSPServerNotFound,
			Message:  err.Error(),
		})
		return
	}

	if err := client.Initialize("file://" + m.rootDir); err != nil {
		m.send(messages.LSPServerStatusMsg{
			Language: language,
			Status:   messages.LSPServerCrashed,
			Message:  fmt.Sprintf("initialize failed: %v", err),
		})
		return
	}

	m.clients[language] = client
	m.send(messages.LSPServerStatusMsg{Language: language, Status: messages.LSPServerReady})
}

// DidOpen notifies the appropriate language server that a document was opened.
func (m *Manager) DidOpen(path, language, text string) {
	m.EnsureServer(language)
	c := m.getClient(language)
	if c != nil {
		c.DidOpen(path, language, text)
	}
}

// DidChange notifies the appropriate language server of document changes.
func (m *Manager) DidChange(path, language string, version int, changes []TextDocumentContentChangeEvent) {
	c := m.getClient(language)
	if c != nil {
		c.DidChange(path, version, changes)
	}
}

// DidClose notifies the appropriate language server that a document was closed.
func (m *Manager) DidClose(path, language string) {
	c := m.getClient(language)
	if c != nil {
		c.DidClose(path)
	}
}

// Completion requests completion items at the given position.
func (m *Manager) Completion(bufferID int, path, language string, line, col int) {
	c := m.getClient(language)
	if c != nil {
		c.Completion(bufferID, path, line, col)
	}
}

// Hover requests hover information at the given position.
func (m *Manager) Hover(bufferID int, path, language string, line, col int) {
	c := m.getClient(language)
	if c != nil {
		c.Hover(bufferID, path, line, col)
	}
}

// Definition requests the definition location for the symbol at the given position.
func (m *Manager) Definition(path, language string, line, col int) {
	c := m.getClient(language)
	if c != nil {
		c.Definition(path, line, col)
	}
}

// ShutdownAll sends shutdown+exit to every running language server.
func (m *Manager) ShutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Shutdown()
	}
}

// getClient returns the Client for lang, or nil if none exists.
func (m *Manager) getClient(lang string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[lang]
}

// LanguageForPath returns the LSP language identifier for the given file path,
// or an empty string if the language is not recognised.
func LanguageForPath(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".py"):
		return "python"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return "typescript"
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".mjs"):
		return "javascript"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	default:
		return ""
	}
}
