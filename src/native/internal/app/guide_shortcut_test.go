package app

import "testing"

// TestGuideShortcutRowReadsTheLivePaletteShortcut checks the guide's shortcut row: once the user sets a palette shortcut,
// the row names that one, formatted as the Settings row formats it, and not the legacy setting the app started with.
func TestGuideShortcutRowReadsTheLivePaletteShortcut(t *testing.T) {
	p := goldenPalette(t)
	if _, err := p.shortcutsManager().SetPalette("Command+Alt+K"); err != nil {
		t.Fatal(err)
	}
	want := shortcutLabelOf("Command+Alt+K")
	if got := p.guideShortcutLabel(); got != want || got == "" {
		t.Fatalf("the guide names %q after the user set Command+Alt+K, want %q", got, want)
	}
}
