//go:build rig

package rigtest

import (
	"testing"
)

// TestJourneyJ31EnabledExtensionRunsAndItsCommandsAreFound is J31: an extension enabled through its review starts,
// the Extensions list shows it Running without being reopened, and typing a command's title at the root finds the
// command. The fixture is acme.views, whose commands are declared, not searched, so only the typed command proves
// the command path. Not covered: the list refresh after a child fails, and a command that runs its own view.
func TestJourneyJ31EnabledExtensionRunsAndItsCommandsAreFound(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.views")
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "view fixtures", "View fixtures")
	_, err := app.WaitFor("Running", elementWait)
	must(t, err)
	shoot(t, app, "j31-1-enabled-shows-running")

	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type("list fixture"))
	waitForRow(t, app, "List fixture", clipWait)
	shoot(t, app, "j31-2-command-found-by-title")
}
