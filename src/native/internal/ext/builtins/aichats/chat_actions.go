package aichats

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Shortcuts of the transcript's actions, as the desktop app binds them. Refuse takes ⌘D only while a tool call waits;
// Delete Chat has no key, as 1.x's Remove Chat has none, so the two never clash.
const (
	shortcutAgain     = "CommandOrControl+R"
	shortcutCopy      = "CommandOrControl+Alt+C"
	shortcutTier      = "CommandOrControl+Shift+M"
	shortcutRefuse    = "CommandOrControl+D"
	shortcutReview    = "CommandOrControl+E"
	toolStatusWaiting = "waiting"
)

// chatActions is the action menu of a transcript, in the order the desktop app lists it: the answers a waiting tool call
// needs, then Ask Again, the copies, Stop while a reply streams, the tier, New Chat, Delete Chat, the review of the
// extension a builder chat wrote, and one Retry row per earlier answer of a conversation.
func chatActions(detail api.ChatDetail) []api.Action {
	id := detail.ID
	var out []api.Action
	if awaitingAnswer(detail.Messages) {
		out = append(out, chatOpAction(api.ChatAllow, id, "Allow", ""), chatOpAction(api.ChatRefuse, id, "Refuse", shortcutRefuse))
	}
	busy := replyStreams(detail.Messages)
	if !busy && lastText(detail.Messages, "user") != "" {
		out = append(out, chatOpAction(api.ChatAgain, id, "Ask Again", shortcutAgain))
	}
	if reply := lastText(detail.Messages, "assistant"); reply != "" {
		out = append(out, withShortcut(api.CopyText("Copy Reply", reply, false), shortcutCopy))
	}
	if message := lastText(detail.Messages, "user"); message != "" {
		out = append(out, api.CopyText("Copy Message", message, false))
	}
	if busy {
		out = append(out, chatOpAction(api.ChatStop, id, "Stop", ""))
	}
	out = append(out, tierSwitchAction(detail), newChatAction(), deleteChatAction(detail.ChatSummary))
	if detail.Kind == kindBuilder && detail.ExtensionID != "" {
		out = append(out, reviewAction(detail.ExtensionID))
	}
	return append(out, retryActions(detail, busy)...)
}

// chatOpAction is a host chat action on the chat with id, titled and bound to shortcut when it is not empty.
func chatOpAction(op, id, title, shortcut string) api.Action {
	return withShortcut(api.Action{Kind: api.ActionChat, Op: op, Title: title, Chat: &api.ChatAction{ChatID: id}}, shortcut)
}

// tierSwitchAction moves the chat to the other tier: Fast while it runs on Smart (the empty tier is Smart), Smart otherwise.
func tierSwitchAction(detail api.ChatDetail) api.Action {
	other, label := api.ModelSmart, "Smart"
	if detail.Tier == api.ModelSmart || detail.Tier == "" {
		other, label = api.ModelFast, "Fast"
	}
	return withShortcut(api.Action{Kind: api.ActionChat, Op: api.ChatTier, Title: "Switch to " + label,
		Chat: &api.ChatAction{ChatID: detail.ID, Tier: other}}, shortcutTier)
}

// newChatAction starts a conversation above the transcript, so Escape returns to it.
func newChatAction() api.Action {
	return api.PushView("New Chat", api.NewChatView("New Chat", api.ModelFast, api.ActionRef{ID: opNewSubmit}))
}

// deleteChatAction asks before the chat and its history are removed.
func deleteChatAction(summary api.ChatSummary) api.Action {
	action := removeAction(summary)
	action.Title = "Delete Chat"
	return action
}

// reviewAction opens the review of the extension a builder chat wrote.
func reviewAction(extensionID string) api.Action {
	return withShortcut(api.Action{Kind: api.ActionBuilder, Op: api.BuilderReview, Title: "Review Extension",
		Builder: &api.BuilderAction{ExtensionID: extensionID}}, shortcutReview)
}

// withShortcut binds action to an accelerator; an empty one leaves it unbound.
func withShortcut(action api.Action, accelerator string) api.Action {
	action.Shortcut = accelerator
	return action
}

// awaitingAnswer reports whether a tool call in the transcript waits for the user.
func awaitingAnswer(messages []api.ChatMessage) bool {
	for _, message := range messages {
		if message.Tool != nil && message.Tool.Status == toolStatusWaiting {
			return true
		}
	}
	return false
}

// lastText is the text of the newest message with role, or empty when there is none.
func lastText(messages []api.ChatMessage, role string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == role && messages[i].Tool == nil && messages[i].Content != "" {
			return messages[i].Content
		}
	}
	return ""
}
