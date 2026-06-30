package filetree

import (
	"testing"

	"github.com/yourusername/toast/internal/config"
)

func TestFileIconForName(t *testing.T) {
	tests := []struct {
		name   string
		marker string
		kind   fileIconKind
	}{
		{name: "main.go", marker: "go", kind: fileIconGo},
		{name: "helper.js", marker: "js", kind: fileIconJavaScript},
		{name: "view.ts", marker: "ts", kind: fileIconTypeScript},
		{name: "README.md", marker: "md", kind: fileIconMarkdown},
		{name: "run.sh", marker: "$ ", kind: fileIconShell},
		{name: "package.json", marker: "{}", kind: fileIconJSON},
		{name: "Makefile", marker: "# ", kind: fileIconBuild},
		{name: "unknown.bin", marker: "--", kind: fileIconUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := fileIconForName(tt.name)
			if icon.marker != tt.marker {
				t.Fatalf("marker = %q, want %q", icon.marker, tt.marker)
			}
			if icon.kind != tt.kind {
				t.Fatalf("kind = %v, want %v", icon.kind, tt.kind)
			}
		})
	}
}

func TestFileIconForName_CommonIDEFiles(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{name: "index.html", marker: "<>"},
		{name: "styles.css", marker: "cs"},
		{name: "main.rs", marker: "rs"},
		{name: "app.py", marker: "py"},
		{name: "app.rb", marker: "rb"},
		{name: "index.php", marker: "ph"},
		{name: "Main.java", marker: "jv"},
		{name: "View.kt", marker: "kt"},
		{name: "App.swift", marker: "sw"},
		{name: "main.c", marker: "c "},
		{name: "main.cpp", marker: "c+"},
		{name: "Program.cs", marker: "c#"},
		{name: "component.jsx", marker: "jx"},
		{name: "component.tsx", marker: "tx"},
		{name: "App.vue", marker: "vu"},
		{name: "App.svelte", marker: "sv"},
		{name: "config.yaml", marker: "ym"},
		{name: "Cargo.toml", marker: "rs"},
		{name: "Dockerfile", marker: "dk"},
		{name: "schema.sql", marker: "sq"},
		{name: "logo.png", marker: "im"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon := fileIconForName(tt.name)
			if icon.marker != tt.marker {
				t.Fatalf("marker = %q, want %q", icon.marker, tt.marker)
			}
		})
	}
}

func TestFileIconMarkersAreFixedWidth(t *testing.T) {
	for _, name := range []string{
		"main.go",
		"index.html",
		"styles.css",
		"main.rs",
		"app.py",
		"run.sh",
		"main.c",
		"Dockerfile",
		"unknown.bin",
	} {
		t.Run(name, func(t *testing.T) {
			if got := len(fileIconForName(name).marker); got != 2 {
				t.Fatalf("marker width = %d, want 2 for %q", got, fileIconForName(name).marker)
			}
		})
	}
}

func TestFileIconColor_AccentUsesThemeAccent(t *testing.T) {
	tm := newTestTheme(t)
	icon := fileIconForName("main.go")

	got := fileIconColor(tm, config.FileIconConfig{Enabled: true, ColorMode: "accent"}, icon)
	want := tm.UI("sidebar_icon_fg")
	if got != want {
		t.Fatalf("accent color = %q, want %q", got, want)
	}
}

func TestFileIconColor_NoneUsesSidebarForeground(t *testing.T) {
	tm := newTestTheme(t)
	icon := fileIconForName("main.go")

	got := fileIconColor(tm, config.FileIconConfig{Enabled: true, ColorMode: "none"}, icon)
	want := tm.UI("sidebar_fg")
	if got != want {
		t.Fatalf("none color = %q, want %q", got, want)
	}
}

func TestFileIconColor_SemanticUsesThemeSyntaxColors(t *testing.T) {
	tm := newTestTheme(t)
	tests := []struct {
		name string
		want string
	}{
		{name: "main.go", want: tm.SyntaxFG("function")},
		{name: "helper.js", want: tm.SyntaxFG("constant")},
		{name: "view.ts", want: tm.SyntaxFG("type")},
		{name: "README.md", want: tm.SyntaxFG("comment")},
		{name: "run.sh", want: tm.SyntaxFG("operator")},
		{name: "package.json", want: tm.SyntaxFG("property")},
		{name: "Makefile", want: tm.UI("diagnostic_warning")},
		{name: "index.html", want: tm.SyntaxFG("tag")},
		{name: "styles.css", want: tm.SyntaxFG("property")},
		{name: "main.rs", want: tm.SyntaxFG("function")},
		{name: "app.py", want: tm.SyntaxFG("function")},
		{name: "unknown.bin", want: tm.UI("gutter_fg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileIconColor(tm, config.FileIconConfig{Enabled: true, ColorMode: "semantic"}, fileIconForName(tt.name))
			if got != tt.want {
				t.Fatalf("semantic color = %q, want %q", got, tt.want)
			}
		})
	}
}
