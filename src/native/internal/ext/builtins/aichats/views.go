package aichats

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// historyView is the AI Chats list: the New Chat row, then one row per saved chat, which refreshes in place.
func historyView(ctx api.Context) *api.View {
	return &api.View{
		Kind: api.ViewList, Title: "AI Chats", Placeholder: "Search AI Chats", Items: historyItems(ctx),
		Refresh: &api.Refresh{Ref: &api.ActionRef{ID: opRefreshHistory}},
	}
}

// historyItems lists New Chat, then every chat the list shows.
func historyItems(ctx api.Context) []api.Item {
	now := ctx.Clock().Now()
	newChat := api.PushView("Start", api.NewChatView("New Chat", api.ModelFast, api.ActionRef{ID: opNewSubmit}))
	items := []api.Item{{ID: "new-chat", Title: "New Chat", Icon: "plus", Actions: []api.Action{newChat}, Primary: &newChat}}
	for _, chat := range ctx.AI().Chats().List() {
		if listed(chat) {
			items = append(items, historyRow(chat, now))
		}
	}
	return items
}

// historyRow is the list row of one chat: Open, then Remove Chat, which asks before it removes the chat and its history.
func historyRow(chat api.ChatSummary, now time.Time) api.Item {
	open := openAction(chat.ID, opOpenChat, "Open")
	return api.Item{
		ID: chat.ID, Title: chatTitle(chat.Title), Subtitle: kindLabel(chat.Kind) + " · " + ago(now, chat.UpdatedAtMs),
		Icon: chatIcon(chat.Kind), Appearance: &api.Appearance{Foreground: "yellow"}, Section: search.KindConversations,
		Primary: &open, Actions: []api.Action{open, removeAction(chat)},
	}
}

// removeAction removes the chat through the host, after the palette asks for confirmation.
func removeAction(chat api.ChatSummary) api.Action {
	title := chatTitle(chat.Title)
	detail := title + " and its history"
	if chat.Kind == "builder" {
		detail += ". Generated extension files stay."
	}
	return api.Action{Kind: api.ActionChat, Op: api.ChatRemove, Title: "Remove Chat", Chat: &api.ChatAction{ChatID: chat.ID},
		Confirm: &api.Confirmation{Title: fmt.Sprintf("Remove %q?", title), Message: detail, ConfirmLabel: "Remove Chat", CancelLabel: "Cancel", Destructive: true}}
}

// openAction runs the handler that shows the chat with id.
func openAction(id, handler, title string) api.Action {
	return api.Run(handler, title, map[string]string{argChatID: id})
}

// chatView is the transcript of one chat. The host draws the composer; Submit, Stop and the tier picker run the built-in's
// handlers, which hand the change to the chat store. A streaming reply refreshes the view on a short tick.
func chatView(ctx api.Context, detail api.ChatDetail) *api.View {
	id := detail.ID
	view := &api.View{
		Kind: api.ViewChat, Title: chatTitle(detail.Title),
		Chat: &api.ChatSpec{
			Messages: detail.Messages, Tier: detail.Tier, Attach: attachable(detail.Tier),
			OnTier: chatRef(opTier, id), Submit: chatRef(opSubmit, id), Stop: chatRef(opStop, id),
			Side: builderPreview(ctx, detail),
		},
	}
	if replyStreams(detail.Messages) {
		view.Refresh = &api.Refresh{IntervalMs: streamTickMs, Ref: chatRef(opRefreshChat, id)}
	}
	return view
}

// builderPreview is the side of a builder chat: the extension the chat wrote, as it is now. Other chats have none.
func builderPreview(ctx api.Context, detail api.ChatDetail) *api.View {
	if detail.Kind != kindBuilder || detail.ExtensionID == "" {
		return nil
	}
	preview, ok := ctx.AIBuilder().ExtensionPreview(detail.ExtensionID)
	if !ok {
		return nil
	}
	return &preview
}

// chatRef names a chat handler with the chat's ID in its args.
func chatRef(handler, id string) *api.ActionRef {
	return &api.ActionRef{ID: handler, Args: map[string]string{argChatID: id}}
}

// replyStreams reports whether the transcript ends with a reply that is still arriving.
func replyStreams(messages []api.ChatMessage) bool {
	return len(messages) > 0 && messages[len(messages)-1].Streaming
}

// attachable reports whether the composer offers pictures: only on the smart tier, which the empty tier means.
func attachable(tier api.ModelTier) bool {
	return tier == "" || tier == api.ModelSmart
}

// continueRows lists up to limit Continue rows for chats whose title holds query. An empty query lists the first ones.
func continueRows(chats []api.ChatSummary, query string, limit int) []api.Item {
	lq := strings.ToLower(query)
	var out []api.Item
	for _, chat := range chats {
		if len(out) >= limit {
			break
		}
		if !listed(chat) || chat.Title == "" {
			continue
		}
		score := float64(scoreRootConversation)
		if lq != "" {
			if score = float64(search.Score(chat.Title, lq)); score == 0 {
				continue
			}
		}
		out = append(out, continueRow(chat, score))
	}
	return out
}

// continueRow is the root row that opens one chat.
func continueRow(chat api.ChatSummary, score float64) api.Item {
	title, subtitle := continueLabels(chat.Kind, oneLine(chat.Title))
	open := openAction(chat.ID, opOpenChat, "Open")
	return api.Item{
		ID: "chat:" + chat.ID, Title: title, Subtitle: subtitle, Icon: chatIcon(chat.Kind), Section: search.KindConversations,
		Score: score, LastUsedMs: lastChangeMs(chat), TiePriority: chatTiePriority(chat.Kind), TintTitle: true,
		Appearance: &api.Appearance{Foreground: "yellow"}, Primary: &open,
	}
}

// lastChangeMs is when a chat was last used: its last change, or its creation when it has none.
func lastChangeMs(chat api.ChatSummary) int64 {
	return max(chat.UpdatedAtMs, chat.CreatedAtMs)
}

// ago names how long ago a Unix-millisecond time was, in the words the list uses.
func ago(now time.Time, unixMs int64) string {
	d := now.Sub(time.UnixMilli(unixMs))
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

// oneLine collapses a title to one line and clips it, as the root rows show it.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > oneLineLimit {
		return string(r[:oneLineLimit]) + "…"
	}
	return s
}

// chatTitle is the name a chat is listed under.
func chatTitle(title string) string {
	if title == "" {
		return "AI Chat"
	}
	return title
}

// chatsSubtitle names the AI Chats command's count.
func chatsSubtitle(count int) string {
	if count == 1 {
		return "1 AI chat"
	}
	return fmt.Sprintf("%d AI chats", count)
}

// listed reports whether a chat of this kind appears in the list and the continue rows. A proofread is not listed.
func listed(chat api.ChatSummary) bool {
	return chat.Kind == "conversation" || chat.Kind == "builder"
}
