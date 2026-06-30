package filetree

import (
	"path/filepath"
	"strings"

	"github.com/yourusername/toast/internal/config"
	"github.com/yourusername/toast/internal/theme"
)

type fileIconKind int

const (
	fileIconUnknown fileIconKind = iota
	fileIconGo
	fileIconJavaScript
	fileIconTypeScript
	fileIconMarkdown
	fileIconShell
	fileIconJSON
	fileIconBuild
)

type fileIcon struct {
	marker string
	kind   fileIconKind
}

func fileIconForName(name string) fileIcon {
	base := strings.ToLower(filepath.Base(name))
	ext := strings.ToLower(filepath.Ext(base))

	switch base {
	case "makefile", "gnumakefile":
		return fileIcon{marker: "# ", kind: fileIconBuild}
	}

	switch ext {
	case ".go":
		return fileIcon{marker: "go", kind: fileIconGo}
	case ".js", ".mjs", ".cjs":
		return fileIcon{marker: "js", kind: fileIconJavaScript}
	case ".ts", ".mts", ".cts":
		return fileIcon{marker: "ts", kind: fileIconTypeScript}
	case ".md", ".markdown":
		return fileIcon{marker: "md", kind: fileIconMarkdown}
	case ".sh", ".bash", ".zsh", ".fish":
		return fileIcon{marker: "$ ", kind: fileIconShell}
	case ".json", ".jsonc":
		return fileIcon{marker: "{}", kind: fileIconJSON}
	case ".mk":
		return fileIcon{marker: "# ", kind: fileIconBuild}
	default:
		return fileIcon{marker: "--", kind: fileIconUnknown}
	}
}

func fileIconColor(tm *theme.Manager, cfg config.FileIconConfig, icon fileIcon) string {
	switch cfg.ColorMode {
	case "none":
		return tm.UI("sidebar_fg")
	case "semantic":
		return semanticFileIconColor(tm, icon)
	default:
		return fileIconAccentColor(tm)
	}
}

func semanticFileIconColor(tm *theme.Manager, icon fileIcon) string {
	accent := fileIconAccentColor(tm)
	switch icon.kind {
	case fileIconGo:
		return firstNonEmpty(tm.SyntaxFG("function"), tm.SyntaxFG("type"), accent)
	case fileIconJavaScript:
		return firstNonEmpty(tm.SyntaxFG("constant"), tm.SyntaxFG("keyword"), accent)
	case fileIconTypeScript:
		return firstNonEmpty(tm.SyntaxFG("type"), tm.SyntaxFG("keyword"), accent)
	case fileIconMarkdown:
		return firstNonEmpty(tm.SyntaxFG("comment"), tm.UI("breadcrumbs_fg"), accent)
	case fileIconShell:
		return firstNonEmpty(tm.SyntaxFG("operator"), tm.SyntaxFG("string"), accent)
	case fileIconJSON:
		return firstNonEmpty(tm.SyntaxFG("property"), tm.SyntaxFG("punctuation"), accent)
	case fileIconBuild:
		return firstNonEmpty(tm.UI("diagnostic_warning"), tm.SyntaxFG("constant"), accent)
	default:
		return firstNonEmpty(tm.UI("gutter_fg"), tm.UI("sidebar_fg"), accent)
	}
}

func fileIconAccentColor(tm *theme.Manager) string {
	return firstNonEmpty(
		tm.UI("sidebar_icon_fg"),
		tm.UI("breadcrumbs_active_fg"),
		tm.UI("gutter_active_fg"),
		tm.UI("sidebar_fg"),
	)
}

func firstNonEmpty(colors ...string) string {
	for _, color := range colors {
		if color != "" {
			return color
		}
	}
	return ""
}
