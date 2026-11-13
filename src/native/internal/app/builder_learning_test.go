package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

func TestBuilderLearningEventsExcludeImagesAndToolContents(t *testing.T) {
	messages, outcomes := builderLearningEvents([]chat.Message{
		{Author: chat.AuthorUser, Text: "visible preference"},
		{Author: chat.AuthorTool, Text: "private result", Status: chat.StatusComplete, Tool: &chat.ToolRun{Input: "private argument"}},
		{Author: chat.AuthorUser, Text: "private image prompt", Images: []chat.Image{{Name: "private"}}},
		{Author: chat.AuthorAssistant, Text: "private image description"},
		{Author: chat.AuthorUser, Text: "another preference"},
	})
	if len(messages) != 2 || messages[0].Text != "visible preference" || messages[1].Text != "another preference" || len(outcomes) != 1 || outcomes[0] != "success" {
		t.Fatalf("unexpected captured events: %+v %v", messages, outcomes)
	}
}
