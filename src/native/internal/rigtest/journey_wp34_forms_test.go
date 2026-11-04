//go:build rig

package rigtest

import (
	"strings"
	"testing"
)

// The WP3.4 journey against the real app under the rig: a fixture form is filled and submitted through its child
// process, and an editor's draft survives SIGKILL and comes back on relaunch. The rig reads labels, values and states,
// so each step checks what the palette shows. Not covered by the rig: the native file dialog (the rig has no dialog
// automation), draft conflicts, and the Escape path through a field being edited.

const (
	wp34FormFixture   = "acme.form"
	wp34EditorFixture = "acme.editor"
	wp34Note          = "Typed before the kill"
)

// TestJourneyWP34FormSubmitAndDraftSurvivesKill submits the form fixture with Name and the required terms, then types
// into the note fixture, waits for its draft to be written, kills the app and relaunches it over the same HOME. The
// note must come back with its text and a recovered status, and Save must commit the draft.
func TestJourneyWP34FormSubmitAndDraftSurvivesKill(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, wp34FormFixture)
	seedExtensionFolder(t, home, wp34EditorFixture)
	app := Launch(t, Options{Home: home, Run: "first"})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "form", "Form fixture")
	clearSearch(t, app)
	enableThroughReview(t, app, "editor", "Editor fixture")
	hidePalette(t, app)

	submitFormFixture(t, app)
	shoot(t, app, "wp34-1-form-submitted")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Editor fixture"))
	openRootFixtureRow(t, app, "Editor fixture: open the note")
	_, err := app.WaitFor("Save   esc Back", elementWait) // the editor's footer: the text area has the focus by now
	must(t, err)
	must(t, app.Type(wp34Note))
	// Each edit clears the status, so "Draft saved" next to the full text means a save after the last key.
	must(t, app.WaitForTogether(elementWait, wp34Note, "Draft saved"))
	shoot(t, app, "wp34-2-draft-saved")
	must(t, app.KillHard())

	relaunched := Launch(t, Options{Home: home, Run: "relaunch"})
	must(t, relaunched.SummonPalette())
	must(t, relaunched.Type("Editor fixture"))
	openRootFixtureRow(t, relaunched, "Editor fixture: open the note")
	_, err = relaunched.WaitFor("Recovered unsaved text", elementWait)
	must(t, err)
	shoot(t, relaunched, "wp34-3-recovered-after-kill")

	must(t, relaunched.Key("ctrl+Return"))
	_, err = relaunched.WaitFor(wp34Note+" · draft removed: true", elementWait) // the draft holds the saved text, then the typed words
	must(t, err)
	shoot(t, relaunched, "wp34-4-committed")
	hidePalette(t, relaunched)
}

// submitFormFixture opens the form fixture, fills Name, checks the required terms row, and submits with Ctrl+Enter.
// The submitted values come back as rows of the next view.
func submitFormFixture(t *testing.T, app *App) {
	t.Helper()
	must(t, app.SummonPalette())
	must(t, app.Type("Form fixture"))
	openRootFixtureRow(t, app, "Form fixture: create a ticket")
	_, err := app.WaitFor("Create Ticket", elementWait)
	must(t, err)

	must(t, app.Key("Return")) // edit Name, the first row
	must(t, app.Type("Ada"))
	_, err = app.WaitFor("Set Name", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // set Name
	_, err = app.WaitFor("Name", elementWait)
	must(t, err)

	must(t, app.Key("Return")) // edit Name again: its value is in the search field, and the caret is after it
	must(t, app.Type(" Lovelace"))
	_, err = app.WaitFor("Ada Lovelace", elementWait) // the Set Name row shows what the field will hold
	must(t, err)
	must(t, app.Key("Return")) // set Name

	for range 3 { // Name, Bio, API token, then Email
		must(t, app.Key("Down"))
	}
	must(t, app.Key("Return")) // edit Email
	must(t, app.Type("not-an-email"))
	must(t, app.Key("Return")) // set Email
	must(t, app.Key("ctrl+Return"))
	_, err = app.WaitFor("Enter an email address", elementWait) // refused, and the Email row is selected again
	must(t, err)
	must(t, app.Key("Return"))                        // edit the rejected address: the caret is after it, so Backspace erases it
	_, err = app.WaitFor("not-an-email", elementWait) // the field shows the address before the first Backspace
	must(t, err)
	for range len("not-an-email") {
		must(t, app.Key("BackSpace"))
	}
	must(t, app.Type("ada@example.test"))
	_, err = app.WaitFor("ada@example.test", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // set Email

	for range 4 { // Website, Seats, Due date, then Accept the terms
		must(t, app.Key("Down"))
	}
	must(t, app.Key("Return")) // check the terms
	must(t, app.Key("ctrl+Return"))
	// The terms row is below the visible rows; reaching the child at all proves the required checkbox passed the host.
	_, err = app.WaitFor("name: Ada Lovelace", elementWait)
	must(t, err)
	_, err = app.WaitFor("email: ada@example.test", elementWait)
	must(t, err)
}

// openRootFixtureRow waits for the fixture's root row, which ranks first for its own title, and opens it.
func openRootFixtureRow(t *testing.T, app *App, label string) {
	t.Helper()
	_, err := app.WaitFor(label, elementWait)
	must(t, err)
	shoot(t, app, "wp34-0-root-"+strings.ReplaceAll(strings.ToLower(label), " ", "-"))
	must(t, app.Key("Return"))
}
