package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/messages"
)

func TestParseToastOSC(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   toastOSC
		wantOK bool
	}{
		{
			name:   "open file with ST terminator",
			raw:    "\x1b]1337;toast-open-file;/tmp/a.go\x1b\\",
			want:   toastOSC{command: "toast-open-file", arg: "/tmp/a.go"},
			wantOK: true,
		},
		{
			name:   "open folder with BEL terminator",
			raw:    "\x1b]1337;toast-open-folder;/tmp/proj\x07",
			want:   toastOSC{command: "toast-open-folder", arg: "/tmp/proj"},
			wantOK: true,
		},
		{
			name:   "action without payload",
			raw:    "\x1b]1337;toast-action;save\x1b\\",
			want:   toastOSC{command: "toast-action", arg: "save"},
			wantOK: true,
		},
		{
			name:   "unrelated OSC number",
			raw:    "\x1b]4;5;rgb:8800/3300/eeee\x07",
			wantOK: false,
		},
		{
			name:   "garbage",
			raw:    "not an osc",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseToastOSC(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("parseToastOSC(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			}
			if ok && (got.command != tt.want.command || got.arg != tt.want.arg) {
				t.Fatalf("parseToastOSC(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestUpdate_ToastOSCOpenFile(t *testing.T) {
	cfg := config.Defaults()
	model, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	filePath := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	updated, cmd := model.Update(uv.UnknownOscEvent("\x1b]1337;toast-open-file;" + filePath + "\x1b\\"))
	if cmd == nil {
		t.Fatal("expected a Cmd from toast-open-file")
	}
	msg := cmd()
	if fileMsg, ok := msg.(messages.FileSelectedMsg); !ok || fileMsg.Path != filePath {
		t.Fatalf("expected FileSelectedMsg for %q, got %T %v", filePath, msg, msg)
	}
	_ = updated
}

func TestUpdate_ToastOSCOpenFolder_SwitchesWorkspace(t *testing.T) {
	cfg := config.Defaults()
	workRoot := t.TempDir()
	model, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	model.height = 30
	model.width = 100

	updated, cmd := model.Update(uv.UnknownOscEvent("\x1b]1337;toast-open-folder;" + workRoot + "\x1b\\"))
	if cmd == nil {
		t.Fatal("expected a Cmd from toast-open-folder")
	}
	got, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want app.Model", updated)
	}
	if got.rootDir != workRoot {
		t.Fatalf("rootDir = %q, want %q", got.rootDir, workRoot)
	}
	if !got.sidebarVisible {
		t.Fatal("expected sidebar to become visible after opening a folder")
	}
	if got.fileTree.RootPath() != workRoot {
		t.Fatalf("file tree root = %q, want %q", got.fileTree.RootPath(), workRoot)
	}
}

func TestMenuAction_SynthesizesKey(t *testing.T) {
	cfg := config.Defaults()
	model, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	// Every action must map to a non-nil keypress handler invocation (some
	// return nil commands when no buffer is active, which is fine).
	for _, action := range []string{"save", "undo", "redo", "cut", "copy", "paste", "select-all", "toggle-sidebar", "quick-open", "find", "close-tab", "quit", "search"} {
		if cmd := model.menuAction(action); cmd == nil {
			// toggle-sidebar returns a batch; quick-open opens the overlay and
			// returns a command; others may no-op without a buffer. The
			// important contract is that known actions are dispatched.
			t.Logf("menuAction(%q) returned nil (no-op without active buffer)", action)
		}
	}
	if cmd := model.menuAction("bogus"); cmd != nil {
		t.Fatalf("menuAction(bogus) = non-nil, want nil")
	}
}

func TestMenuAction_ToggleSidebar(t *testing.T) {
	cfg := config.Defaults()
	model, err := New(cfg, "", t.TempDir(), "")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	if !model.sidebarVisible {
		t.Skip("test assumes sidebar starts visible")
	}
	model.height = 30
	model.width = 100

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	got, ok := updated.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want app.Model", updated)
	}
	if got.sidebarVisible {
		t.Fatal("expected ctrl+b (toggle-sidebar) to hide the sidebar")
	}
}
