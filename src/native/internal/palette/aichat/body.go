package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// body draws the transcript above the composer on the pane's own surface, as every other palette view does. A chat with no messages is blank, as the
// desktop app shows it, except for the sign-in line of an account that is not signed in.
func (p *page) body(c *ui.Context, h search.Host) {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok {
		return
	}
	k := design.Tokens(c)
	ui.Column(c).Grow(1).MinHeight(0).Padding(8, 16, 12, 16).Children(func() {
		transcript(c, p, record, k)
		composerRow(c, p, record, h)
	})
}

// transcript is the scrolling list of messages, which follows the newest one. An empty chat is blank.
func transcript(c *ui.Context, p *page, record chat.Chat, k *design.TokenSet) {
	if len(record.Messages) == 0 {
		signInLine(c, p.deps, k)
		return
	}
	ui.List(c, &p.list, len(record.Messages), func(i int) {
		ui.Column(c).Padding(6, 4).Children(func() {
			renderMessage(c, p.deps, record.Messages[i], i == len(record.Messages)-1)
		})
	}).Grow(1)
}

// signInLine is what an empty chat shows when the account is not signed in: the one thing a chat cannot do without.
func signInLine(c *ui.Context, deps Deps, k *design.TokenSet) {
	if deps.SignedIn() {
		ui.Column(c).Grow(1)
		return
	}
	ui.Column(c).Grow(1).Center().Children(func() {
		ui.Text(c, "Not signed in: sign in from Nevermind Account to chat with AI.").FontSize(13).TextColor(k.Tertiary)
	})
}
