package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/yourusername/toast/internal/messages"
)

// Client manages a single language server subprocess over JSON-RPC 2.0.
type Client struct {
	language string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	mu       sync.Mutex
	nextID   int
	pending  map[int]chan json.RawMessage
	send     func(tea.Msg)
}

// NewClient starts the language server subprocess and returns a ready Client.
// The send function is used to dispatch Bubble Tea messages to the application.
func NewClient(language, command string, args []string, send func(tea.Msg)) (*Client, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		send(messages.LSPServerStatusMsg{
			Language: language,
			Status:   messages.LSPServerNotFound,
			Message:  err.Error(),
		})
		return nil, fmt.Errorf("lsp: start %q: %w", command, err)
	}

	send(messages.LSPServerStatusMsg{
		Language: language,
		Status:   messages.LSPServerStarting,
	})

	c := &Client{
		language: language,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdoutPipe),
		nextID:   1,
		pending:  make(map[int]chan json.RawMessage),
		send:     send,
	}

	go c.readLoop()

	return c, nil
}

// Initialize sends the LSP initialize request followed by the initialized notification.
func (c *Client) Initialize(rootURI string) error {
	params := map[string]interface{}{
		"rootUri": rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"documentationFormat": []string{"plaintext", "markdown"},
					},
				},
				"hover": map[string]interface{}{
					"contentFormat": []string{"plaintext", "markdown"},
				},
				"publishDiagnostics": map[string]interface{}{},
			},
		},
	}

	ctx := context.Background()
	_, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("lsp: initialize: %w", err)
	}

	if err := c.notify("initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("lsp: initialized notification: %w", err)
	}

	c.send(messages.LSPServerStatusMsg{
		Language: c.language,
		Status:   messages.LSPServerReady,
	})

	return nil
}

// DidOpen notifies the server that a document has been opened.
func (c *Client) DidOpen(path, languageID, text string) error {
	return c.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": TextDocumentItem{
			URI:        URIFromPath(path),
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
}

// DidChange notifies the server of incremental or full document changes.
func (c *Client) DidChange(path string, version int, changes []TextDocumentContentChangeEvent) error {
	return c.notify("textDocument/didChange", map[string]interface{}{
		"textDocument": VersionedTextDocumentIdentifier{
			URI:     URIFromPath(path),
			Version: version,
		},
		"contentChanges": changes,
	})
}

// DidClose notifies the server that a document has been closed.
func (c *Client) DidClose(path string) error {
	return c.notify("textDocument/didClose", map[string]interface{}{
		"textDocument": TextDocumentIdentifier{URI: URIFromPath(path)},
	})
}

// Completion requests completion items at the given position.
// The result is dispatched asynchronously via the send function.
func (c *Client) Completion(bufferID int, path string, line, col int) {
	go func() {
		params := CompletionParams{
			TextDocument: TextDocumentIdentifier{URI: URIFromPath(path)},
			Position:     Position{Line: line, Character: col},
		}
		raw, err := c.call(context.Background(), "textDocument/completion", params)
		if err != nil {
			return
		}

		// The result may be CompletionList or []CompletionItem.
		var itemList []LSPCompletionItem
		// Try array first.
		if err := json.Unmarshal(raw, &itemList); err != nil {
			// Try CompletionList.
			var list struct {
				Items []LSPCompletionItem `json:"items"`
			}
			if err2 := json.Unmarshal(raw, &list); err2 != nil {
				return
			}
			itemList = list.Items
		}

		result := messages.CompletionResultMsg{BufferID: bufferID}
		for _, item := range itemList {
			result.Items = append(result.Items, messages.CompletionItem{
				Label:         item.Label,
				Kind:          item.Kind,
				Detail:        item.Detail,
				Documentation: item.Documentation,
				InsertText:    item.InsertText,
			})
		}
		c.send(result)
	}()
}

// Hover requests hover information at the given position.
// The result is dispatched asynchronously via the send function.
func (c *Client) Hover(bufferID int, path string, line, col int) {
	go func() {
		params := HoverParams{
			TextDocument: TextDocumentIdentifier{URI: URIFromPath(path)},
			Position:     Position{Line: line, Character: col},
		}
		raw, err := c.call(context.Background(), "textDocument/hover", params)
		if err != nil {
			return
		}

		var result HoverResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return
		}

		// Extract plain text from the contents field (string or MarkupContent).
		contents := ""
		switch v := result.Contents.(type) {
		case string:
			contents = v
		case map[string]interface{}:
			if val, ok := v["value"].(string); ok {
				contents = val
			}
		}

		msg := messages.HoverResultMsg{
			BufferID: bufferID,
			Contents: contents,
		}
		if result.Range != nil {
			msg.RangeLine = result.Range.Start.Line
			msg.RangeCol = result.Range.Start.Character
			msg.RangeEndLine = result.Range.End.Line
			msg.RangeEndCol = result.Range.End.Character
		}
		c.send(msg)
	}()
}

// Definition requests the definition location for the symbol at the given position.
// The result is dispatched asynchronously via the send function.
func (c *Client) Definition(path string, line, col int) {
	go func() {
		params := DefinitionParams{
			TextDocument: TextDocumentIdentifier{URI: URIFromPath(path)},
			Position:     Position{Line: line, Character: col},
		}
		raw, err := c.call(context.Background(), "textDocument/definition", params)
		if err != nil {
			return
		}

		// Result may be Location or []Location.
		var loc Location
		if err := json.Unmarshal(raw, &loc); err != nil {
			var locs []Location
			if err2 := json.Unmarshal(raw, &locs); err2 != nil || len(locs) == 0 {
				return
			}
			loc = locs[0]
		}

		c.send(messages.DefinitionResultMsg{
			Path: PathFromURI(loc.URI),
			Line: loc.Range.Start.Line,
			Col:  loc.Range.Start.Character,
		})
	}()
}

// Shutdown sends a shutdown request and an exit notification, then waits for the process.
func (c *Client) Shutdown() error {
	ctx := context.Background()
	if _, err := c.call(ctx, "shutdown", nil); err != nil {
		// Best-effort: still attempt exit.
		_ = c.notify("exit", nil)
		return fmt.Errorf("lsp: shutdown: %w", err)
	}
	if err := c.notify("exit", nil); err != nil {
		return fmt.Errorf("lsp: exit: %w", err)
	}
	_ = c.stdin.Close()
	_ = c.cmd.Wait()
	return nil
}

// call sends a JSON-RPC request and blocks until the response arrives.
func (c *Client) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := RequestMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.writeMessage(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case raw := <-ch:
		return raw, nil
	}
}

// notify sends a JSON-RPC notification (no ID, no response expected).
func (c *Client) notify(method string, params interface{}) error {
	msg := NotificationMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.writeMessage(msg)
}

// writeMessage serialises v as JSON and writes it with a Content-Length header.
func (c *Client) writeMessage(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("lsp: marshal: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := io.WriteString(c.stdin, header); err != nil {
		return fmt.Errorf("lsp: write header: %w", err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		return fmt.Errorf("lsp: write body: %w", err)
	}
	return nil
}

// readLoop continuously reads messages from the server until the connection closes.
func (c *Client) readLoop() {
	for {
		raw, err := c.readMessage()
		if err != nil {
			if err == io.EOF || err == io.ErrClosedPipe {
				break
			}
			c.send(messages.LSPServerStatusMsg{
				Language: c.language,
				Status:   messages.LSPServerCrashed,
				Message:  err.Error(),
			})
			break
		}
		c.dispatch(raw)
	}
}

// readMessage reads a single Content-Length-framed JSON-RPC message.
func (c *Client) readMessage() (json.RawMessage, error) {
	var contentLength int

	// Read headers until blank line.
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("lsp: invalid Content-Length %q", val)
			}
			contentLength = n
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("lsp: missing or zero Content-Length")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("lsp: read body: %w", err)
	}

	return json.RawMessage(body), nil
}

// dispatch routes a raw message to either a pending response channel or
// the notification handler.
func (c *Client) dispatch(raw json.RawMessage) {
	// Peek at the "id" field to distinguish response from notification.
	var envelope struct {
		ID     *int            `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  *ResponseError  `json:"error"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}

	if envelope.ID != nil {
		// It is a response to one of our requests.
		c.mu.Lock()
		ch, ok := c.pending[*envelope.ID]
		if ok {
			delete(c.pending, *envelope.ID)
		}
		c.mu.Unlock()

		if ok {
			if envelope.Error != nil {
				// Send nil so the caller unblocks; error detail is lost here,
				// which is acceptable for best-effort LSP integration.
				ch <- nil
			} else {
				ch <- envelope.Result
			}
		}
		return
	}

	// No ID: it is a server-initiated notification.
	if envelope.Method != "" {
		handleNotification(envelope.Method, envelope.Params, c.language, c.send)
	}
}
