package preview

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/yourusername/toast/internal/theme"
)

// Model is a scrollable markdown preview pane.
type Model struct {
	theme  *theme.Manager
	width  int
	height int

	// raw markdown content
	content string

	// rendered lines (ANSI-styled)
	lines []string

	// scroll offset (line index of first visible line)
	offset int
}

func New(tm *theme.Manager) Model {
	return Model{theme: tm}
}

// SetContent re-renders the markdown and resets the scroll position.
func (m *Model) SetContent(markdown string) {
	m.content = markdown
	m.offset = 0
	m.rerender()
}

// rerender rebuilds m.lines from m.content using a theme-derived glamour style.
func (m *Model) rerender() {
	if m.width == 0 {
		m.lines = nil
		return
	}

	style := m.buildStyle()

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(m.width),
	)
	if err != nil {
		m.lines = []string{"[preview error: " + err.Error() + "]"}
		return
	}

	rendered, err := r.Render(m.content)
	if err != nil {
		m.lines = []string{"[preview error: " + err.Error() + "]"}
		return
	}

	m.lines = strings.Split(rendered, "\n")
}

// ptr returns a pointer to v (needed for glamour's *bool / *string / *uint fields).
func ptr[T any](v T) *T { return &v }

// buildStyle constructs a glamour StyleConfig from the active toast theme.
func (m *Model) buildStyle() ansi.StyleConfig {
	// UI colors
	fg := m.theme.UI("foreground")
	bg := m.theme.UI("background")
	codeBG := m.theme.UI("completion_bg") // slightly elevated surface
	subdued := m.theme.UI("breadcrumbs_fg")
	gutterFG := m.theme.UI("gutter_fg")

	// Syntax colors
	keyword := m.theme.SyntaxFG("keyword")
	str := m.theme.SyntaxFG("string")
	comment := m.theme.SyntaxFG("comment")
	function := m.theme.SyntaxFG("function")
	typ := m.theme.SyntaxFG("type")
	number := m.theme.SyntaxFG("number")
	operator := m.theme.SyntaxFG("operator")
	constant := m.theme.SyntaxFG("constant")
	punctuation := m.theme.SyntaxFG("punctuation")
	builtin := m.theme.SyntaxFG("builtin")
	module := m.theme.SyntaxFG("module")

	// Fallback: use foreground when a specific color isn't set.
	if keyword == "" { keyword = fg }
	if str == "" { str = fg }
	if comment == "" { comment = subdued }
	if function == "" { function = fg }
	if typ == "" { typ = fg }
	if number == "" { number = fg }
	if operator == "" { operator = fg }
	if constant == "" { constant = fg }
	if punctuation == "" { punctuation = fg }
	if builtin == "" { builtin = fg }
	if module == "" { module = fg }
	if codeBG == "" { codeBG = bg }

	var margin uint = 1

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           ptr(fg),
				BackgroundColor: ptr(bg),
			},
			Margin: ptr(margin),
		},

		// Headings — cascade from big to small using syntax accent colors.
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:       ptr(true),
				BlockSuffix: "\n",
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(function),
				Bold:   ptr(true),
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(keyword),
				Bold:   ptr(true),
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(typ),
				Bold:   ptr(true),
				Prefix: "### ",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(str),
				Bold:   ptr(true),
				Prefix: "#### ",
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(fg),
				Prefix: "##### ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(subdued),
				Prefix: "###### ",
			},
		},

		Paragraph: ansi.StyleBlock{},

		Text: ansi.StylePrimitive{},
		Strong: ansi.StylePrimitive{
			Bold:  ptr(true),
			Color: ptr(fg),
		},
		Emph: ansi.StylePrimitive{
			Italic: ptr(true),
			Color:  ptr(comment),
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: ptr(true),
			Color:      ptr(subdued),
		},

		HorizontalRule: ansi.StylePrimitive{
			Color:  ptr(gutterFG),
			Format: "\n--------\n",
		},

		// Lists
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: ptr(fg)},
			},
			LevelIndent: 2,
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       ptr(fg),
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       ptr(fg),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{Color: ptr(fg)},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},

		// Block quote — subdued with a bar prefix.
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  ptr(comment),
				Italic: ptr(true),
			},
			Indent:      ptr(uint(1)),
			IndentToken: ptr("│ "),
		},

		// Inline code — string color on the elevated surface.
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           ptr(str),
				BackgroundColor: ptr(codeBG),
			},
		},

		// Fenced code blocks — full syntax via Chroma.
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: ptr(fg),
				},
				Margin: ptr(uint(1)),
			},
			Chroma: &ansi.Chroma{
				Text:           ansi.StylePrimitive{Color: ptr(fg)},
				Comment:        ansi.StylePrimitive{Color: ptr(comment), Italic: ptr(true)},
				CommentPreproc: ansi.StylePrimitive{Color: ptr(module)},
				Keyword:        ansi.StylePrimitive{Color: ptr(keyword), Bold: ptr(true)},
				KeywordType:    ansi.StylePrimitive{Color: ptr(typ)},
				Operator:       ansi.StylePrimitive{Color: ptr(operator)},
				Punctuation:    ansi.StylePrimitive{Color: ptr(punctuation)},
				Name:           ansi.StylePrimitive{Color: ptr(fg)},
				NameBuiltin:    ansi.StylePrimitive{Color: ptr(builtin)},
				NameTag:        ansi.StylePrimitive{Color: ptr(keyword)},
				NameAttribute:  ansi.StylePrimitive{Color: ptr(typ)},
				NameClass:      ansi.StylePrimitive{Color: ptr(typ), Bold: ptr(true), Underline: ptr(true)},
				NameConstant:   ansi.StylePrimitive{Color: ptr(constant)},
				NameFunction:   ansi.StylePrimitive{Color: ptr(function)},
				LiteralNumber:  ansi.StylePrimitive{Color: ptr(number)},
				LiteralString:  ansi.StylePrimitive{Color: ptr(str)},
				LiteralStringEscape: ansi.StylePrimitive{Color: ptr(constant)},
				GenericDeleted:  ansi.StylePrimitive{Color: ptr(m.theme.Git("deleted"))},
				GenericInserted: ansi.StylePrimitive{Color: ptr(m.theme.Git("added"))},
				GenericEmph:     ansi.StylePrimitive{Italic: ptr(true)},
				GenericStrong:   ansi.StylePrimitive{Bold: ptr(true)},
				GenericSubheading: ansi.StylePrimitive{Color: ptr(subdued)},
				Background:      ansi.StylePrimitive{BackgroundColor: ptr(codeBG)},
			},
		},

		// Links
		Link: ansi.StylePrimitive{
			Color:     ptr(function),
			Underline: ptr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: ptr(typ),
			Bold:  ptr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     ptr(str),
			Underline: ptr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  ptr(subdued),
			Format: "Image: {{.text}} →",
		},

		// Table
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: ptr(fg)},
			},
			CenterSeparator: ptr("┼"),
			ColumnSeparator: ptr("│"),
			RowSeparator:    ptr("─"),
		},
	}
}

// Update handles messages for the preview pane.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rerender()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			m.offset = m.clampOffset(m.offset + 1)
		case "pgup", "ctrl+u":
			m.offset = m.clampOffset(m.offset - m.height/2)
		case "pgdown", "ctrl+d":
			m.offset = m.clampOffset(m.offset + m.height/2)
		case "home", "g":
			m.offset = 0
		case "end", "G":
			m.offset = m.maxOffset()
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			if m.offset > 0 {
				m.offset--
			}
		case tea.MouseWheelDown:
			m.offset = m.clampOffset(m.offset + 1)
		}
	}
	return m, nil
}

func (m Model) maxOffset() int {
	max := len(m.lines) - m.height
	if max < 0 {
		return 0
	}
	return max
}

func (m Model) clampOffset(o int) int {
	max := m.maxOffset()
	if o < 0 {
		return 0
	}
	if o > max {
		return max
	}
	return o
}

// View renders the visible portion of the markdown preview.
func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("")
	}

	bg := lipgloss.Color(m.theme.UI("background"))
	base := lipgloss.NewStyle().Background(bg)

	if len(m.lines) == 0 {
		return tea.NewView(base.Width(m.width).Height(m.height).Render(""))
	}

	var sb strings.Builder
	for i := 0; i < m.height; i++ {
		lineIdx := m.offset + i
		if lineIdx < len(m.lines) {
			sb.WriteString(m.lines[lineIdx])
		}
		if i < m.height-1 {
			sb.WriteByte('\n')
		}
	}

	return tea.NewView(sb.String())
}
