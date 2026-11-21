//go:build rig

package rigtest

import "testing"

// TestJourneyJ11ChangePaletteHotkey is the J11 smoke, driven from the keyboard only (J27's keyboard part): Settings opens
// the palette hotkey recorder, Control+Shift+Space is recorded and saved with Enter, that key summons the palette, and the
// old key no longer does. The "Shortcut set" confirmation is a HUD toast in its own window, which the palette tree does not
// expose, so the save is checked by the summon with the new key. The conflict messages and launch at login of J11 are not covered yet.
func TestJourneyJ11ChangePaletteHotkey(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	must(t, app.Type("settings"))
	_, err := app.WaitFor("Settings", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search Settings", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Press a keyboard shortcut", elementWait)
	must(t, err)
	shoot(t, app, "j11-1-recorder-open")

	must(t, app.PressHotkey("Control+Shift+Space"))
	_, err = app.WaitFor("Save shortcut for “Set Nevermind shortcut”", elementWait)
	must(t, err)
	shoot(t, app, "j11-2-recorded")

	must(t, app.Key("Return"))
	_, err = app.WaitFor("Open Nevermind Shortcut", elementWait)
	must(t, err)
	shoot(t, app, "j11-3-saved")

	must(t, app.Key("Escape"))
	must(t, app.DismissWithEscape())

	must(t, app.PressHotkey("Alt+Shift+Space"))
	if _, err := app.WaitPalette(hideWait, "visible", isVisible); err == nil {
		t.Fatal("the old palette hotkey still shows Nevermind")
	}
	must(t, app.PressHotkey("Control+Shift+Space"))
	_, err = app.WaitPalette(summonWait, "visible and focused", isVisibleAndFocused)
	must(t, err)
	shoot(t, app, "j11-4-new-hotkey-summons")
	must(t, app.DismissWithEscape())
}
