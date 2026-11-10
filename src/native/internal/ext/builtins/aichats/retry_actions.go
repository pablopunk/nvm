package aichats

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// retryActions lists one Retry row per answer of a conversation that follows a request. Retry keeps the chat and its
// later messages, and starts a new chat from that request. A builder chat owns its extension, and an extension has one
// chat, so it lists none; a chat that is still answering lists none either, as Ask Again hides then.
func retryActions(detail api.ChatDetail, busy bool) []api.Action {
	if detail.Kind != kindConversation || busy {
		return nil
	}
	var out []api.Action
	requested := false
	for i, message := range detail.Messages {
		switch {
		case message.Role == "user" && message.Tool == nil:
			requested = true
		case requested && retryable(message):
			title := fmt.Sprintf("Retry message %d: %s", i+1, retryPreview(message))
			out = append(out, api.Action{Kind: api.ActionChat, Op: api.ChatRetry, Title: title,
				Chat: &api.ChatAction{ChatID: detail.ID, Index: i}})
		}
	}
	return out
}

// retryable reports whether the message is an answer, or the notice of a turn that failed or was cut short.
func retryable(message api.ChatMessage) bool {
	if message.Tool != nil || message.Streaming {
		return false
	}
	return message.Role == "assistant" || message.Role == "system"
}

// retryPreview names the answer a row retries: its first words, or its notice title when it has no text.
func retryPreview(message api.ChatMessage) string {
	text := strings.Join(strings.Fields(message.Content), " ")
	if text == "" {
		text = strings.Join(strings.Fields(message.Title), " ")
	}
	if text == "" {
		return "no text"
	}
	if r := []rune(text); len(r) > retryPreviewRunes {
		return string(r[:retryPreviewRunes]) + "…"
	}
	return text
}

const retryPreviewRunes = 40
