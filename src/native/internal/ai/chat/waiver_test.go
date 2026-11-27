package chat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// The waiver of a turn is built from the message the turn answers. A launch asked for in an older message grants
// nothing now.
func TestTheTurnsWaiverSeesOnlyTheCurrentMessage(t *testing.T) {
	f := newFixture(t)
	var seen []string
	f.svc.opts.Tools.Waiver = func(message string) agent.Waiver {
		seen = append(seen, message)
		return nil
	}
	c := Chat{Kind: KindConversation, Messages: []Message{
		{Author: AuthorUser, Text: "open github.com"},
		{Author: AuthorAssistant, Text: "Opened github.com."},
		{Author: AuthorUser, Text: "thanks"},
	}}
	f.svc.setupFor(c)
	if len(seen) != 1 || seen[0] != "thanks" {
		t.Fatalf("the waiver saw %q; want only the current message", seen)
	}
}
