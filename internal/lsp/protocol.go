package lsp

import "strings"

// RequestMessage is a JSON-RPC 2.0 request.
type RequestMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ResponseMessage is a JSON-RPC 2.0 response.
type ResponseMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// NotificationMessage is a JSON-RPC 2.0 notification (no ID).
type NotificationMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ResponseError is a JSON-RPC 2.0 error object.
type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Position is a zero-based line/character offset in a text document.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a span within a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextDocumentIdentifier identifies a text document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is a text document sent to the server on open.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// VersionedTextDocumentIdentifier identifies a versioned text document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentContentChangeEvent describes a change to a text document.
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

// DiagnosticSeverity constants as defined by the LSP specification.
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

// LSPDiagnostic is a diagnostic reported by the language server.
type LSPDiagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
}

// PublishDiagnosticsParams are the params for textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []LSPDiagnostic `json:"diagnostics"`
}

// CompletionParams are the params for textDocument/completion.
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// LSPCompletionItem is a single completion item returned by the server.
type LSPCompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// HoverParams are the params for textDocument/hover.
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// HoverResult is the result of a textDocument/hover request.
type HoverResult struct {
	Contents interface{} `json:"contents"`
	Range    *Range      `json:"range,omitempty"`
}

// DefinitionParams are the params for textDocument/definition.
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Location is a location inside a resource, such as a line inside a text file.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// URIFromPath converts a filesystem path to a file:// URI.
func URIFromPath(path string) string {
	return "file://" + path
}

// PathFromURI converts a file:// URI back to a filesystem path.
func PathFromURI(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}
