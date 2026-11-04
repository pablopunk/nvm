package extview

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWithWebviewThemeInsertsAfterTheHeadStartTag proves the style goes inside the head, after a doctype, so the
// document keeps standards mode, and that a document without a head gets the style in front of it.
func TestWithWebviewThemeInsertsAfterTheHeadStartTag(t *testing.T) {
	declarations := []string{"--nvm-accent:#8a6a00"}
	withHead := withWebviewTheme(`<!doctype html><HEAD lang="en"><title>x</title></HEAD><body>hi</body>`, declarations)
	if !strings.HasPrefix(withHead, `<!doctype html><HEAD lang="en"><style data-nevermind-theme>`) {
		t.Fatalf("style not inside the head: %s", withHead)
	}
	if !strings.HasSuffix(withHead, `<title>x</title></HEAD><body>hi</body>`) {
		t.Fatalf("document changed after the style: %s", withHead)
	}
	bare := withWebviewTheme(`<p>plain</p>`, declarations)
	if !strings.HasPrefix(bare, `<style data-nevermind-theme>`) || !strings.HasSuffix(bare, `<p>plain</p>`) {
		t.Fatalf("headless document = %s", bare)
	}
}

// TestWithWebviewThemeIgnoresHeadLikeNames proves a tag such as <header> is not taken for the head.
func TestWithWebviewThemeIgnoresHeadLikeNames(t *testing.T) {
	html := `<header>top</header><head><title>t</title></head>`
	themed := withWebviewTheme(html, []string{"--nvm-accent:#000"})
	if !strings.HasPrefix(themed, `<header>top</header><head><style data-nevermind-theme>`) {
		t.Fatalf("style placed by <header>: %s", themed)
	}
}

// TestWebviewStyleSanitizesValues proves a theme value cannot close the style block or break its rule.
func TestWebviewStyleSanitizesValues(t *testing.T) {
	style := webviewStyleElement([]string{"--nvm-accent:red}</style><script>alert(1)</script>"})
	if strings.Contains(style, "</style><script>") || strings.Count(style, "}") != 2 {
		t.Fatalf("unsafe value reached the style: %s", style)
	}
}

// TestWebviewThemeDeclarationsFollowTheAppearance proves the page reads the palette's tokens for the appearance the
// palette shows, as Electron's --nvm-* variables do, and that the radii are the palette's pixel values.
func TestWebviewThemeDeclarationsFollowTheAppearance(t *testing.T) {
	light := strings.Join(webviewThemeDeclarations(false), ";")
	dark := strings.Join(webviewThemeDeclarations(true), ";")
	for _, want := range []string{"--nvm-surface-canvas:#f5f2eb", "--nvm-text-strong:#282721", "--nvm-radius-md:12px", "--nvm-font-sans:"} {
		if !strings.Contains(light, want) {
			t.Errorf("light declarations miss %q", want)
		}
	}
	if !strings.Contains(dark, "--nvm-surface-canvas:#1a1f28") || light == dark {
		t.Errorf("dark declarations do not follow the dark tokens")
	}
}

// TestCSSColorWritesOpaqueAsHexAndTranslucentAsRGBA proves the colors a page reads are valid CSS.
func TestCSSColorWritesOpaqueAsHexAndTranslucentAsRGBA(t *testing.T) {
	if got := cssColor(ui.RGB(0x8a, 0x6a, 0x00)); got != "#8a6a00" {
		t.Errorf("opaque = %q", got)
	}
	if got := cssColor(ui.RGBA(0x36, 0x31, 0x29, 0.5)); !strings.HasPrefix(got, "rgba(54,49,41,0.") {
		t.Errorf("translucent = %q", got)
	}
}

// TestPageBackgroundIsThePaletteSurfaceInBothAppearances proves a page fills with the palette surface, so the view
// looks like the palette while the page has not painted.
func TestPageBackgroundIsThePaletteSurfaceInBothAppearances(t *testing.T) {
	if got := pageBackground(); got != "light-dark(#f5f2eb, #1a1f28)" {
		t.Fatalf("pageBackground() = %q", got)
	}
}
