//go:build rig

package rigtest

import "testing"

// TestJourneyJ01HotkeyHidesAShownPalette is the toggle of J01: the summon shortcut shows the palette, and pressing it
// again while the palette is shown hides it, as Electron's togglePalette does.
func TestJourneyJ01HotkeyHidesAShownPalette(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	shoot(t, app, "j01-toggle-1-shown")

	must(t, app.PressHotkey(DefaultHotkey))
	_, err := app.WaitPalette(hideWait, "hidden", isHidden)
	must(t, err)
	shoot(t, app, "j01-toggle-2-press-hides")

	must(t, app.SummonPalette())
	shoot(t, app, "j01-toggle-3-press-shows-again")
	must(t, app.DismissWithEscape())
}
