package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// chatActions is the action menu of a chat, the Command-K list of the page. The answers a waiting tool call needs come
// first, then what the reply does, the copies, Stop while a reply streams, the tier, and the chat itself.
func (p *page) chatActions() []search.Action {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return nil
	}
	busy := p.deps.Service.Busy(p.id)
	var acts []search.Action
	if chat.AwaitingAnswer(record) {
		acts = append(acts, p.allowAction(), p.refuseAction())
	}
	if !busy {
		acts = append(acts, p.askAgainAction())
	}
	acts = append(acts, p.copyReplyAction(), p.copyMessageAction())
	if busy {
		acts = append(acts, p.stopAction())
	}
	if i := latestReasoning(record.Messages); i >= 0 {
		acts = append(acts, p.thinkingAction(i))
	}
	if p.previewable(record) {
		acts = append(acts, p.previewAction(record))
	}
	if p.previewing(record) {
		acts = append(acts, p.focusPreviewAction())
	}
	acts = append(acts, p.switchTierAction(record.Tier), p.newChatAction(), p.deleteChatAction(record))
	if record.Kind == chat.KindBuilder && record.Extension != "" && p.deps.Review != nil {
		acts = append(acts, p.reviewAction())
	}
	return acts
}

// allowAction answers a waiting tool call with yes. Enter does the same on an empty composer.
func (p *page) allowAction() search.Action {
	return search.Action{Title: "Allow", Label: "↵", Dismiss: search.DismissNever, Run: func(h search.Host) {
		report(h, p.deps.Service.Decide(p.id, true))
	}}
}

// refuseAction answers a waiting tool call with no, which the model receives as the user's refusal.
func (p *page) refuseAction() search.Action {
	return search.Action{Title: "Refuse", Label: "⌃D", Key: ui.KeyD, Mods: palette.Mod, Dismiss: search.DismissNever,
		Run: func(h search.Host) { report(h, p.deps.Service.Decide(p.id, false)) }}
}

// askAgainAction drops the last reply and asks the last question again.
func (p *page) askAgainAction() search.Action {
	return search.Action{Title: "Ask Again", Label: "⌃R", Key: ui.KeyR, Mods: palette.Mod, Dismiss: search.DismissNever,
		Run: func(h search.Host) { report(h, p.deps.Service.Regenerate(p.id)) }}
}

// copyReplyAction copies the newest reply.
func (p *page) copyReplyAction() search.Action {
	return search.Action{Title: "Copy Reply", Label: "⌃⌥C", Key: ui.KeyC, Mods: palette.Mod | ui.Alt, Dismiss: search.DismissNever,
		Run: p.copyLastReply}
}

// copyMessageAction copies the newest message the user sent.
func (p *page) copyMessageAction() search.Action {
	return search.Action{Title: "Copy Message", Dismiss: search.DismissNever, Run: p.copyLastMessage}
}

// stopAction stops the reply in flight, as the composer's Stop control does.
func (p *page) stopAction() search.Action {
	return search.Action{Title: "Stop", Dismiss: search.DismissNever, Run: func(search.Host) {
		p.deps.Service.Stop(p.id)
	}}
}

// switchTierAction moves the chat to the other tier: Smart while it is Fast, and Fast while it is Smart.
func (p *page) switchTierAction(tier chat.Tier) search.Action {
	other := tierLabel(chat.TierSmart)
	if tier == chat.TierSmart {
		other = tierLabel(chat.TierFast)
	}
	return search.Action{Title: "Switch to " + other, Label: "⌃⇧M", Key: ui.KeyM, Mods: palette.Mod | ui.Shift,
		Dismiss: search.DismissNever, Run: p.toggleTier}
}

// newChatAction starts a conversation above this chat, so Escape returns to it.
func (p *page) newChatAction() search.Action {
	return search.Action{Title: "New Chat", Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(newChat(p.deps))
	}}
}

// deleteChatAction asks before the chat and its history are removed.
func (p *page) deleteChatAction(record chat.Chat) search.Action {
	return search.Action{Title: "Delete Chat", Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(removeConfirm(p.deps, record))
	}}
}

// reviewAction opens the review of the extension this builder chat wrote.
func (p *page) reviewAction() search.Action {
	return search.Action{Title: "Review Extension", Label: "⌃E", Key: ui.KeyE, Mods: palette.Mod, Dismiss: search.DismissNever,
		Run: p.review}
}

// thinkingAction opens or closes the reasoning block of entry index, for keyboard users and the menu.
func (p *page) thinkingAction(index int) search.Action {
	title := "Show Thinking"
	if p.thinkingOpen[index] {
		title = "Hide Thinking"
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(search.Host) {
		p.thinkingOpen[index] = !p.thinkingOpen[index]
	}}
}
