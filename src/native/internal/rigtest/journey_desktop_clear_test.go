//go:build rig

package rigtest

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// historyEntriesLabel reads the palette row that counts clipboard history entries, after the palette is back at root.
func historyEntriesLabel(t *testing.T, app *App) string {
	t.Helper()
	clearSearch(t, app)
	must(t, app.Type("history"))
	return waitForRowPrefix(t, app, "history entries=", elementWait)
}

// waitForHistoryCopy reads the history count until the clipboard watcher has recorded a copy, or the wait ends.
func waitForHistoryCopy(t *testing.T, app *App) string {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	for {
		label := historyEntriesLabel(t, app)
		if !strings.HasSuffix(label, "entries=0") || time.Now().After(deadline) {
			return label
		}
		time.Sleep(pollInterval)
	}
}

// runFixtureClear types the clear row's query and runs it with Return. The confirmation view then shows the host's
// question, and the caller answers it.
func runFixtureClear(t *testing.T, app *App) {
	t.Helper()
	clearSearch(t, app)
	must(t, app.Type("fixture clear"))
	waitForRow(t, app, "Fixture clear history", elementWait)
	must(t, app.Key("Return"))
}

// TestJourneyDesktopClearNeedsTheUsersAnswer clears clipboard history from the desktop fixture twice. Cancel leaves the
// history as it was. Confirm clears it. The fixture never asks for confirmation itself, so the palette question is the
// host's.
func TestJourneyDesktopClearNeedsTheUsersAnswer(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.desktop")
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	clearSearch(t, app)
	enableThroughReview(t, app, "desktop", "Desktop fixture")
	hidePalette(t, app)

	must(t, app.ClipboardSet(uniqueClipboardText("clear probe entry")))
	must(t, app.SummonPalette())
	before := waitForHistoryCopy(t, app)
	if strings.HasSuffix(before, "entries=0") {
		t.Fatalf("the clipboard copy did not reach history: %q", before)
	}

	runFixtureClear(t, app)
	waitForRow(t, app, "Clear clipboard history", elementWait)
	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	if after := historyEntriesLabel(t, app); after != before {
		t.Fatalf("cancelling the clear changed history: %q, was %q", after, before)
	}

	runFixtureClear(t, app)
	waitForRow(t, app, "Clear clipboard history", elementWait)
	must(t, app.Key("Return"))
	// The confirmed run finishes off the UI thread and then dismisses the palette; summoning before that hide lands
	// would find the palette still shown, skip the press, and then lose it to the hide.
	if _, err := app.WaitPalette(elementWait, "hidden after the confirmed run", isHidden); err != nil {
		t.Fatal(err)
	}
	must(t, app.SummonPalette())
	if cleared := historyEntriesLabel(t, app); cleared != "history entries=0" {
		t.Fatalf("confirming the clear left %q, want history entries=0", cleared)
	}
	shoot(t, app, "desktop-clear-confirmed")
}

// TestJourneyDesktopClearEscapeCancelsAtOnce presses Escape on the host's question. The call must be refused at once:
// the history is unchanged, and a second clear shows its question within the usual wait, which it could not do if the
// first call still held the extension for the confirmation timeout.
func TestJourneyDesktopClearEscapeCancelsAtOnce(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.desktop")
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	clearSearch(t, app)
	enableThroughReview(t, app, "desktop", "Desktop fixture")
	hidePalette(t, app)

	must(t, app.ClipboardSet(uniqueClipboardText("escape probe entry")))
	must(t, app.SummonPalette())
	before := waitForHistoryCopy(t, app)
	if strings.HasSuffix(before, "entries=0") {
		t.Fatalf("the clipboard copy did not reach history: %q", before)
	}

	runFixtureClear(t, app)
	waitForRow(t, app, "Clear clipboard history", elementWait)
	must(t, app.Key("Escape"))
	if after := historyEntriesLabel(t, app); after != before {
		t.Fatalf("Escape on the question changed history: %q, was %q", after, before)
	}

	runFixtureClear(t, app)
	waitForRow(t, app, "Clear clipboard history", elementWait)
	must(t, app.Key("Escape"))
	if after := historyEntriesLabel(t, app); after != before {
		t.Fatalf("the second Escape changed history: %q, was %q", after, before)
	}
}

// uniqueClipboardText makes a copy the clipboard watcher sees as new. The rig's X clipboard outlives a launch, so the
// same text copied again (a rerun, or an earlier journey) is not a change and never reaches history.
func uniqueClipboardText(prefix string) string {
	return prefix + " " + strconv.FormatInt(time.Now().UnixNano(), 36)
}
