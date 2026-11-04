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

// J13 (PLAN.md 3.1): Tab asks AI, the reply streams into the chat, and Escape stops it while frames are still due. The
// fake backend counts the stream as abandoned, not completed, so the stop is proven on the backend side. The palette
// stays open, the question stays in the transcript, and the last word never arrives. A second Escape, with nothing in
// flight, goes back to the root as it does on any page. The same holds after a reply that completes and after a turn
// the backend refuses with 402 (Credits needed): each Escape then goes back to the root. Continuing the conversation
// later and the first-token progress are not covered yet.
func TestJourneyJ13AskAIStreamsAndEscapeStopsTheReply(t *testing.T) {
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
	time.Sleep(streamFrameDelay * 8)
	if completed, abandoned := fake.StreamOutcomes(); completed != 0 || abandoned != 1 {
		t.Fatalf("chat streams on the backend: %d completed, %d abandoned; want the one stream abandoned", completed, abandoned)
	}
	if paletteShows(t, app, streamFinalWord) {
		t.Fatal("the reply finished after Escape; the stream was not stopped")
	}
	visible, err := app.PaletteVisible()
	must(t, err)
	if !visible {
		t.Fatal("Escape closed the palette while the reply was streaming; it must only stop the reply")
	}
	if !paletteShows(t, app, aiQuestion) {
		t.Fatal("the question is gone from the transcript after the stop")
	}
	shoot(t, app, "j13-2-stopped")

	escapeChatToRoot(t, app)
	shoot(t, app, "j13-3-escape-back-to-root")

	fake.SetChunkDelay(0)
	fake.SetReply(completedReply)
	askAIFromRoot(t, app, completedQuestion)
	_, err = app.WaitFor(completedWord, streamWait)
	must(t, err)
	waitForCompletedStreams(t, fake, 1, streamWait)
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
