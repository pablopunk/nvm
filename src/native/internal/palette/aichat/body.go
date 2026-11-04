package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// body draws the transcript above the model pill. An empty chat says what it is for, or that sign-in comes first.
func (p *page) body(c *ui.Context, h search.Host) {
	p.host = h
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	k := design.Tokens(c)
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
		modelPill(c, p.deps, record)
	})
}

// emptyTranscript is the hint shown in a chat with no messages.
func emptyTranscript(c *ui.Context, deps Deps, record chat.Chat, k *design.TokenSet) {
	text := "Ask anything. Replies stream in as Markdown."
	if record.Kind == chat.KindBuilder {
		text = "Describe what you want; the model writes a Go extension, it is checked and installed."
	}
	if !deps.SignedIn() {
		text = "Not signed in: sign in from Nevermind Account to chat with AI."
	}
	ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
		ui.Text(c, text).FontSize(13).TextColor(k.Tertiary)
	})
}

// modelPill names the tier the next reply uses and the route the backend resolved for it, when it has said.
func modelPill(c *ui.Context, deps Deps, record chat.Chat) {
	text := "Resolving model…"
	if model, ok := deps.Service.Model(record.Tier); ok && model.Name != "" {
		text = model.Name
	}
	design.Chip(c, design.ChipData{Icon: "sparkles", Bold: tierLabel(record.Tier), Text: text})
}

// tierLabel is the word the pill shows for a tier.
func tierLabel(tier chat.Tier) string {
	if tier == chat.TierSmart {
		return "Smart"
	}
	return "Fast"
}
