//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// disabledSettle is how long J16 waits before it asserts that a query shows no rows. The palette searches
// asynchronously and the rig has no event that says a search finished, so the wait is the one allowance here.
const disabledSettle = time.Second

// TestJourneyJ16DroppedExtensionIsDisabledUntilReviewed is J16: a folder dropped into the extensions folder before
// launch is listed as Disabled, runs nothing (its rows never reach the palette), and its review declares its
// capabilities. Enter on the row opens the review, and only Enable there turns it on; then its rows appear.
// Not covered by the rig yet: a file dropped while the app runs (the reload path), and a folder that fails
// validation (PLAN.md J16 is covered only for a valid folder).
func TestJourneyJ16DroppedExtensionIsDisabledUntilReviewed(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.good")
	app := Launch(t, Options{Home: home})

	must(t, app.SummonPalette())
	must(t, app.Type("alpha"))
	time.Sleep(disabledSettle)
	assertNoRow(t, app, "alpha")
	shoot(t, app, "j16-1-root-no-rows-before-review")
	hidePalette(t, app)

	openExtensionsView(t, app)
	chooseExtensionRow(t, app, "good", "Good fixture")
	_, err := app.WaitFor("Disabled", elementWait)
	must(t, err)
	shoot(t, app, "j16-2-listed-disabled")

	must(t, app.Key("Return"))
	_, err = app.WaitFor("Review Good fixture", elementWait)
	must(t, err)
	_, err = app.WaitFor("desktop.apps", elementWait)
	must(t, err)
	shoot(t, app, "j16-3-review-declares-capability")
	must(t, app.Key("Return"))

	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("alpha"))
	waitForRow(t, app, "alpha", clipWait)
	shoot(t, app, "j16-4-enabled-after-review")
}
