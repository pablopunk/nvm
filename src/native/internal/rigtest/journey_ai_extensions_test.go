//go:build rig

package rigtest

import (
	"strings"
	"testing"
	"time"
)

const (
	aiExtensionID       = "acme.views"
	aiExtensionTitle    = "View fixtures"
	aiExtensionQuestion = "Turn off the extension “" + aiExtensionTitle + "”"
	aiExtensionRefused  = "EXTENSION-REFUSED-MARK"
	aiExtensionOff      = "EXTENSION-OFF-MARK"
	aiExtensionReview   = "EXTENSION-REVIEW-MARK"
	disableViewFixtures = `return nil, ctx.Extensions().Disable("` + aiExtensionID + `")`
	reviewViewFixtures  = `return nil, ctx.Extensions().OpenReview("` + aiExtensionID + `")`
	extensionRowSettle  = time.Second
)

// TestJourneyAIProgramTurnsAnExtensionOffAndOnlyOpensItsReview (#328): with View fixtures enabled by the user, the AI
// turns it off. Disable is a Setting call: Ctrl+D refuses the first try and Enter allows the second, after which the
// Extensions list shows it Disabled and its command is gone from the root. A second chat asks to turn it back on:
// OpenReview only shows the review, which the user cancels, and the extension stays Disabled.
func TestJourneyAIProgramTurnsAnExtensionOffAndOnlyOpensItsReview(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, aiExtensionID)
	app, fake := launchSignedInWithScript(t, home, scriptOf(
		programTurn(t, "call_disable_refused", disableViewFixtures, aiExtensionRefused),
		programTurn(t, "call_disable_allowed", disableViewFixtures, aiExtensionOff),
		programTurn(t, "call_open_review", reviewViewFixtures, aiExtensionReview),
	))
	openExtensionsView(t, app)
	enableThroughReview(t, app, "view fixtures", aiExtensionTitle)
	_, err := app.WaitFor("Running", elementWait)
	must(t, err)
	hidePalette(t, app)

	askFirstQuestion(t, app, "Turn off View fixtures")
	waitForApproval(t, app, aiExtensionQuestion)
	shoot(t, app, "ai-extension-1-asks")
	refuseApproval(t, app)
	waitForTurnEnd(t, app, aiExtensionRefused)
	modelReceived(t, fake, "the user declined")

	askFollowUp(t, app, "Try again, I will allow it")
	waitForApproval(t, app, aiExtensionQuestion)
	allowApproval(t, app)
	waitForTurnEnd(t, app, aiExtensionOff)
	modelReceived(t, fake, "turned off the extension")
	assertExtensionDisabled(t, app)
	shoot(t, app, "ai-extension-2-list-shows-disabled")

	hidePalette(t, app)
	askFirstQuestion(t, app, "Turn View fixtures back on")
	_, err = app.WaitFor("Review "+aiExtensionTitle, programRunWait)
	must(t, err)
	waitForRow(t, app, "Enable", elementWait)
	shoot(t, app, "ai-extension-3-review-opened-not-enabled")
	waitForModelRequest(t, fake, aiExtensionReview, "stays off until the user enables it")
	modelReceived(t, fake, "stays off until the user enables it")
	assertExtensionDisabled(t, app)

	searchRootFor(t, app, "list fixture")
	time.Sleep(extensionRowSettle)
	assertNoRow(t, app, "List fixture")
	shoot(t, app, "ai-extension-4-command-gone-from-root")
}

// assertExtensionDisabled opens the Extensions list and checks View fixtures reads Disabled.
func assertExtensionDisabled(t *testing.T, app *App) {
	t.Helper()
	leaveTheChat(t, app)
	openExtensionsView(t, app)
	chooseExtensionRow(t, app, "view fixtures", aiExtensionTitle)
	_, err := app.WaitFor("Disabled", elementWait)
	must(t, err)
}

// waitForModelRequest waits until the fake backend has received a chat request holding text, for a turn whose page the
// palette does not show, such as one behind the review the program opened. why names the turn in the failure.
func waitForModelRequest(t *testing.T, fake *FakeBackend, why, text string) {
	t.Helper()
	deadline := time.Now().Add(programRunWait)
	for !strings.Contains(lastProgramResult(t, fake), text) {
		if time.Now().After(deadline) {
			t.Fatalf("%s: the model never received %q", why, text)
		}
		time.Sleep(pollInterval)
	}
}

// leaveTheChat leaves the palette hidden with the root as the page the next summon shows. A chat the palette returned
// to from a review stays the page a summon restores, so Escape leaves it before the palette hides.
func leaveTheChat(t *testing.T, app *App) {
	t.Helper()
	hidePalette(t, app)
	must(t, app.SummonPalette())
	for range escapePresses {
		if !paletteShows(t, app, chatComposerPlaceholder) {
			break
		}
		must(t, app.Key("Escape"))
		time.Sleep(pollInterval)
		if visible, err := app.PaletteVisible(); err == nil && !visible {
			must(t, app.SummonPalette())
		}
	}
	hidePalette(t, app)
}

// chatComposerPlaceholder is the chat page's composer, which the root does not have.
const chatComposerPlaceholder = "Message AI"
