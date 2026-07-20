package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/yourusername/toast/internal/config"
)

// Keybinding-check helpers.
// All of them use m.cfg.Keybindings so that user-configured overrides
// are honoured. Fallback defaults are merged during config.Load().

func (m *Model) isQuit(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionQuit)
}

func (m *Model) isToggleSidebar(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionToggleSidebar)
}

func (m *Model) isSave(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionSave)
}

func (m *Model) isNewFile(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionNewFile)
}

func (m *Model) isCloseTab(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionCloseTab)
}

func (m *Model) isUndo(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionUndo)
}

func (m *Model) isRedo(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionRedo)
}

func (m *Model) isNextTab(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionNextTab)
}

func (m *Model) isPrevTab(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionPrevTab)
}

func (m *Model) isSearch(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionSearch)
}

func (m *Model) isFindReplace(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionFindReplace)
}

func (m *Model) isQuickOpen(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionQuickOpen)
}

func (m *Model) isGoToLine(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionGoToLine)
}

func (m *Model) isGoToDefinition(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionGoToDefinition)
}

func (m *Model) isToggleFocus(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionToggleFocus)
}

func (m *Model) isMarkdownPreview(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionMarkdownPreview)
}

func (m *Model) isShowHover(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionShowHover)
}

func (m *Model) isTriggerCompletion(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.Match(msg, config.ActionTriggerCompletion)
}

func (m *Model) isEscape(msg tea.KeyPressMsg) bool {
	return m.cfg.Keybindings.MatchEscape(msg)
}
