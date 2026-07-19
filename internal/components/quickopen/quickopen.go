// Package quickopen implements a Ctrl+P quick-open overlay with fuzzy file
// search across the workspace.
package quickopen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourusername/toast/internal/messages"
	"github.com/yourusername/toast/internal/theme"
)

const (
	overlayWidth     = 60
	overlayMinHeight = 6  // border + input + separator + summary + border + padding
	overlayMaxHeight = 24
	maxDisplayItems  = 18
	maxFiles         = 10000 // cap to avoid memory issues
)

// ── Text input ────────────────────────────────────────────────────────────────

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
		if t.cursor < len([]rune(t.value)) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len([]rune(t.value))
	case "ctrl+u":
		runes := []rune(t.value)
		t.value = string(runes[t.cursor:])
		t.cursor = 0
	case "ctrl+k":
		runes := []rune(t.value)
		t.value = string(runes[:t.cursor])
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

// ── File item ─────────────────────────────────────────────────────────────────

// FileItem represents a file in the quick open results.
type FileItem struct {
	Path         string
	RelativePath string
}

// ── Internal messages ─────────────────────────────────────────────────────────

// LoadFilesMsg carries the full list of project files loaded in the background.
type LoadFilesMsg struct {
	Files []FileItem
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the quick open overlay component.
type Model struct {
	theme   *theme.Manager
	rootDir string
	ignored []string

	open    bool
	query   textInput
	results []FileItem
	files   []FileItem // full project file list (cached after first load)
	cursor  int
	loaded  bool

	width  int
	height int
}

// New creates a quick open model.
func New(tm *theme.Manager, rootDir string, ignoredPatterns []string) Model {
	return Model{
		theme:   tm,
		rootDir: rootDir,
		ignored: ignoredPatterns,
	}
}

// Open returns a model with the overlay activated and a command to load
// the project file list if it hasn't been loaded yet.
func (m Model) Open() (Model, tea.Cmd) {
	m.open = true
	m.query = textInput{}
	m.cursor = 0
	m.results = nil
	if m.loaded {
		// Already have files; re-filter with empty query (shows all).
		m.filterResults()
		return m, nil
	}
	return m, m.loadFiles()
}

// IsOpen reports whether the overlay is visible.
func (m Model) IsOpen() bool { return m.open }

// Update handles messages for the quick open overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.open {
		return m, nil
	}

	switch msg := msg.(type) {
	case LoadFilesMsg:
		m.files = msg.Files
		sort.Slice(m.files, func(i, j int) bool {
			return m.files[i].RelativePath < m.files[j].RelativePath
		})
		m.loaded = true
		m.filterResults()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "escape":
			m.open = false
			return m, func() tea.Msg { return messages.QuickOpenCloseMsg{} }

		case "enter":
			if len(m.results) > 0 && m.cursor >= 0 && m.cursor < len(m.results) {
				path := m.results[m.cursor].Path
				m.open = false
				return m, func() tea.Msg { return messages.FileSelectedMsg{Path: path} }
			}

		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "ctrl+p", "ctrl+n", "ctrl+j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}

		default:
			prev := m.query.Value()
			m.query.handleKey(msg)
			if m.query.Value() != prev {
				m.cursor = 0
				m.filterResults()
			}
		}
	}

	return m, nil
}

// View renders the quick open overlay. Returns an empty string when closed.
func (m Model) View() string {
	if !m.open {
		return ""
	}

	bgStr := m.theme.UI("completion_bg")
	if bgStr == "" {
		bgStr = "#313244"
	}
	fgStr := m.theme.UI("completion_fg")
	if fgStr == "" {
		fgStr = "#cdd6f4"
	}
	borderStr := m.theme.UI("hover_border")
	if borderStr == "" {
		borderStr = "#585b70"
	}
	selBgStr := m.theme.UI("sidebar_selected_bg")
	if selBgStr == "" {
		selBgStr = "#45475a"
	}
	selFgStr := m.theme.UI("sidebar_selected_fg")
	if selFgStr == "" {
		selFgStr = "#cdd6f4"
	}

	bg := lipgloss.Color(bgStr)
	fg := lipgloss.Color(fgStr)
	border := lipgloss.Color(borderStr)
	selBg := lipgloss.Color(selBgStr)
	selFg := lipgloss.Color(selFgStr)

	innerWidth := overlayWidth - 4 // border (2) + padding (2)

	// ── Build content lines ────────────────────────────────────────────────

	var lines []string

	// Input line
	inputText := "Quick Open: " + m.query.View("Search files...")
	inputRunes := []rune(inputText)
	if len(inputRunes) < innerWidth {
		inputText += strings.Repeat(" ", innerWidth-len(inputRunes))
	}
	lines = append(lines, inputText)

	// Separator
	lines = append(lines, strings.Repeat("─", innerWidth))

	// Results
	display := m.results
	if len(display) > maxDisplayItems {
		display = display[:maxDisplayItems]
	}

	for _, item := range display {
		rel := item.RelativePath
		// Truncate if needed
		relRunes := []rune(rel)
		if len(relRunes)+2 > innerWidth {
			// Show end of the path if it's too long
			keep := innerWidth - 5 // "…" + " " + prefix
			if keep < 10 {
				keep = 10
			}
			rel = "…" + string(relRunes[len(relRunes)-keep:])
		}
		line := " " + rel
		lineRunes := []rune(line)
		if len(lineRunes) < innerWidth {
			line += strings.Repeat(" ", innerWidth-len(lineRunes))
		}
		lines = append(lines, line)
	}

	// Calculate rendered lines so far (content inside border/padding)
	contentLines := len(lines)

	// Summary line
	var summary string
	switch {
	case !m.loaded:
		summary = "Loading files..."
	case m.query.Value() == "":
		summary = fmt.Sprintf("%d files in project", len(m.files))
	case len(m.results) == 0:
		summary = "No matching files"
	default:
		summary = fmt.Sprintf("%d files matched", len(m.results))
	}
	summaryRunes := []rune(summary)
	if len(summaryRunes) < innerWidth {
		summary += strings.Repeat(" ", innerWidth-len(summaryRunes))
	}
	lines = append(lines, summary)

	// Convert lines to styled text with ANSI
	var styledLines []string
	for i, line := range lines {
		if i == 0 {
			// Input line: bold
			styledLines = append(styledLines,
				lipgloss.NewStyle().Bold(true).Foreground(fg).Background(bg).Render(line))
		} else if i == 1 {
			// Separator
			styledLines = append(styledLines,
				lipgloss.NewStyle().Foreground(border).Background(bg).Render(line))
		} else if i < contentLines {
			// Result rows
			resultIdx := i - 2 // skip input and separator
			if resultIdx < len(display) && resultIdx == m.cursor {
				styledLines = append(styledLines,
					lipgloss.NewStyle().
						Background(selBg).
						Foreground(selFg).
						Render("▸"+line[1:]))
			} else {
				styledLines = append(styledLines,
					lipgloss.NewStyle().Foreground(fg).Background(bg).Render(line))
			}
		} else {
			// Summary
			styledLines = append(styledLines,
				lipgloss.NewStyle().Foreground(border).Background(bg).Render(line))
		}
	}

	content := strings.Join(styledLines, "\n")

	// Determine box height based on how many result lines we have
	boxHeight := len(lines) + 2 // border top + bottom
	if boxHeight < overlayMinHeight {
		boxHeight = overlayMinHeight
	}
	if boxHeight > overlayMaxHeight {
		boxHeight = overlayMaxHeight
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(bg).
		Background(bg).
		Foreground(fg).
		Padding(0, 1).
		Width(overlayWidth).
		Height(boxHeight).
		Render(content)

	return box
}

// Render is an alias for View, matching the pattern used by other overlays.
func (m Model) Render() string { return m.View() }

// ── File loading ──────────────────────────────────────────────────────────────

// loadFiles returns a command that walks the project directory and collects
// all file paths matching the ignored patterns.
func (m Model) loadFiles() tea.Cmd {
	rootDir := m.rootDir
	ignored := m.ignored

	return func() tea.Msg {
		var files []FileItem

		err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if path == rootDir {
				return nil
			}

			// Skip hidden directories and files matching ignored patterns.
			if d.IsDir() {
				if isIgnored(d.Name(), ignored) {
					return filepath.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			if isIgnored(d.Name(), ignored) {
				return nil
			}

			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			rel, err := filepath.Rel(rootDir, path)
			if err != nil {
				rel = path
			}

			files = append(files, FileItem{
				Path:         path,
				RelativePath: rel,
			})

			if len(files) >= maxFiles {
				return filepath.SkipAll
			}

			return nil
		})
		if err != nil {
			// Return whatever we have on error
		}

		return LoadFilesMsg{Files: files}
	}
}

// ── Fuzzy matching ────────────────────────────────────────────────────────────

// fuzzyMatchScore returns true with a score if query matches target (case-insensitive).
// Characters in query must appear in order within target. The score favours
// consecutive matches, matches after separators, and matches at word boundaries.
func fuzzyMatchScore(target, query string) (bool, int) {
	if query == "" {
		return true, 0
	}

	targetLower := strings.ToLower(target)
	queryLower := strings.ToLower(query)

	qi := 0
	score := 0
	lastMatch := -2 // ensure first match doesn't get a consecutive bonus

	for si := 0; si < len(targetLower) && qi < len(queryLower); si++ {
		if targetLower[si] == queryLower[qi] {
			// Consecutive match bonus
			if si == lastMatch+1 {
				score += 8
			}
			// Match after path separator bonus
			if si == 0 || targetLower[si-1] == '/' || targetLower[si-1] == '\\' {
				score += 12
			}
			// Word boundary bonus (separator chars)
			if si > 0 && (targetLower[si-1] == '_' || targetLower[si-1] == '-' ||
				targetLower[si-1] == '.' || targetLower[si-1] == ' ' ||
				targetLower[si-1] == '(' || targetLower[si-1] == '[') {
				score += 8
			}
			// Uppercase in camelCase bonus (only applies when query is lowercase
			// but target has uppercase at this position)
			if target[si] >= 'A' && target[si] <= 'Z' && queryLower[qi] == query[qi] {
				score += 6
			}
			score += 1
			qi++
			lastMatch = si
		}
	}

	if qi == len(queryLower) {
		return true, score
	}
	return false, 0
}

// filterResults updates m.results by fuzzy-matching m.query.Value() against all
// project files. Results are sorted by descending score (best match first).
func (m *Model) filterResults() {
	if !m.loaded {
		m.results = nil
		return
	}

	query := m.query.Value()
	if query == "" {
		// Empty query: show all files, sorted alphabetically.
		m.results = make([]FileItem, len(m.files))
		copy(m.results, m.files)
		return
	}

	type scored struct {
		item  FileItem
		score int
	}

	var scoredResults []scored
	for _, file := range m.files {
		if ok, score := fuzzyMatchScore(file.RelativePath, query); ok {
			scoredResults = append(scoredResults, scored{item: file, score: score})
		}
	}

	// Sort by score descending; ties broken by path length then alphabetically.
	sort.Slice(scoredResults, func(i, j int) bool {
		if scoredResults[i].score != scoredResults[j].score {
			return scoredResults[i].score > scoredResults[j].score
		}
		if len(scoredResults[i].item.RelativePath) != len(scoredResults[j].item.RelativePath) {
			return len(scoredResults[i].item.RelativePath) < len(scoredResults[j].item.RelativePath)
		}
		return scoredResults[i].item.RelativePath < scoredResults[j].item.RelativePath
	})

	m.results = make([]FileItem, len(scoredResults))
	for i, s := range scoredResults {
		m.results[i] = s.item
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// isIgnored checks if a file or directory name matches any of the given patterns.
func isIgnored(name string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
	}
	return false
}
