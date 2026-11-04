package aichat

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Copy for the palette's answers to actions and refusals. Each names the state and says what happened.
const (
	toastStillAnswering  = "This chat is still answering. Stop the reply first."
	toastNothingToAsk    = "There is no question to ask again yet."
	toastCopied          = "Copied the last reply."
	toastCopiedMessage   = "Copied the last message."
	toastNoReply         = "There is no reply to copy yet."
	toastNoMessage       = "There is no message to copy yet."
	toastFailed          = "That did not work. Try again."
	toastNothingToReview = "This extension is not installed any more."
)

// review opens the review of the extension this builder chat wrote, on top of the chat, so Escape returns to it.
func (p *page) review(h search.Host) {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok || record.Extension == "" || p.deps.Review == nil {
		return
	}
	view := p.deps.Review(record.Extension)
	if view == nil {
		h.Toast(toastNothingToReview)
		return
	}
	h.Push(view)
}

// copyLastReply copies the newest reply's text and says so on the palette.
func (p *page) copyLastReply(h search.Host) {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	text, found := chat.LastReply(record)
	if !found || text == "" {
		h.Toast(toastNoReply)
		return
	}
	p.deps.Copy(text)
	h.Toast(toastCopied)
}

// copyLastMessage copies the text of the newest message the user sent and says so on the palette.
func (p *page) copyLastMessage(h search.Host) {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	text, found := lastUserText(record)
	if !found {
		h.Toast(toastNoMessage)
		return
	}
	p.deps.Copy(text)
	h.Toast(toastCopiedMessage)
}

// lastUserText is the text of the newest message the user sent, and whether there is one.
func lastUserText(record chat.Chat) (string, bool) {
	for i := len(record.Messages) - 1; i >= 0; i-- {
		if record.Messages[i].Author == chat.AuthorUser {
			return record.Messages[i].Text, true
		}
	}
	return "", false
}

// toggleTier switches the chat between the fast and smart tiers. The next reply uses the new tier.
func (p *page) toggleTier(h search.Host) {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	next := chat.TierSmart
	if record.Tier == chat.TierSmart {
		next = chat.TierFast
	}
	if err := p.deps.Service.SetTier(p.id, next); err != nil {
		h.Toast(refusalCopy(err))
		return
	}
	h.Toast("Next replies use the " + tierLabel(next) + " model.")
}

// report toasts a refusal the transcript does not show. A nil error is silent.
func report(h search.Host, err error) {
	if err == nil {
		return
	}
	h.Toast(refusalCopy(err))
}

// refusalCopy is the palette line for a refusal the transcript does not carry.
func refusalCopy(err error) string {
	switch {
	case errors.Is(err, chat.ErrBusy):
		return toastStillAnswering
	case errors.Is(err, chat.ErrNothingToAsk):
		return toastNothingToAsk
	case errors.Is(err, chat.ErrEmpty):
		return ""
	}
	return toastFailed
}
