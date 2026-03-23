package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yourusername/toast/internal/theme"
)

// vsCodeTheme represents the JSON structure of a VSCode color theme file.
type vsCodeTheme struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Colors      map[string]string  `json:"colors"`
	TokenColors []vsCodeTokenColor `json:"tokenColors"`
}

type vsCodeTokenColor struct {
	Name     string          `json:"name"`
	Scope    json.RawMessage `json:"scope"`
	Settings struct {
		Foreground string `json:"foreground"`
		Background string `json:"background"`
		FontStyle  string `json:"fontStyle"`
	} `json:"settings"`
}

// scopes returns the scope(s) as a normalized string slice.
// VSCode themes allow scope to be a string or []string.
func (tc *vsCodeTokenColor) scopes() []string {
	if len(tc.Scope) == 0 {
		return nil
	}
	// Try as string first.
	var single string
	if err := json.Unmarshal(tc.Scope, &single); err == nil {
		// A single scope string may be comma-separated.
		parts := strings.Split(single, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	// Try as []string.
	var multi []string
	if err := json.Unmarshal(tc.Scope, &multi); err == nil {
		return multi
	}
	return nil
}

// scopeEntry is a flattened (scope, settings) pair from tokenColors.
type scopeEntry struct {
	scope    string
	fg       string
	bold     bool
	italic   bool
	order    int // position in tokenColors array (later = higher priority)
}

// ConvertResult holds the output of a successful conversion.
type ConvertResult struct {
	Name       string
	OutputPath string
}

// missingToken records a token that could not be resolved.
type missingToken struct {
	Category string // "UI", "Syntax", "Git"
	Token    string
	Searched string // human-readable description of what was looked for
}

// ConvertVSCode reads a VSCode theme JSON file, converts it to a Toast theme,
// and writes the result to outputDir. Returns an error listing all missing
// mappings if the theme is incomplete.
func ConvertVSCode(inputPath, outputDir string) (*ConvertResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading VSCode theme: %w", err)
	}

	var vsc vsCodeTheme
	if err := json.Unmarshal(data, &vsc); err != nil {
		return nil, fmt.Errorf("parsing VSCode theme JSON: %w", err)
	}

	if vsc.Name == "" {
		return nil, fmt.Errorf("VSCode theme has no \"name\" field")
	}
	if vsc.Colors == nil {
		return nil, fmt.Errorf("VSCode theme has no \"colors\" object")
	}
	if len(vsc.TokenColors) == 0 {
		return nil, fmt.Errorf("VSCode theme has no \"tokenColors\" array")
	}

	var missing []missingToken

	// Resolve UI tokens.
	ui := make(map[string]string, len(uiMappings))
	for _, m := range uiMappings {
		val := resolveColorMapping(vsc.Colors, m)
		if val == "" {
			missing = append(missing, missingToken{
				Category: "UI",
				Token:    m.ToastKey,
				Searched: describeMappingSearch(m),
			})
			continue
		}
		ui[m.ToastKey] = val
	}

	// Build scope index for syntax resolution.
	index := buildScopeIndex(vsc.TokenColors)

	// Resolve syntax tokens.
	syntax := make(map[string]theme.SyntaxStyle, len(syntaxMappings))
	for _, sm := range syntaxMappings {
		entry := resolveSyntax(index, sm.Scopes)
		if entry == nil {
			missing = append(missing, missingToken{
				Category: "Syntax",
				Token:    sm.ToastKey,
				Searched: fmt.Sprintf("scopes %v in tokenColors", sm.Scopes),
			})
			continue
		}
		syntax[sm.ToastKey] = theme.SyntaxStyle{
			FG:     entry.fg,
			Bold:   entry.bold,
			Italic: entry.italic,
		}
	}

	// Resolve git tokens.
	git := make(map[string]string, len(gitMappings))
	for _, m := range gitMappings {
		val := resolveColorMapping(vsc.Colors, m)
		if val == "" {
			missing = append(missing, missingToken{
				Category: "Git",
				Token:    m.ToastKey,
				Searched: describeMappingSearch(m),
			})
			continue
		}
		git[m.ToastKey] = val
	}

	if len(missing) > 0 {
		return nil, formatMissingError(missing)
	}

	// Determine variant.
	variant := "dark"
	if vsc.Type == "light" {
		variant = "light"
	}

	t := theme.Theme{
		Name:    vsc.Name,
		Variant: variant,
		UI:      ui,
		Syntax:  syntax,
		Git:     git,
	}

	fileName := toKebab(vsc.Name) + ".json"
	outPath := filepath.Join(outputDir, fileName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating themes directory: %w", err)
	}

	out, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling theme: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return nil, fmt.Errorf("writing theme file: %w", err)
	}

	return &ConvertResult{
		Name:       vsc.Name,
		OutputPath: outPath,
	}, nil
}

// resolveColorMapping tries Primary, then Fallbacks, then Derive for a mapping.
// Returns the normalized color or "" if unresolvable.
func resolveColorMapping(colors map[string]string, m mapping) string {
	if m.Primary != "" {
		if v, ok := colors[m.Primary]; ok {
			return normalizeColor(v)
		}
	}
	for _, fb := range m.Fallbacks {
		if v, ok := colors[fb]; ok {
			return normalizeColor(v)
		}
	}
	if m.Derive != nil {
		if v := m.Derive(colors); v != "" {
			return normalizeColor(v)
		}
	}
	return ""
}

// buildScopeIndex flattens tokenColors into a list of (scope, settings) pairs.
func buildScopeIndex(tokenColors []vsCodeTokenColor) []scopeEntry {
	var entries []scopeEntry
	for i, tc := range tokenColors {
		for _, scope := range tc.scopes() {
			entries = append(entries, scopeEntry{
				scope:  scope,
				fg:     normalizeColor(tc.Settings.Foreground),
				bold:   strings.Contains(tc.Settings.FontStyle, "bold"),
				italic: strings.Contains(tc.Settings.FontStyle, "italic"),
				order:  i,
			})
		}
	}
	return entries
}

// resolveSyntax finds the best tokenColors match for the given query scopes.
// It tries each query scope in priority order and returns the first that matches.
// For a given query, a tokenColors scope matches if it equals the query or is a
// prefix of the query (TextMate scope selector semantics).
// Among matches, the most specific (longest scope) wins; ties broken by order.
func resolveSyntax(index []scopeEntry, queryScopes []string) *scopeEntry {
	for _, query := range queryScopes {
		var best *scopeEntry
		var bestLen int
		var bestOrder int
		for i := range index {
			e := &index[i]
			if e.fg == "" {
				continue
			}
			if !scopeMatches(e.scope, query) {
				continue
			}
			matchLen := len(e.scope)
			if best == nil || matchLen > bestLen || (matchLen == bestLen && e.order > bestOrder) {
				best = e
				bestLen = matchLen
				bestOrder = e.order
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// scopeMatches returns true if tokenScope matches queryScope.
// tokenScope "keyword" matches queryScope "keyword" (exact) and
// "keyword.control" (prefix), but not "keyword_other".
func scopeMatches(tokenScope, queryScope string) bool {
	if tokenScope == queryScope {
		return true
	}
	return strings.HasPrefix(queryScope, tokenScope+".")
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// toKebab converts a name like "Kanagawa Wave" to "kanagawa-wave".
func toKebab(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// normalizeColor normalizes a hex color string:
//   - Strips alpha channel: "#1F1F2880" → "#1f1f28"
//   - Expands shorthand: "#fff" → "#ffffff"
//   - Lowercases
//   - Returns "" for empty input
func normalizeColor(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	if !strings.HasPrefix(c, "#") {
		return c
	}
	hex := strings.ToLower(c[1:])

	// Expand 3-char shorthand (#fff → ffffff).
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	// Strip alpha channel (8-char or 4-char hex).
	if len(hex) == 8 {
		hex = hex[:6]
	} else if len(hex) == 4 {
		// #rgba shorthand → expand then strip alpha.
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}

	return "#" + hex
}

// describeMappingSearch returns a human-readable description of what VSCode
// keys were searched for a mapping.
func describeMappingSearch(m mapping) string {
	if m.Primary == "" && m.Derive != nil {
		return "derived value (should not fail)"
	}
	keys := []string{fmt.Sprintf("%q", m.Primary)}
	for _, fb := range m.Fallbacks {
		keys = append(keys, fmt.Sprintf("%q", fb))
	}
	return strings.Join(keys, " or ") + " in theme colors object"
}

// formatMissingError builds a detailed error message from a list of missing tokens.
func formatMissingError(missing []missingToken) error {
	var b strings.Builder
	fmt.Fprintf(&b, "migration failed -- %d required color(s) not found in VSCode theme\n", len(missing))

	categories := []string{"UI", "Syntax", "Git"}
	for _, cat := range categories {
		var items []missingToken
		for _, m := range missing {
			if m.Category == cat {
				items = append(items, m)
			}
		}
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n  %s tokens:\n", cat)
		for _, item := range items {
			fmt.Fprintf(&b, "    %s: need %s\n", item.Token, item.Searched)
		}
	}

	return fmt.Errorf("%s", b.String())
}
