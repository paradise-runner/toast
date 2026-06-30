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
	fileIconRust
	fileIconPython
	fileIconRuby
	fileIconPHP
	fileIconJVM
	fileIconSwift
	fileIconC
	fileIconDotNet
	fileIconFunctional
	fileIconMarkup
	fileIconStyle
	fileIconData
	fileIconConfig
	fileIconDatabase
	fileIconDocker
	fileIconText
	fileIconImage
	fileIconArchive
	fileIconBinary
)

type fileIcon struct {
	marker string
	kind   fileIconKind
}

func fileIconForName(name string) fileIcon {
	base := strings.ToLower(filepath.Base(name))
	ext := strings.ToLower(filepath.Ext(base))

	if strings.HasPrefix(base, ".env") {
		return fileIcon{marker: "$ ", kind: fileIconShell}
	}
	if strings.HasPrefix(base, "docker-compose.") || strings.HasPrefix(base, "compose.") {
		return fileIcon{marker: "dk", kind: fileIconDocker}
	}
	if icon, ok := exactFileIcon(base); ok {
		return icon
	}
	if icon, ok := extensionFileIcon(ext); ok {
		return icon
	}

	return fileIcon{marker: "--", kind: fileIconUnknown}
}

func exactFileIcon(base string) (fileIcon, bool) {
	switch base {
	case "makefile", "gnumakefile", "rakefile", "justfile":
		return fileIcon{marker: "# ", kind: fileIconBuild}, true
	case "dockerfile", ".dockerignore":
		return fileIcon{marker: "dk", kind: fileIconDocker}, true
	case "go.mod", "go.sum", "go.work":
		return fileIcon{marker: "go", kind: fileIconGo}, true
	case "cargo.toml", "cargo.lock", "rust-toolchain", "rust-toolchain.toml":
		return fileIcon{marker: "rs", kind: fileIconRust}, true
	case "pyproject.toml", "requirements.txt", "poetry.lock", "pipfile":
		return fileIcon{marker: "py", kind: fileIconPython}, true
	case "gemfile", "gemfile.lock":
		return fileIcon{marker: "rb", kind: fileIconRuby}, true
	case "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb":
		return fileIcon{marker: "{}", kind: fileIconJSON}, true
	case "tsconfig.json":
		return fileIcon{marker: "ts", kind: fileIconTypeScript}, true
	case "jsconfig.json":
		return fileIcon{marker: "js", kind: fileIconJavaScript}, true
	case ".gitignore", ".npmignore", ".eslintignore", ".prettierignore":
		return fileIcon{marker: "ig", kind: fileIconConfig}, true
	default:
		return fileIcon{}, false
	}
}

func extensionFileIcon(ext string) (fileIcon, bool) {
	switch ext {
	case ".go":
		return fileIcon{marker: "go", kind: fileIconGo}, true
	case ".js", ".mjs", ".cjs":
		return fileIcon{marker: "js", kind: fileIconJavaScript}, true
	case ".jsx":
		return fileIcon{marker: "jx", kind: fileIconJavaScript}, true
	case ".ts", ".mts", ".cts":
		return fileIcon{marker: "ts", kind: fileIconTypeScript}, true
	case ".tsx":
		return fileIcon{marker: "tx", kind: fileIconTypeScript}, true
	case ".rs":
		return fileIcon{marker: "rs", kind: fileIconRust}, true
	case ".py", ".pyw":
		return fileIcon{marker: "py", kind: fileIconPython}, true
	case ".rb":
		return fileIcon{marker: "rb", kind: fileIconRuby}, true
	case ".php":
		return fileIcon{marker: "ph", kind: fileIconPHP}, true
	case ".java":
		return fileIcon{marker: "jv", kind: fileIconJVM}, true
	case ".kt", ".kts":
		return fileIcon{marker: "kt", kind: fileIconJVM}, true
	case ".swift":
		return fileIcon{marker: "sw", kind: fileIconSwift}, true
	case ".c":
		return fileIcon{marker: "c ", kind: fileIconC}, true
	case ".h", ".hpp", ".hxx":
		return fileIcon{marker: "h ", kind: fileIconC}, true
	case ".cpp", ".cc", ".cxx":
		return fileIcon{marker: "c+", kind: fileIconC}, true
	case ".cs":
		return fileIcon{marker: "c#", kind: fileIconDotNet}, true
	case ".fs", ".fsx":
		return fileIcon{marker: "f#", kind: fileIconDotNet}, true
	case ".ex", ".exs":
		return fileIcon{marker: "ex", kind: fileIconFunctional}, true
	case ".erl", ".hrl":
		return fileIcon{marker: "er", kind: fileIconFunctional}, true
	case ".hs", ".lhs":
		return fileIcon{marker: "hs", kind: fileIconFunctional}, true
	case ".lua":
		return fileIcon{marker: "lu", kind: fileIconShell}, true
	case ".pl", ".pm":
		return fileIcon{marker: "pl", kind: fileIconShell}, true
	case ".r":
		return fileIcon{marker: "r ", kind: fileIconFunctional}, true
	case ".scala":
		return fileIcon{marker: "sc", kind: fileIconJVM}, true
	case ".clj", ".cljs", ".cljc":
		return fileIcon{marker: "cl", kind: fileIconFunctional}, true
	case ".ml", ".mli":
		return fileIcon{marker: "ml", kind: fileIconFunctional}, true
	case ".zig":
		return fileIcon{marker: "zg", kind: fileIconC}, true
	case ".nim":
		return fileIcon{marker: "nm", kind: fileIconC}, true
	case ".dart":
		return fileIcon{marker: "dt", kind: fileIconJVM}, true
	case ".html", ".htm", ".xml", ".xsl", ".xsd":
		return fileIcon{marker: "<>", kind: fileIconMarkup}, true
	case ".vue":
		return fileIcon{marker: "vu", kind: fileIconMarkup}, true
	case ".svelte":
		return fileIcon{marker: "sv", kind: fileIconMarkup}, true
	case ".astro":
		return fileIcon{marker: "as", kind: fileIconMarkup}, true
	case ".css":
		return fileIcon{marker: "cs", kind: fileIconStyle}, true
	case ".scss", ".sass":
		return fileIcon{marker: "ss", kind: fileIconStyle}, true
	case ".less":
		return fileIcon{marker: "ls", kind: fileIconStyle}, true
	case ".md", ".markdown":
		return fileIcon{marker: "md", kind: fileIconMarkdown}, true
	case ".txt":
		return fileIcon{marker: "tt", kind: fileIconText}, true
	case ".rst":
		return fileIcon{marker: "rt", kind: fileIconText}, true
	case ".adoc":
		return fileIcon{marker: "ad", kind: fileIconText}, true
	case ".sh", ".bash", ".zsh", ".fish":
		return fileIcon{marker: "$ ", kind: fileIconShell}, true
	case ".ps1":
		return fileIcon{marker: "ps", kind: fileIconShell}, true
	case ".json", ".jsonc":
		return fileIcon{marker: "{}", kind: fileIconJSON}, true
	case ".yaml", ".yml":
		return fileIcon{marker: "ym", kind: fileIconData}, true
	case ".toml":
		return fileIcon{marker: "tm", kind: fileIconData}, true
	case ".ini", ".conf", ".cfg":
		return fileIcon{marker: "cf", kind: fileIconConfig}, true
	case ".csv":
		return fileIcon{marker: "cv", kind: fileIconData}, true
	case ".tsv":
		return fileIcon{marker: "tv", kind: fileIconData}, true
	case ".sql":
		return fileIcon{marker: "sq", kind: fileIconDatabase}, true
	case ".graphql", ".gql":
		return fileIcon{marker: "gq", kind: fileIconData}, true
	case ".proto":
		return fileIcon{marker: "pb", kind: fileIconData}, true
	case ".mk":
		return fileIcon{marker: "# ", kind: fileIconBuild}, true
	case ".gradle":
		return fileIcon{marker: "gr", kind: fileIconBuild}, true
	case ".svg":
		return fileIcon{marker: "vg", kind: fileIconImage}, true
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".tif", ".tiff":
		return fileIcon{marker: "im", kind: fileIconImage}, true
	case ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar":
		return fileIcon{marker: "ar", kind: fileIconArchive}, true
	case ".exe", ".dll", ".so", ".dylib", ".a", ".o", ".wasm":
		return fileIcon{marker: "bn", kind: fileIconBinary}, true
	default:
		return fileIcon{}, false
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
	case fileIconMarkup:
		return firstNonEmpty(tm.SyntaxFG("tag"), tm.SyntaxFG("module"), accent)
	case fileIconStyle:
		return firstNonEmpty(tm.SyntaxFG("property"), tm.SyntaxFG("string"), accent)
	case fileIconRust, fileIconPython, fileIconRuby, fileIconPHP, fileIconJVM, fileIconSwift,
		fileIconC, fileIconDotNet, fileIconFunctional:
		return firstNonEmpty(tm.SyntaxFG("function"), tm.SyntaxFG("type"), tm.SyntaxFG("keyword"), accent)
	case fileIconData, fileIconConfig:
		return firstNonEmpty(tm.SyntaxFG("property"), tm.SyntaxFG("punctuation"), accent)
	case fileIconDatabase:
		return firstNonEmpty(tm.SyntaxFG("keyword"), tm.SyntaxFG("constant"), accent)
	case fileIconDocker:
		return firstNonEmpty(tm.UI("diagnostic_info"), tm.SyntaxFG("module"), accent)
	case fileIconText:
		return firstNonEmpty(tm.SyntaxFG("comment"), tm.UI("breadcrumbs_fg"), accent)
	case fileIconImage, fileIconArchive, fileIconBinary:
		return firstNonEmpty(tm.SyntaxFG("module"), tm.UI("gutter_fg"), accent)
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
