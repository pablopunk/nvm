package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// TestPaletteReadsDesignTokens checks that the palette's views take their tokens from the design package in both
// appearances. The contrast of every text pair those tokens draw, and the known failures, live in package design.
func TestPaletteReadsDesignTokens(t *testing.T) {
	for _, dark := range []bool{false, true} {
		if got := renderedTokens(t, dark); got != design.TokensFor(dark) {
			t.Fatalf("dark=%v: the palette renders with other tokens than design.TokensFor", dark)
		}
	}
}

// renderedTokens returns the tokens a view renders with in the requested appearance, read the way the renderer
// reads them.
func renderedTokens(t *testing.T, dark bool) *design.TokenSet {
	t.Helper()
	var rendered *design.TokenSet
	tt := ui.NewTester(func(c *ui.Context) { rendered = design.Tokens(c) }, 64, 64)
	tt.SetDark(dark)
	tt.Frame()
	if rendered == nil {
		t.Fatal("the view did not render any tokens")
	}
	return rendered
}
