package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestThePalettePanelDrawsItsGlassAsActive checks the palette keeps its material and draws it as the active window, so
// macOS does not frost the panel while it is not the key window.
func TestThePalettePanelDrawsItsGlassAsActive(t *testing.T) {
	appearance := paletteWindowAppearance()
	if appearance.Vibrancy != mygo.VibrancyGlass {
		t.Fatalf("palette vibrancy = %q, want %q", appearance.Vibrancy, mygo.VibrancyGlass)
	}
	if !appearance.ActiveAppearance {
		t.Fatal("palette panel is not drawn as active, so macOS frosts its glass while it is not key")
	}
}
