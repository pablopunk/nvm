package aichat

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// messageCopyActions names each text-bearing turn so keyboard users can choose the exact message.
func (x *ExtensionChat) messageCopyActions() []search.Action {
	if x.spec.CopyMessage == nil {
		return nil
	}
	var actions []search.Action
	copyMessage := x.spec.CopyMessage
	for i, message := range x.spec.Messages {
		if (message.Author != chat.AuthorUser && message.Author != chat.AuthorAssistant) || message.Text == "" {
			continue
		}
		index := i
		preview := []rune(strings.Join(strings.Fields(message.Text), " "))
		if len(preview) > 40 {
			preview = append(preview[:40], '…')
		}
		title := fmt.Sprintf("Copy message %d: %s", i+1, string(preview))
		actions = append(actions, search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			copyMessage(h, index)
		}})
	}
	return actions
}
