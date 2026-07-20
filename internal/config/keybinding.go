// Package config provides configuration file loading, saving, and defaults.
package config

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Action identifiers for keybindings. Each constant represents an action that
// can be triggered by a configurable keyboard shortcut.
const (
	ActionQuit              = "quit"
	ActionToggleSidebar     = "toggle_sidebar"
	ActionSave              = "save"
	ActionNewFile           = "new_file"
	ActionCloseTab          = "close_tab"
	ActionUndo              = "undo"
	ActionRedo              = "redo"
	ActionNextTab           = "next_tab"
	ActionPrevTab           = "prev_tab"
	ActionSearch            = "search"
	ActionFindReplace       = "find_replace"
	ActionQuickOpen         = "quick_open"
	ActionGoToLine          = "go_to_line"
	ActionGoToDefinition    = "go_to_definition"
	ActionToggleFocus       = "toggle_focus"
	ActionMarkdownPreview   = "markdown_preview"
	ActionShowHover         = "show_hover"
	ActionTriggerCompletion = "trigger_completion"
	ActionEscape            = "escape"
)

// modifierOrder defines the canonical modifier ordering used by
// bubbletea/ultraviolet's Keystroke() format: ctrl+alt+shift+meta+hyper+super.
var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// modifierRank returns the sort rank of a modifier, or -1 if unknown.
func modifierRank(m string) int {
	for i, mod := range modifierOrder {
		if m == mod {
			return i
		}
	}
	return -1
}

// normalizeKeyString ensures modifiers appear in the canonical keystroke order
// (ctrl+alt+shift+meta+hyper+super) and preserves the key as the last segment.
// For example "super+shift+z" becomes "shift+super+z".
func normalizeKeyString(s string) string {
	if s == "" {
		return s
	}
	parts := splitKeyString(s)
	if len(parts) <= 1 {
		return s
	}
	// The last part is the key; everything before it is modifiers.
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]

	// Sort modifiers by their canonical order, keeping unknown modifiers at
	// the end in their original relative order (stable sort).
	sort.SliceStable(mods, func(i, j int) bool {
		return modifierRank(mods[i]) < modifierRank(mods[j])
	})

	var sb strings.Builder
	for _, m := range mods {
		sb.WriteString(m)
		sb.WriteByte('+')
	}
	sb.WriteString(key)
	return sb.String()
}

// splitKeyString splits a key string like "ctrl+shift+f" into ["ctrl", "shift", "f"].
func splitKeyString(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '+' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// normalizeKeyStrings normalizes every key string in the map to the canonical
// keystroke modifier ordering.
func normalizeKeyStrings(m KeybindingMap) {
	for action, keys := range m {
		normalized := make([]string, len(keys))
		for i, k := range keys {
			normalized[i] = normalizeKeyString(k)
		}
		m[action] = normalized
	}
}

// KeybindingMap maps action names to their key combinations. Each action can
// have multiple alternative key combinations (e.g. "ctrl+s" and "super+s").
type KeybindingMap map[string][]string

// DefaultKeybindings returns the default keybinding configuration. These are
// used when the user does not override them in their config file.
func DefaultKeybindings() KeybindingMap {
	m := KeybindingMap{
		ActionQuit:              {"ctrl+q"},
		ActionToggleSidebar:     {"ctrl+b"},
		ActionSave:              {"ctrl+s", "super+s"},
		ActionNewFile:           {"ctrl+n", "super+n"},
		ActionCloseTab:          {"ctrl+w", "super+w"},
		ActionUndo:              {"ctrl+z", "super+z"},
		ActionRedo:              {"ctrl+y", "ctrl+shift+z", "super+y", "shift+super+z"},
		ActionNextTab:           {"ctrl+alt+right"},
		ActionPrevTab:           {"ctrl+alt+left"},
		ActionSearch:            {"ctrl+shift+f"},
		ActionFindReplace:       {"ctrl+f", "super+f"},
		ActionQuickOpen:         {"ctrl+p", "super+p"},
		ActionGoToLine:          {"ctrl+g", "super+l"},
		ActionGoToDefinition:    {"f12"},
		ActionToggleFocus:       {"ctrl+shift+e"},
		ActionMarkdownPreview:   {"ctrl+shift+m"},
		ActionShowHover:         {"ctrl+shift+k"},
		ActionTriggerCompletion: {"ctrl+space", "super+space"},
		ActionEscape:            {"escape"},
	}
	normalizeKeyStrings(m)
	return m
}

// Match returns true when the given key press matches one of the key
// combinations registered for the specified action. Both the stored key
// strings and msg.String() are normalized to the canonical modifier ordering
// before comparison, so the modifier order in the config file does not matter.
func (km KeybindingMap) Match(msg tea.KeyPressMsg, action string) bool {
	keys, ok := km[action]
	if !ok {
		return false
	}
	keyStr := normalizeKeyString(msg.String())
	for _, k := range keys {
		if k == keyStr {
			return true
		}
	}
	return false
}

// MatchEscape returns true when the key press matches the configured escape
// action OR the raw escape key code. This handles the special case where
// escape can arrive as either the "escape" string or the KeyEscape code.
func (km KeybindingMap) MatchEscape(msg tea.KeyPressMsg) bool {
	if km.Match(msg, ActionEscape) {
		return true
	}
	return msg.String() == "escape" || msg.Code == tea.KeyEscape
}

// Merge returns a new KeybindingMap composed of the receiver's entries
// overridden by any non-nil entries in overrides. Key strings in the result
// are normalized to the canonical keystroke modifier ordering. Actions set to
// an empty slice in overrides are deleted (unbound). nil entries in overrides
// are treated as no-ops.
func (km KeybindingMap) Merge(overrides KeybindingMap) KeybindingMap {
	result := make(KeybindingMap, len(km))
	for k, v := range km {
		result[k] = v
	}
	for k, v := range overrides {
		if v == nil {
			// Explicit nil keeps the original (acts as a no-op).
			continue
		}
		if len(v) == 0 {
			// Empty slice means "unbind this action" — delete it from
			// the result so Match returns false for it.
			delete(result, k)
		} else {
			result[k] = v
		}
	}
	normalizeKeyStrings(result)
	return result
}
