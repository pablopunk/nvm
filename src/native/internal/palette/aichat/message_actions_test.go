package aichat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestMessageCopyActionsTargetEachTextTurn(t *testing.T) {
	var selected []int
	x := NewExtensionChat(nil, nil)
	x.SetSpec(ExtensionChatSpec{
		Messages: []chat.Message{
			{Author: chat.AuthorUser, Text: "first question"},
			{Author: chat.AuthorAssistant, Text: "first answer"},
			{Author: chat.AuthorTool},
			{Author: chat.AuthorUser, Text: "second question"},
		},
		CopyMessage: func(_ search.Host, index int) { selected = append(selected, index) },
	})
	actions := x.messageCopyActions()
	if len(actions) != 3 || actions[0].Title != "Copy message 1: first question" || actions[2].Title != "Copy message 4: second question" {
		t.Fatalf("actions = %+v", actions)
	}
	actions[0].Run(nil)
	actions[2].Run(nil)
	if len(selected) != 2 || selected[0] != 0 || selected[1] != 3 {
		t.Fatalf("selected = %v", selected)
	}
}
