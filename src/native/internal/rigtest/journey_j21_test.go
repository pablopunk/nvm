//go:build rig

package rigtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// J21 (PLAN.md 3.1): Fix Selected Text with AI runs from the palette over the app the selection is in. The rig has no
// third-party text editor, so the source is a floating note: its text is selected, the palette's command copies the
// selection with the real X11 focus and xdotool path, the proofread streams through the fake backend, the result page
// offers the correction, and Enter replaces the selection in the note and restores the clipboard. Not covered: a
// third-party source app, the triage and classify commands, the indicator's frames, and the failure copy.

const (
	j21Input     = "this are selected text"
	j21Corrected = "This is selected text."
	j21Clipboard = "rig clipboard kept by the fix"
	j21AIWait    = 15 * time.Second
)

// TestJourneyJ21FixSelectedTextReplacesTheSelection is the J21 smoke described above.
func TestJourneyJ21FixSelectedTextReplacesTheSelection(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetReply(j21Corrected)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)
	must(t, app.ClipboardSet(j21Clipboard))

	createNoteWith(t, app, j21Input)
	display := os.Getenv("DISPLAY")
	note := waitForWindowID(t, display, j21Input)
	must(t, activateWindow(display, note))
	must(t, app.Key("ctrl+a"))

	must(t, app.SummonPalette())
	must(t, app.Type("Fix Selected Text"))
	_, err := app.WaitFor("Fix Selected Text with AI", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor(j21Corrected, j21AIWait)
	must(t, err)
	shoot(t, app, "j21-1-result")

	must(t, app.Key("Return"))
	waitNoteOnDisk(t, app, j21Corrected)
	waitClipboardEquals(t, app, j21Clipboard)
	shoot(t, app, "j21-2-replaced")
}

// createNoteWith types text into a new Quick Note, then leaves the editor for the list, where the newest note is the row
// after New Note. ctrl+f opens that note in its floating window.
func createNoteWith(t *testing.T, app *App, text string) {
	t.Helper()
	must(t, app.SummonPalette())
	must(t, app.Type("Floating Notes"))
	_, err := app.WaitFor("Floating Notes", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search Floating Notes", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Untitled note", elementWait)
	must(t, err)
	must(t, app.Type(text))
	_, err = app.WaitFor(text, elementWait)
	must(t, err)
	time.Sleep(autosaveSettle)
	must(t, app.Key("Escape"))
	must(t, app.Key("Down"))
	must(t, app.Key("ctrl+f"))
}

// activateWindow gives the keyboard to window id on display.
func activateWindow(display, id string) error {
	_, err := xdotoolOn(display, "windowactivate", "--sync", id)
	return err
}

// waitNoteOnDisk polls the notes file under the app's home until it holds want, failing after j21AIWait. The note is
// the only thing the rig can read after the paste; the palette cannot see a floating window.
func waitNoteOnDisk(t *testing.T, app *App, want string) {
	t.Helper()
	deadline := time.Now().Add(j21AIWait)
	for {
		if noteFileHolds(app.Home(), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no note on disk holds %q; app log:\n%s", want, tail(app.Log(), 20))
		}
		time.Sleep(pollInterval)
	}
}

// noteFileHolds reports whether a notes.json under home holds text in one of its notes.
func noteFileHolds(home, text string) bool {
	found := false
	_ = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil || found || info.IsDir() || info.Name() != "notes.json" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		found = strings.Contains(string(data), text)
		return nil
	})
	return found
}
