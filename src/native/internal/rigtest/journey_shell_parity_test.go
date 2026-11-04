//go:build rig

package rigtest

import "testing"

// TestJourneyShellParityEscapeHidesTheRootWithText is the Escape part of the 1.x key contract: with text in the search
// field, one Escape at the root hides the palette, and the next summon starts on the root again.
func TestJourneyShellParityEscapeHidesTheRootWithText(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	must(t, app.Type("settings"))
	waitForRow(t, app, settingsRootRow, elementWait)
	must(t, app.Key("Escape"))
	if _, err := app.WaitPalette(hideWait, "hidden", isHidden); err != nil {
		t.Fatalf("one Escape with text in the field should hide the palette: %v", err)
	}
	must(t, app.SummonPalette())
	if paletteShows(t, app, "settings") {
		t.Fatal("the summon after the Escape kept the typed text; the palette should start empty")
	}
	hidePalette(t, app)
}
