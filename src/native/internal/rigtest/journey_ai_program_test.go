//go:build rig

package rigtest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// The AI program journeys drive the built-in AI's run_program tool on the rig. The fake backend plays the model: it
// answers each question with a run_program call whose Go the journey wrote, then with a text that marks the turn's end.
// The program runs in the app's real child, the gate asks in the chat's real approval row, and the journey answers
// with Enter or Ctrl+D as a user does. Each journey then checks the change in the app's own pages.

const (
	programRunWait = 30 * time.Second
	// doubledCause is the text of a refusal that named its cause twice, which the fix in this branch removed.
	doubledCause = "invalid input: invalid input"
)

// programCall is a run_program call carrying code, as the model sends it.
func programCall(t *testing.T, id, code string) fixtures.ToolCall {
	t.Helper()
	args, err := json.Marshal(map[string]string{"code": code})
	must(t, err)
	return fixtures.ToolCall{ID: id, Name: "run_program", Arguments: []string{string(args)}}
}

// programTurn is one scripted turn: the model runs code, then says done once it has the result.
func programTurn(t *testing.T, id, code, done string) []fixtures.ScriptedReply {
	t.Helper()
	return []fixtures.ScriptedReply{fixtures.ToolCallReply("", programCall(t, id, code)), fixtures.TextReply(done)}
}

// scriptOf joins turns into the fake's reply queue.
func scriptOf(turns ...[]fixtures.ScriptedReply) []fixtures.ScriptedReply {
	var replies []fixtures.ScriptedReply
	for _, turn := range turns {
		replies = append(replies, turn...)
	}
	return replies
}

// launchSignedInWithScript starts the fake backend with the script, launches the app on home, signs in and leaves the
// palette hidden.
func launchSignedInWithScript(t *testing.T, home string, script []fixtures.ScriptedReply) (*App, *FakeBackend) {
	t.Helper()
	fake := StartFakeBackend(t)
	fake.SetScript(script...)
	app := Launch(t, Options{Backend: fake.URL, Home: home})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)
	return app, fake
}

// askFirstQuestion summons the palette and asks from the root, which opens the chat.
func askFirstQuestion(t *testing.T, app *App, question string) {
	t.Helper()
	must(t, app.SummonPalette())
	askAIFromRoot(t, app, question)
}

// askFollowUp sends the next question in the open chat.
func askFollowUp(t *testing.T, app *App, question string) {
	t.Helper()
	must(t, app.Type(question))
	must(t, app.Key("Return"))
}

// waitForApproval waits until the chat's approval row asks with question, the detail the gate built from host data.
func waitForApproval(t *testing.T, app *App, question string) {
	t.Helper()
	_, err := app.WaitFor(approvalWaiting, programRunWait)
	must(t, err)
	_, err = app.WaitFor(question, elementWait)
	must(t, err)
}

// allowApproval answers the row with Enter; refuseApproval with Ctrl+D.
func allowApproval(t *testing.T, app *App)  { t.Helper(); must(t, app.Key("Return")) }
func refuseApproval(t *testing.T, app *App) { t.Helper(); must(t, app.Key("ctrl+d")) }

// waitForTurnEnd waits for the scripted text that ends the turn, and checks no approval row is left.
func waitForTurnEnd(t *testing.T, app *App, done string) {
	t.Helper()
	if _, err := app.WaitFor(done, programRunWait); err != nil {
		shoot(t, app, "turn-did-not-end")
		t.Fatal(err)
	}
	if paletteShows(t, app, approvalWaiting) {
		t.Fatalf("an approval row is still waiting after %q", done)
	}
}

// lastProgramResult is the newest chat request the model received: it carries the result of the last program.
func lastProgramResult(t *testing.T, fake *FakeBackend) string {
	t.Helper()
	bodies := chatRequestBodies(fake)
	if len(bodies) == 0 {
		t.Fatal("the fake backend received no chat request")
	}
	return bodies[len(bodies)-1]
}

// modelReceived fails unless the newest chat request holds every text, and checks no refusal named its cause twice.
func modelReceived(t *testing.T, fake *FakeBackend, texts ...string) {
	t.Helper()
	body := lastProgramResult(t, fake)
	for _, text := range texts {
		if !strings.Contains(body, text) {
			t.Fatalf("the model did not receive %q; the last request ends with %q", text, tailOf(body, 600))
		}
	}
	if strings.Contains(body, doubledCause) {
		t.Fatalf("a refusal named its cause twice: %q", tailOf(body, 600))
	}
}

func tailOf(text string, limit int) string {
	if len(text) > limit {
		return text[len(text)-limit:]
	}
	return text
}

// openRootCommand hides the palette, summons it, and opens the root command titled title.
func openRootCommand(t *testing.T, app *App, title, placeholder string) {
	t.Helper()
	hidePalette(t, app)
	must(t, app.SummonPalette())
	must(t, app.Type(title))
	waitForRow(t, app, title, elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor(placeholder, elementWait)
	must(t, err)
}
