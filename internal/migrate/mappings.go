package migrate

// mapping defines how a single Toast token resolves from VSCode color keys.
// The converter tries Primary first, then each Fallback in order.
// If Derive is set and no key is found, the derive function computes the value
// from the already-resolved colors map.
type mapping struct {
	ToastKey  string
	Primary   string
	Fallbacks []string
	Derive    func(colors map[string]string) string
}

// syntaxMapping defines how a Toast syntax token resolves from VSCode
// tokenColors scopes. Scopes are tried in priority order; for each scope,
// the best (most specific) tokenColors entry wins.
type syntaxMapping struct {
	ToastKey string
	Scopes   []string
}

var uiMappings = []mapping{
	{ToastKey: "background", Primary: "editor.background"},
	{ToastKey: "foreground", Primary: "editor.foreground"},
	{ToastKey: "cursor", Primary: "editorCursor.foreground"},
	{ToastKey: "selection", Primary: "editor.selectionBackground"},
	{ToastKey: "line_highlight", Primary: "editor.lineHighlightBackground"},
	{ToastKey: "border", Primary: "editorGroup.border", Fallbacks: []string{"panel.border"}},

	{ToastKey: "tab_active_bg", Primary: "tab.activeBackground", Fallbacks: []string{"editor.background"}},
	{ToastKey: "tab_active_fg", Primary: "tab.activeForeground", Fallbacks: []string{"editor.foreground"}},
	{ToastKey: "tab_inactive_bg", Primary: "tab.inactiveBackground", Fallbacks: []string{"editorGroupHeader.tabsBackground"}},
	{ToastKey: "tab_inactive_fg", Primary: "tab.inactiveForeground", Fallbacks: []string{"sideBar.foreground"}},

	{ToastKey: "sidebar_bg", Primary: "sideBar.background"},
	{ToastKey: "sidebar_fg", Primary: "sideBar.foreground", Fallbacks: []string{"editor.foreground"}},
	{ToastKey: "sidebar_selected_bg", Primary: "list.activeSelectionBackground"},
	{ToastKey: "sidebar_selected_fg", Primary: "list.activeSelectionForeground", Fallbacks: []string{"editor.foreground"}},

	{ToastKey: "statusbar_bg", Primary: "statusBar.background"},
	{ToastKey: "statusbar_fg", Primary: "statusBar.foreground"},

	{ToastKey: "breadcrumbs_fg", Primary: "breadcrumb.foreground", Fallbacks: []string{"editorLineNumber.foreground"}},
	{ToastKey: "breadcrumbs_active_fg", Primary: "breadcrumb.focusForeground", Fallbacks: []string{"editor.foreground"}},

	{ToastKey: "gutter_fg", Primary: "editorLineNumber.foreground"},
	{ToastKey: "gutter_active_fg", Primary: "editorLineNumber.activeForeground", Fallbacks: []string{"editor.foreground"}},

	{ToastKey: "diagnostic_error", Primary: "editorError.foreground"},
	{ToastKey: "diagnostic_warning", Primary: "editorWarning.foreground"},
	{ToastKey: "diagnostic_info", Primary: "editorInfo.foreground", Fallbacks: []string{"editorWarning.foreground"}},
	{ToastKey: "diagnostic_hint", Primary: "editorHint.foreground", Fallbacks: []string{"editorInfo.foreground", "editorWarning.foreground"}},

	{ToastKey: "completion_bg", Primary: "editorSuggestWidget.background", Fallbacks: []string{"editorWidget.background"}},
	{ToastKey: "completion_fg", Primary: "editorSuggestWidget.foreground", Fallbacks: []string{"editor.foreground"}},
	{ToastKey: "completion_selected", Primary: "editorSuggestWidget.selectedBackground", Fallbacks: []string{"list.activeSelectionBackground"}},

	{ToastKey: "hover_bg", Primary: "editorHoverWidget.background", Fallbacks: []string{"editorWidget.background"}},
	{ToastKey: "hover_fg", Primary: "editorHoverWidget.foreground", Fallbacks: []string{"editor.foreground"}},
	{ToastKey: "hover_border", Primary: "editorHoverWidget.border", Fallbacks: []string{"editorGroup.border"}},

	{ToastKey: "search_match_bg", Primary: "editor.findMatchHighlightBackground"},
	{
		ToastKey:  "search_match_fg",
		Primary:   "editor.findMatchHighlightForeground",
		Fallbacks: []string{"editor.background"},
	},
	{ToastKey: "search_current_bg", Primary: "editor.findMatchBackground"},
	{
		ToastKey:  "search_current_fg",
		Primary:   "editor.findMatchForeground",
		Fallbacks: []string{"editor.background"},
	},
}

var syntaxMappings = []syntaxMapping{
	{ToastKey: "keyword", Scopes: []string{"keyword", "keyword.control", "storage.type", "storage.modifier"}},
	{ToastKey: "string", Scopes: []string{"string", "string.quoted"}},
	{ToastKey: "number", Scopes: []string{"constant.numeric"}},
	{ToastKey: "comment", Scopes: []string{"comment", "comment.line", "comment.block"}},
	{ToastKey: "function", Scopes: []string{"entity.name.function", "support.function"}},
	{ToastKey: "type", Scopes: []string{"entity.name.type", "support.type", "storage.type"}},
	{ToastKey: "variable", Scopes: []string{"variable", "variable.other"}},
	{ToastKey: "constant", Scopes: []string{"constant.language", "constant.other"}},
	{ToastKey: "operator", Scopes: []string{"keyword.operator"}},
	{ToastKey: "punctuation", Scopes: []string{"punctuation", "punctuation.definition"}},
	{ToastKey: "tag", Scopes: []string{"entity.name.tag"}},
	{ToastKey: "attribute", Scopes: []string{"entity.other.attribute-name"}},
	{ToastKey: "property", Scopes: []string{"variable.other.property", "support.type.property-name"}},
	{ToastKey: "module", Scopes: []string{"entity.name.namespace", "entity.name.type.module"}},
	{ToastKey: "builtin", Scopes: []string{"support.function", "variable.language"}},
}

var gitMappings = []mapping{
	{ToastKey: "added", Primary: "editorGutter.addedBackground", Fallbacks: []string{"gitDecoration.addedResourceForeground"}},
	{ToastKey: "modified", Primary: "editorGutter.modifiedBackground", Fallbacks: []string{"gitDecoration.modifiedResourceForeground"}},
	{ToastKey: "deleted", Primary: "editorGutter.deletedBackground", Fallbacks: []string{"gitDecoration.deletedResourceForeground"}},
	{ToastKey: "untracked", Primary: "gitDecoration.untrackedResourceForeground", Fallbacks: []string{"gitDecoration.ignoredResourceForeground"}},
	{ToastKey: "conflict", Primary: "merge.incomingHeaderBackground", Fallbacks: []string{"editorWarning.foreground", "gitDecoration.conflictingResourceForeground"}},
}
