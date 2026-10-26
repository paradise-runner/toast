package editor

import (
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// ── Folding ──────────────────────────────────────────────────────────────────
//
// Folding hides the lines below a "header" line: an indentation block in code
// or a section under a Markdown heading. Fold regions are derived from the
// buffer text and never stored in it, so an edit can never corrupt them; only
// the set of collapsed headers is user state (`Model.collapsed`).
//
// The screen-row mapping lives in wrap.go: every line contributes its visual
// rows to the prefix table unless a collapsed span hides it (zero rows), which
// is what makes screen rows, the viewport and mouse hit-testing fold-aware in
// both wrap and non-wrap mode.

const (
	foldCaretWidth = 1
	// foldCountWidth is the gutter space reserved for the hidden-line count
	// shown next to a collapsed caret. Larger counts are capped at 999.
	foldCountWidth = 3
	foldCellWidth  = foldCaretWidth + foldCountWidth
	// gutterChrome is everything in the gutter except the line-number digits:
	// space + diff bar + fold cell + space.
	gutterChrome = 1 + 1 + foldCellWidth + 1
)

// foldRegion is one foldable range. start is the visible header line, end the
// last line hidden while the region is collapsed. depth is the nesting depth
// (0 = top level), used by FoldAll.
type foldRegion struct {
	start int
	end   int
	depth int
}

// foldSpan is a collapsed region. Spans never overlap and are sorted by start.
type foldSpan struct {
	start int
	end   int
}

// ── Region computation ───────────────────────────────────────────────────────

// computeFoldRegions derives the foldable regions for the given lines. Markdown
// files fold by heading level; everything else folds by indentation.
func computeFoldRegions(lines []string, tabWidth int, markdown bool) []foldRegion {
	if len(lines) < 2 {
		return nil
	}
	if markdown {
		return computeHeadingRegions(lines)
	}
	return computeIndentRegions(lines, tabWidth)
}

// computeIndentRegions builds one region per indentation block. A line opens a
// block when a later line is indented deeper; the block ends at the last
// non-blank line before the indentation returns to the header's level or less.
// Nested blocks are emitted with their nesting depth.
func computeIndentRegions(lines []string, tabWidth int) []foldRegion {
	n := len(lines)
	indent := make([]int, n)
	blank := make([]bool, n)
	lastNonBlank := make([]int, n)

	last := -1
	for i, line := range lines {
		blank[i] = strings.TrimSpace(line) == ""
		if !blank[i] {
			indent[i] = indentWidth(line, tabWidth)
			last = i
		}
		lastNonBlank[i] = last
	}

	var regions []foldRegion
	var stack []int // header line indexes of the open blocks

	closeTo := func(end int) {
		for len(stack) > 0 {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if end > start {
				regions = append(regions, foldRegion{start: start, end: end, depth: len(stack)})
			}
		}
	}

	for i := 0; i < n; i++ {
		if blank[i] {
			continue
		}
		for len(stack) > 0 && indent[i] <= indent[stack[len(stack)-1]] {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			headerIndent := indent[start]
			end := i - 1
			if end >= 0 {
				end = lastNonBlank[end]
			}
			// A line that closes a bracket at the header's own indent belongs
			// to the block (Go's `}` at column 0, a trailing `)` of a call),
			// so folding a function hides its closing brace too. Lines like
			// `} else {` are excluded: they carry more than closers.
			if indent[i] == headerIndent && closingOnly(lines[i]) {
				end = i
			}
			if end > start {
				regions = append(regions, foldRegion{start: start, end: end, depth: len(stack)})
			}
		}
		stack = append(stack, i)
	}
	closeTo(lastNonBlank[n-1])

	sort.Slice(regions, func(i, j int) bool {
		if regions[i].start != regions[j].start {
			return regions[i].start < regions[j].start
		}
		return regions[i].end < regions[j].end
	})
	return regions
}

// computeHeadingRegions builds one region per Markdown heading section. A
// section ends before the next heading of the same or a higher level, so the
// document outline is foldable even when its body is not indented. Fenced code
// blocks are skipped so `# comment` inside a fence is not treated as a heading.
func computeHeadingRegions(lines []string) []foldRegion {
	n := len(lines)
	levels := make([]int, n)
	minLevel := 0
	fenced := false
	fence := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fenced {
			if strings.HasPrefix(trimmed, fence) {
				fenced = false
			}
			continue
		}
		if mark, ok := markdownFence(trimmed); ok {
			fenced = true
			fence = mark
			continue
		}
		if level := markdownHeadingLevel(line); level > 0 {
			levels[i] = level
			if minLevel == 0 || level < minLevel {
				minLevel = level
			}
		}
	}

	var regions []foldRegion
	for i := 0; i < n; i++ {
		if levels[i] == 0 {
			continue
		}
		end := n - 1
		for j := i + 1; j < n; j++ {
			if levels[j] > 0 && levels[j] <= levels[i] {
				end = j - 1
				break
			}
		}
		for end > i && strings.TrimSpace(lines[end]) == "" {
			end--
		}
		if end > i {
			regions = append(regions, foldRegion{start: i, end: end, depth: levels[i] - minLevel})
		}
	}
	return regions
}

// closingOnly reports whether a line contains nothing but closing brackets and
// separators (optionally followed by a comment), i.e. it is the tail of the
// block opened on a previous line rather than a new statement.
func closingOnly(line string) bool {
	trimmed := strings.TrimSpace(line)
	if i := strings.Index(trimmed, "//"); i >= 0 {
		trimmed = strings.TrimSpace(trimmed[:i])
	}
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		switch r {
		case '}', ')', ']', ';', ',':
		default:
			return false
		}
	}
	return true
}

// indentWidth returns the display width of a line's leading whitespace, with
// tabs expanded to the editor's tab width.
func indentWidth(line string, tabWidth int) int {
	col := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			col++
		case '\t':
			col = nextDisplayColumn(col, "\t", tabWidth)
		default:
			return col
		}
	}
	return col
}

// markdownFence reports the fence marker of a code-fence line.
func markdownFence(trimmed string) (string, bool) {
	for _, mark := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, mark) {
			return mark, true
		}
	}
	return "", false
}

// markdownHeadingLevel returns the ATX heading level of a line (1-6), or 0 when
// the line is not a heading. Up to three leading spaces are allowed, and the
// hashes must be followed by a space/tab or the end of the line so that
// "#hashtag" is not mistaken for a heading.
func markdownHeadingLevel(line string) int {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	j := i
	for j < len(line) && line[j] == '#' {
		j++
	}
	level := j - i
	if level < 1 || level > 6 {
		return 0
	}
	if j < len(line) && line[j] != ' ' && line[j] != '\t' {
		return 0
	}
	return level
}

// ── Region cache ─────────────────────────────────────────────────────────────

// ensureFolds rebuilds the region table when the buffer content (or path)
// changed, and re-anchors collapsed spans whose header still opens a region.
func (m *Model) ensureFolds() {
	if m.buf == nil {
		m.foldRegions = nil
		m.foldStarts = nil
		m.collapsed = nil
		return
	}
	gen := m.buf.Generation()
	if m.foldStarts != nil && m.foldCacheGen == gen && m.foldCachePath == m.path {
		return
	}

	lines := splitLines(m.buf.String())
	m.foldRegions = computeFoldRegions(lines, normalizedTabWidth(m.cfg.Editor.TabWidth), isMarkdownPath(m.path))
	m.foldStarts = make([]int, len(lines))
	for _, region := range m.foldRegions {
		if region.start < 0 || region.start >= len(m.foldStarts) {
			continue
		}
		if end := region.end + 1; end > m.foldStarts[region.start] {
			m.foldStarts[region.start] = end
		}
	}
	m.pruneCollapsed()

	m.foldCacheGen = gen
	m.foldCachePath = m.path
	m.foldRev++
}

// invalidateFolds discards the derived fold region cache so the next
// ensureFolds pass rebuilds it from the current buffer text.
func (m *Model) invalidateFolds() {
	m.foldRegions = nil
	m.foldStarts = nil
	m.foldCacheGen = -1
	m.foldCachePath = ""
	m.foldRev++
}

// pruneCollapsed drops collapsed spans whose header no longer opens a region
// and widens/narrows surviving spans to their region's new extent, so a fold
// stays folded while its body is edited.
func (m *Model) pruneCollapsed() {
	if len(m.collapsed) == 0 {
		return
	}
	kept := m.collapsed[:0]
	for _, span := range m.collapsed {
		if end, ok := m.foldEnd(span.start); ok {
			kept = append(kept, foldSpan{start: span.start, end: end})
		}
	}
	m.collapsed = kept
}

// foldEnd returns the last line hidden when the region starting at line is
// collapsed. ensureFolds must have run.
func (m *Model) foldEnd(line int) (int, bool) {
	if line < 0 || line >= len(m.foldStarts) {
		return 0, false
	}
	if v := m.foldStarts[line]; v != 0 {
		return v - 1, true
	}
	return 0, false
}

// isHidden reports whether line is hidden by a collapsed span.
func (m *Model) isHidden(line int) bool {
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= line })
	return i > 0 && line <= m.collapsed[i-1].end
}

// isCollapsed reports whether the fold starting at line is collapsed.
func (m *Model) isCollapsed(line int) bool {
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= line })
	return i < len(m.collapsed) && m.collapsed[i].start == line
}

// collapsedIndex returns the index of the collapsed span starting at line.
func (m *Model) collapsedIndex(line int) (int, bool) {
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= line })
	if i < len(m.collapsed) && m.collapsed[i].start == line {
		return i, true
	}
	return 0, false
}

// ── Toggling ─────────────────────────────────────────────────────────────────

// collapse folds the region starting at line, replacing any span that overlaps
// it so the collapsed set stays sorted and non-overlapping.
func (m *Model) collapse(line int) bool {
	m.ensureFolds()
	end, ok := m.foldEnd(line)
	if !ok {
		return false
	}
	if _, exists := m.collapsedIndex(line); exists {
		return false
	}
	kept := m.collapsed[:0]
	for _, span := range m.collapsed {
		if span.end < line || span.start > end {
			kept = append(kept, span)
		}
	}
	m.collapsed = append(kept, foldSpan{start: line, end: end})
	sort.Slice(m.collapsed, func(i, j int) bool { return m.collapsed[i].start < m.collapsed[j].start })
	m.foldRev++
	return true
}

// expand unfolds the region starting at line.
func (m *Model) expand(line int) bool {
	i, ok := m.collapsedIndex(line)
	if !ok {
		return false
	}
	m.collapsed = append(m.collapsed[:i], m.collapsed[i+1:]...)
	m.foldRev++
	return true
}

// toggleFoldAt folds or unfolds the region starting at line and returns true
// when a region was affected.
func (m *Model) toggleFoldAt(line int) bool {
	m.ensureFolds()
	if m.isCollapsed(line) {
		m.expand(line)
	} else if !m.collapse(line) {
		return false
	}
	m.snapCursorToVisible()
	m.ensureViewportVisible()
	return true
}

// snapCursorToVisible moves the cursor to the header of the span hiding it. A
// caret clicked on a header above the cursor (or FoldAll) must not leave the
// cursor on a hidden line, because the next clampViewport would expand the fold
// again to reveal it.
func (m *Model) snapCursorToVisible() {
	if !m.isHidden(m.cursor.line) {
		return
	}
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= m.cursor.line })
	if i == 0 {
		return
	}
	m.cursor.line = m.collapsed[i-1].start
	m.cursor.col = m.clampCol(m.cursor.line, m.preferredCol)
	m.preferredCol = m.cursor.col
}

// foldTargetLine returns the line whose region ToggleFold should act on: the
// cursor line when it is a header, otherwise the innermost region containing
// the cursor.
func (m *Model) foldTargetLine(line int) (int, bool) {
	m.ensureFolds()
	if _, ok := m.foldEnd(line); ok {
		return line, true
	}
	best := -1
	for _, region := range m.foldRegions {
		if region.start <= line && line <= region.end && region.start > best {
			best = region.start
		}
	}
	if best < 0 {
		return 0, false
	}
	return best, true
}

// ToggleFold folds or unfolds the region at the cursor (or the innermost region
// containing it).
func (m *Model) ToggleFold() {
	if m.buf == nil || m.cannotDisplayFile() {
		return
	}
	m.ensureFolds()
	if line, ok := m.foldTargetLine(m.cursor.line); ok {
		m.toggleFoldAt(line)
	}
}

// FoldAll collapses every top-level region.
func (m *Model) FoldAll() {
	if m.buf == nil || m.cannotDisplayFile() {
		return
	}
	m.ensureFolds()
	spans := make([]foldSpan, 0, len(m.foldRegions))
	for _, region := range m.foldRegions {
		if region.depth == 0 {
			spans = append(spans, foldSpan{start: region.start, end: region.end})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	m.collapsed = spans
	m.foldRev++
	m.snapCursorToVisible()
	m.ensureViewportVisible()
	m.clampViewport()
}

// UnfoldAll expands every collapsed region.
func (m *Model) UnfoldAll() {
	if m.buf == nil {
		return
	}
	if len(m.collapsed) > 0 {
		m.collapsed = nil
		m.foldRev++
	}
	m.clampViewport()
}

// revealLine expands the collapsed span hiding line, if any. The cursor must
// never sit on a hidden line, so every cursor placement (go-to-line, find,
// LSP navigation, undo) reveals its target.
func (m *Model) revealLine(line int) {
	m.ensureFolds()
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= line })
	if i > 0 && line <= m.collapsed[i-1].end {
		m.collapsed = append(m.collapsed[:i-1], m.collapsed[i:]...)
		m.foldRev++
	}
}

// visibleLineAtOrAfter returns line when it is visible, otherwise the first
// visible line after the span hiding it.
func (m *Model) visibleLineAtOrAfter(line int) int {
	lineCount := 0
	if m.buf != nil {
		lineCount = m.buf.LineCount()
	}
	if lineCount == 0 {
		return 0
	}
	if line < 0 {
		line = 0
	}
	if line >= lineCount {
		line = lineCount - 1
	}
	if !m.isHidden(line) {
		return line
	}
	i := sort.Search(len(m.collapsed), func(i int) bool { return m.collapsed[i].start >= line })
	if i > 0 {
		if next := m.collapsed[i-1].end + 1; next < lineCount {
			return next
		}
	}
	// The hidden line is in the last visible span: fall back to the header,
	// which is always visible.
	return m.collapsed[i-1].start
}

// ensureViewportVisible keeps the viewport anchored on a visible line after a
// fold toggle.
func (m *Model) ensureViewportVisible() {
	if m.buf == nil || m.buf.LineCount() == 0 {
		m.viewportTop = 0
		return
	}
	m.viewportTop = m.visibleLineAtOrAfter(m.viewportTop)
}

// ── Movement helpers ─────────────────────────────────────────────────────────

// prevVisibleLine returns the closest visible line above line.
func (m *Model) prevVisibleLine(line int) (int, bool) {
	if line <= 0 {
		return 0, false
	}
	row := m.visualRowFromTop(line)
	if row <= 0 {
		return 0, false
	}
	return m.lineAtRow(row - 1), true
}

// nextVisibleLine returns the closest visible line below line.
func (m *Model) nextVisibleLine(line int) (int, bool) {
	row := m.visualRowFromTop(line) + m.visualRowsForLine(line)
	if row >= m.totalRows() {
		return 0, false
	}
	return m.lineAtRow(row), true
}

// lineNumWidth is the gutter space occupied by the line-number digits.
func (m *Model) lineNumWidth() int {
	if width := m.gutterWidth - gutterChrome; width >= 1 {
		return width
	}
	return 1
}

// foldCaretX returns the screen column of the fold caret.
func (m *Model) foldCaretX() int {
	return m.lineNumWidth() + 2
}

// foldCaretHit reports whether (x, y) lands on a fold indicator cell.
func (m *Model) foldCaretHit(x, y int) bool {
	caretX := m.foldCaretX()
	return x >= caretX && x < caretX+foldCellWidth && y >= 0 && y < m.viewHeight
}

// foldCaretLine returns the buffer line under screen row y.
func (m *Model) foldCaretLine(y int) (int, bool) {
	if m.buf == nil || m.viewHeight <= 0 || y < 0 || y >= m.viewHeight {
		return 0, false
	}
	targetRow := m.visualRowFromTop(m.viewportTop) + y
	if targetRow >= m.totalRows() {
		return 0, false
	}
	return m.lineAtRow(targetRow), true
}

// foldCountLabel formats the hidden-line count. The gutter reserves
// foldCountWidth columns, so larger counts are capped rather than shifting the
// content out of alignment.
func foldCountLabel(hidden int) string {
	if hidden < 0 {
		hidden = 0
	}
	max := 1
	for i := 0; i < foldCountWidth; i++ {
		max *= 10
	}
	max--
	if hidden > max {
		return strconv.Itoa(max)
	}
	return strconv.Itoa(hidden)
}

// renderFoldCell renders the caret and hidden-line count for one row. A row
// that is not a fold header renders blank so the gutter stays aligned.
func (m Model) renderFoldCell(line int) string {
	caret := " "
	caretColor := m.uiColor("gutter_fg", "")
	count := ""
	if end, ok := m.foldEnd(line); ok {
		if m.isCollapsed(line) {
			caret = "▸"
			count = foldCountLabel(end - line)
		} else {
			caret = "▾"
		}
	}
	if m.theme != nil && line == m.cursor.line {
		if active := m.theme.UI("gutter_active_fg"); active != "" {
			caretColor = active
		}
	}
	countColor := m.uiColor("muted", caretColor)
	bg := lipgloss.Color(m.uiColor("background", ""))
	if caretColor == "" {
		caretColor = countColor
	}
	padded := strings.Repeat(" ", foldCountWidth-len(count)) + count
	return lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(caretColor)).Render(caret) +
		lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color(countColor)).Render(padded)
}
