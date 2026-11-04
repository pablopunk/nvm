package aichat

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// HistoryView lists the AI chats, conversations and builder chats alike, most recent first, with a row that starts a new
// conversation. The list is answered from the store each time the palette searches, so it always shows what is saved.
func HistoryView(deps Deps) *palette.View {
	return &palette.View{
		Title:       "AI Chats",
		Placeholder: "Search AI Chats",
		Search: func(q string) []search.Item {
			return historyRows(deps, q)
		},
	}
}

// historyRows is the list for query q: the new-chat row, then each listed chat whose title contains q.
func historyRows(deps Deps, q string) []search.Item {
	query := strings.ToLower(strings.TrimSpace(q))
	items := []search.Item{{Title: "New Chat", Icon: "plus", Actions: []search.Action{
		{Title: "Start", Run: func(h search.Host) { h.Push(newChat(deps)) }},
	}}}
	for _, record := range deps.Service.Chats() {
		if !chat.Listed(record.Kind) || (query != "" && !strings.Contains(strings.ToLower(record.Title), query)) {
			continue
		}
		items = append(items, ConversationRow(deps, record))
	}
	return items
}

// ConversationRow is the list row of one saved conversation: Open, then Remove action, which asks before it deletes the
// conversation, as 1.x's AI chat rows do.
func ConversationRow(deps Deps, record chat.Chat) search.Item {
	id := record.ID
	title := chatTitle(record)
	return search.Item{
		ID:        id,
		Title:     title,
		Subtitle:  kindLabel(record.Kind) + " · " + ago(deps.Now(), record.UpdatedAt),
		Icon:      ChatIcon(record.Kind),
		Tone:      "yellow",
		TintTitle: true,
		Actions: []search.Action{
			{Title: "Open", Run: func(h search.Host) { h.Push(ChatView(deps, id)) }},
			{Title: "Remove action", Dismiss: search.DismissNever, Run: func(h search.Host) {
				h.Push(palette.Confirm("Remove action", "Delete “"+title+"” from Nevermind", func() {
					if err := deps.Service.Delete(id); err != nil {
						h.Toast(refusalCopy(err))
					}
				}))
			}},
		},
	}
}

// newChat starts a conversation on the fast tier and returns its page. A store that refuses the write shows a
// page with nothing in it, and the palette says why on the next key.
func newChat(deps Deps) *palette.View {
	record, err := deps.Service.NewChat(chat.KindConversation, chat.TierFast)
	if err != nil {
		return ChatView(deps, "")
	}
	return ChatView(deps, record.ID)
}

// ago names how long ago t was, in the words the list uses.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%d d ago", int(d/(24*time.Hour)))
	}
}
