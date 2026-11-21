//go:build rig

package rigtest

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

const (
	completedQuestion = "Say done"
	completedReply    = "A short finished reply, MARK-DONE."
	completedWord     = "MARK-DONE"
	refusedQuestion   = "Say no"
)

// J13 (PLAN.md 3.1): Tab asks AI and the reply streams into the chat. Leaving the chat with Escape does not abandon the
// reply: 1.x only aborts a reply from its Stop button, so the fake backend sees the one stream run to its end (completed,
// never abandoned) and the palette stays open. A second Escape, with nothing in flight, goes back to the root as it does
// on any page. The same holds after a reply that completes and after a turn the backend refuses with 402 (Credits
// needed). Continuing the conversation later and the first-token progress are not covered yet.
func TestJourneyJ13AskAIStreamsAndEscapeLeavesTheReplyRunning(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetReply(streamedReply)
	fake.SetChunkDelay(streamFrameDelay)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)
	must(t, app.SummonPalette())

	askAIFromRoot(t, app, aiQuestion)
	_, err := app.WaitFor(streamedFirstWord, streamWait)
	must(t, err)
	shoot(t, app, "j13-1-streaming")

	must(t, app.Key("Escape"))
	requireChatLeft(t, app)
	waitForCompletedStreams(t, fake, 1, streamWait)
	if _, abandoned := fake.StreamOutcomes(); abandoned != 0 {
		t.Fatalf("Escape abandoned the reply: %d abandoned streams; 1.x keeps the reply running until its Stop button", abandoned)
	}
	shoot(t, app, "j13-2-left-while-streaming")

	fake.SetChunkDelay(0)
	fake.SetReply(completedReply)
	askAIFromRoot(t, app, completedQuestion)
	_, err = app.WaitFor(completedWord, streamWait)
	must(t, err)
	waitForCompletedStreams(t, fake, 2, streamWait)
	escapeChatToRoot(t, app)

	fake.SetMode(fixtures.ModeInsufficientCredits)
	askAIFromRoot(t, app, refusedQuestion)
	_, err = app.WaitFor(creditsNoticeTitle, elementWait)
	must(t, err)
	escapeChatToRoot(t, app)
	shoot(t, app, "j13-4-refused-escape-back-to-root")

	must(t, app.Type("settings"))
	waitForRow(t, app, settingsRootRow, elementWait)
}

// escapeChatToRoot presses Escape on a chat page with nothing in flight and checks the page is gone and the palette is
// still shown on the root.
func escapeChatToRoot(t *testing.T, app *App) {
	t.Helper()
	must(t, app.Key("Escape"))
	requireChatLeft(t, app)
}

// requireChatLeft checks that the chat page is gone and the palette is still shown on the root.
func requireChatLeft(t *testing.T, app *App) {
	t.Helper()
	if !waitUntilPaletteLacks(t, app, "Message AI", elementWait) {
		t.Fatal("Escape on the chat did not leave it: the composer is still shown")
	}
	visible, err := app.PaletteVisible()
	must(t, err)
	if !visible {
		t.Fatal("Escape on the chat closed the palette; it should go back to the root")
	}
}

// waitForCompletedStreams polls the fake until n chat streams reached their end, failing the test after timeout.
func waitForCompletedStreams(t *testing.T, fake *FakeBackend, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if completed, _ := fake.StreamOutcomes(); completed >= n {
			return
		}
		if time.Now().After(deadline) {
			completed, abandoned := fake.StreamOutcomes()
			t.Fatalf("chat streams on the backend: %d completed, %d abandoned; want %d completed", completed, abandoned, n)
		}
		time.Sleep(pollInterval)
	}
}
