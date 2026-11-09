//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// Real extension sources are deliberately slow; reopening uses the host's stale snapshot.
func TestJourneyLoadingGrid(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			home := t.TempDir()
			seedExtensionFolder(t, home, "acme.loading")
			app := Launch(t, Options{Home: home, Theme: theme})
			openExtensionsView(t, app)
			enableThroughReview(t, app, "loading", "Loading fixtures")
			hidePalette(t, app)
			for _, layout := range []string{"list", "grid", "compact"} {
				must(t, app.SummonPalette())
				must(t, app.Type("Slow "+layout+" fixture"))
				openRootFixtureRowExactly(t, app, "Slow "+layout+" fixture")
				_, err := app.WaitFor("Loading…", elementWait)
				must(t, err)
				shoot(t, app, layout+"-cold")
				waitForRow(t, app, "Nevermind", 10*time.Second)
				if layout != "list" {
					must(t, app.Key("Right"))
					must(t, app.Key("Down"))
					must(t, app.Key("Left"))
					must(t, app.Key("Up"))
				}
				shoot(t, app, layout+"-settled")
				must(t, app.Key("ctrl+k"))
				_, err = app.WaitFor("Open", elementWait)
				must(t, err)
				must(t, app.Key("Escape"))
				hidePalette(t, app)
				must(t, app.SummonPalette())
				must(t, app.Type("Slow "+layout+" fixture"))
				openRootFixtureRowExactly(t, app, "Slow "+layout+" fixture")
				// A warm open shows the cached rows at once and refreshes without a status flash.
				waitForRow(t, app, "Nevermind", elementWait)
				shoot(t, app, layout+"-warm")
				must(t, app.Key("Return"))
				_, err = app.WaitFor("Opened fixture", elementWait)
				must(t, err)
				hidePalette(t, app)
			}
		})
	}
}
