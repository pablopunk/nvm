//go:build rig

package rigtest

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// approvalWaiting is the approval row's copy (aichat's awaiting line) on this platform: the refuse key is the palette's
// modifier with D.
var approvalWaiting = "Waiting for you. Enter allows it, " + palette.KeyLabel("⌃D") + " refuses it."

// The words the scripted model answers with.
const (
	approvalClipboardText  = "fixture clipboard text"
	approvalAllowedAnswer  = "ALLOWED-MARK"
	approvalRefusedAnswer  = "REFUSED-MARK"
	approvalRefusedResult  = "The user did not allow this tool call."
	approvalFirstQuestion  = "Read my clipboard"
	approvalSecondQuestion = "Read it once more"
)

// chatRequestBodies returns the bodies of the chat requests the fake received, oldest first.
func chatRequestBodies(fake *FakeBackend) []string {
	var bodies []string
	for _, request := range fake.Requests() {
		if request.Path == fixtures.ChatCompletionsPath {
			bodies = append(bodies, request.Body)
		}
	}
	return bodies
}

// TestJourneyJ32ApprovalRowAllowsWithEnterAndRefusesWithCtrlD is J32: a tool that asks for approval waits in the chat
// with its keys shown. Enter allows the read, and the clipboard text reaches the model's next request. Ctrl+D (⌘D on
// macOS) refuses the next read, and the model's next request carries the refusal instead. The model is scripted
// with the fake backend's reply queue. list_folder never asks, so the tool is read_clipboard, which does.
func TestJourneyJ32ApprovalRowAllowsWithEnterAndRefusesWithCtrlD(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetScript(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "call_allow", Name: "read_clipboard", Arguments: []string{"{}"}}),
		fixtures.TextReply(approvalAllowedAnswer),
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "call_refuse", Name: "read_clipboard", Arguments: []string{"{}"}}),
		fixtures.TextReply(approvalRefusedAnswer),
	)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	must(t, app.ClipboardSet(approvalClipboardText))
	hidePalette(t, app)
	must(t, app.SummonPalette())

	askAIFromRoot(t, app, approvalFirstQuestion)
	_, err := app.WaitFor(approvalWaiting, streamWait)
	must(t, err)
	shoot(t, app, "j32-1-awaiting")
	must(t, app.Key("Return"))
	_, err = app.WaitFor(approvalAllowedAnswer, streamWait)
	must(t, err)
	if bodies := chatRequestBodies(fake); len(bodies) < 2 || !strings.Contains(bodies[1], approvalClipboardText) {
		t.Fatalf("after Enter the model did not receive the clipboard text (%d chat requests)", len(bodies))
	}
	shoot(t, app, "j32-2-allowed")

	must(t, app.Type(approvalSecondQuestion))
	must(t, app.Key("Return"))
	_, err = app.WaitFor(approvalWaiting, streamWait)
	must(t, err)
	must(t, app.Key("ctrl+d"))
	_, err = app.WaitFor(approvalRefusedAnswer, streamWait)
	must(t, err)
	if bodies := chatRequestBodies(fake); len(bodies) < 4 || !strings.Contains(bodies[3], approvalRefusedResult) {
		t.Fatalf("after ⌘D the model did not receive the refusal (%d chat requests)", len(bodies))
	}
	shoot(t, app, "j32-3-refused")
}
