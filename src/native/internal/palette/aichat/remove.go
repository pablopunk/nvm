package aichat

import (
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// removeConfirm asks before a chat and its history are removed. Its first row removes the chat and goes back to the
// list; Cancel and Escape go back without removing anything. A builder chat's extension files stay, as the desktop app
// keeps them.
func removeConfirm(deps Deps, record chat.Chat) *palette.View {
	id := record.ID
	title := chatTitle(record)
	detail := title + " and its history"
	if record.Kind == chat.KindBuilder {
		detail += ". Generated extension files stay."
	}
	return &palette.View{
		Title:       "Confirm",
		Placeholder: "Remove \"" + title + "\" and its history?",
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: "Remove Chat", Subtitle: detail, Icon: "check", Tone: "red", Actions: []search.Action{
					{Title: "Remove Chat", Run: func(h search.Host) { removeChat(deps, id, h) }},
				}},
				{Title: "Cancel", Icon: "x", Actions: []search.Action{
					{Title: "Cancel", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }},
				}},
			}
		},
	}
}

// removeChat removes the chat, then shows the list again. A refusal is toasted.
func removeChat(deps Deps, id string, h search.Host) {
	if err := deps.Service.Delete(id); err != nil {
		h.Toast(refusalCopy(err))
		return
	}
	h.Pop()
	h.Refresh()
}

// chatTitle is the name a chat is listed under.
func chatTitle(record chat.Chat) string {
	if record.Title == "" {
		return "AI Chat"
	}
	return record.Title
}
