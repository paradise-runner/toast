// Package editor implements the text editor component with viewport, cursor,
// and basic editing operations backed by an EditBuffer (rope-based).
package editor

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yourusername/toast/internal/buffer"
	"github.com/yourusername/toast/internal/clipboard"
	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/syntax"
	"github.com/yourusername/toast/internal/theme"
)

// cursorPos holds the cursor position as a zero-based line and byte column.
type cursorPos struct {
	line int
	col  int // byte offset within the line (not including the newline)
}

// fileLoadedMsg is an internal message dispatched when an async file load completes.
type fileLoadedMsg struct {
	bufferID int
	path     string
	content  string
	isBinary bool
}

// Model is the bubbletea model for the editor component.
type Model struct {
	theme           *theme.Manager
	cfg             config.Config
	bufferID        int
	pendingBufferID int
	path            string
	buf      *buffer.EditBuffer

	cursor          cursorPos
	preferredCol    int        // remembered column for vertical moves
	selectionAnchor *cursorPos // nil = no active selection

	// Mouse drag tracking
	mouseDragging   bool
	mouseDragAnchor cursorPos

	// Multi-click detection
	lastClickTime time.Time
	lastClickPos  cursorPos
	clickCount    int

	viewportTop  int
	viewportLeft int
	viewHeight   int
	viewWidth    int
	gutterWidth  int

	diagnostics []messages.Diagnostic
	lineKinds   []messages.GitLineKind
	focused     bool
	binaryFile  bool
	wrapMode    bool
	highlighter *syntax.Highlighter
}

// New creates a new editor Model with an empty buffer.
func New(tm *theme.Manager, cfg config.Config) Model {
	return Model{
		theme:   tm,
		cfg:     cfg,
		buf:     buffer.NewEditBuffer(""),
		focused: true,
	}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewHeight = msg.Height
		m.viewWidth = msg.Width
		m.clampViewport()

	case tea.KeyPressMsg:
		if m.focused {
			return m.handleKey(msg)
		}

	case tea.MouseClickMsg:
		if m.focused {
			return m.handleMouseClick(msg)
		}

	case tea.MouseMotionMsg:
		if m.focused {
			return m.handleMouseMotion(msg)
		}

	case tea.MouseReleaseMsg:
		if m.focused {
			return m.handleMouseRelease(msg)
		}

	case tea.MouseWheelMsg:
		if m.focused {
			return m.handleMouseWheel(msg)
		}

	case tea.PasteMsg:
		if m.focused && m.buf != nil && msg.Content != "" && !m.binaryFile {
			preModified := false
			if m.buf != nil {
				preModified = m.buf.Modified()
			}
			if _, _, active := m.selectionRange(); active {
				m.deleteSelection()
			}
			offset := m.cursorOffset()
			m.buf.Insert(offset, msg.Content)
			lines := strings.Split(msg.Content, "\n")
			if len(lines) == 1 {
				m.cursor.col += len(msg.Content)
			} else {
				m.cursor.line += len(lines) - 1
				m.cursor.col = len(lines[len(lines)-1])
			}
			m.preferredCol = m.cursor.col
			m.recomputeGutterWidth()
			m.reparseSyntax()
			m.clampViewport()
			if m.buf != nil && m.buf.Modified() != preModified {
				return m, m.emitModified()
			}
			return m, nil
		}

	case fileLoadedMsg:
		if msg.bufferID != m.pendingBufferID {
			return m, nil
		}
		m.binaryFile = msg.isBinary
		m.bufferID = msg.bufferID
		m.path = msg.path
		m.cursor = cursorPos{}
		m.selectionAnchor = nil
		m.mouseDragging = false
		m.preferredCol = 0
		m.viewportTop = 0
		m.viewportLeft = 0
		m.wrapMode = isMarkdownPath(msg.path)
		if msg.isBinary {
			m.buf = buffer.NewEditBuffer("")
			m.highlighter = nil
			return m, nil
		}
		m.buf = buffer.NewEditBuffer(msg.content)
		m.recomputeGutterWidth()
		// Initialize syntax highlighter for the file.
		h, _ := syntax.NewHighlighter(msg.path, m.theme)
		m.highlighter = h
		if m.highlighter != nil {
			m.highlighter.Parse([]byte(msg.content))
		}

	case messages.DiagnosticsUpdatedMsg:
		if msg.Path == m.path {
			m.diagnostics = msg.Diagnostics
		}

	case messages.GitDiffUpdatedMsg:
		if msg.BufferID == m.bufferID {
			m.lineKinds = msg.LineKinds
		}

	case messages.GoToLineMsg:
		return m.handleGoToLine(msg)
	}

	return m, nil
}

// openFile reads a file asynchronously and returns a fileLoadedMsg.
func openFile(bufferID int, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return fileLoadedMsg{bufferID: bufferID, path: path, content: ""}
		}
		if IsBinary(data) {
			return fileLoadedMsg{bufferID: bufferID, path: path, isBinary: true}
		}
		return fileLoadedMsg{bufferID: bufferID, path: path, content: string(data)}
	}
}

// OpenFile is the exported wrapper for openFile, allowing the app layer to
// trigger an asynchronous file load into the editor.
func (m *Model) OpenFile(bufferID int, path string) tea.Cmd {
	m.pendingBufferID = bufferID
	return openFile(bufferID, path)
}

// Path returns the path of the currently loaded file.
func (m Model) Path() string { return m.path }

// BufferID returns the buffer ID of the currently loaded file.
func (m Model) BufferID() int { return m.bufferID }

// handleKey routes all key events to the appropriate handler.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.binaryFile {
		return m, nil
	}
	// Capture the modified state before handling the key.
	// Defaults to false when no buffer is loaded; emitModified() also
	// guards against nil buf, so the two nil-checks are consistent.
	preModified := false
	if m.buf != nil {
		preModified = m.buf.Modified()
	}

	// Handle Super (Cmd) key combinations for copy/cut/paste/select-all and arrow jumps.
	if msg.Mod.Contains(tea.ModSuper) {
		switch msg.Code {
		case 'c':
			if text := m.selectedText(); text != "" {
				clipboard.Copy(text)
			}
			m.clampViewport()
			return m, nil
		case 'x':
			if text := m.selectedText(); text != "" {
				clipboard.Copy(text)
				m.deleteSelection()
			}
			m.reparseSyntax()
			m.clampViewport()
			if m.buf != nil && m.buf.Modified() != preModified {
				return m, m.emitModified()
			}
			return m, nil
		case 'v':
			text := clipboard.Paste()
			if text != "" {
				if _, _, active := m.selectionRange(); active {
					m.deleteSelection()
				}
				offset := m.cursorOffset()
				m.buf.Insert(offset, text)
				lines := strings.Split(text, "\n")
				if len(lines) == 1 {
					m.cursor.col += len(text)
				} else {
					m.cursor.line += len(lines) - 1
					m.cursor.col = len(lines[len(lines)-1])
				}
				m.preferredCol = m.cursor.col
				m.recomputeGutterWidth()
			}
			m.reparseSyntax()
			m.clampViewport()
			if m.buf != nil && m.buf.Modified() != preModified {
				return m, m.emitModified()
			}
			return m, nil
		case 'a':
			anchor := cursorPos{line: 0, col: 0}
			m.selectionAnchor = &anchor
			lastLine := m.buf.LineCount() - 1
			if lastLine < 0 {
				lastLine = 0
			}
			m.cursor.line = lastLine
			m.cursor.col = m.lineContentLen(lastLine)
			m.preferredCol = m.cursor.col
			m.clampViewport()
			return m, nil
		}
		// Cmd+Arrow: line/document jump
		if msg.Mod.Contains(tea.ModShift) {
			switch msg.Code {
			case tea.KeyLeft:
				m.ensureAnchor()
				m.cursor.col = 0
				m.preferredCol = 0
				m.clampViewport()
				return m, nil
			case tea.KeyRight:
				m.ensureAnchor()
				m.cursor.col = m.lineContentLen(m.cursor.line)
				m.preferredCol = m.cursor.col
				m.clampViewport()
				return m, nil
			case tea.KeyUp:
				m.ensureAnchor()
				m.cursor = cursorPos{line: 0, col: 0}
				m.preferredCol = 0
				m.clampViewport()
				return m, nil
			case tea.KeyDown:
				m.ensureAnchor()
				lastLine := m.buf.LineCount() - 1
				if lastLine < 0 {
					lastLine = 0
				}
				m.cursor.line = lastLine
				m.cursor.col = m.lineContentLen(lastLine)
				m.preferredCol = m.cursor.col
				m.clampViewport()
				return m, nil
			}
		} else {
			switch msg.Code {
			case tea.KeyLeft:
				m.clearSelection()
				m.cursor.col = 0
				m.preferredCol = 0
				m.clampViewport()
				return m, nil
			case tea.KeyRight:
				m.clearSelection()
				m.cursor.col = m.lineContentLen(m.cursor.line)
				m.preferredCol = m.cursor.col
				m.clampViewport()
				return m, nil
			case tea.KeyUp:
				m.clearSelection()
				m.cursor = cursorPos{line: 0, col: 0}
				m.preferredCol = 0
				m.clampViewport()
				return m, nil
			case tea.KeyDown:
				m.clearSelection()
				lastLine := m.buf.LineCount() - 1
				if lastLine < 0 {
					lastLine = 0
				}
				m.cursor.line = lastLine
				m.cursor.col = m.lineContentLen(lastLine)
				m.preferredCol = m.cursor.col
				m.clampViewport()
				return m, nil
			}
		}
	}

	// Handle Alt (Option) + arrow: word movement.
	// Some terminals (Ghostty, macOS Terminal) send alt+b / alt+f (readline-style)
	// instead of alt+left / alt+right, so we match both variants.
	if msg.Mod.Contains(tea.ModAlt) {
		if msg.Mod.Contains(tea.ModShift) {
			switch msg.Code {
			case tea.KeyLeft:
				m.ensureAnchor()
				m.moveCursorWordLeft()
				m.clampViewport()
				return m, nil
			case tea.KeyRight:
				m.ensureAnchor()
				m.moveCursorWordRight()
				m.clampViewport()
				return m, nil
			}
		} else {
			switch msg.Code {
			case tea.KeyLeft, 'b':
				m.clearSelection()
				m.moveCursorWordLeft()
				m.clampViewport()
				return m, nil
			case tea.KeyRight, 'f':
				m.clearSelection()
				m.moveCursorWordRight()
				m.clampViewport()
				return m, nil
			case tea.KeyBackspace:
				m.deleteWordBackward()
				m.clampViewport()
				return m, nil
			}
		}
	}

	switch msg.String() {
	case "up":
		m.clearSelection()
		m.moveCursorUp(1)
	case "down":
		m.clearSelection()
		m.moveCursorDown(1)
	case "left":
		m.clearSelection()
		m.moveCursorLeft()
	case "right":
		m.clearSelection()
		m.moveCursorRight()
	case "home":
		m.clearSelection()
		m.cursor.col = 0
		m.preferredCol = 0
	case "end":
		m.clearSelection()
		m.cursor.col = m.lineContentLen(m.cursor.line)
		m.preferredCol = m.cursor.col
	case "ctrl+home":
		m.clearSelection()
		m.cursor = cursorPos{line: 0, col: 0}
		m.preferredCol = 0
	case "ctrl+end":
		m.clearSelection()
		lastLine := m.buf.LineCount() - 1
		if lastLine < 0 {
			lastLine = 0
		}
		m.cursor.line = lastLine
		m.cursor.col = m.lineContentLen(lastLine)
		m.preferredCol = m.cursor.col
	case "pgup":
		m.clearSelection()
		lines := m.viewHeight
		if lines < 1 {
			lines = 1
		}
		m.moveCursorUp(lines)
	case "pgdown":
		m.clearSelection()
		lines := m.viewHeight
		if lines < 1 {
			lines = 1
		}
		m.moveCursorDown(lines)

	// Shift+movement: extend selection
	case "shift+up":
		m.ensureAnchor()
		m.moveCursorUp(1)
	case "shift+down":
		m.ensureAnchor()
		m.moveCursorDown(1)
	case "shift+left":
		m.ensureAnchor()
		m.moveCursorLeft()
	case "shift+right":
		m.ensureAnchor()
		m.moveCursorRight()
	case "shift+home":
		m.ensureAnchor()
		m.cursor.col = 0
		m.preferredCol = 0
	case "shift+end":
		m.ensureAnchor()
		m.cursor.col = m.lineContentLen(m.cursor.line)
		m.preferredCol = m.cursor.col
	case "ctrl+shift+home":
		m.ensureAnchor()
		m.cursor = cursorPos{line: 0, col: 0}
		m.preferredCol = 0
	case "ctrl+shift+end":
		m.ensureAnchor()
		lastLine := m.buf.LineCount() - 1
		if lastLine < 0 {
			lastLine = 0
		}
		m.cursor.line = lastLine
		m.cursor.col = m.lineContentLen(lastLine)
		m.preferredCol = m.cursor.col
	case "ctrl+a":
		// Readline-style beginning-of-line (Ghostty sends \x01 for Cmd+Left).
		m.clearSelection()
		m.cursor.col = 0
		m.preferredCol = 0
	case "ctrl+e":
		// Readline-style end-of-line (useful when Cmd+Right is eaten by the terminal).
		m.clearSelection()
		m.cursor.col = m.lineContentLen(m.cursor.line)
		m.preferredCol = m.cursor.col
	case "ctrl+z":
		m.buf.Undo()
		m.clampCursor()
	case "ctrl+y":
		m.buf.Redo()
		m.clampCursor()
	case "ctrl+c":
		if text := m.selectedText(); text != "" {
			clipboard.Copy(text)
			// Selection stays active (VS Code behavior)
		}
	case "ctrl+x":
		if text := m.selectedText(); text != "" {
			clipboard.Copy(text)
			m.deleteSelection()
		}
	case "ctrl+v":
		text := clipboard.Paste()
		if text != "" {
			if _, _, active := m.selectionRange(); active {
				m.deleteSelection()
			}
			offset := m.cursorOffset()
			m.buf.Insert(offset, text)
			// Advance cursor past pasted text (handle multi-line paste).
			lines := strings.Split(text, "\n")
			if len(lines) == 1 {
				m.cursor.col += len(text)
			} else {
				m.cursor.line += len(lines) - 1
				m.cursor.col = len(lines[len(lines)-1])
			}
			m.preferredCol = m.cursor.col
			m.recomputeGutterWidth()
		}
	case "enter":
		if _, _, active := m.selectionRange(); active {
			m.deleteSelection()
		}
		m.insertNewline()
	case "backspace":
		if _, _, active := m.selectionRange(); active {
			m.deleteSelection()
		} else {
			m.deleteBackward()
		}
	case "delete":
		if _, _, active := m.selectionRange(); active {
			m.deleteSelection()
		} else {
			m.deleteForward()
		}
	case "tab":
		if _, _, active := m.selectionRange(); active {
			m.deleteSelection()
		}
		m.insertTab()
	case "shift+tab":
		m.dedent()
	case "ctrl+s":
		saveCmd := m.save()
		if saveCmd == nil {
			return m, nil
		}
		// save() calls buf.MarkSaved() synchronously, so buf.Modified() is already
		// false by the time emitModified() is called here. Only batch a
		// Modified=false notification when the buffer was dirty before saving,
		// consistent with the preModified guard used throughout handleKey.
		if preModified {
			return m, tea.Batch(saveCmd, m.emitModified())
		}
		return m, saveCmd
	default:
		// Printable character input.
		if msg.Mod.Contains(tea.ModAlt) {
			break // don't insert alt-key combos as text
		}
		if len(msg.Text) > 0 {
			if _, _, active := m.selectionRange(); active {
				m.deleteSelection()
			}
			// Only insert printable runes — filter out escape sequences and control chars.
			var printable []rune
			for _, r := range []rune(msg.Text) {
				if unicode.IsPrint(r) {
					printable = append(printable, r)
				}
			}
			if len(printable) > 0 {
				m.insertRunes(printable)
			}
		}
	}

	m.reparseSyntax()
	m.clampViewport()
	if m.buf != nil && m.buf.Modified() != preModified {
		return m, m.emitModified()
	}
	return m, nil
}

// handleMouseClick handles left-click and multi-click positioning.
func (m Model) handleMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if m.binaryFile {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseLeft:
		clickLine, clickCol := m.screenToBuffer(msg.X, msg.Y)
		newPos := cursorPos{line: clickLine, col: clickCol}

		// Multi-click detection
		const doubleClickMs = 500
		samePos := abs(newPos.col-m.lastClickPos.col) <= 1 && newPos.line == m.lastClickPos.line
		elapsed := time.Since(m.lastClickTime).Milliseconds()
		if samePos && elapsed < doubleClickMs {
			m.clickCount++
			if m.clickCount > 3 {
				m.clickCount = 3
			}
		} else {
			m.clickCount = 1
		}
		m.lastClickTime = time.Now()
		m.lastClickPos = newPos

		switch m.clickCount {
		case 2: // double-click: select word
			m.cursor = newPos
			m.selectWordAt(newPos)
		case 3: // triple-click: select line
			anchor := cursorPos{line: newPos.line, col: 0}
			m.selectionAnchor = &anchor
			m.cursor = cursorPos{line: newPos.line, col: m.lineContentLen(newPos.line)}
			m.preferredCol = m.cursor.col
		default: // single click
			m.cursor = newPos
			m.preferredCol = newPos.col
			m.clearSelection()
			m.mouseDragAnchor = newPos
			m.mouseDragging = true
		}
	}
	m.clampViewport()
	return m, nil
}

// handleMouseMotion handles mouse drag for selection.
func (m Model) handleMouseMotion(msg tea.MouseMotionMsg) (tea.Model, tea.Cmd) {
	if m.binaryFile {
		return m, nil
	}
	if msg.Button == tea.MouseLeft && m.mouseDragging {
		dragLine, dragCol := m.screenToBuffer(msg.X, msg.Y)
		m.cursor = cursorPos{line: dragLine, col: dragCol}
		m.preferredCol = dragCol
		anchor := m.mouseDragAnchor
		m.selectionAnchor = &anchor
	}
	m.clampViewport()
	return m, nil
}

// handleMouseRelease handles mouse button release.
func (m Model) handleMouseRelease(msg tea.MouseReleaseMsg) (tea.Model, tea.Cmd) {
	if m.binaryFile {
		return m, nil
	}
	if msg.Button == tea.MouseLeft && m.mouseDragging {
		m.mouseDragging = false
		// If cursor didn't move from press position, clear the selection.
		if m.selectionAnchor != nil && *m.selectionAnchor == m.cursor {
			m.clearSelection()
		}
	}
	m.clampViewport()
	return m, nil
}

// handleMouseWheel handles scroll wheel events.
func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if m.binaryFile {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.moveCursorUp(1)
	case tea.MouseWheelDown:
		m.moveCursorDown(1)
	}
	m.clampViewport()
	return m, nil
}

// ── Cursor movement ──────────────────────────────────────────────────────────

func (m *Model) moveCursorUp(n int) {
	if m.cursor.line == 0 {
		m.cursor.col = 0
		m.preferredCol = 0
		return
	}
	m.cursor.line -= n
	if m.cursor.line < 0 {
		m.cursor.line = 0
	}
	m.cursor.col = m.clampCol(m.cursor.line, m.preferredCol)
}

func (m *Model) moveCursorDown(n int) {
	lastLine := m.buf.LineCount() - 1
	if lastLine < 0 {
		lastLine = 0
	}
	if m.cursor.line >= lastLine {
		m.cursor.col = m.lineContentLen(lastLine)
		m.preferredCol = m.cursor.col
		return
	}
	m.cursor.line += n
	if m.cursor.line > lastLine {
		m.cursor.line = lastLine
	}
	m.cursor.col = m.clampCol(m.cursor.line, m.preferredCol)
}

func (m *Model) moveCursorLeft() {
	if m.cursor.col > 0 {
		line := m.buf.LineAt(m.cursor.line)
		// Step back one rune.
		sub := line[:m.cursor.col]
		_, size := utf8.DecodeLastRuneInString(sub)
		m.cursor.col -= size
	} else if m.cursor.line > 0 {
		m.cursor.line--
		m.cursor.col = m.lineContentLen(m.cursor.line)
	}
	m.preferredCol = m.cursor.col
}

func (m *Model) moveCursorRight() {
	lineLen := m.lineContentLen(m.cursor.line)
	if m.cursor.col < lineLen {
		line := m.buf.LineAt(m.cursor.line)
		_, size := utf8.DecodeRuneInString(line[m.cursor.col:])
		m.cursor.col += size
	} else {
		lastLine := m.buf.LineCount() - 1
		if lastLine < 0 {
			lastLine = 0
		}
		if m.cursor.line < lastLine {
			m.cursor.line++
			m.cursor.col = 0
		}
	}
	m.preferredCol = m.cursor.col
}

// moveCursorWordLeft moves left past non-word chars then past word chars.
func (m *Model) moveCursorWordLeft() {
	// First move left at least one character so we don't stay in place.
	if m.cursor.col == 0 {
		if m.cursor.line > 0 {
			m.cursor.line--
			m.cursor.col = m.lineContentLen(m.cursor.line)
		}
		m.preferredCol = m.cursor.col
		return
	}

	line := m.buf.LineAt(m.cursor.line)
	col := m.cursor.col

	// Skip trailing non-word chars.
	for col > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:col])
		if isWordChar(r) {
			break
		}
		col -= size
	}
	// Skip word chars.
	for col > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:col])
		if !isWordChar(r) {
			break
		}
		col -= size
	}
	m.cursor.col = col
	m.preferredCol = col
}

// moveCursorWordRight moves right past non-word chars then past word chars.
func (m *Model) moveCursorWordRight() {
	lineLen := m.lineContentLen(m.cursor.line)
	if m.cursor.col >= lineLen {
		lastLine := m.buf.LineCount() - 1
		if lastLine < 0 {
			lastLine = 0
		}
		if m.cursor.line < lastLine {
			m.cursor.line++
			m.cursor.col = 0
		}
		m.preferredCol = m.cursor.col
		return
	}

	line := m.buf.LineAt(m.cursor.line)
	col := m.cursor.col

	// Skip leading non-word chars.
	for col < lineLen {
		r, size := utf8.DecodeRuneInString(line[col:])
		if isWordChar(r) {
			break
		}
		col += size
	}
	// Skip word chars.
	for col < lineLen {
		r, size := utf8.DecodeRuneInString(line[col:])
		if !isWordChar(r) {
			break
		}
		col += size
	}
	m.cursor.col = col
	m.preferredCol = col
}

// isWordChar returns true for letters, digits, and underscore.
func isWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// ── Editing operations ───────────────────────────────────────────────────────

func (m *Model) insertRunes(runes []rune) {
	if len(runes) == 0 {
		return
	}
	offset := m.cursorOffset()
	text := string(runes)
	m.buf.Insert(offset, text)
	m.cursor.col += len(text)
	m.preferredCol = m.cursor.col
}

// insertNewline inserts a newline and auto-indents based on the current line's leading whitespace.
func (m *Model) insertNewline() {
	line := m.buf.LineAt(m.cursor.line)
	indent := ""
	if m.cfg.Editor.AutoIndent {
		indent = leadingWhitespace(line)
		// Don't over-indent if cursor is before the existing indent.
		if m.cursor.col < len(indent) {
			indent = indent[:m.cursor.col]
		}
	}
	offset := m.cursorOffset()
	text := "\n" + indent
	m.buf.Insert(offset, text)
	m.cursor.line++
	m.cursor.col = len(indent)
	m.preferredCol = m.cursor.col
	m.recomputeGutterWidth()
}

// insertTab inserts spaces equal to tab width.
func (m *Model) insertTab() {
	tabWidth := m.cfg.Editor.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}
	spaces := strings.Repeat(" ", tabWidth)
	offset := m.cursorOffset()
	m.buf.Insert(offset, spaces)
	m.cursor.col += tabWidth
	m.preferredCol = m.cursor.col
}

// dedent removes up to tabWidth leading spaces from the current line.
func (m *Model) dedent() {
	tabWidth := m.cfg.Editor.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}
	line := m.buf.LineAt(m.cursor.line)
	toRemove := 0
	for toRemove < tabWidth && toRemove < len(line) && line[toRemove] == ' ' {
		toRemove++
	}
	if toRemove == 0 {
		return
	}
	lineStart := m.buf.OffsetForLine(m.cursor.line)
	m.buf.Delete(lineStart, lineStart+toRemove)
	m.cursor.col -= toRemove
	if m.cursor.col < 0 {
		m.cursor.col = 0
	}
	m.preferredCol = m.cursor.col
}

// deleteBackward deletes the rune immediately before the cursor (or the preceding newline).
func (m *Model) deleteBackward() {
	if m.cursor.col > 0 {
		line := m.buf.LineAt(m.cursor.line)
		_, size := utf8.DecodeLastRuneInString(line[:m.cursor.col])
		offset := m.cursorOffset()
		m.buf.Delete(offset-size, offset)
		m.cursor.col -= size
	} else if m.cursor.line > 0 {
		// Delete the newline at the end of the previous line.
		prevLineLen := m.lineContentLen(m.cursor.line - 1)
		offset := m.cursorOffset()
		m.buf.Delete(offset-1, offset) // delete the \n
		m.cursor.line--
		m.cursor.col = prevLineLen
		m.recomputeGutterWidth()
	}
	m.preferredCol = m.cursor.col
}

// deleteWordBackward deletes from the cursor back to the start of the previous word.
// If a selection is active, it deletes the selection instead (consistent with deleteBackward).
// Word boundary follows isWordChar: letters, digits, underscore.
// At col 0 on a non-first line, deletes the preceding newline (joins lines).
func (m *Model) deleteWordBackward() {
	if _, _, active := m.selectionRange(); active {
		m.deleteSelection()
		return
	}
	if m.cursor.col == 0 {
		if m.cursor.line == 0 {
			return // at buffer start, nothing to delete
		}
		// Join with previous line by deleting the preceding newline.
		prevLineLen := m.lineContentLen(m.cursor.line - 1)
		offset := m.cursorOffset()
		m.buf.Delete(offset-1, offset)
		m.cursor.line--
		m.cursor.col = prevLineLen
		m.recomputeGutterWidth()
		m.preferredCol = m.cursor.col
		return
	}
	// Compute start of word to delete.
	line := m.buf.LineAt(m.cursor.line)
	col := m.cursor.col
	// Skip trailing non-word chars.
	for col > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:col])
		if isWordChar(r) {
			break
		}
		col -= size
	}
	// Skip word chars.
	for col > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:col])
		if !isWordChar(r) {
			break
		}
		col -= size
	}
	end := m.cursorOffset()
	start := m.buf.OffsetForLine(m.cursor.line) + col
	m.buf.Delete(start, end)
	m.cursor.col = col
	m.preferredCol = col
}

// deleteForward deletes the rune immediately after the cursor (or the following newline).
func (m *Model) deleteForward() {
	lineLen := m.lineContentLen(m.cursor.line)
	if m.cursor.col < lineLen {
		line := m.buf.LineAt(m.cursor.line)
		_, size := utf8.DecodeRuneInString(line[m.cursor.col:])
		offset := m.cursorOffset()
		m.buf.Delete(offset, offset+size)
	} else {
		// Delete the newline (joining with the next line).
		lastLine := m.buf.LineCount() - 1
		if m.cursor.line < lastLine {
			offset := m.cursorOffset()
			m.buf.Delete(offset, offset+1) // delete \n
			m.recomputeGutterWidth()
		}
	}
}

// save writes the current buffer to disk, optionally trimming trailing whitespace
// and inserting a final newline.
func (m *Model) save() tea.Cmd {
	if m.path == "" {
		return nil
	}
	content := m.buf.String()

	if m.cfg.Editor.TrimTrailingWhitespaceOnSave {
		content = trimTrailingWhitespace(content)
	}
	if m.cfg.Editor.InsertFinalNewlineOnSave {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
	}

	path := m.path
	bufferID := m.bufferID

	// Sync the buffer to the trimmed/finalized content.
	if content != m.buf.String() {
		m.buf = buffer.NewEditBuffer(content)
	}
	m.buf.MarkSaved()

	return tea.Batch(
		func() tea.Msg { return messages.FileSavingMsg{BufferID: bufferID, Path: path} },
		func() tea.Msg {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return messages.FileSaveFailedMsg{BufferID: bufferID, Path: path, Err: err}
			}
			return messages.FileSavedMsg{BufferID: bufferID, Path: path}
		},
	)
}

// ── View ─────────────────────────────────────────────────────────────────────

// View renders the editor: gutter (line numbers + git diff bar) + content.
func (m Model) View() tea.View {
	if m.viewHeight == 0 || m.viewWidth == 0 {
		return tea.NewView("")
	}

	// Binary file — render centered error message instead of buffer content.
	if m.binaryFile {
		line1 := "Binary file \u2014 cannot display"
		line2 := filepath.Base(m.path)
		msgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.theme.UI("muted"))).
			Width(m.viewWidth).
			Align(lipgloss.Center)
		midRow := (m.viewHeight - 2) / 2
		var sb strings.Builder
		for row := 0; row < m.viewHeight; row++ {
			switch row {
			case midRow:
				sb.WriteString(msgStyle.Render(line1))
			case midRow + 1:
				sb.WriteString(msgStyle.Render(line2))
			default:
				sb.WriteString(strings.Repeat(" ", m.viewWidth))
			}
			if row < m.viewHeight-1 {
				sb.WriteByte('\n')
			}
		}
		v := tea.NewView(sb.String())
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	bgColor := lipgloss.Color(m.theme.UI("background"))
	fgColor := lipgloss.Color(m.theme.UI("foreground"))

	// Empty state: no file is open.
	if m.path == "" {
		emptyStyle := lipgloss.NewStyle().
			Width(m.viewWidth).
			Height(m.viewHeight).
			Background(bgColor).
			Foreground(fgColor).
			Align(lipgloss.Center, lipgloss.Center)
		return tea.NewView(emptyStyle.Render("No file open\nOpen a file from the file tree"))
	}

	gutterFgStr := m.theme.UI("gutter_fg")
	var gutterFg color.Color
	if gutterFgStr != "" {
		gutterFg = lipgloss.Color(gutterFgStr)
	} else {
		gutterFg = lipgloss.Color(m.theme.UI("foreground"))
	}
	lineHighlight := lipgloss.Color(m.theme.UI("line_highlight"))
	gutterStyle := lipgloss.NewStyle().Background(bgColor).Foreground(gutterFg)

	lineCount := m.buf.LineCount()
	var sb strings.Builder

	for screenRow := 0; screenRow < m.viewHeight; screenRow++ {
		bufLine := m.viewportTop + screenRow

		// ── Gutter ──────────────────────────────────────────────────────────
		lineNumStr := ""
		diffBar := ""
		if bufLine < lineCount {
			lineNumStr = fmt.Sprintf("%*d", m.gutterWidth-3, bufLine+1) // right-align, leave space for diff bar + space
			var kind messages.GitLineKind
			if bufLine < len(m.lineKinds) {
				kind = m.lineKinds[bufLine]
			}
			diffBar = gitDiffBar(kind, m.theme)
		} else {
			lineNumStr = strings.Repeat(" ", m.gutterWidth-3)
			diffBar = gitDiffBar(messages.GitLineUnchanged, m.theme)
		}

		gutter := gutterStyle.Render(lineNumStr+" ") + diffBar + gutterStyle.Render(" ")

		// ── Content ─────────────────────────────────────────────────────────
		isCurrentLine := bufLine == m.cursor.line

		var lineContent string
		if bufLine < lineCount {
			raw := m.buf.LineAt(bufLine)
			// Strip trailing newline for display.
			if len(raw) > 0 && raw[len(raw)-1] == '\n' {
				raw = raw[:len(raw)-1]
			}
			lineContent = raw
		}

		// Apply viewport left offset (horizontal scroll, byte-based for simplicity).
		if m.viewportLeft > 0 && len(lineContent) > m.viewportLeft {
			lineContent = lineContent[m.viewportLeft:]
		} else if m.viewportLeft > 0 {
			lineContent = ""
		}

		// Content width available (subtract gutter).
		contentWidth := m.viewWidth - m.gutterWidth
		if contentWidth < 0 {
			contentWidth = 0
		}

		// Compute selection range for this line (raw line-relative offsets, no viewportLeft adjustment).
		var lineSelRange *[2]int
		if start, end, active := m.selectionRange(); active {
			lineStart := m.buf.OffsetForLine(bufLine)
			selStartOff := m.buf.OffsetForLine(start.line) + start.col
			selEndOff := m.buf.OffsetForLine(end.line) + end.col
			lineContentLen := m.lineContentLen(bufLine)
			lineContentEnd := lineStart + lineContentLen
			if lineContentEnd > selStartOff && lineStart < selEndOff {
				rawStart := selStartOff - lineStart
				if rawStart < 0 {
					rawStart = 0
				}
				rawEnd := selEndOff - lineStart
				if rawEnd > lineContentLen {
					rawEnd = lineContentLen
				}
				r := [2]int{rawStart, rawEnd}
				lineSelRange = &r
			}
		}

		var renderedContent string
		if isCurrentLine && m.focused {
			// Render current line with highlight background; the cursor is
			// drawn by the terminal (see View return value below).
			if len(lineContent) > contentWidth {
				lineContent = lineContent[:contentWidth]
			}
			rendered := m.renderHighlightedLine(bufLine, lineContent, lineHighlight, contentWidth, lineSelRange, m.viewportLeft)
			renderedContent = rendered
		} else {
			// Normal line — apply syntax highlighting.
			if len(lineContent) > contentWidth {
				lineContent = lineContent[:contentWidth]
			}
			renderedContent = m.renderHighlightedLine(bufLine, lineContent, bgColor, contentWidth, lineSelRange, m.viewportLeft)
		}

		if screenRow > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(gutter)
		sb.WriteString(renderedContent)
	}

	v := tea.NewView(sb.String())
	if m.focused {
		cursorScreenX := m.gutterWidth + (m.cursor.col - m.viewportLeft)
		cursorScreenY := m.cursor.line - m.viewportTop
		v.Cursor = &tea.Cursor{
			Position: tea.Position{X: cursorScreenX, Y: cursorScreenY},
			Shape:    tea.CursorBar,
			Blink:    true,
		}
	}
	return v
}

// renderHighlightedLine renders a line with syntax highlighting applied.
// If padWidth > 0, pads the result to that width with the background color.
// selRange, if non-nil, is a [start, end) pair of raw line-relative byte offsets
// (before lineOffset adjustment) indicating the selected region.
// lineOffset is the raw line-relative byte index where `text` starts (normally m.viewportLeft).
func (m Model) renderHighlightedLine(bufLine int, text string, bg color.Color, padWidth int, selRange *[2]int, lineOffset int) string {
	// Determine selection background.
	var selBg color.Color = lipgloss.Color("#45475a") // default
	if m.theme != nil {
		if s := m.theme.UI("selection"); s != "" {
			selBg = lipgloss.Color(s)
		}
	}

	var fgColor color.Color
	if m.theme != nil {
		fgColor = lipgloss.Color(m.theme.UI("foreground"))
	}
	baseStyle := lipgloss.NewStyle().Background(bg).Foreground(fgColor)

	// offset is the raw line-relative byte index where `text` starts.
	offset := lineOffset

	// bgAt returns the background for a raw line-relative byte position.
	bgAt := func(rawPos int) color.Color {
		if selRange != nil && rawPos >= selRange[0] && rawPos < selRange[1] {
			return selBg
		}
		return bg
	}

	// renderSegment renders textSlice (a substring of `text`) where segStart is
	// the raw line-relative byte offset of textSlice[0]. It splits the slice at
	// selection boundaries so each sub-chunk gets the right background.
	renderSegment := func(textSlice string, segStart int, fgStyle lipgloss.Style) string {
		if len(textSlice) == 0 {
			return ""
		}
		type chunk struct {
			s  string
			bg color.Color
		}
		var chunks []chunk
		pos := segStart
		remaining := textSlice
		segEnd := segStart + len(textSlice)
		for len(remaining) > 0 {
			nextBoundary := segEnd
			if selRange != nil {
				if selRange[0] > pos && selRange[0] < nextBoundary {
					nextBoundary = selRange[0]
				}
				if selRange[1] > pos && selRange[1] < nextBoundary {
					nextBoundary = selRange[1]
				}
			}
			chunkLen := nextBoundary - pos
			if chunkLen <= 0 || chunkLen > len(remaining) {
				chunkLen = len(remaining)
			}
			chunks = append(chunks, chunk{s: remaining[:chunkLen], bg: bgAt(pos)})
			remaining = remaining[chunkLen:]
			pos += chunkLen
		}
		var out strings.Builder
		for _, c := range chunks {
			style := fgStyle.Background(c.bg)
			out.WriteString(style.Render(c.s))
		}
		return out.String()
	}

	if m.highlighter == nil || len(text) == 0 {
		rendered := renderSegment(text, offset, baseStyle)
		if padWidth > 0 {
			visLen := lipgloss.Width(rendered)
			if visLen < padWidth {
				rendered += baseStyle.Render(strings.Repeat(" ", padWidth-visLen))
			}
		}
		return rendered
	}

	// Get the raw line content (with newline) for the highlighter.
	rawLine := ""
	if bufLine < m.buf.LineCount() {
		rawLine = m.buf.LineAt(bufLine)
	}

	spans := m.highlighter.HighlightLine(bufLine, rawLine)
	if len(spans) == 0 {
		rendered := renderSegment(text, offset, baseStyle)
		if padWidth > 0 {
			visLen := lipgloss.Width(rendered)
			if visLen < padWidth {
				rendered += baseStyle.Render(strings.Repeat(" ", padWidth-visLen))
			}
		}
		return rendered
	}

	// Adjust spans for viewport left offset.
	var adjustedSpans []syntax.Span
	for _, s := range spans {
		s.Start -= offset
		s.End -= offset
		if s.End <= 0 {
			continue
		}
		if s.Start < 0 {
			s.Start = 0
		}
		if s.End > len(text) {
			s.End = len(text)
		}
		if s.Start < s.End {
			adjustedSpans = append(adjustedSpans, s)
		}
	}

	// Sort by start offset.
	sort.Slice(adjustedSpans, func(i, j int) bool {
		return adjustedSpans[i].Start < adjustedSpans[j].Start
	})

	var out strings.Builder
	pos := 0
	for _, span := range adjustedSpans {
		if span.Start < pos {
			// Overlapping span — already rendered this region; skip.
			continue
		}
		if span.Start > pos {
			// gap before this span: segStart is pos+offset (raw line-relative)
			out.WriteString(renderSegment(text[pos:span.Start], pos+offset, baseStyle))
		}
		spanStyle := m.theme.SyntaxStyle(span.Style)
		out.WriteString(renderSegment(text[span.Start:span.End], span.Start+offset, spanStyle))
		pos = span.End
	}
	if pos < len(text) {
		out.WriteString(renderSegment(text[pos:], pos+offset, baseStyle))
	}

	if padWidth > 0 {
		visLen := lipgloss.Width(out.String())
		if visLen < padWidth {
			out.WriteString(baseStyle.Render(strings.Repeat(" ", padWidth-visLen)))
		}
	}

	return out.String()
}

// ── Public accessors ─────────────────────────────────────────────────────────

// CursorLine returns the zero-based cursor line.
func (m Model) CursorLine() int { return m.cursor.line }

// CursorCol returns the zero-based cursor byte column.
func (m Model) CursorCol() int { return m.cursor.col }

// Focus gives the editor keyboard focus.
func (m *Model) Focus() { m.focused = true }

// Blur removes keyboard focus from the editor.
func (m *Model) Blur() { m.focused = false }

// IsModified returns true if the current buffer has unsaved changes.
func (m Model) IsModified() bool {
	if m.buf == nil {
		return false
	}
	return m.buf.Modified()
}

// emitModified returns a Cmd that dispatches a BufferModifiedMsg reflecting
// the current modified state of the buffer. Returns nil when no buffer is
// loaded or when the buffer ID has not been assigned (bufferID == 0, which
// is the case before a file is opened via fileLoadedMsg).
func (m Model) emitModified() tea.Cmd {
	if m.buf == nil || m.bufferID == 0 {
		return nil
	}
	modified := m.buf.Modified()
	bufferID := m.bufferID
	return func() tea.Msg {
		return messages.BufferModifiedMsg{BufferID: bufferID, Modified: modified}
	}
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// cursorOffset returns the absolute byte offset of the cursor in the buffer.
func (m *Model) cursorOffset() int {
	return m.buf.OffsetForLine(m.cursor.line) + m.cursor.col
}

// lineContentLen returns the byte length of the line content excluding any trailing newline.
func (m *Model) lineContentLen(line int) int {
	raw := m.buf.LineAt(line)
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		return len(raw) - 1
	}
	return len(raw)
}

// clampCol returns col clamped to [0, lineContentLen(line)], with respect to
// preferredCol for vertical navigation.
func (m *Model) clampCol(line, preferred int) int {
	maxCol := m.lineContentLen(line)
	if preferred > maxCol {
		return maxCol
	}
	return preferred
}

// clampCursor ensures cursor stays within valid buffer bounds after an undo/redo.
func (m *Model) clampCursor() {
	lineCount := m.buf.LineCount()
	if lineCount == 0 {
		m.cursor = cursorPos{}
		return
	}
	if m.cursor.line >= lineCount {
		m.cursor.line = lineCount - 1
	}
	maxCol := m.lineContentLen(m.cursor.line)
	if m.cursor.col > maxCol {
		m.cursor.col = maxCol
	}
	m.preferredCol = m.cursor.col
}

// ── Selection helpers ────────────────────────────────────────────────────────

// selectionRange returns the normalized (start ≤ end) selection endpoints.
// active is false when selectionAnchor is nil.
func (m *Model) selectionRange() (start, end cursorPos, active bool) {
	if m.selectionAnchor == nil {
		return cursorPos{}, cursorPos{}, false
	}
	a, b := *m.selectionAnchor, m.cursor
	if a.line > b.line || (a.line == b.line && a.col > b.col) {
		a, b = b, a
	}
	return a, b, true
}

// clearSelection sets selectionAnchor to nil.
func (m *Model) clearSelection() {
	m.selectionAnchor = nil
}

// ensureAnchor sets the selection anchor to the current cursor position
// if no selection is active. Call before moving the cursor to extend a selection.
func (m *Model) ensureAnchor() {
	if m.selectionAnchor == nil {
		anchor := m.cursor
		m.selectionAnchor = &anchor
	}
}

// selectedText returns the text covered by the current selection without
// modifying the buffer. Returns "" if no selection is active.
func (m *Model) selectedText() string {
	start, end, active := m.selectionRange()
	if !active {
		return ""
	}
	startOff := m.buf.OffsetForLine(start.line) + start.col
	endOff := m.buf.OffsetForLine(end.line) + end.col
	return m.buf.Slice(startOff, endOff)
}

// deleteSelection deletes the selected byte range from the buffer, moves the
// cursor to the selection start, clears the anchor, and returns the deleted
// text. Returns "" and is a no-op when no selection is active.
func (m *Model) deleteSelection() string {
	start, end, active := m.selectionRange()
	if !active {
		return ""
	}
	startOff := m.buf.OffsetForLine(start.line) + start.col
	endOff := m.buf.OffsetForLine(end.line) + end.col
	text := m.buf.Slice(startOff, endOff)
	m.buf.Delete(startOff, endOff)
	m.cursor = start
	m.preferredCol = start.col
	m.selectionAnchor = nil
	m.recomputeGutterWidth()
	return text
}

// ── Mouse helpers ─────────────────────────────────────────────────────────────

// screenToBuffer translates a screen (X, Y) position to a buffer (line, col).
func (m *Model) screenToBuffer(x, y int) (line, col int) {
	line = m.viewportTop + y
	lineCount := m.buf.LineCount()
	if line >= lineCount {
		line = lineCount - 1
	}
	if line < 0 {
		line = 0
	}
	col = m.viewportLeft + (x - m.gutterWidth)
	if col < 0 {
		col = 0
	}
	lineLen := m.lineContentLen(line)
	if col > lineLen {
		col = lineLen
	}
	return line, col
}

// selectWordAt sets selectionAnchor and cursor to cover the word at pos.
func (m *Model) selectWordAt(pos cursorPos) {
	line := m.buf.LineAt(pos.line)
	col := pos.col
	if col > len(line) {
		col = len(line)
	}
	start := col
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:start])
		if !isWordChar(r) {
			break
		}
		start -= size
	}
	end := col
	lineLen := m.lineContentLen(pos.line)
	for end < lineLen {
		r, size := utf8.DecodeRuneInString(line[end:])
		if !isWordChar(r) {
			break
		}
		end += size
	}
	anchor := cursorPos{line: pos.line, col: start}
	m.selectionAnchor = &anchor
	m.cursor = cursorPos{line: pos.line, col: end}
	m.preferredCol = end
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// clampViewport scrolls the viewport so the cursor is visible.
func (m *Model) clampViewport() {
	// Vertical.
	if m.cursor.line < m.viewportTop {
		m.viewportTop = m.cursor.line
	}
	if m.viewHeight > 0 && m.cursor.line >= m.viewportTop+m.viewHeight {
		m.viewportTop = m.cursor.line - m.viewHeight + 1
	}
	if m.viewportTop < 0 {
		m.viewportTop = 0
	}

	// Horizontal: ensure cursor col is visible.
	cursorScreenCol := m.cursor.col - m.viewportLeft
	contentWidth := m.viewWidth - m.gutterWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	if cursorScreenCol < 0 {
		m.viewportLeft += cursorScreenCol
		if m.viewportLeft < 0 {
			m.viewportLeft = 0
		}
	}
	if cursorScreenCol >= contentWidth {
		m.viewportLeft += cursorScreenCol - contentWidth + 1
	}
}

// recomputeGutterWidth updates the gutter width based on the current line count.
func (m *Model) recomputeGutterWidth() {
	lineCount := m.buf.LineCount()
	if lineCount < 1 {
		lineCount = 1
	}
	digits := len(fmt.Sprintf("%d", lineCount))
	// gutter = digits + 1 space + diff bar (1) + 1 space
	m.gutterWidth = digits + 3
}

// reparseSyntax re-parses the syntax tree from the current buffer content.
// Call this after any operation that mutates m.buf so that HighlightLine
// reflects the current text rather than the stale tree from file load.
func (m *Model) reparseSyntax() {
	if m.highlighter == nil || m.buf == nil {
		return
	}
	m.highlighter.Parse([]byte(m.buf.String()))
}

// leadingWhitespace returns the leading whitespace of a line string.
func leadingWhitespace(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	// Whole line is whitespace (or empty).
	// Strip trailing newline before returning.
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return line[:len(line)-1]
	}
	return line
}

// trimTrailingWhitespace removes trailing whitespace from each line of content.
func trimTrailingWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

// isMarkdownPath returns true if path has a .md or .markdown extension.
func isMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}
