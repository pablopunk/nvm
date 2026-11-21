//go:build rig

package rigtest

import (
	"testing"
	"time"
)

func TestJourneyDeclarativeViewNavigation(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.navigation")
	app := Launch(t, Options{Home: home, Theme: "dark"})
	openExtensionsView(t, app)
	enableThroughReview(t, app, "navigation", "Navigation fixture")
	_, err := app.WaitFor("Running", elementWait)
	must(t, err)
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("Navigation fixture"))
	waitForRow(t, app, "Navigation fixture", clipWait)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Open branch", 10*time.Second)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Replace destination", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Replacement destination", elementWait)
	must(t, err)
	must(t, app.Key("Escape"))
	_, err = app.WaitFor("Open branch", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Go to root destination", elementWait)
	must(t, err)
	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Root destination", elementWait)
	must(t, err)
	must(t, app.Key("Escape"))
	_, err = app.WaitFor("Navigation fixture", elementWait)
	must(t, err)
}
