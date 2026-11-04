//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// J14 (PLAN.md 3.1): Automate writes an extension, checks it, and installs it off. The fake backend first answers with a
// source that does not compile; the host sends the compile error back, the model's second reply compiles, and the
// chat says the extension is installed but off. The user presses Control+E, turns it on in the review, and the
// extension's root row shows on the root. Editing is TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow; the
// failure path after the retries is not covered.

const (
	builderRequest     = "make a clock"
	builderRow         = `Automate "make a clock"`
	builderNotice      = "installed but off"
	builderRetryNotice = "Asking the model to fix it"
	builderReviewTitle = "Review Clock journey"
	builderRootRow     = "Clock journey row"
	builderWait        = 60 * time.Second
)

// badSourceReply names api.Itme, which the API does not have: the compile check must refuse it.
var badSourceReply = "I will write a clock.\n\n```json\n" +
	`{"apiVersion":"1.0","id":"user.clock-journey","title":"Clock journey"}` + "\n```\n\n```go\n" +
	"package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\n" +
	"func RootItems(ctx api.Context) ([]api.Itme, error) { return nil, nil }\n```\n"

// goodSourceReply is the fixed extension: one root row.
var goodSourceReply = "The compile error was an unknown name. Here is the fixed extension.\n\n```json\n" +
	`{"apiVersion":"1.0","id":"user.clock-journey","title":"Clock journey"}` + "\n```\n\n```go\n" +
	"package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\n" +
	"func RootItems(ctx api.Context) ([]api.Item, error) {\n" +
	"\treturn []api.Item{{ID: \"clock-row\", Title: \"" + builderRootRow + "\", Subtitle: \"Built by the builder\"}}, nil\n}\n```\n"

// TestJourneyJ14AutomateBuildsReviewsEnablesAndRunsAnExtension drives the whole builder path on the rig.
func TestJourneyJ14AutomateBuildsReviewsEnablesAndRunsAnExtension(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.QueueReplies(badSourceReply, goodSourceReply)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type(builderRequest))
	waitSearchValue(t, app, builderRequest)
	waitForRow(t, app, builderRow, elementWait)
	must(t, app.Key("Return"))

	_, err := app.WaitFor(builderRetryNotice, builderWait)
	must(t, err)
	shoot(t, app, "j14-1-retry")

	_, err = app.WaitFor(builderNotice, builderWait)
	must(t, err)
	shoot(t, app, "j14-2-installed-off")
	assertNoRow(t, app, builderRootRow)

	must(t, app.Key("ctrl+e"))
	_, err = app.WaitFor(builderReviewTitle, elementWait)
	must(t, err)
	shoot(t, app, "j14-3-review")
	must(t, app.Key("Return"))

	if !waitUntilPaletteLacks(t, app, builderReviewTitle, elementWait) {
		t.Fatal("Return on Enable did not leave the review")
	}
	must(t, app.Key("Escape"))
	waitForRow(t, app, builderRootRow, builderWait)
	shoot(t, app, "j14-4-row-on-root")
}
