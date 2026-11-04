//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// TestJourneyJ01HideAndSummonStartOnTheRoot is the stack part of J01: a page opened from Settings does not survive the
// hide, so the next summon shows the root with no Settings or Account page left on the stack, as Electron's onShown does.
func TestJourneyJ01HideAndSummonStartOnTheRoot(t *testing.T) {
	app := Launch(t, Options{})
	openAccountPage(t, app)
	shoot(t, app, "j01-stack-1-account-open")

	must(t, app.PressHotkey(DefaultHotkey))
	_, err := app.WaitPalette(hideWait, "hidden", isHidden)
	must(t, err)

	must(t, app.SummonPalette())
	if paletteShows(t, app, "Nevermind Account") || paletteShows(t, app, "Search Settings") {
		t.Fatal("summon after a hide showed a page from Settings; the palette should start on the root")
	}
	must(t, app.Type("settings"))
	waitForRow(t, app, settingsRootRow, elementWait)
	shoot(t, app, "j01-stack-2-summoned-root")
	hidePalette(t, app)
}

// waitUntilPaletteLacks polls until no palette element carries text, and reports whether that happened in time.
func waitUntilPaletteLacks(t *testing.T, app *App, text string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for paletteShows(t, app, text) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
	return true
}
