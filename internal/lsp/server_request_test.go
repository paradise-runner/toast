package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/messages"
)

// TestClientRespondsToServerRequests verifies the client answers
// server-initiated JSON-RPC requests. harper-ls sends workspace/configuration
// and client/registerCapability requests inside its initialized handler and
// blocks until they are answered, so a client that drops them would stall the
// server before any diagnostics are published. The test runs a fake server as
// a subprocess of the test binary and checks both that the responses arrive
// and that diagnostics flow afterwards.
func TestClientRespondsToServerRequests(t *testing.T) {
	verdictFile := filepath.Join(t.TempDir(), "verdict.txt")
	t.Setenv("TOAST_LSP_HELPER", "1")
	t.Setenv("TOAST_LSP_HELPER_OUT", verdictFile)

	sendCh := make(chan tea.Msg, 16)
	client, err := NewClient("markdown", os.Args[0], []string{"-test.run=^TestHarperLikeHelperServer$"}, func(msg tea.Msg) {
		sendCh <- msg
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := client.Initialize("file:///proj"); err != nil {
		client.Shutdown()
		t.Fatalf("Initialize: %v", err)
	}
	if err := client.DidOpen("/tmp/proj/notes.md", "markdown", "teh quikc brown fox"); err != nil {
		client.Shutdown()
		t.Fatalf("DidOpen: %v", err)
	}

	select {
	case msg := <-sendCh:
		diag, ok := msg.(messages.DiagnosticsUpdatedMsg)
		if !ok {
			client.Shutdown()
			t.Fatalf("expected DiagnosticsUpdatedMsg, got %T", msg)
		}
		if diag.Path != "/tmp/proj/notes.md" {
			client.Shutdown()
			t.Fatalf("diagnostics for %q, want /tmp/proj/notes.md", diag.Path)
		}
		if len(diag.Diagnostics) != 1 {
			client.Shutdown()
			t.Fatalf("expected 1 diagnostic, got %d", len(diag.Diagnostics))
		}
		d := diag.Diagnostics[0]
		if d.Severity != SeverityHint || d.Source != "harper" || d.Message != "Did you mean 'the'?" {
			client.Shutdown()
			t.Fatalf("unexpected diagnostic: %#v", d)
		}
	case <-time.After(15 * time.Second):
		client.Shutdown()
		verdict, _ := os.ReadFile(verdictFile)
		t.Fatalf("timed out waiting for diagnostics; helper verdict: %s", verdict)
	}

	if err := client.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	verdict, err := os.ReadFile(verdictFile)
	if err != nil {
		t.Fatalf("reading helper verdict: %v", err)
	}
	if strings.TrimSpace(string(verdict)) != "ok" {
		t.Fatalf("helper server reported a failure: %s", verdict)
	}
}

// TestHarperLikeHelperServer is not a real test: it is the fake language
// server subprocess used by TestClientRespondsToServerRequests. It is only
// active when TOAST_LSP_HELPER is set; it drives the LSP conversation and
// records its verdict (ok or FAIL: ...) in TOAST_LSP_HELPER_OUT. It calls
// os.Exit itself so the test harness never prints PASS to stdout, which would
// corrupt the LSP stream.
func TestHarperLikeHelperServer(t *testing.T) {
	if os.Getenv("TOAST_LSP_HELPER") != "1" {
		return
	}
	verdictPath := os.Getenv("TOAST_LSP_HELPER_OUT")

	fail := func(format string, args ...interface{}) {
		writeVerdict(verdictPath, "FAIL: "+fmt.Sprintf(format, args...))
		os.Exit(2)
	}
	ok := func() {
		writeVerdict(verdictPath, "ok")
		os.Exit(0)
	}

	r := bufio.NewReader(os.Stdin)
	w := bufio.NewWriter(os.Stdout)
	write := func(v interface{}) {
		body, err := json.Marshal(v)
		if err != nil {
			fail("marshal: %v", err)
		}
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
			fail("write header: %v", err)
		}
		if _, err := w.Write(body); err != nil {
			fail("write body: %v", err)
		}
		if err := w.Flush(); err != nil {
			fail("flush: %v", err)
		}
	}
	read := func() (id *int, method string, params, result json.RawMessage) {
		var contentLength int
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				fail("read header: %v", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				contentLength, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
			}
		}
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(r, body); err != nil {
			fail("read body: %v", err)
		}
		var envelope struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			fail("unmarshal %q: %v", body, err)
		}
		return envelope.ID, envelope.Method, envelope.Params, envelope.Result
	}
	readRequest := func(wantMethod string) *int {
		id, method, _, _ := read()
		if method != wantMethod {
			fail("expected %q request, got %q", wantMethod, method)
		}
		return id
	}

	// Handshake.
	id := readRequest("initialize")
	write(ResponseMessage{JSONRPC: "2.0", ID: id, Result: map[string]interface{}{
		"capabilities": map[string]interface{}{},
	}})
	readRequest("initialized")

	// Server-initiated requests, as harper-ls sends on initialized. The client
	// may interleave its didOpen notification with the responses, so messages
	// are processed in arrival order rather than a fixed sequence.
	write(RequestMessage{JSONRPC: "2.0", ID: 9001, Method: "workspace/configuration",
		Params: map[string]interface{}{"items": []interface{}{}}})
	write(RequestMessage{JSONRPC: "2.0", ID: 9002, Method: "client/registerCapability",
		Params: map[string]interface{}{"registrations": []interface{}{}}})

	pending := map[int]string{9001: "workspace/configuration", 9002: "client/registerCapability"}
	opened := false
	for len(pending) > 0 || !opened {
		respID, method, params, result := read()
		if respID != nil {
			want, ok := pending[*respID]
			if !ok {
				fail("unexpected response to id %d", *respID)
			}
			delete(pending, *respID)
			switch want {
			case "workspace/configuration":
				var configResult []interface{}
				if err := json.Unmarshal(result, &configResult); err != nil || len(configResult) != 0 {
					fail("workspace/configuration result = %s, want []", result)
				}
			case "client/registerCapability":
				if string(result) != "null" {
					fail("client/registerCapability result = %s, want null", result)
				}
			}
			continue
		}
		if method != "textDocument/didOpen" {
			fail("expected textDocument/didOpen, got %q", method)
		}
		var openedDoc struct {
			TextDocument struct {
				URI        string `json:"uri"`
				LanguageID string `json:"languageId"`
				Text       string `json:"text"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(params, &openedDoc); err != nil {
			fail("didOpen params: %v", err)
		}
		if openedDoc.TextDocument.LanguageID != "markdown" {
			fail("languageId = %q, want markdown", openedDoc.TextDocument.LanguageID)
		}
		if openedDoc.TextDocument.Text != "teh quikc brown fox" {
			fail("didOpen text = %q", openedDoc.TextDocument.Text)
		}
		opened = true

		write(NotificationMessage{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: map[string]interface{}{
			"uri": openedDoc.TextDocument.URI,
			"diagnostics": []interface{}{
				map[string]interface{}{
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 0, "character": 0},
						"end":   map[string]interface{}{"line": 0, "character": 3},
					},
					"severity": SeverityHint,
					"message":  "Did you mean 'the'?",
					"source":   "harper",
				},
			},
		}})
	}

	// Shutdown handshake.
	id = readRequest("shutdown")
	write(ResponseMessage{JSONRPC: "2.0", ID: id, Result: json.RawMessage("null")})
	readRequest("exit")

	ok()
}

func writeVerdict(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}
