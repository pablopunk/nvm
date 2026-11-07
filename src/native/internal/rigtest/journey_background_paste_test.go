//go:build rig

package rigtest

import (
	"fmt"
	"os"
	"testing"
)

func TestJourneyBackgroundShortcutPastesItsFollowUpIntoTheSource(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.paste")
	app := Launch(t, Options{Home: home})
	openExtensionsView(t, app)
	enableThroughReview(t, app, "paste fixture", "Paste fixture")
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("Background paste fixture"))
	waitForRow(t, app, "Background paste fixture", elementWait)
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("Keyboard Shortcuts"))
	waitForRow(t, app, "Keyboard Shortcuts", elementWait)
	must(t, app.Key("Return"))
	must(t, app.Type("Background paste fixture"))
	waitForRow(t, app, "Background paste fixture", elementWait)
	shoot(t, app, "background-shortcut-binding")
	hidePalette(t, app)
	createNoteWith(t, app, "replace this")
	display := os.Getenv("DISPLAY")
	note := waitForWindowID(t, display, j21FloatingTitle)
	must(t, activateWindow(display, note))
	must(t, app.Key("ctrl+a"))
	// Summoning captures the source; closing leaves the shortcut free to run without opening the palette.
	must(t, app.SummonPalette())
	hidePalette(t, app)
	for round := 1; round <= 3; round++ {
		draft := fmt.Sprintf("replace round %d", round)
		must(t, app.Key("ctrl+a"))
		must(t, app.Type(draft))
		waitNoteOnDisk(t, app, draft)
		must(t, app.Key("ctrl+a"))
		clipboard := fmt.Sprintf("clipboard before background paste %d", round)
		must(t, app.ClipboardSet(clipboard))
		must(t, app.PressHotkey("Control+Alt+Shift+P"))
		waitNoteOnDisk(t, app, "nvm paste fixture")
		waitClipboardEquals(t, app, clipboard)
	}
	window, err := app.PaletteWindow()
	must(t, err)
	if window.Visible {
		t.Fatal("background paste opened the palette")
	}
	shoot(t, app, "background-paste-finished")
}
