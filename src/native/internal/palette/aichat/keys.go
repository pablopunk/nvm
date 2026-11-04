package aichat

import (
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Copy for the palette's answers to keys and refusals. Each names the state and says what happened.
const (
	toastStillAnswering = "This chat is still answering. Stop it with Escape first."
	toastNothingToAsk   = "There is no question to ask again yet."
	toastCopied         = "Copied the last reply."
	toastNoReply        = "There is no reply to copy yet."
	toastFailed         = "That did not work. Try again."
)

// key answers the keys of a chat page before the palette does. It reports whether it used one. Escape stops a
// reply in flight and otherwise falls through, so it still closes the page.
func (p *page) key(c *ui.Context, h search.Host) bool {
	p.host = h
	switch {
	case c.Shortcut(0, ui.KeyEscape) && p.deps.Service.Busy(p.id):
		p.deps.Service.Stop(p.id)
	case c.Shortcut(palette.Mod, ui.KeyD) && p.deps.Service.Awaiting(p.id):
		p.report(p.deps.Service.Decide(p.id, false))
	case c.Shortcut(palette.Mod, ui.KeyR):
		p.report(p.deps.Service.Regenerate(p.id))
	case c.Shortcut(palette.Mod|ui.Alt, ui.KeyC):
		p.copyLastReply(h)
	case c.Shortcut(palette.Mod|ui.Shift, ui.KeyM):
		p.toggleTier(h)
	default:
		return false
	}
	return true
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
