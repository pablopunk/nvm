package aichat

import (
	"fmt"
	"unicode"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// messageCopyActions names each text-bearing turn so keyboard users can choose the exact message.
func (x *ExtensionChat) messageCopyActions() []search.Action {
	if x.spec.CopyMessage == nil {
		return nil
	}
	if x.spec.CopyRevision != nil && x.spec.CopyRevision == x.copyRevision {
		return x.copyActions
	}
	var actions []search.Action
	copyMessage := x.spec.CopyMessage
	for i, message := range x.spec.Messages {
		if (message.Author != chat.AuthorUser && message.Author != chat.AuthorAssistant) || message.Text == "" {
			continue
		}
		index := i
		title := fmt.Sprintf("Copy message %d: %s", i+1, messageCopyPreview(message.Text))
		actions = append(actions, search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			copyMessage(h, index)
		}})
	}
	x.copyRevision, x.copyActions = x.spec.CopyRevision, actions
	return actions
}

func messageCopyPreview(text string) string {
	preview := make([]rune, 0, 41)
	space := false
	for offset, char := range text {
		if offset >= 256 || len(preview) >= 40 {
			return string(append(preview, '…'))
		}
		if unicode.IsSpace(char) {
			space = len(preview) > 0
			continue
		}
		if space && len(preview) < 39 {
			preview = append(preview, ' ')
		}
		preview = append(preview, char)
		space = false
	}
	return string(preview)
}
