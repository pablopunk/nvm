package chat

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestRequestKeepsTheLastTurnsAndTruncatesLongText checks the history limit and the per-message cap.
func TestRequestKeepsTheLastTurnsAndTruncatesLongText(t *testing.T) {
	var messages []Message
	for i := 0; i < MaxHistoryMessages+5; i++ {
		messages = append(messages, Message{Author: AuthorUser, Text: "q", Status: StatusComplete})
	}
	messages = append(messages,
		Message{Author: AuthorNotice, Text: "dropped", Status: StatusComplete},
		Message{Author: AuthorAssistant, Text: strings.Repeat("é", maxMessageRunes+10), Status: StatusComplete},
		Message{Author: AuthorAssistant, Text: "partial", Status: StatusStreaming},
	)
	request := Request("system text", Chat{Messages: messages})
	if request[0].Role != backend.RoleSystem {
		t.Fatalf("first role = %q, want system", request[0].Role)
	}
	turns := request[1:]
	if len(turns) != MaxHistoryMessages {
		t.Fatalf("turns = %d, want %d", len(turns), MaxHistoryMessages)
	}
	if last := turns[len(turns)-1]; last.Role != backend.RoleAssistant || len(string(last.Content)) < maxMessageRunes {
		t.Fatalf("last turn = %+v, want the long reply, truncated to the cap", last)
	}
}

// TestTitleIsTheFirstLineCutToSixtyRunes checks the title a chat takes from its first message.
func TestTitleIsTheFirstLineCutToSixtyRunes(t *testing.T) {
	if got := TitleFor("  Plan   a trip\nto Lisbon"); got != "Plan a trip" {
		t.Fatalf("title = %q, want the first line with spaces collapsed", got)
	}
	long := TitleFor(strings.Repeat("ö", 100))
	if n := len([]rune(long)); n != titleRunes {
		t.Fatalf("title has %d runes, want %d", n, titleRunes)
	}
}
