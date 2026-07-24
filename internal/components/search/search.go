// Package search provides the project-wide search panel. It executes
// ripgrep (or a configurable alternative), parses the JSON output, and
// displays results in a scrollable overlay that replaces the editor pane.
package search

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

// ── JSON types ────────────────────────────────────────────────────────────────

// rgMatch is a single JSON line from ripgrep --json output.
type rgMatch struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber     int `json:"line_number"`
		AbsoluteOffset int `json:"absolute_offset"`
		Submatches     []struct {
			Match struct {
				Text string `json:"text"`
			} `json:"match"`
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"submatches"`
	} `json:"data"`
}

// ── Result ────────────────────────────────────────────────────────────────────

// Result is a single display-ready search result.
type Result struct {
	Path    string
	Line    int
	Col     int
	Content string
	Start   int // byte offset of match within Content
	End     int // byte offset of match end within Content
}

// ── Internal messages ─────────────────────────────────────────────────────────

type runSearchMsg struct {
	query   string
	rootDir string
}

// batchMsg carries all ripgrep results plus completion statistics in a single
// update so that thousands of matches do not generate thousands of tiny
// Update calls.
type batchMsg struct {
	results      []messages.SearchResultMsg
	totalFiles   int
	totalMatches int
}

// ── Layout constants ──────────────────────────────────────────────────────────

const (
	headerRows  = 3 // title bar + input field + separator line
	summaryRows = 1 // status line at the bottom
)

// ── textInput ─────────────────────────────────────────────────────────────────

// textInput is a minimal single-line text input for bubbletea v2.
type textInput struct {
	value  string
	cursor int // rune offset
}

func (t *textInput) setCursorFromX(x int) {
	if x < 0 {
		x = 0
	}
	runes := []rune(t.value)
	if x > len(runes) {
		x = len(runes)
	}
	t.cursor = x
}

// insert inserts text at the cursor position. Multi-line text is collapsed to
// the first line.
func (t *textInput) insert(text string) {
	if text == "" {
		return
	}
	// Take only the first line.
	if idx := strings.IndexAny(text, "\n\r"); idx >= 0 {
		text = text[:idx]
	}
	runes := []rune(t.value)
	insert := []rune(text)
	merged := make([]rune, 0, len(runes)+len(insert))
	merged = append(merged, runes[:t.cursor]...)
	merged = append(merged, insert...)
	merged = append(merged, runes[t.cursor:]...)
	t.value = string(merged)
	t.cursor += len(insert)
}

func (t *textInput) handleKey(msg tea.KeyPressMsg) {
	runes := []rune(t.value)

	switch msg.String() {
	case "backspace":
		if t.cursor > 0 {
			t.value = string(runes[:t.cursor-1]) + string(runes[t.cursor:])
			t.cursor--
		}
	case "delete":
		if t.cursor < len(runes) {
			t.value = string(runes[:t.cursor]) + string(runes[t.cursor+1:])
		}
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		if t.cursor < len(runes) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len(runes)
	case "ctrl+u":
		t.value = string(runes[t.cursor:])
		t.cursor = 0
	case "ctrl+k":
		t.value = string(runes[:t.cursor])
	default:
		if text := msg.Text; text != "" {
			t.insert(text)
		}
	}
}

func (t textInput) view(placeholder string) string {
	if t.value == "" {
		return placeholder + " "
	}
	runes := []rune(t.value)
	if t.cursor >= len(runes) {
		return t.value + "█"
	}
	return string(runes[:t.cursor]) + "█" + string(runes[t.cursor+1:])
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the Bubbletea model for the project-wide search panel.
type Model struct {
	theme   *theme.Manager
	rootDir string
	cfg     config.SearchConfig

	query  textInput
	active bool

	results      []Result
	cursor       int // selected result index
	offset       int // scroll offset into results
	width        int
	height       int
	running      bool
	done         bool
	totalFiles   int
	totalMatches int
}

// New creates a new search panel.
func New(tm *theme.Manager, rootDir string, cfg config.SearchConfig) Model {
	return Model{
		theme:   tm,
		rootDir: rootDir,
		cfg:     cfg,
	}
}

// ── tea.Model ─────────────────────────────────────────────────────────────────

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// ── Lifecycle ──────────────────────────────────────────────────────

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case messages.SearchOpenMsg:
		m.active = true

	case messages.SearchCloseMsg:
		m.active = false

	// ── Results ────────────────────────────────────────────────────────

	case messages.SearchResultMsg:
		m.results = append(m.results, Result{
			Path:    msg.Path,
			Line:    msg.Line,
			Col:     msg.Col,
			Content: msg.Content,
			Start:   msg.MatchStart,
			End:     msg.MatchEnd,
		})
		m.totalMatches++
		m.clampScroll()

	case batchMsg:
		for _, r := range msg.results {
			m.results = append(m.results, Result{
				Path:    r.Path,
				Line:    r.Line,
				Col:     r.Col,
				Content: r.Content,
				Start:   r.MatchStart,
				End:     r.MatchEnd,
			})
		}
		m.running = false
		m.done = true
		m.totalMatches = msg.totalMatches
		m.totalFiles = msg.totalFiles
		m.clampScroll()

	case messages.SearchDoneMsg:
		m.running = false
		m.done = true
		m.totalMatches = msg.TotalMatches
		m.totalFiles = msg.TotalFiles

	// ── Search trigger ─────────────────────────────────────────────────

	case runSearchMsg:
		if msg.query == "" {
			break
		}
		// If the user typed more since the debounce fired, skip this run.
		if msg.query != m.query.value {
			break
		}
		m.results = nil
		m.cursor = 0
		m.offset = 0
		m.running = true
		m.done = false
		m.totalFiles = 0
		m.totalMatches = 0
		cmds = append(cmds, runSearch(msg.query, msg.rootDir, m.cfg.Command, m.cfg.Args))

	// ── Paste ──────────────────────────────────────────────────────────

	case tea.PasteMsg:
		if !m.active {
			break
		}
		prev := m.query.value
		m.query.insert(msg.Content)
		if m.query.value != prev {
			cmds = append(cmds, m.debounceSearch(m.query.value))
		}

	// ── Keyboard ───────────────────────────────────────────────────────

	case tea.KeyPressMsg:
		if !m.active {
			break
		}
		switch msg.String() {
		case "escape":
			m.active = false
			cmds = append(cmds, func() tea.Msg { return messages.SearchCloseMsg{} })

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.clampScroll()
			}

		case "down", "ctrl+n":
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.clampScroll()
			}

		case "enter":
			if m.cursor < len(m.results) {
				r := m.results[m.cursor]
				cmds = append(cmds, func() tea.Msg {
					return messages.FileSelectedMsg{Path: r.Path}
				})
			}

		default:
			prev := m.query.value
			m.query.handleKey(msg)
			if m.query.value != prev {
				cmds = append(cmds, m.debounceSearch(m.query.value))
			}
		}

	// ── Mouse wheel ────────────────────────────────────────────────────

	case tea.MouseWheelMsg:
		if !m.active {
			break
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.cursor > 0 {
				m.cursor--
				m.clampScroll()
			}
		case tea.MouseWheelDown:
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.clampScroll()
			}
		}

	// ── Mouse click ────────────────────────────────────────────────────

	case tea.MouseClickMsg:
		if !m.active {
			break
		}
		if msg.Button != tea.MouseLeft {
			break
		}

		// Close button.
		if m.hitClose(msg.X, msg.Y) {
			m.active = false
			cmds = append(cmds, func() tea.Msg { return messages.SearchCloseMsg{} })
			break
		}

		// Input row.
		if msg.Y == 1 {
			m.query.setCursorFromX(msg.X)
			break
		}

		// Result row.
		if idx, ok := m.resultAtY(msg.Y); ok && idx < len(m.results) {
			m.cursor = idx
			m.clampScroll()
			r := m.results[idx]
			cmds = append(cmds, func() tea.Msg {
				return messages.FileSelectedMsg{Path: r.Path}
			})
		}

	// ── Mouse motion ───────────────────────────────────────────────────

	case tea.MouseMotionMsg:
		if !m.active {
			break
		}
		if idx, ok := m.resultAtY(msg.Y); ok && idx < len(m.results) {
			m.cursor = idx
			m.clampScroll()
		}
	}

	return m, tea.Batch(cmds...)
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the search panel.
func (m Model) View() tea.View {
	if m.height <= 0 || m.width <= 0 {
		return tea.NewView("")
	}

	// Base colours. The panel shares the editor's background so it reads as a
	// solid surface, not a transparent overlay.
	bg := lipgloss.Color(m.theme.UI("background"))
	fg := lipgloss.Color(m.theme.UI("foreground"))
	selBG := lipgloss.Color(m.theme.UI("sidebar_selected_bg"))
	selFG := lipgloss.Color(m.theme.UI("sidebar_selected_fg"))
	matchBG := lipgloss.Color(m.theme.UI("search_match_bg"))
	matchFG := lipgloss.Color(m.theme.UI("search_match_fg"))
	currentBG := lipgloss.Color(firstColor(m.theme.UI("search_current_bg"), m.theme.UI("search_match_bg")))
	currentFG := lipgloss.Color(firstColor(m.theme.UI("search_current_fg"), m.theme.UI("search_match_fg")))
	subtle := lipgloss.Color(m.theme.UI("statusbar_fg"))

	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	selected := lipgloss.NewStyle().Background(selBG).Foreground(selFG)
	matchStyle := lipgloss.NewStyle().Background(matchBG).Foreground(matchFG)
	currentStyle := lipgloss.NewStyle().Background(currentBG).Foreground(currentFG)
	subtleStyle := lipgloss.NewStyle().Background(bg).Foreground(subtle)
	subtleSelected := lipgloss.NewStyle().Background(selBG).Foreground(selFG)

	bold := base.Copy().Bold(true)

	var sb strings.Builder

	// ── Title bar ───────────────────────────────────────────────────────

	title := "Search"
	if cx, ok := m.closeX(); ok {
		gap := cx - len(title)
		if gap < 0 {
			gap = 0
		}
		title += strings.Repeat(" ", gap) + "x"
	}
	sb.WriteString(bold.Width(m.width).Render(title) + "\n")

	// ── Input row ───────────────────────────────────────────────────────

	sb.WriteString(base.Width(m.width).Render(m.query.view("Search...")) + "\n")

	// ── Separator ───────────────────────────────────────────────────────

	sb.WriteString(base.Width(m.width).Render(strings.Repeat("─", m.width)) + "\n")

	// ── Results ─────────────────────────────────────────────────────────

	listH := m.listHeight()
	end := m.offset + listH
	if end > len(m.results) {
		end = len(m.results)
	}

	for i := m.offset; i < end; i++ {
		r := m.results[i]
		rel, err := filepath.Rel(m.rootDir, r.Path)
		if err != nil {
			rel = r.Path
		}
		prefixText := fmt.Sprintf("%s:%d", rel, r.Line)
		content := strings.TrimRight(r.Content, "\n\r")

		// Pick per-row styles. The selected row uses the sidebar selection
		// colours for the whole row (incl. the path prefix) so the highlight
		// fills the entire width; the match on a selected row uses the
		// "current match" colours which stay readable on the selection bg.
		rowStyle := base
		subtleRow := subtleStyle
		matchHL := matchStyle
		if i == m.cursor {
			rowStyle = selected
			subtleRow = subtleSelected
			matchHL = currentStyle
		}

		var contentLine string
		if r.Start >= 0 && r.End > r.Start && r.End <= len(content) {
			pre := content[:r.Start]
			matchText := content[r.Start:r.End]
			post := content[r.End:]
			contentLine = rowStyle.Render(pre) + matchHL.Render(matchText) + rowStyle.Render(post)
		} else {
			contentLine = rowStyle.Render(content)
		}

		row := subtleRow.Render(prefixText) + " " + contentLine

		// Truncate to the panel width (ANSI-aware, grapheme-safe) then pad
		// the remainder with the row background so the highlight is solid.
		row = ansi.Truncate(row, m.width, "")
		pad := m.width - lipgloss.Width(row)
		if pad < 0 {
			pad = 0
		}
		row += rowStyle.Render(strings.Repeat(" ", pad))
		sb.WriteString(row + "\n")
	}

	// Pad remaining rows so the summary sits at the bottom.
	for i := end - m.offset; i < listH; i++ {
		sb.WriteString(base.Width(m.width).Render("") + "\n")
	}

	// ── Summary ─────────────────────────────────────────────────────────

	var status string
	switch {
	case m.running:
		status = fmt.Sprintf("Searching… %d matches", len(m.results))
	case m.done:
		status = fmt.Sprintf("%d matches in %d files", m.totalMatches, m.totalFiles)
	case m.query.value == "":
		status = "Type to search"
	default:
		status = "No results"
	}
	sb.WriteString(base.Width(m.width).Render(subtleStyle.Render(status)))

	return tea.NewView(sb.String())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// listHeight returns the number of rows available for result display.
func (m Model) listHeight() int {
	n := m.height - headerRows - summaryRows
	if n < 0 {
		return 0
	}
	return n
}

// closeX returns the X position of the close button and whether it fits.
func (m Model) closeX() (int, bool) {
	need := len("Search") + 1 + 1 // "Search" + space + "x"
	if m.width < need {
		return 0, false
	}
	return m.width - 1, true
}

// hitClose returns true when (x, y) lands on the close button.
func (m Model) hitClose(x, y int) bool {
	cx, ok := m.closeX()
	return ok && y == 0 && x == cx
}

// resultAtY converts a viewport Y coordinate to a result index.
func (m Model) resultAtY(y int) (int, bool) {
	row := y - headerRows
	if row < 0 || row >= m.listHeight() {
		return 0, false
	}
	idx := m.offset + row
	if idx < 0 || idx >= len(m.results) {
		return 0, false
	}
	return idx, true
}

// clampScroll keeps the cursor visible in the results list.
func (m *Model) clampScroll() {
	listH := m.listHeight()
	if listH <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listH {
		m.offset = m.cursor - listH + 1
	}
}

// firstColor returns the first non-empty theme value, or "" if none are set.
func firstColor(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// debounceSearch waits 200 ms then fires a search if the query is still current.
func (m Model) debounceSearch(query string) tea.Cmd {
	rootDir := m.rootDir
	return func() tea.Msg {
		time.Sleep(200 * time.Millisecond)
		return runSearchMsg{query: query, rootDir: rootDir}
	}
}

// ── Search execution ──────────────────────────────────────────────────────────

// runSearch executes the configured search command (default: rg --json) and
// returns a batchMsg containing all results.
func runSearch(query, rootDir, command string, args []string) tea.Cmd {
	return func() tea.Msg {
		fullArgs := make([]string, 0, len(args)+3)
		fullArgs = append(fullArgs, args...)
		fullArgs = append(fullArgs, "--", query, rootDir)

		cmd := exec.Command(command, fullArgs...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return batchMsg{}
		}
		if err := cmd.Start(); err != nil {
			return batchMsg{}
		}

		var (
			results      []messages.SearchResultMsg
			totalFiles   int
			totalMatches int
			seen         = make(map[string]bool)
		)

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10 MB max line

		for scanner.Scan() {
			var m rgMatch
			if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
				continue
			}
			if m.Type != "match" {
				continue
			}

			path := m.Data.Path.Text
			if !seen[path] {
				seen[path] = true
				totalFiles++
			}

			content := m.Data.Lines.Text
			lineNum := m.Data.LineNumber
			for _, sub := range m.Data.Submatches {
				totalMatches++
				results = append(results, messages.SearchResultMsg{
					Path:       path,
					Line:       lineNum,
					Col:        sub.Start,
					Content:    content,
					MatchStart: sub.Start,
					MatchEnd:   sub.End,
				})
			}
		}
		_ = cmd.Wait()

		return batchMsg{
			results:      results,
			totalFiles:   totalFiles,
			totalMatches: totalMatches,
		}
	}
}
