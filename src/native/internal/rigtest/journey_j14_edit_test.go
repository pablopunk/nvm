//go:build rig

package rigtest

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

const (
	builderEditedRow = "Clock journey edited row"
	builderEditWords = "say edited instead"
	builderChatHint  = builderDoneText // the extension's own chat reopens, with its transcript
	builderListQuery = "clock journey"
	builderTitle     = "Clock journey"
	builderEditDone  = "The edited clock is on the root."
)

// TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow is the edit path of WP3.9: Edit with AI on a builder's extension
// opens a chat bound to it, the model writes the same id again, and the edited row replaces the old one on the root,
// enabled, as the write installs and enables it.
func TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetScript(
		fixtures.ToolCallReply("", writeCall(t, "call_create", clockManifest, clockSource(builderRootRow))),
		fixtures.TextReply(builderDoneText),
		fixtures.ToolCallReply("", writeCall(t, "call_edit", clockManifest, clockSource(builderEditedRow))),
		fixtures.TextReply(builderEditDone),
	)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type(builderRequest))
	waitSearchValue(t, app, builderRequest)
	waitForRow(t, app, builderRow, elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor(builderDoneText, builderWait)
	must(t, err)
	hidePalette(t, app)

	openExtensionsView(t, app)
	chooseExtensionRow(t, app, builderListQuery, builderTitle)
	must(t, app.Key("ctrl+k"))
	_, err = app.WaitFor("Edit with AI", elementWait)
	must(t, err)
	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor(builderChatHint, elementWait)
	must(t, err)
	shoot(t, app, "j14e-1-edit-chat")

	must(t, app.Type(builderEditWords))
	_, err = app.WaitFor(builderEditWords, elementWait) // the chat's composer holds the words, so Enter sends them
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor(builderEditDone, builderWait)
	must(t, err)
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type(builderListQuery))
	waitForRow(t, app, builderEditedRow, builderWait)
	assertNoRow(t, app, builderRootRow)
	shoot(t, app, "j14e-2-edited-row-on-root")
}
