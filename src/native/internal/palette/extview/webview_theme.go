package extview

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// webviewFontSans is the sans-serif stack the palette's text uses, which the page reads as --nvm-font-sans.
const webviewFontSans = `system-ui, -apple-system, "Segoe UI", sans-serif`

// webviewRadii are the palette's corner radii in pixels, the values of styles/theme.css.
var webviewRadii = []struct {
	name string
	px   int
}{{"radius-xs", 4}, {"radius-sm", 8}, {"radius-md", 12}, {"radius-lg", 16}, {"radius-pill", 999}}

// webviewThemeColors names the palette tokens a page reads, in the order they are written.
var webviewThemeColors = []struct {
	name  string
	color func(t *design.TokenSet) ui.Color
}{
	{"surface-canvas", func(t *design.TokenSet) ui.Color { return t.Bg }},
	{"surface-panel", func(t *design.TokenSet) ui.Color { return t.Panel }},
	{"surface-raised", func(t *design.TokenSet) ui.Color { return t.Raised }},
	{"surface-interactive", func(t *design.TokenSet) ui.Color { return t.Interactive }},
	{"surface-selected", func(t *design.TokenSet) ui.Color { return t.Selected }},
	{"surface-sunken", func(t *design.TokenSet) ui.Color { return t.Sunken }},
	{"text-strong", func(t *design.TokenSet) ui.Color { return t.Strong }},
	{"text-primary", func(t *design.TokenSet) ui.Color { return t.Primary }},
	{"text-secondary", func(t *design.TokenSet) ui.Color { return t.Secondary }},
	{"text-muted", func(t *design.TokenSet) ui.Color { return t.Muted }},
	{"accent", func(t *design.TokenSet) ui.Color { return t.Accent }},
	{"accent-fill", func(t *design.TokenSet) ui.Color { return t.AccentFill }},
	{"accent-contrast", func(t *design.TokenSet) ui.Color { return t.AccentContrast }},
	{"accent-soft", func(t *design.TokenSet) ui.Color { return t.AccentSoft }},
}

// webviewThemeDeclarations returns the --nvm-* declarations of the palette theme for the light or dark tokens.
func webviewThemeDeclarations(dark bool) []string {
	tokens := design.TokensFor(dark)
	declarations := []string{"--nvm-font-sans:" + webviewFontSans}
	for _, entry := range webviewThemeColors {
		declarations = append(declarations, fmt.Sprintf("--nvm-%s:%s", entry.name, cssColor(entry.color(tokens))))
	}
	for _, radius := range webviewRadii {
		declarations = append(declarations, fmt.Sprintf("--nvm-%s:%dpx", radius.name, radius.px))
	}
	return declarations
}

// pageBackground is the palette surface in both appearances: the color a page fills with until it paints its own.
func pageBackground() string {
	return "light-dark(" + cssColor(design.TokensFor(false).Bg) + ", " + cssColor(design.TokensFor(true).Bg) + ")"
}

// cssColor writes a palette color as CSS: hex when opaque, rgba otherwise.
func cssColor(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", c.R, c.G, c.B, float64(c.A)/255)
}

// webviewStyleElement is the style block a page starts with: the theme variables, the color scheme, and no
// scrollbars, which the palette's own scrolling replaces.
func webviewStyleElement(declarations []string) string {
	sanitized := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		sanitized = append(sanitized, unsafeStyleCharacters.Replace(declaration)+";")
	}
	return `<style data-nevermind-theme>:root{color-scheme:light dark;scrollbar-width:none;` +
		strings.Join(sanitized, "") + `}::-webkit-scrollbar{display:none}</style>`
}

// unsafeStyleCharacters are removed from theme values so a value cannot close the style block or the rule.
var unsafeStyleCharacters = strings.NewReplacer("<", "", ";", "", "{", "", "}", "")

// withWebviewTheme inserts the theme style after the document's <head> start tag, so a leading doctype keeps the
// page in standards mode. A document with no head gets the style in front of it.
func withWebviewTheme(html string, declarations []string) string {
	style := webviewStyleElement(declarations)
	head := headStartTag(html)
	if head == nil {
		return style + html
	}
	return html[:head[1]] + style + html[head[1]:]
}

// headStartTag returns the byte offsets of the first <head ...> start tag in html as [start, end], or nil.
func headStartTag(html string) []int {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<head")
	for start >= 0 {
		end := strings.IndexByte(lower[start:], '>')
		if end < 0 {
			return nil
		}
		next := start + 5
		if next < len(lower) && (lower[next] == '>' || lower[next] == ' ' || lower[next] == '\t' || lower[next] == '\n' || lower[next] == '/') {
			return []int{start, start + end + 1}
		}
		offset := strings.Index(lower[start+1:], "<head")
		if offset < 0 {
			return nil
		}
		start += 1 + offset
	}
	return nil
}
