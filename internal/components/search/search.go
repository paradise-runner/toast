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

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

// rgMatch holds the top-level JSON object from ripgrep --json output.
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

// Result is a single search result entry.
type Result struct {
	Path    string
	Line    int
	Col     int
	Content string
	Start   int
	End     int
}

// runSearchMsg is an internal message that triggers a ripgrep run.
type runSearchMsg struct {
	query   string
	rootDir string
}

// searchBatchMsg carries all ripgrep results plus a done signal in one Cmd return.
type searchBatchMsg struct {
	results []messages.SearchResultMsg
	done    messages.SearchDoneMsg
}

// textInput is a minimal single-line text input for v2 bubbletea.
type textInput struct {
	value  string
	cursor int
}

func (t *textInput) Value() string { return t.value }

func (t *textInput) handleKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "backspace":
		if t.cursor > 0 {
			runes := []rune(t.value)
			runes = append(runes[:t.cursor-1], runes[t.cursor:]...)
			t.value = string(runes)
			t.cursor--
		}
	case "delete":
		runes := []rune(t.value)
		if t.cursor < len(runes) {
			runes = append(runes[:t.cursor], runes[t.cursor+1:]...)
			t.value = string(runes)
		}
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		runes := []rune(t.value)
		if t.cursor < len(runes) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len([]rune(t.value))
	case "ctrl+u":
		t.value = string([]rune(t.value)[t.cursor:])
		t.cursor = 0
	case "ctrl+k":
		t.value = string([]rune(t.value)[:t.cursor])
	default:
		if len(msg.Text) > 0 {
			runes := []rune(t.value)
			text := []rune(msg.Text)
			runes = append(runes[:t.cursor], append(text, runes[t.cursor:]...)...)
			t.value = string(runes)
			t.cursor += len(text)
		}
	}
}

func (t *textInput) View(placeholder string) string {
	v := t.value
	if v == "" {
		return placeholder + " "
	}
	runes := []rune(v)
	if t.cursor >= len(runes) {
		return v + "█"
	}
	return string(runes[:t.cursor]) + "█" + string(runes[t.cursor+1:])
}

// Model is the Bubbletea model for the search panel.
type Model struct {
	theme        *theme.Manager
	rootDir      string
	query        textInput
	results      []Result
	cursor       int
	offset       int
	height       int
	width        int
	running      bool
	done         bool
	totalFiles   int
	totalMatches int
	lastQuery    string
	active       bool
}

// New creates a new search panel with a focused text input.
func New(tm *theme.Manager, rootDir string) Model {
	return Model{
		theme:   tm,
		rootDir: rootDir,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case messages.SearchOpenMsg:
		m.active = true

	case messages.SearchCloseMsg:
		m.active = false

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

	case searchBatchMsg:
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
		m.totalMatches = msg.done.TotalMatches
		m.totalFiles = msg.done.TotalFiles
		m.clampScroll()

	case messages.SearchDoneMsg:
		m.running = false
		m.done = true
		m.totalMatches = msg.TotalMatches
		m.totalFiles = msg.TotalFiles

	case runSearchMsg:
		if msg.query == "" {
			break
		}
		// Only start if query still matches what user typed.
		if msg.query != m.query.Value() {
			break
		}
		m.results = nil
		m.cursor = 0
		m.offset = 0
		m.running = true
		m.done = false
		m.totalFiles = 0
		m.totalMatches = 0
		m.lastQuery = msg.query
		cmds = append(cmds, runRipgrep(msg.query, msg.rootDir))

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
			if len(m.results) > 0 && m.cursor < len(m.results) {
				r := m.results[m.cursor]
				cmds = append(cmds, func() tea.Msg {
					return messages.FileSelectedMsg{Path: r.Path}
				})
			}
		default:
			prev := m.query.Value()
			m.query.handleKey(msg)
			if m.query.Value() != prev {
				cmds = append(cmds, m.debounceSearch(m.query.Value()))
			}
		}

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

	case tea.MouseReleaseMsg:
		if !m.active {
			break
		}
		// Reserve top 3 rows for the input area; results start at row 3.
		const headerRows = 3
		if msg.Button == tea.MouseLeft {
			row := msg.Y - headerRows
			if row >= 0 {
				idx := m.offset + row
				if idx >= 0 && idx < len(m.results) {
					m.cursor = idx
					m.clampScroll()
					r := m.results[m.cursor]
					cmds = append(cmds, func() tea.Msg {
						return messages.FileSelectedMsg{Path: r.Path}
					})
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the search panel.
func (m Model) View() tea.View {
	if m.height == 0 || m.width == 0 {
		return tea.NewView("")
	}

	// Theme colours.
	bgColor := m.theme.UI("editor_bg")
	fgColor := m.theme.UI("foreground")
	selectedBG := m.theme.UI("sidebar_selected_bg")
	selectedFG := m.theme.UI("sidebar_selected_fg")
	matchBG := m.theme.UI("search_match_bg")
	matchFG := m.theme.UI("search_match_fg")
	subtleFG := m.theme.UI("statusbar_fg")

	baseStyle := lipgloss.NewStyle()
	if bgColor != "" {
		baseStyle = baseStyle.Background(lipgloss.Color(bgColor))
	}
	if fgColor != "" {
		baseStyle = baseStyle.Foreground(lipgloss.Color(fgColor))
	}

	selectedStyle := baseStyle.Copy()
	if selectedBG != "" {
		selectedStyle = selectedStyle.Background(lipgloss.Color(selectedBG))
	}
	if selectedFG != "" {
		selectedStyle = selectedStyle.Foreground(lipgloss.Color(selectedFG))
	}

	matchStyle := lipgloss.NewStyle()
	if matchBG != "" {
		matchStyle = matchStyle.Background(lipgloss.Color(matchBG))
	}
	if matchFG != "" {
		matchStyle = matchStyle.Foreground(lipgloss.Color(matchFG))
	}

	subtleStyle := baseStyle.Copy()
	if subtleFG != "" {
		subtleStyle = subtleStyle.Foreground(lipgloss.Color(subtleFG))
	}

	var sb strings.Builder

	// Header: title + input.
	title := baseStyle.Copy().Bold(true).Render("Search")
	sb.WriteString(title + "\n")
	inputLine := baseStyle.Width(m.width).Render(m.query.View("Search..."))
	sb.WriteString(inputLine + "\n")
	sb.WriteString(baseStyle.Width(m.width).Render(strings.Repeat("─", m.width)) + "\n")

	// Results area: height minus 3 header rows and 1 summary row.
	listHeight := m.height - 4
	if listHeight < 0 {
		listHeight = 0
	}

	end := m.offset + listHeight
	if end > len(m.results) {
		end = len(m.results)
	}

	for i := m.offset; i < end; i++ {
		r := m.results[i]
		rel, err := filepath.Rel(m.rootDir, r.Path)
		if err != nil {
			rel = r.Path
		}
		label := subtleStyle.Render(fmt.Sprintf("%s:%d", rel, r.Line))

		var contentLine string
		content := strings.TrimRight(r.Content, "\n")
		if r.Start >= 0 && r.End > r.Start && r.End <= len(content) {
			pre := content[:r.Start]
			match := content[r.Start:r.End]
			post := content[r.End:]
			if i == m.cursor {
				selMatch := matchStyle.Copy()
				if selectedBG != "" {
					selMatch = selMatch.Background(lipgloss.Color(selectedBG))
				}
				contentLine = selectedStyle.Render(pre) + selMatch.Render(match) + selectedStyle.Render(post)
			} else {
				contentLine = baseStyle.Render(pre) + matchStyle.Render(match) + baseStyle.Render(post)
			}
		} else {
			if i == m.cursor {
				contentLine = selectedStyle.Render(content)
			} else {
				contentLine = baseStyle.Render(content)
			}
		}

		sb.WriteString(label + " " + contentLine + "\n")
	}

	// Pad remaining rows.
	rendered := end - m.offset
	for i := rendered; i < listHeight; i++ {
		sb.WriteString(baseStyle.Width(m.width).Render("") + "\n")
	}

	// Summary line.
	var summary string
	switch {
	case m.running:
		summary = subtleStyle.Render(fmt.Sprintf("Searching... %d matches", len(m.results)))
	case m.done:
		summary = subtleStyle.Render(fmt.Sprintf("%d matches in %d files", m.totalMatches, m.totalFiles))
	case m.lastQuery == "":
		summary = subtleStyle.Render("Type to search")
	default:
		summary = subtleStyle.Render("No results")
	}
	sb.WriteString(baseStyle.Width(m.width).Render(summary))

	return tea.NewView(sb.String())
}

// debounceSearch sleeps 200ms then returns a runSearchMsg.
func (m Model) debounceSearch(query string) tea.Cmd {
	rootDir := m.rootDir
	return func() tea.Msg {
		time.Sleep(200 * time.Millisecond)
		return runSearchMsg{query: query, rootDir: rootDir}
	}
}

// clampScroll keeps the cursor visible within the results list.
func (m *Model) clampScroll() {
	// Reserve 3 header + 1 summary = 4 fixed rows; list occupies the rest.
	listHeight := m.height - 4
	if listHeight <= 0 {
		listHeight = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listHeight {
		m.offset = m.cursor - listHeight + 1
	}
}

// runRipgrep executes ripgrep and streams results back as individual messages.
func runRipgrep(query, rootDir string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("rg", "--json", "--", query, rootDir)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return messages.SearchDoneMsg{}
		}
		if err := cmd.Start(); err != nil {
			return messages.SearchDoneMsg{}
		}

		var (
			totalMatches int
			totalFiles   int
			seen         = make(map[string]struct{})
			results      []messages.SearchResultMsg
		)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			var m rgMatch
			if err := json.Unmarshal(line, &m); err != nil {
				continue
			}
			if m.Type != "match" {
				continue
			}
			path := m.Data.Path.Text
			content := m.Data.Lines.Text
			lineNum := m.Data.LineNumber

			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				totalFiles++
			}

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

		return searchBatchMsg{
			results: results,
			done: messages.SearchDoneMsg{
				TotalMatches: totalMatches,
				TotalFiles:   totalFiles,
			},
		}
	}
}
