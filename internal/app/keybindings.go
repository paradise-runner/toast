package app

import tea "charm.land/bubbletea/v2"

func isQuit(msg tea.KeyPressMsg) bool          { return msg.String() == "ctrl+q" }
func isToggleSidebar(msg tea.KeyPressMsg) bool { return msg.String() == "ctrl+b" }
func isSave(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+s" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == 's')
}
func isNewFile(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+n" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == 'n')
}
func isCloseTab(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+w" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == 'w')
}
func isUndo(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+z" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == 'z')
}
func isRedo(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+y" || msg.String() == "ctrl+shift+z" ||
		(msg.Mod.Contains(tea.ModSuper) && msg.Code == 'y') ||
		(msg.Mod.Contains(tea.ModSuper) && msg.Mod.Contains(tea.ModShift) && msg.Code == 'z')
}
func isNextTab(msg tea.KeyPressMsg) bool { return msg.String() == "ctrl+tab" }
func isPrevTab(msg tea.KeyPressMsg) bool { return msg.String() == "ctrl+shift+tab" }
func isSearch(msg tea.KeyPressMsg) bool  { return msg.String() == "ctrl+shift+f" }
func isGoToLine(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+g" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == 'l')
}
func isGoToDefinition(msg tea.KeyPressMsg) bool { return msg.String() == "f12" }
func isMarkdownPreview(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+shift+m"
}
func isShowHover(msg tea.KeyPressMsg) bool      { return msg.String() == "ctrl+shift+k" }
func isTriggerCompletion(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+space" || (msg.Mod.Contains(tea.ModSuper) && msg.Code == ' ')
}
