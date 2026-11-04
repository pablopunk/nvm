package app

import (
	"testing"
)

// TestGuideShortcutLabelComesFromTheLiveShortcutTable checks the label the guide's shortcut row names: once the user sets
// a palette shortcut, the shortcut table the built-in reads spells it as the Settings row does.
func TestGuideShortcutLabelComesFromTheLiveShortcutTable(t *testing.T) {
	p := goldenPalette(t)
	if _, err := p.shortcutsManager().SetPalette("Command+Alt+K"); err != nil {
		t.Fatal(err)
	}
	want := p.shortcutLabelOf("Command+Alt+K")
	if want == "" || p.extensionShortcuts().Palette().Label != want {
		t.Fatalf("label = %q, want %q", p.extensionShortcuts().Palette().Label, want)
	}
}

// openGuide opens the guide from the Welcome row, as the user does: a search for it, then Get Started.
func openGuide(t *testing.T, p *App) {
	t.Helper()
	goldenQuery(p, "welcome")
	itemTitled(t, p, "Welcome to Nevermind").Actions[0].Run(p)
}
