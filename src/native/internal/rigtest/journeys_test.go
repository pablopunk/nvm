//go:build rig

package rigtest

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The journeys run against the real app under the rig (Xvfb, openbox, a tray host and dbus). They
// are smoke versions of PLAN.md section 3.1: each one checks the pass condition the rig can see, and
// its test name and comment say what is not covered yet.

const (
	elementWait = 5 * time.Second
	clipWait    = 10 * time.Second // the clipboard watcher polls every 800 ms
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// shoot saves the palette window and the whole display for the test's artifacts. A failed capture
// fails the test: the screenshots are part of the evidence.
func shoot(t *testing.T, app *App, name string) {
	t.Helper()
	palette, err := app.CapturePalette(name)
	if err != nil {
		t.Errorf("palette screenshot %s: %v", name, err)
	} else {
		t.Logf("screenshot %s", palette)
	}
	desktop, err := app.Screenshot(name)
	if err != nil {
		t.Errorf("desktop screenshot %s: %v", name, err)
	} else {
		t.Logf("screenshot %s", desktop)
	}
}

// waitSearchValue waits until the palette's search field reads want.
func waitSearchValue(t *testing.T, app *App, want string) {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	var got string
	var err error
	for {
		got, err = app.SearchValue()
		if err == nil && got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("search field = %q (err %v), want %q", got, err, want)
		}
		time.Sleep(pollInterval)
	}
}

// TestJourneyJ01SummonAndDismiss is J01: the hotkey shows the palette, Escape hides it, the hotkey
// shows it again with the query reset, and a click on the desktop hides it.
// Not covered by the rig yet: focus returns to the previous app, the display under the cursor,
// full-screen apps (PLAN.md J01, J26).
func TestJourneyJ01SummonAndDismiss(t *testing.T) {
	app := Launch(t, Options{})

	started := time.Now()
	must(t, app.SummonPalette())
	t.Logf("summon took %s", time.Since(started).Round(time.Millisecond))
	_, err := app.WaitFor("Clipboard History", elementWait)
	must(t, err)
	shoot(t, app, "j01-1-summoned")

	must(t, app.DismissWithEscape())
	shoot(t, app, "j01-2-escape-hides")

	must(t, app.SummonPalette())
	shoot(t, app, "j01-3-hotkey-again")

	must(t, app.Type("zz"))
	waitSearchValue(t, app, "zz")
	must(t, app.ClickAway())
	shoot(t, app, "j01-4-click-away-hides")

	must(t, app.SummonPalette())
	waitSearchValue(t, app, "")
	shoot(t, app, "j01-5-reset-on-summon")
	must(t, app.DismissWithEscape())
}

// TestJourneyJ02SearchAndOpenClipboardHistory is the J02 smoke: typing "cl" shows the Clipboard
// History row, and Enter opens it. The frecency part of J02 (the intended app becomes the top hit
// after three launches) is not covered yet.
func TestJourneyJ02SearchAndOpenClipboardHistory(t *testing.T) {
	app := Launch(t, Options{})
	must(t, app.SummonPalette())

	must(t, app.Type("cl"))
	_, err := app.WaitFor("Clipboard History", elementWait)
	must(t, err)
	shoot(t, app, "j02-1-typed-cl")

	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search Clipboard History", elementWait)
	must(t, err)
	shoot(t, app, "j02-2-clipboard-opened")
}

// TestJourneyJ05ClipboardTextAppearsInHistory is the J05 smoke: text set on the clipboard with
// xclip shows up in Clipboard History. The paste into the frontmost app, pinning, deleting and the
// restart durability of J05 are not covered yet.
// concealedSettle is how long J06 waits after a concealed copy, so the watcher (which polls about once a second
// while the clipboard is idle) has seen it before the next copy replaces it. This is the one wait in the rig
// journeys, because the rig has no event that says the watcher has polled.
const concealedSettle = 2 * time.Second

// TestJourneyJ06ConcealedCopyNeverAppearsInHistory copies a password the way a password manager does (the KDE
// concealed hint on X11) and checks that Clipboard History never shows it: the view is opened, searched for the
// text, and shows its empty state. The app log must not hold the text either. A normal copy made afterwards does
// appear, so the empty state is not a broken history. Smoke version of PLAN.md J06: the file on disk and the
// toasts are not inspected here.
func TestJourneyJ06ConcealedCopyNeverAppearsInHistory(t *testing.T) {
	app := Launch(t, Options{})
	const secret = "canary-secret-123"
	must(t, app.ClipboardSetConcealed(secret))
	time.Sleep(concealedSettle)
	nonce := fmt.Sprintf("rig-clip-%d", time.Now().UnixNano())
	must(t, app.ClipboardSet(nonce))

	must(t, app.SummonPalette())
	must(t, app.Type("clipboard"))
	_, err := app.WaitFor("Clipboard History", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	must(t, app.Type("canary"))
	_, err = app.WaitFor("No clipboard items found.", elementWait)
	must(t, err)
	shoot(t, app, "j06-1-concealed-absent")
	if strings.Contains(app.Log(), secret) {
		t.Fatal("the app log holds the concealed text")
	}

	must(t, app.Key("Escape"))
	_, err = app.WaitFor(nonce, clipWait)
	must(t, err)
	shoot(t, app, "j06-2-normal-copy-present")
}

func TestJourneyJ05ClipboardTextAppearsInHistory(t *testing.T) {
	app := Launch(t, Options{})
	nonce := fmt.Sprintf("rig-clip-%d", time.Now().UnixNano())
	must(t, app.ClipboardSet(nonce))
	got, err := app.ClipboardGet()
	must(t, err)
	if got != nonce {
		t.Fatalf("xclip did not own the clipboard: read %q, want %q", got, nonce)
	}

	must(t, app.SummonPalette())
	must(t, app.Type("clipboard"))
	_, err = app.WaitFor("Clipboard History", elementWait)
	must(t, err)
	must(t, app.Key("Return"))

	_, err = app.WaitFor(nonce, clipWait)
	must(t, err)
	shoot(t, app, "j05-1-clipboard-history")
}
