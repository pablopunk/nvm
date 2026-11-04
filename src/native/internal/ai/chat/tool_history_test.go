package chat

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestRequestAnswersCallsThatNeverFinished checks a restart that cut a call off: the request still answers it.
func TestRequestAnswersCallsThatNeverFinished(t *testing.T) {
	chat := Chat{Messages: []Message{
		{Author: AuthorUser, Text: "find it", Status: StatusComplete},
		{Author: AuthorAssistant, Status: StatusComplete, Calls: []backend.ToolCall{{ID: "x1", Type: "function", Function: backend.ToolFunction{Name: "search_files", Arguments: "{}"}}}},
		{Author: AuthorTool, Status: StatusRunning, Tool: &ToolRun{CallID: "x1", Name: "search_files"}},
	}}
	turns := Request("system", chat)[1:]
	if len(turns) != 3 || turns[2].Role != backend.RoleTool || turns[2].ToolCallID != "x1" || !strings.Contains(string(turns[2].Content), unfinishedResult) {
		t.Fatalf("turns = %+v", turns)
	}
}

// TestRequestNeverStartsWithAnOrphanAnswer checks the history cut: a window never opens on a tool answer.
func TestRequestNeverStartsWithAnOrphanAnswer(t *testing.T) {
	var messages []Message
	for i := 0; i < 12; i++ {
		id := string(rune('a' + i))
		messages = append(messages,
			Message{Author: AuthorUser, Text: "q" + id, Status: StatusComplete},
			Message{Author: AuthorAssistant, Status: StatusComplete, Calls: []backend.ToolCall{{ID: id, Type: "function", Function: backend.ToolFunction{Name: "x", Arguments: "{}"}}}},
			Message{Author: AuthorTool, Status: StatusComplete, Text: "r" + id, Tool: &ToolRun{CallID: id, Name: "x"}},
			Message{Author: AuthorAssistant, Text: "a" + id, Status: StatusComplete},
		)
	}
	turns := Request("system", Chat{Messages: messages})[1:]
	if len(turns) > MaxHistoryMessages {
		t.Fatalf("history = %d messages, want at most %d", len(turns), MaxHistoryMessages)
	}
	if turns[0].Role == backend.RoleTool {
		t.Fatal("the history opens on a tool answer whose call was cut")
	}
	for i, turn := range turns {
		if turn.Role == backend.RoleTool && (i == 0 || turns[i-1].Role != backend.RoleAssistant && turns[i-1].Role != backend.RoleTool) {
			t.Fatalf("tool answer %d has no call before it", i)
		}
	}
}
