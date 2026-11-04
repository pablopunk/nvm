package aichat

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// reasoned is a finished reply that carries its reasoning.
func reasoned(text, reasoning string) chat.Message {
	m := assistant(text)
	m.Reasoning = reasoning
	return m
}

// thinkingReply is a reply that is still arriving with its reasoning: no text yet, 42 seconds into the turn.
func thinkingReply(reasoning string) chat.Message {
	m := streaming("")
	m.At = goldenNow.Add(-42 * time.Second)
	m.Reasoning = reasoning
	return m
}

// TestReasoningBlockIsClosedUntilTheUserOpensIt checks the model's reasoning sits in a Thinking block that starts
// closed, and that a click opens it to show the reasoning text.
func TestReasoningBlockIsClosedUntilTheUserOpensIt(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan", chat.TierSmart, user("Plan a trip."), reasoned("Three days in Lisbon.", "comparing the old town with the coast"))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if !tt.HasText("Thinking") || tt.HasText("comparing the old town") {
		t.Fatal("the Thinking block should show closed, without its reasoning")
	}
	if err := tt.Click("Thinking"); err != nil {
		t.Fatalf("Click Thinking: %v", err)
	}
	tt.Frame()
	if !tt.HasText("comparing the old town") {
		t.Fatal("opening Thinking did not show the reasoning")
	}
}

// TestShowThinkingInTheMenuTogglesTheBlock checks the Command-K menu can open the reasoning too, for keyboard users.
func TestShowThinkingInTheMenuTogglesTheBlock(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan", chat.TierSmart, user("Plan a trip."), reasoned("Three days in Lisbon.", "weighing the pace"))
	view := ChatView(r.deps(), record.ID)
	if got := menuTitles(view); !contains(got, "Show Thinking") {
		t.Fatalf("menu = %v, want Show Thinking", got)
	}
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Show Thinking")
	tt.Frame()
	if !tt.HasText("weighing the pace") || !contains(menuTitles(view), "Hide Thinking") {
		t.Fatal("Show Thinking did not open the reasoning block")
	}
}

// TestElapsedTimeShowsOnTheThinkingRow checks a reply with no text yet shows its state and how long the turn has run.
func TestElapsedTimeShowsOnTheThinkingRow(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan", chat.TierSmart, user("Plan a trip."), thinkingReply(""))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if !tt.HasText("Thinking") || !tt.HasText("42s") {
		t.Fatal("the thinking row should show Thinking and the 42s elapsed")
	}
}

// TestBuilderShowsWritingAndItsSteps checks a builder reply that streams the extension says Writing extension, and a
// step the workshop sets, such as Checking, shows under the transcript.
func TestBuilderShowsWritingAndItsSteps(t *testing.T) {
	r := newRig(t, nil)
	record := r.builderWith("", user("A clock."), streaming("```json\n{\"id\":\"user.clock\"}\n```"))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if !tt.HasText("Writing extension…") {
		t.Fatal("a streaming builder reply should say Writing extension…")
	}
	r.service.SetStep(record.ID, "Checking…")
	tt.Frame()
	if !tt.HasText("Checking…") {
		t.Fatal("the builder step Checking… is not shown")
	}
}

// TestElapsedTextNamesSecondsThenMinutes checks the elapsed time reads in seconds, then minutes and seconds.
func TestElapsedTextNamesSecondsThenMinutes(t *testing.T) {
	cases := map[time.Duration]string{0: "0s", 42 * time.Second: "42s", 59 * time.Second: "59s", 98 * time.Second: "1m 38s", 10 * time.Minute: "10m 00s"}
	for d, want := range cases {
		if got := elapsedText(d); got != want {
			t.Fatalf("elapsedText(%v) = %q, want %q", d, got, want)
		}
	}
}

// TestNoTimeYetStillShowsZeroSeconds checks a turn that began now reads 0s, not nothing.
func TestNoTimeYetStillShowsZeroSeconds(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan", chat.TierSmart, user("Plan a trip."), streaming(""))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if !tt.HasText("0s") {
		t.Fatal("a turn that began now should read 0s")
	}
}
