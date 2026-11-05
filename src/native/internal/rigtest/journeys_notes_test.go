//go:build rig

package rigtest

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The notes and snippets journeys (PLAN.md J09, J10, J30) against the real app under the rig. Like the other
// journeys they are smokes: each checks the pass condition the rig can see and says what it leaves out.

// autosaveSettle is how long the rig waits after typing before it kills the app: the editor autosaves 400 ms after
// the last keystroke, so this is past that and the note is on disk. The draft gap (kill between the draft and the
// note write) is covered by the notes package's crash tests, not by the rig.
const autosaveSettle = 1500 * time.Millisecond

// waitClipboard polls the clipboard until it holds want, failing the test after clipWait.
func waitClipboardEquals(t *testing.T, app *App, want string) {
	t.Helper()
	deadline := time.Now().Add(clipWait)
	var got string
	var err error
	for {
		got, err = app.ClipboardGet()
		if err == nil && got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("clipboard = %q (err %v), want %q", got, err, want)
		}
		time.Sleep(pollInterval)
	}
}

// TestJourneyJ09SnippetCreateFindAndCopy is the J09 smoke: a snippet is created through the snippets form (name,
// keyword, content), found at the root by its keyword, and Enter puts its text on the clipboard. The rig has no
// paste gesture into another app, so the snippet's Copy to Clipboard row is what is asserted.
// Editing and deleting, and the paste into another app, are not covered yet.
func TestJourneyJ09SnippetCreateFindAndCopy(t *testing.T) {
	app := Launch(t, Options{})
	nonce := fmt.Sprintf("Rig text %d", time.Now().UnixNano())
	name := "Rig signature"

	must(t, app.SummonPalette())
	must(t, app.Type("Snippets"))
	_, err := app.WaitFor("Snippets", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search Snippets", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // the first row creates a snippet
	_, err = app.WaitFor("Create Snippet", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // edit Name, the first row: the field is typed in the search field
	must(t, app.Type(name))
	_, err = app.WaitFor(name, elementWait)
	must(t, err)
	must(t, app.Key("Return")) // set Name
	must(t, app.Key("Down"))   // Keyword
	must(t, app.Key("Return"))
	must(t, app.Type(";rig"))
	_, err = app.WaitFor(";rig", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // set Keyword
	must(t, app.Key("Down"))   // Content opens the text editor
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Save   esc Back", elementWait)
	must(t, err)
	must(t, app.Type(nonce))
	_, err = app.WaitFor(nonce, elementWait)
	must(t, err)
	must(t, app.Key("ctrl+Return")) // the editor's save returns to the form
	_, err = app.WaitFor("Create Snippet", elementWait)
	must(t, err)
	must(t, app.Key("ctrl+Return")) // save: the form closes back to the list
	_, err = app.WaitFor(name, elementWait)
	must(t, err)
	shoot(t, app, "j09-1-created")

	must(t, app.Key("Escape")) // the list back to the root, with the query empty
	must(t, app.Type(";rig"))
	_, err = app.WaitFor(name, elementWait)
	must(t, err)
	shoot(t, app, "j09-2-found-by-keyword")

	// Enter pastes where the platform can send the paste gesture, and the rig's X11 has xdotool, so the copy is the
	// menu's Copy to Clipboard row, the second action after Paste.
	must(t, app.Key(actionMenuKey))
	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	waitClipboardEquals(t, app, nonce)
}

// TestJourneyJ10NoteSurvivesKillAndRelaunch is the J10 and J30 smoke: a note is typed, the app is killed with
// SIGKILL while it is open, and the next launch over the same home finds the note with its text. The second launch
// also takes over the store's lease from the dead process, which is part of the same pass condition. The draft gap
// and the quarantine notice are covered by unit tests.
func TestJourneyJ10NoteSurvivesKillAndRelaunch(t *testing.T) {
	home := t.TempDir()
	app := Launch(t, Options{Home: home, Run: "first"})
	nonce := fmt.Sprintf("Rig note %d", time.Now().UnixNano())

	must(t, app.SummonPalette())
	must(t, app.Type("Floating Notes"))
	_, err := app.WaitFor("Floating Notes", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Find a note", elementWait)
	must(t, err)
	must(t, app.Key("Return")) // the first row starts a note
	_, err = app.WaitFor("Untitled note", elementWait)
	must(t, err)
	must(t, app.Type(nonce))
	_, err = app.WaitFor(nonce, elementWait)
	must(t, err)
	time.Sleep(autosaveSettle)
	shoot(t, app, "j10-1-typed")

	must(t, app.KillHard())
	if strings.Contains(app.Log(), "panic") {
		t.Fatalf("the app panicked before the kill:\n%s", tail(app.Log(), 20))
	}

	relaunched := Launch(t, Options{Home: home, Run: "relaunch"})
	must(t, relaunched.SummonPalette())
	must(t, relaunched.Type("Floating Notes"))
	_, err = relaunched.WaitFor("Floating Notes", elementWait)
	must(t, err)
	must(t, relaunched.Key("Return"))
	_, err = relaunched.WaitFor("Find a note", elementWait)
	must(t, err)
	must(t, relaunched.Key("Down")) // the newest note is the row after "New Note"
	must(t, relaunched.Key("Return"))
	_, err = relaunched.WaitFor(nonce, elementWait)
	must(t, err)
	shoot(t, relaunched, "j10-2-reopened")
	if strings.Contains(relaunched.Log(), "quarantin") {
		t.Fatalf("the relaunch set a notes file aside:\n%s", tail(relaunched.Log(), 20))
	}
}
