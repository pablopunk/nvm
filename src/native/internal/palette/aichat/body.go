package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// body draws the transcript above the composer on the pane's own surface. With no account connected the sign-in gate
// replaces both, as the Electron chat does.
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
	ui.Column(c).Grow(1).MinHeight(0).Padding(8, 16, 12, 16).Children(func() {
		transcript(c, p, record)
		composerRow(c, p, record, h)
	})
}

// transcript is the scrolling list of messages, which follows the newest one. An empty chat is blank.
func transcript(c *ui.Context, p *page, record chat.Chat) {
	if len(record.Messages) == 0 {
		ui.Column(c).Grow(1)
		return
	}
	ui.List(c, &p.list, len(record.Messages), func(i int) {
		ui.Column(c).Padding(6, 4).Children(func() {
			renderMessage(c, p.deps, record.Messages[i], i == len(record.Messages)-1)
		})
	}).Grow(1)
}
