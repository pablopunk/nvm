package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestCommandSubtitleFallsBackToTheExtensionTitle: a command with no subtitle shows the extension's title, as 1.x does.
func TestCommandSubtitleFallsBackToTheExtensionTitle(t *testing.T) {
	if got := commandSubtitle(api.Command{Title: "Toggle System Theme", Subtitle: "Switch light / dark mode"}, "System Theme"); got != "Switch light / dark mode" {
		t.Fatalf("subtitle = %q; want the command's own", got)
	}
	if got := commandSubtitle(api.Command{Title: "Toggle System Theme"}, "System Theme"); got != "System Theme" {
		t.Fatalf("subtitle = %q; want the extension title", got)
	}
}

// TestEnabledConvertedToggleShowsItsCommandInTheRoot: an enabled extension's command is a root row, named by its
// extension at the right edge and by its own subtitle.
func TestEnabledConvertedToggleShowsItsCommandInTheRoot(t *testing.T) {
	_, _, row := awaitFixtureCommand(t, "acme.theme", "toggle system theme", "Toggle System Theme")
	if row.Kind != "Acme Theme" || row.Subtitle != "Switch between light and dark mode" {
		t.Fatalf("row kind %q subtitle %q; want the extension at the right edge and the command's subtitle", row.Kind, row.Subtitle)
	}
}

// TestDisabledExtensionCommandIsNotInTheRoot matches 1.x: a disabled extension's commands are not root rows. Its row is in
// the Extensions list instead, where it shows Disabled and offers Enable.
func TestDisabledExtensionCommandIsNotInTheRoot(t *testing.T) {
	m := newFixtureManager(t)
	files := fixtureFiles(t, "acme.theme")
	if err := m.Install("acme.theme", files, false); err != nil {
		t.Fatal(err)
	}
	p := &App{}
	p.exts.mgr.Store(m)

	for _, row := range collectExtensionRows(t, p, "toggle system theme") {
		if row.Title == "Toggle System Theme" {
			t.Fatalf("a disabled extension showed its command %q at the root", row.Title)
		}
	}
	listing := m.List()
	if len(listing) != 1 || listing[0].Enabled {
		t.Fatalf("listing = %+v; want the disabled extension in the Extensions list", listing)
	}
}
