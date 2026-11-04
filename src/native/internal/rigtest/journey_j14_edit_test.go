//go:build rig

package rigtest

import (
	"testing"
	"time"
)

const (
	builderEditedRow = "Clock journey edited row"
	builderEditWords = "say edited instead"
	builderChatHint  = "review extension"
	builderListQuery = "clock journey"
	builderTitle     = "Clock journey"
)

// editedSourceReply is the same extension with its row retitled: the edit keeps the id, so the chat stays bound.
var editedSourceReply = "Here is the edited extension.\n\n```json\n" +
	`{"apiVersion":"1.0","id":"user.clock-journey","title":"Clock journey"}` + "\n```\n\n```go\n" +
	"package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\n" +
	"func RootItems(ctx api.Context) ([]api.Item, error) {\n" +
	"\treturn []api.Item{{ID: \"clock-row\", Title: \"" + builderEditedRow + "\", Subtitle: \"Edited by the builder\"}}, nil\n}\n```\n"

// TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow is the edit path of WP3.9: Edit with AI on a builder's
// extension opens a chat bound to it, the reply is installed off, the old row stays hidden until the review turns
// the new version on, and then the new row shows.
func TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.QueueReplies(goodSourceReply, editedSourceReply)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type(builderRequest))
	waitSearchValue(t, app, builderRequest)
	waitForRow(t, app, builderRow, elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor(builderNotice, builderWait)
	must(t, err)
	must(t, app.Key("ctrl+e"))
	_, err = app.WaitFor(builderReviewTitle, elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	if !waitUntilPaletteLacks(t, app, builderReviewTitle, elementWait) {
		t.Fatal("Return on Enable did not leave the review")
	}
	must(t, app.Key("Escape"))
	waitForRow(t, app, builderRootRow, builderWait)
	shoot(t, app, "j14e-1-created-and-enabled")

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
	shoot(t, app, "j14e-2-edit-chat")

	must(t, app.Type(builderEditWords))
	waitSearchValue(t, app, builderEditWords)
	must(t, app.Key("Return"))
	_, err = app.WaitFor(builderNotice, builderWait)
	must(t, err)
	shoot(t, app, "j14e-3-edited-installed-off")

	hidePalette(t, app)
	must(t, app.SummonPalette())
	time.Sleep(disabledSettle)
	assertNoRow(t, app, builderRootRow)
	assertNoRow(t, app, builderEditedRow)
	hidePalette(t, app)

	openExtensionsView(t, app)
	enableThroughReview(t, app, builderListQuery, builderTitle)
	if !waitUntilPaletteLacks(t, app, "Review "+builderTitle, elementWait) {
		t.Fatal("Return on Enable did not leave the edited review")
	}
	hidePalette(t, app)
	must(t, app.SummonPalette())
	waitForRow(t, app, builderEditedRow, builderWait)
	assertNoRow(t, app, builderRootRow)
	shoot(t, app, "j14e-4-new-row-on-root")
}
