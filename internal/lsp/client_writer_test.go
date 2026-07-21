package lsp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestWriteMessageDoesNotBlockCallerOnFullPipe reproduces the freeze that used
// to happen when DidChange wrote the full document synchronously to a language
// server's stdin on the UI goroutine: if the server was slow to drain its input
// pipe, the OS pipe buffer filled and the write (and therefore the whole editor)
// blocked. The outgoing queue must absorb that backpressure off the caller.
func TestWriteMessageDoesNotBlockCallerOnFullPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	c := &Client{
		language: "test",
		stdin:    w,
		pending:  make(map[int]chan json.RawMessage),
		send:     func(tea.Msg) {},
		nextID:   1,
	}
	c.wqCond = sync.NewCond(&c.wqMu)
	go c.writeLoop()

	// Each notify marshals a body far larger than a typical OS pipe buffer, and
	// we send many of them — enough that the writer goroutine is guaranteed to
	// be blocked writing into the full pipe while the caller keeps going.
	big := strings.Repeat("x", 256*1024) // 256 KiB per message
	for i := 0; i < 200; i++ {           // ~51 MiB total, well beyond any pipe buffer
		if err := c.notify("textDocument/didChange", map[string]interface{}{
			"text": big,
		}); err != nil {
			t.Fatalf("notify #%d returned error (caller must not block): %v", i, err)
		}
	}

	// Enqueue a flush sentinel. Because the writer is stuck on the full pipe it
	// must NOT be able to reach this sentinel, so done stays open — proving the
	// backpressure lives in the queue (background) rather than on the caller.
	done := make(chan struct{})
	if !c.enqueueWrite(outJob{done: done}) {
		t.Fatal("enqueueWrite rejected the flush sentinel")
	}
	select {
	case <-done:
		t.Fatal("flush completed while pipe is full; writes are not being offloaded to the writer goroutine")
	case <-time.After(100 * time.Millisecond):
		// Good: the sentinel is still pending behind the queued writes.
	}

	// Drain the read end so the writer can finish and we do not leak a blocked
	// goroutine past the test.
	go io.Copy(io.Discard, r)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("flush did not complete after draining the pipe")
	}
}

// TestChangeEventsAreEnqueuedInOrder verifies that the writer goroutine emits
// queued notifications in FIFO order once the pipe is drained, so didOpen /
// didChange / didClose sequencing the LSP server relies on is preserved.
func TestChangeEventsAreEnqueuedInOrder(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	c := &Client{
		language: "test",
		stdin:    w,
		pending:  make(map[int]chan json.RawMessage),
		send:     func(tea.Msg) {},
		nextID:   1,
	}
	c.wqCond = sync.NewCond(&c.wqMu)
	go c.writeLoop()

	methods := []string{"textDocument/didOpen", "textDocument/didChange", "textDocument/didClose"}
	for _, method := range methods {
		if err := c.notify(method, map[string]interface{}{"marker": method}); err != nil {
			t.Fatalf("notify %s: %v", method, err)
		}
	}

	br := bufio.NewReader(r)
	for _, want := range methods {
		if got := readFrameMethod(t, br); got != want {
			t.Fatalf("out-of-order write: got %q want %q", got, want)
		}
	}
}

// readFrameMethod parses one LSP frame's headers and body and returns the
// "method" field, failing the test on read errors.
func readFrameMethod(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	var headerLen int
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
			if err != nil {
				t.Fatalf("parse Content-Length: %v", err)
			}
			headerLen = n
		}
	}
	if headerLen == 0 {
		t.Fatal("missing Content-Length")
	}
	body := make([]byte, headerLen)
	if _, err := io.ReadFull(br, body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	var env struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return env.Method
}
