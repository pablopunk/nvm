//go:build rig

package rigtest

import "testing"

func TestJourneyActionMenuSearchPreservesTheRootAndRunsItsMatch(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			home := t.TempDir()
			seedExtensionFolder(t, home, "acme.views")
			app := Launch(t, Options{Home: home, Theme: theme})
			openExtensionsView(t, app)
			enableThroughReview(t, app, "view fixtures", "View fixtures")
			hidePalette(t, app)
			must(t, app.SummonPalette())
			must(t, app.Type("list fixture"))
			waitForRow(t, app, "List fixture", clipWait)
			must(t, app.Key("ctrl+k"))
			_, err := app.WaitFor("Search actions", elementWait)
			must(t, err)
			must(t, app.Type("alias"))
			waitForRow(t, app, "Set alias", elementWait)
			waitSearchValue(t, app, "list fixture")
			shoot(t, app, "action-search-filtered")
			must(t, app.Key("Escape"))
			waitSearchValue(t, app, "list fixture")
			must(t, app.Key("ctrl+k"))
			must(t, app.Type("alias"))
			must(t, app.Key("Return"))
			_, err = app.WaitFor("Alias for “List fixture”", elementWait)
			must(t, err)
			shoot(t, app, "action-search-opened-alias")
		})
	}
}
