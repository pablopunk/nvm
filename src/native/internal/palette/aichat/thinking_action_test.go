package aichat

import (
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// reasoningSpec is a transcript whose newest reply has reasoning, with a menu of Copy Reply, the tier row and New Chat.
func reasoningSpec(reasoning string) ExtensionChatSpec {
	return ExtensionChatSpec{
		Messages: []chat.Message{
			{Author: chat.AuthorUser, Text: "Plan a trip."},
			{Author: chat.AuthorAssistant, Text: "Three days in Lisbon.", Reasoning: reasoning},
		},
		Actions: func() []search.Action {
			return []search.Action{{Title: "Copy Reply"}, {Title: "Switch to Smart"}, {Title: "New Chat"}}
		},
	}
}

func actionTitlesOf(actions []search.Action) []string {
	titles := make([]string, 0, len(actions))
	for _, action := range actions {
		titles = append(titles, action.Title)
	}
	return titles
}

// TestThinkingRowSitsBeforeTheTierRowAndTogglesTheReasoning checks the menu lists Show Thinking after the reply's own
// rows and before the tier, and that Show and Hide open and close the block, refreshing the palette each time.
func TestThinkingRowSitsBeforeTheTierRowAndTogglesTheReasoning(t *testing.T) {
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(reasoningSpec("weighing the pace"))
	want := []string{"Copy Reply", "Show Thinking", "Switch to Smart", "New Chat"}
	if got := actionTitlesOf(x.actions()); !slices.Equal(got, want) {
		t.Fatalf("menu = %v, want %v", got, want)
	}
	host := &refreshHost{}
	findAction(t, x.actions(), "Show Thinking").Run(host)
	if !x.page.thinkingOpen[1] || host.refreshed != 1 {
		t.Fatalf("Show Thinking opened = %v, refreshes = %d; want the block open and one refresh", x.page.thinkingOpen[1], host.refreshed)
	}
	if findAction(t, x.actions(), "Hide Thinking").Title != "Hide Thinking" {
		t.Fatal("the open block's row does not say Hide Thinking")
	}
}

// TestThinkingRowIsAbsentWithoutReasoning keeps the row out of a transcript whose replies have no reasoning.
func TestThinkingRowIsAbsentWithoutReasoning(t *testing.T) {
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(reasoningSpec(""))
	if slices.Contains(actionTitlesOf(x.actions()), "Show Thinking") {
		t.Fatal("Show Thinking listed for a reply with no reasoning")
	}
}
