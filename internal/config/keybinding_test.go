package config

import (
	"testing"
)

func TestDefaultKeybindingsNotEmpty(t *testing.T) {
	km := DefaultKeybindings()
	if len(km) == 0 {
		t.Fatal("expected non-empty default keybindings")
	}
	// Spot-check a few expected actions.
	actions := []string{
		ActionQuit, ActionSave, ActionUndo, ActionRedo,
		ActionNextTab, ActionPrevTab, ActionQuickOpen,
		ActionFindReplace, ActionGoToDefinition, ActionToggleFocus,
	}
	for _, a := range actions {
		if _, ok := km[a]; !ok {
			t.Errorf("missing default action %q", a)
		}
	}
}

func TestDefaultKeybindingsHaveStrings(t *testing.T) {
	km := DefaultKeybindings()
	for action, keys := range km {
		if len(keys) == 0 {
			t.Errorf("action %q has no key combinations", action)
		}
		for _, k := range keys {
			if k == "" {
				t.Errorf("action %q has an empty key string", action)
			}
		}
	}
}

func TestMergeNil(t *testing.T) {
	defaults := DefaultKeybindings()
	merged := defaults.Merge(nil)
	if len(merged) != len(defaults) {
		t.Errorf("Merge with nil should keep all defaults, got %d entries, want %d",
			len(merged), len(defaults))
	}
}

func TestMergeOverride(t *testing.T) {
	defaults := DefaultKeybindings()
	overrides := KeybindingMap{
		ActionQuit: {"ctrl+c"},
	}
	merged := defaults.Merge(overrides)

	// Override should replace the value
	if v, ok := merged[ActionQuit]; !ok || v[0] != "ctrl+c" {
		t.Errorf("expected ActionQuit to be overridden to [ctrl+c], got %v", v)
	}

	// Other defaults preserved
	if _, ok := merged[ActionSave]; !ok {
		t.Error("expected ActionSave to still be present after merge")
	}

	// Original unchanged
	if v := defaults[ActionQuit]; v[0] != "ctrl+q" {
		t.Errorf("expected original to remain ctrl+q, got %v", v)
	}
}

func TestMergeEmptySliceUnbinds(t *testing.T) {
	defaults := DefaultKeybindings()
	overrides := KeybindingMap{
		ActionQuit: {},
	}
	merged := defaults.Merge(overrides)

	// Empty slice means unbind the action — it should no longer be present.
	if _, ok := merged[ActionQuit]; ok {
		t.Error("expected ActionQuit to be removed after empty-slice merge")
	}
}

func TestMergeExtraAction(t *testing.T) {
	defaults := DefaultKeybindings()
	overrides := KeybindingMap{
		"custom_action": {"ctrl+alt+del"},
	}
	merged := defaults.Merge(overrides)

	if v, ok := merged["custom_action"]; !ok {
		t.Error("expected custom action to be present in merged map")
	} else if v[0] != "ctrl+alt+del" {
		t.Errorf("expected custom_action to be ctrl+alt+del, got %v", v)
	}
}

func TestMergeNilInOverridesNoop(t *testing.T) {
	defaults := DefaultKeybindings()
	overrides := KeybindingMap{
		ActionQuit: nil,
	}
	merged := defaults.Merge(overrides)
	if v := merged[ActionQuit]; v == nil || v[0] != "ctrl+q" {
		t.Errorf("expected nil override to keep default ctrl+q, got %v", v)
	}
}

func TestNormalizeKeyString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ctrl+q", "ctrl+q"},
		{"ctrl+shift+z", "ctrl+shift+z"},
		{"super+shift+z", "shift+super+z"},
		{"shift+super+z", "shift+super+z"},
		{"alt+ctrl+shift+x", "ctrl+alt+shift+x"},
		{"super+alt+ctrl+p", "ctrl+alt+super+p"},
		{"f12", "f12"},
		{"", ""},
		{"escape", "escape"},
		{"ctrl+space", "ctrl+space"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeKeyString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeKeyString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMergeNormalizesKeyStrings(t *testing.T) {
	defaults := DefaultKeybindings()
	overrides := KeybindingMap{
		ActionNextTab: {"super+alt+right"},
	}
	merged := defaults.Merge(overrides)
	keys := merged[ActionNextTab]
	if len(keys) != 1 || keys[0] != "alt+super+right" {
		t.Errorf("expected alt+super+right, got %v", keys)
	}
}
