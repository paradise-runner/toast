package commandpalette

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/paradise-runner/toast/internal/config"
	"github.com/paradise-runner/toast/internal/messages"
	"github.com/paradise-runner/toast/internal/theme"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	tm, err := theme.NewManager("toast-dark", "")
	if err != nil {
		t.Fatalf("theme.NewManager: %v", err)
	}
	return New(tm, config.DefaultKeybindings())
}

func commandByID(id string) Command {
	for _, c := range DefaultCommands() {
		if c.ID == id {
			return c
		}
	}
	return Command{}
}

func TestDefaultCommands_WellFormed(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range DefaultCommands() {
		if c.ID == "" {
			t.Fatalf("command %q has empty ID", c.Label)
		}
		if c.Label == "" {
			t.Fatalf("command %q has empty label", c.ID)
		}
		if c.Category == "" {
			t.Fatalf("command %q has empty category", c.ID)
		}
		if seen[c.ID] {
			t.Fatalf("duplicate command ID %q", c.ID)
		}
		seen[c.ID] = true
		// Theme Picker is a palette-only action (no default keybinding); every
		// other command must exist in the default keybinding map.
		if _, ok := config.DefaultKeybindings()[c.ID]; !ok && c.ID != config.ActionThemePicker {
			t.Errorf("command %q (%s) is missing a default keybinding and is not palette-only", c.Label, c.ID)
		}
	}
}

func TestOpenResetsState(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	if !m.IsOpen() {
		t.Fatal("expected palette to be open")
	}
	if m.query.Value() != "" {
		t.Fatalf("expected empty query, got %q", m.query.Value())
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}
	if len(m.results) != len(DefaultCommands()) {
		t.Fatalf("expected %d results, got %d", len(DefaultCommands()), len(m.results))
	}
}

func TestFilterFuzzyOrdering(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	for _, r := range "save" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}
	if len(m.results) == 0 {
		t.Fatal("expected results for query 'save'")
	}
	if m.results[0].ID != config.ActionSave {
		t.Fatalf("expected Save first, got %q", m.results[0].ID)
	}
}

func TestSelectEmitsMessageWithLabel(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	for _, r := range "quit" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}
	m, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	selectMsg, ok := cmd().(messages.CommandPaletteSelectMsg)
	if !ok {
		t.Fatalf("expected CommandPaletteSelectMsg, got %T", cmd())
	}
	if selectMsg.ActionID != config.ActionQuit {
		t.Fatalf("expected ActionQuit, got %q", selectMsg.ActionID)
	}
	if selectMsg.Label == "" {
		t.Fatal("expected label to be populated")
	}
	if m.IsOpen() {
		t.Fatal("expected palette to close on select")
	}
}

func TestMRUOrderingEmptyQuery(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	// Move to the last result and select it.
	for m.cursor < len(m.results)-1 {
		m, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	}
	last := m.results[m.cursor]
	m, cmd := m.Update(tea.KeyPressMsg{Text: "enter"})
	if cmd == nil {
		t.Fatal("expected select cmd")
	}
	if selectMsg, ok := cmd().(messages.CommandPaletteSelectMsg); !ok || selectMsg.ActionID != last.ID {
		t.Fatalf("expected select of %q, got %T %+v", last.ID, cmd(), cmd())
	}
	if m.usage[last.ID] != 1 {
		t.Fatalf("expected usage count 1, got %d", m.usage[last.ID])
	}

	// Reopening with an empty query surfaces the most-recently-used command first.
	m = m.Open()
	if m.results[0].ID != last.ID {
		t.Fatalf("expected MRU %q first, got %q", last.ID, m.results[0].ID)
	}
}

func TestEscapeEmitsCloseMsg(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	m, cmd := m.Update(tea.KeyPressMsg{Text: "escape"})
	if cmd == nil {
		t.Fatal("expected close cmd")
	}
	if _, ok := cmd().(messages.CommandPaletteCloseMsg); !ok {
		t.Fatalf("expected CommandPaletteCloseMsg, got %T", cmd())
	}
	if m.IsOpen() {
		t.Fatal("expected palette to close on escape")
	}
}

func TestNavigation(t *testing.T) {
	m := newTestModel(t)
	m = m.Open()
	m, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "up"})
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 after up, got %d", m.cursor)
	}
	// Down past the end clamps to the last result.
	for i := 0; i < len(m.results)+5; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	}
	if m.cursor != len(m.results)-1 {
		t.Fatalf("expected cursor clamped to last result, got %d", m.cursor)
	}
}

func TestToggleStatePill(t *testing.T) {
	m := newTestModel(t)
	cmd := commandByID(config.ActionToggleSidebar)
	m.SetToggleState(config.ActionToggleSidebar, true)
	row := m.rowLine(cmd, 60)
	if !strings.Contains(row, "●on") {
		t.Fatalf("expected on pill in row, got %q", row)
	}
	m.SetToggleState(config.ActionToggleSidebar, false)
	row = m.rowLine(cmd, 60)
	if !strings.Contains(row, "○off") {
		t.Fatalf("expected off pill in row, got %q", row)
	}
}

func TestKeybindingHintInRow(t *testing.T) {
	m := newTestModel(t)
	cmd := commandByID(config.ActionSave)
	row := m.rowLine(cmd, 60)
	if !strings.Contains(row, "ctrl+s") {
		t.Fatalf("expected keybinding hint in row, got %q", row)
	}
	// The palette's own toggle key is not shown as a hint.
	paletteCmd := commandByID(config.ActionCommandPalette)
	row = m.rowLine(paletteCmd, 60)
	if strings.Contains(row, "ctrl+shift+p") {
		t.Fatalf("expected no palette key hint, got %q", row)
	}
}
