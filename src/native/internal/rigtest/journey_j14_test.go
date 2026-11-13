//go:build rig

package rigtest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// J14 (PLAN.md 3.1): Automate writes an extension with write_extension, which checks it, installs it, and enables it, as
// 1.x does. The fake backend first calls write_extension with a source that does not compile; the tool's error goes back
// to the model in the same turn, the model calls write_extension again with a source that compiles, and the chat answers.
// The new row shows on the root without a review. Editing is TestJourneyJ14EditChangesAnExtensionAndShowsTheNewRow; the
// failure after the model gives up is not covered.

const (
	builderRequest  = "make a clock"
	builderRow      = `Automate "make a clock"`
	builderRootRow  = "Clock journey row"
	builderDoneText = "The clock is installed and on the root."
	builderWait     = 60 * time.Second
	// builderCompileError names api.Itme, which the API does not have: the check must refuse it and say why.
	builderCompileError = "Itme"
	clockManifest       = `{"apiVersion":"1.0","id":"user.clock-journey","title":"Clock journey"}`
)

// badClockSource names api.Itme, which the API does not have.
const badClockSource = "package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\n" +
	"func RootItems(ctx api.Context) ([]api.Itme, error) { return nil, nil }\n"

// clockSource provides one suggestion at the empty root and the same item when its title is searched.
func clockSource(row string) string {
	return "package main\n\nimport (\n\"strings\"\n\"github.com/pablopunk/nvm/native/internal/ext/api\"\n)\n\n" +
		"func RootItems(ctx api.Context) ([]api.Item, error) {\n" +
		"\treturn []api.Item{{ID: \"clock-row\", Title: \"" + row + "\", Subtitle: \"Built by the builder\"}}, nil\n}\n" +
		"func SearchItems(ctx api.Context, query string) ([]api.Item, error) {\n" +
		"\tif strings.Contains(strings.ToLower(\"" + row + "\"), strings.ToLower(query)) { return RootItems(ctx) }\n" +
		"\treturn nil, nil\n}\n"
}

// writeCall is a write_extension call with the manifest and source given, as the model sends them.
func writeCall(t *testing.T, id, manifest, source string) fixtures.ToolCall {
	t.Helper()
	args, err := json.Marshal(map[string]string{"manifest": manifest, "source": source})
	must(t, err)
	return fixtures.ToolCall{ID: id, Name: "write_extension", Arguments: []string{string(args)}}
}

// TestJourneyJ14AutomateBuildsAndRunsAnExtension drives the builder path on the rig: the first write fails to compile,
// the model fixes it in the same turn, and the enabled extension's row shows on the root.
func TestJourneyJ14AutomateBuildsAndRunsAnExtension(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetScript(
		fixtures.ToolCallReply("", writeCall(t, "call_bad", clockManifest, badClockSource)),
		fixtures.ToolCallReply("", writeCall(t, "call_good", clockManifest, clockSource(builderRootRow))),
		fixtures.TextReply(builderDoneText),
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
	shoot(t, app, "j14-1-chat-done")
	// Request 0 asks and is answered with the bad write; request 1 carries that write's error and is answered with the
	// good write; request 2 carries the good write's result and is answered with the text.
	if bodies := chatRequestBodies(fake); len(bodies) != 3 || !strings.Contains(bodies[1], builderCompileError) {
		t.Fatalf("the model did not get the compile error back in the same turn (%d chat requests)", len(bodies))
	}

	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type(builderListQuery))
	waitForRow(t, app, builderRootRow, builderWait)
	shoot(t, app, "j14-2-row-on-root")
}
