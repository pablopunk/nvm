package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// body draws the transcript above the model pill. With no account connected the sign-in gate replaces it, as the
// Electron chat does; an empty chat says what it is for.
func (p *page) body(c *ui.Context, h search.Host) {
	p.host = h
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	k := design.Tokens(c)
	if !p.deps.SignedIn() {
		ui.Column(c).Grow(1).Padding(8, 16, 8, 16).Children(func() {
			signInGate(c, p.deps.SignIn(), k)
		})
		return
	}
	ui.Column(c).Grow(1).Padding(8, 16, 8, 16).Children(func() {
		if len(record.Messages) == 0 {
			emptyTranscript(c, p.deps, record, k)
		} else {
			ui.List(c, &p.list, len(record.Messages), func(i int) {
				ui.Column(c).Padding(6, 4).Children(func() {
					renderMessage(c, p.deps, record.Messages[i], i == len(record.Messages)-1)
				})
			}).Grow(1)
		}
		modelPill(c, record)
	})
}

// emptyTranscript is the hint shown in a chat with no messages.
func emptyTranscript(c *ui.Context, deps Deps, record chat.Chat, k *design.TokenSet) {
	text := "Ask anything. Replies stream in as Markdown."
	if record.Kind == chat.KindBuilder {
		text = "Describe what you want; the model writes a Go extension, it is checked and installed."
	}
	ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(k.Tertiary)
	})
}

// modelPill names the tier the next reply uses. The backend's model and route names are never shown.
func modelPill(c *ui.Context, record chat.Chat) {
	design.Chip(c, design.ChipData{Icon: "sparkles", Bold: tierLabel(record.Tier)})
}

// tierLabel is the word the pill shows for a tier.
func tierLabel(tier chat.Tier) string {
	if tier == chat.TierSmart {
		return "Smart"
	}
	return "Fast"
}
