//go:build rig

package rigtest

import (
	"testing"
	"time"
)

// J13 (PLAN.md 3.1): Tab asks AI, the reply streams into the chat, and Escape stops it while frames are still due. The
// fake backend counts the stream as abandoned, not completed, so the stop is proven on the backend side. The palette
// stays open, the question stays in the transcript, and the last word never arrives. Continuing the conversation later
// and the first-token progress are not covered yet.
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
}
