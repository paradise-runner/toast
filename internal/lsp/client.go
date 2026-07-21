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
	"time"

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

	// Outgoing message queue, drained by writeLoop. All writes to stdin go
	// through the queue so that callers on the UI goroutine never block on a
	// slow or stalled language server's pipe. Writes are serialised in queue
	// order, preserving JSON-RPC notification ordering (important for
	// textDocument/didOpen, didChange, didClose sequencing).
	wqMu     sync.Mutex
	wqCond   *sync.Cond
	wq       []outJob
	wqErr    error // sticky write error; once set, no more writes are attempted
	wqClosed bool  // set by Shutdown to stop writeLoop after it drains the queue
}

// outJob is a single queued outgoing write. If done is non-nil the job is a
// flush sentinel: writeLoop closes done once all previously queued jobs (and
// this one) have been written, without writing anything itself.
type outJob struct {
	body []byte
	done chan struct{}
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
		return nil, fmt.Errorf("lsp: start %q: %w", command, err)
	}

	c := &Client{
		language: language,
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdoutPipe),
		nextID:   1,
		pending:  make(map[int]chan json.RawMessage),
		send:     send,
	}
	c.wqCond = sync.NewCond(&c.wqMu)

	go c.readLoop()
	go c.writeLoop()

	return c, nil
}

// Initialize sends the LSP initialize request followed by the initialized notification.
func (c *Client) Initialize(rootURI string) error {
	params := map[string]interface{}{
		"rootUri": rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"definition": map[string]interface{}{},
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
func (c *Client) Completion(bufferID, generation int, path string, line, sourceCol, protocolCol int) {
	go func() {
		params := CompletionParams{
			TextDocument: TextDocumentIdentifier{URI: URIFromPath(path)},
			Position:     Position{Line: line, Character: protocolCol},
		}
		raw, err := c.call(context.Background(), "textDocument/completion", params)
		if err != nil {
			return
		}

		result, ok := parseCompletionResult(raw, bufferID, generation, path, line, sourceCol)
		if !ok {
			return
		}
		c.send(result)
	}()
}

func parseCompletionResult(raw json.RawMessage, bufferID, generation int, path string, line, col int) (messages.CompletionResultMsg, bool) {
	result := messages.CompletionResultMsg{BufferID: bufferID, Generation: generation, Path: path, Line: line, Col: col}
	if len(raw) == 0 || string(raw) == "null" {
		return result, true
	}

	// The result may be CompletionList or []CompletionItem.
	var itemList []LSPCompletionItem
	var defaultEditRange *Range
	defaultInsertTextFormat := 0
	if err := json.Unmarshal(raw, &itemList); err != nil {
		var list struct {
			Items        []LSPCompletionItem `json:"items"`
			ItemDefaults *struct {
				EditRange        json.RawMessage `json:"editRange"`
				InsertTextFormat int             `json:"insertTextFormat"`
			} `json:"itemDefaults,omitempty"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return messages.CompletionResultMsg{}, false
		}
		itemList = list.Items
		if list.ItemDefaults != nil {
			defaultEditRange = parseCompletionEditRange(list.ItemDefaults.EditRange)
			defaultInsertTextFormat = list.ItemDefaults.InsertTextFormat
		}
	}

	for _, item := range itemList {
		completion := messages.CompletionItem{
			Label:            item.Label,
			Kind:             item.Kind,
			Detail:           item.Detail,
			Documentation:    completionDocumentation(item.Documentation),
			InsertText:       item.InsertText,
			InsertTextFormat: item.InsertTextFormat,
		}
		if completion.InsertTextFormat == 0 {
			completion.InsertTextFormat = defaultInsertTextFormat
		}
		if item.TextEdit != nil {
			editRange := item.TextEdit.Range
			if editRange == nil {
				editRange = item.TextEdit.Insert
			}
			if editRange != nil {
				completion.TextEdit = &messages.TextEdit{
					Line:    editRange.Start.Line,
					Col:     editRange.Start.Character,
					EndLine: editRange.End.Line,
					EndCol:  editRange.End.Character,
					NewText: item.TextEdit.NewText,
				}
			}
		} else if defaultEditRange != nil {
			newText := item.TextEditText
			if newText == "" {
				newText = item.InsertText
			}
			if newText == "" {
				newText = item.Label
			}
			completion.TextEdit = &messages.TextEdit{
				Line:    defaultEditRange.Start.Line,
				Col:     defaultEditRange.Start.Character,
				EndLine: defaultEditRange.End.Line,
				EndCol:  defaultEditRange.End.Character,
				NewText: newText,
			}
		}
		result.Items = append(result.Items, completion)
	}
	return result, true
}

func parseCompletionEditRange(raw json.RawMessage) *Range {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var direct struct {
		Start *Position `json:"start"`
		End   *Position `json:"end"`
	}
	if err := json.Unmarshal(raw, &direct); err == nil && direct.Start != nil && direct.End != nil {
		return &Range{Start: *direct.Start, End: *direct.End}
	}
	var insertReplace struct {
		Insert *Range `json:"insert"`
	}
	if err := json.Unmarshal(raw, &insertReplace); err == nil {
		return insertReplace.Insert
	}
	return nil
}

func completionDocumentation(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case map[string]interface{}:
		if text, ok := value["value"].(string); ok {
			return text
		}
	}
	return ""
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
func (c *Client) Definition(bufferID int, path string, sourceLine, sourceCol, protocolCol int, navigate bool) {
	go func() {
		params := DefinitionParams{
			TextDocument: TextDocumentIdentifier{URI: URIFromPath(path)},
			Position:     Position{Line: sourceLine, Character: protocolCol},
		}
		raw, err := c.call(context.Background(), "textDocument/definition", params)
		var loc Location
		if err == nil {
			loc, _ = parseDefinitionLocation(raw)
		}

		c.send(messages.DefinitionResultMsg{
			BufferID:   bufferID,
			SourceLine: sourceLine,
			SourceCol:  sourceCol,
			Path:       PathFromURI(loc.URI),
			Line:       loc.Range.Start.Line,
			Col:        loc.Range.Start.Character,
			Navigate:   navigate,
		})
	}()
}

func parseDefinitionLocation(raw json.RawMessage) (Location, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return Location{}, false
	}
	var loc Location
	if err := json.Unmarshal(raw, &loc); err == nil && loc.URI != "" {
		return loc, true
	}
	var locations []Location
	if err := json.Unmarshal(raw, &locations); err == nil && len(locations) > 0 && locations[0].URI != "" {
		return locations[0], true
	}
	var links []LocationLink
	if err := json.Unmarshal(raw, &links); err == nil && len(links) > 0 && links[0].TargetURI != "" {
		return Location{URI: links[0].TargetURI, Range: links[0].TargetSelectionRange}, true
	}
	return Location{}, false
}

// Shutdown sends a shutdown request and an exit notification, then waits for the process.
//
// The shutdown request is given a short timeout so a hung language server
// cannot block application exit; the exit notification and stdin close only
// happen after the writer goroutine has flushed all queued messages.
func (c *Client) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.call(ctx, "shutdown", nil); err != nil {
		// Best-effort: still attempt exit, then flush and tear down.
		_ = c.notify("exit", nil)
		c.flushWrites()
		c.stopWrites()
		_ = c.stdin.Close()
		_ = c.cmd.Wait()
		return fmt.Errorf("lsp: shutdown: %w", err)
	}
	if err := c.notify("exit", nil); err != nil {
		return fmt.Errorf("lsp: exit: %w", err)
	}
	c.flushWrites() // ensure the queued exit notification reaches the server
	c.stopWrites()  // tell writeLoop to exit once the queue is empty
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

// writeMessage serialises v as JSON and enqueues it for the writer goroutine.
//
// It never blocks on the server's stdin pipe: marshalling happens inline, and
// the framed message is appended to an unbounded queue serviced by writeLoop.
// This keeps the UI goroutine responsive even when a language server is slow
// to read its input (e.g. busy computing diagnostics) or its pipe buffer is
// full, which previously froze the whole editor. Ordering is preserved because
// writeLoop is the sole writer.
func (c *Client) writeMessage(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("lsp: marshal: %w", err)
	}
	c.enqueueWrite(outJob{body: body})
	return nil
}

// enqueueWrite appends a job to the outgoing queue. Unbounded, so it cannot
// block the caller; backpressure is absorbed by the queue and applied only
// inside writeLoop (off the UI goroutine).
func (c *Client) enqueueWrite(job outJob) bool {
	c.wqMu.Lock()
	if c.wqClosed || c.wqErr != nil {
		c.wqMu.Unlock()
		return false
	}
	c.wq = append(c.wq, job)
	c.wqCond.Signal()
	c.wqMu.Unlock()
	return true
}

// flushWrites blocks until the writer goroutine has flushed every job enqueued
// so far. Used by Shutdown to guarantee the exit notification reaches the
// server before stdin is closed.
func (c *Client) flushWrites() {
	done := make(chan struct{})
	c.wqMu.Lock()
	if c.wqErr != nil {
		c.wqMu.Unlock()
		return
	}
	c.wq = append(c.wq, outJob{done: done})
	c.wqCond.Signal()
	c.wqMu.Unlock()
	<-done
}

// stopWrites marks the writer for shutdown: after the queue drains, writeLoop
// exits. New enqueueWrite calls after this return false.
func (c *Client) stopWrites() {
	c.wqMu.Lock()
	c.wqClosed = true
	c.wqCond.Signal()
	c.wqMu.Unlock()
}

// writeLoop is the sole goroutine that writes framed messages to the server's
// stdin. It services outJob jobs in FIFO order. On a write error it records
// the sticky error, drains remaining jobs (signalling their flush dones),
// and unblocks any callers waiting on responses that will never arrive.
func (c *Client) writeLoop() {
	for {
		c.wqMu.Lock()
		for len(c.wq) == 0 && c.wqErr == nil && !c.wqClosed {
			c.wqCond.Wait()
		}
		if c.wqErr != nil {
			// Drain remaining jobs without writing; signal flush dones.
			remaining := c.wq
			c.wq = nil
			c.wqMu.Unlock()
			for _, job := range remaining {
				if job.done != nil {
					close(job.done)
				}
			}
			return
		}
		batch := c.wq
		c.wq = nil
		closed := c.wqClosed
		c.wqMu.Unlock()

		for _, job := range batch {
			if job.done != nil {
				close(job.done)
				continue
			}
			if err := c.writeRaw(job.body); err != nil {
				c.failWrites(err)
				return
			}
		}

		if closed {
			c.wqMu.Lock()
			empty := len(c.wq) == 0
			c.wqMu.Unlock()
			if empty {
				return
			}
		}
	}
}

// writeRaw frames body with a Content-Length header and writes it to stdin.
// Must only be called from writeLoop.
func (c *Client) writeRaw(body []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return fmt.Errorf("lsp: write header: %w", err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		return fmt.Errorf("lsp: write body: %w", err)
	}
	return nil
}

// failWrites records a sticky write error, drops the remaining queue (signalling
// any flush dones), and unblocks callers waiting in call() on responses that
// will never arrive. The read loop will separately surface the server crash
// (stdout closes) so we do not send a status message here to avoid duplicates.
func (c *Client) failWrites(err error) {
	c.wqMu.Lock()
	c.wqErr = err
	c.wqClosed = true
	remaining := c.wq
	c.wq = nil
	c.wqMu.Unlock()

	for _, job := range remaining {
		if job.done != nil {
			close(job.done)
		}
	}

	// Unblock goroutines blocked in call(): deliver a nil result to each.
	c.mu.Lock()
	for id, ch := range c.pending {
		select {
		case ch <- nil:
		default:
			// Channel already has a (real) response buffered; leave it.
		}
		delete(c.pending, id)
	}
	c.mu.Unlock()
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
