//go:build rig

package rigtest

import "testing"

func TestJourneyFormCTAEditsValidatesAndSubmitsWithItsShortcut(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		t.Run(theme, func(t *testing.T) {
			home := t.TempDir()
			seedExtensionFolder(t, home, "acme.form")
			app := Launch(t, Options{Home: home, Theme: theme})
			openExtensionsView(t, app)
			enableThroughReview(t, app, "form fixture", "Form fixture")
			hidePalette(t, app)
			must(t, app.SummonPalette())
			must(t, app.Type("Form fixture"))
			waitForRow(t, app, "Form fixture", elementWait)
			must(t, app.Key("Return"))
			waitForRow(t, app, "Name", elementWait)
			must(t, app.Type("Name"))
			waitForRow(t, app, "Create Ticket", elementWait)
			shoot(t, app, "form-primary-action")
			must(t, app.Key("Return"))
			must(t, app.Type("A"))
			must(t, app.Key("ctrl+Return"))
			waitForRow(t, app, "Name", elementWait)
			waitSearchValue(t, app, "")
			must(t, app.Key("Return"))
			must(t, app.Key("ctrl+a"))
			must(t, app.Type("Ada"))
			must(t, app.Key("ctrl+Return"))
			waitForRow(t, app, "Accept the terms", elementWait)
			must(t, app.Key("Return"))
			must(t, app.Key("ctrl+Return"))
			_, err := app.WaitFor("Ada", elementWait)
			must(t, err)
			shoot(t, app, "form-submitted")
		})
	}
}
