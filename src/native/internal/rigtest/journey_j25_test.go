//go:build rig

package rigtest

import (
	"testing"
)

// TestJourneyJ25FirstRunGuideBecomesHelp is the J25 smoke: a fresh profile shows the Welcome row first, Enter opens
// the Getting Started guide, and Done for now turns the row into Help, which a search finds. The journey never runs a system command; the
// permission prompts and the no-prompt-before-use rule of J25 are not covered yet.
func TestJourneyJ25FirstRunGuideBecomesHelp(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	_, err := app.WaitFor("Welcome to Nevermind", elementWait)
	must(t, err)
	shoot(t, app, "j25-1-welcome")

	must(t, app.Key("Return"))
	_, err = app.WaitFor("Permissions when needed", elementWait)
	must(t, err)
	shoot(t, app, "j25-2-guide")

	must(t, app.Type("done for now"))
	_, err = app.WaitFor("Done for now", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // Done for now pops the guide back to the root, as 1.x does; it does not hide the palette
	_, err = app.WaitFor("Help", elementWait)
	must(t, err)
	shoot(t, app, "j25-3-done-root")
	must(t, app.Key("Escape"))
	shoot(t, app, "j25-4-after-escape")
	must(t, app.DismissWithEscape())

	must(t, app.SummonPalette())
	must(t, app.Type("help"))
	_, err = app.WaitFor("Help", elementWait)
	must(t, err)
	shoot(t, app, "j25-3-help")
	clearSearch(t, app)
	must(t, app.DismissWithEscape())
}
