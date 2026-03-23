package messages

// FileSelectedMsg - emitted when user selects a file in tree or search result
type FileSelectedMsg struct{ Path string }

// BufferOpenedMsg - emitted when a file is loaded into a buffer
type BufferOpenedMsg struct {
	BufferID int
	Path     string
}

// BufferClosedMsg - emitted when a tab/buffer is closed
type BufferClosedMsg struct{ BufferID int }

// BufferModifiedMsg - emitted when buffer content changes
type BufferModifiedMsg struct {
	BufferID int
	Modified bool
}

// ActiveBufferChangedMsg - emitted when active tab changes
type ActiveBufferChangedMsg struct {
	BufferID int
	Path     string
}

// FileSavedMsg - emitted after successful save
type FileSavedMsg struct {
	BufferID int
	Path     string
}

// FileSavingMsg - emitted by the editor immediately before writing to disk
type FileSavingMsg struct {
	BufferID int
	Path     string
}

// FileSaveFailedMsg - emitted when writing to disk fails
type FileSaveFailedMsg struct {
	BufferID int
	Path     string
	Err      error
}

// DiagnosticsUpdatedMsg - emitted when LSP publishes diagnostics
type DiagnosticsUpdatedMsg struct {
	Path        string
	Diagnostics []Diagnostic
}

// Diagnostic - single LSP diagnostic
type Diagnostic struct {
	Line, Col, EndLine, EndCol int
	Severity                   int // 1=error, 2=warning, 3=info, 4=hint
	Message, Source            string
}

// CompletionRequestMsg - editor wants completion items
type CompletionRequestMsg struct {
	BufferID  int
	Path      string
	Line, Col int
}

// CompletionResultMsg - completion items from LSP
type CompletionResultMsg struct {
	BufferID int
	Items    []CompletionItem
}

// CompletionItem - single completion item
type CompletionItem struct {
	Label, Detail, Documentation, InsertText string
	Kind                                     int
	TextEdit                                 *TextEdit
}

// TextEdit - ranged replacement in buffer
type TextEdit struct {
	Line, Col, EndLine, EndCol int
	NewText                    string
}

// HoverRequestMsg - editor wants hover info
type HoverRequestMsg struct {
	BufferID  int
	Path      string
	Line, Col int
}

// HoverResultMsg - hover content from LSP
type HoverResultMsg struct {
	BufferID                                       int
	Contents                                       string
	RangeLine, RangeCol, RangeEndLine, RangeEndCol int
}

// DefinitionRequestMsg - go-to-definition
type DefinitionRequestMsg struct {
	BufferID  int
	Path      string
	Line, Col int
}

// DefinitionResultMsg - definition location
type DefinitionResultMsg struct {
	Path      string
	Line, Col int
}

// ThemeChangedMsg - active theme changed
type ThemeChangedMsg struct{ ThemeName string }

// GitStatusUpdatedMsg - git status refreshed
type GitStatusUpdatedMsg struct {
	FileStatuses  map[string]GitStatus
	Branch        string
	Ahead, Behind int
}

// GitStatus represents the git status of a file
type GitStatus int

const (
	GitStatusClean GitStatus = iota
	GitStatusModified
	GitStatusAdded
	GitStatusDeleted
	GitStatusUntracked
	GitStatusConflict
)

// GitDiffUpdatedMsg - line-level diff for gutter
type GitDiffUpdatedMsg struct {
	BufferID  int
	Path      string
	LineKinds []GitLineKind
}

// GitLineKind represents the kind of change on a line
type GitLineKind int

const (
	GitLineUnchanged GitLineKind = iota
	GitLineAdded
	GitLineModified
	GitLineDeleted
)

// SearchOpenMsg - open the search panel
type SearchOpenMsg struct{}

// SearchCloseMsg - close the search panel
type SearchCloseMsg struct{}

// SearchResultMsg - single search result
type SearchResultMsg struct {
	Path                 string
	Line, Col            int
	Content              string
	MatchStart, MatchEnd int
}

// SearchDoneMsg - search finished
type SearchDoneMsg struct {
	TotalMatches, TotalFiles int
}

// SidebarToggleMsg - toggle sidebar visibility
type SidebarToggleMsg struct{}

// SidebarResizeMsg - resize the sidebar
type SidebarResizeMsg struct{ Width int }

// FileChangedOnDiskMsg - fsnotify external change
type FileChangedOnDiskMsg struct{ Path string }

// LSPServerStatusMsg - language server status
type LSPServerStatusMsg struct {
	Language string
	Status   LSPServerStatus
	Message  string
}

// LSPServerStatus represents the status of a language server
type LSPServerStatus int

const (
	LSPServerStarting LSPServerStatus = iota
	LSPServerReady
	LSPServerCrashed
	LSPServerNotFound
)

// ThemePickerOpenMsg - open the theme picker dialog
type ThemePickerOpenMsg struct{}

// ThemePickerClosedMsg - theme picker closed; ThemeName is the chosen theme
type ThemePickerClosedMsg struct{ ThemeName string }

// CloseTabRequestMsg - user requested closing a tab; app checks for unsaved changes.
type CloseTabRequestMsg struct {
	BufferID int
	Path     string
}

// CloseTabConfirmedMsg - result of the close-tab confirmation dialog.
// Cancelled=true means the user pressed Escape/Cancel.
// If Cancelled=false: Save=true means save first; Save=false means discard.
type CloseTabConfirmedMsg struct {
	BufferID  int
	Path      string
	Save      bool
	Cancelled bool
}

// FileCreatedMsg - emitted after a new file is created from the sidebar context menu
type FileCreatedMsg struct{ Path string }

// DirCreatedMsg - emitted after a new directory is created from the sidebar context menu
type DirCreatedMsg struct{ Path string }

// FileDeletedMsg - emitted after a file is removed from disk via the sidebar
type FileDeletedMsg struct{ Path string }

// DirDeletedMsg - emitted after a directory is removed from disk via the sidebar
type DirDeletedMsg struct{ Path string }

// SaveConfigMsg - emitted when a component needs app to persist updated config
type SaveConfigMsg struct{ Config interface{} }

// GoToLineMsg - emitted by the go-to-line overlay to jump to a specific line.
// Line is zero-based (converted from the user's 1-based input).
type GoToLineMsg struct{ Line int }

// GoToLineCancelMsg - emitted when the go-to-line overlay is dismissed without jumping.
type GoToLineCancelMsg struct{}

// QuitRequestMsg - emitted when the user requests to quit (e.g. exit button click).
// The app layer intercepts this and checks for unsaved changes.
type QuitRequestMsg struct{}

// QuitConfirmedMsg - result of the quit confirmation dialog.
// Cancelled=true means the user pressed Escape/Cancel.
// If Cancelled=false: Save=true means save then quit; Save=false means discard and quit.
type QuitConfirmedMsg struct {
	Save      bool
	Cancelled bool
}
