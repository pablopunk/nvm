package aichat

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// renderMessage draws one transcript entry: the user's line as a bubble, a notice as a card, and a reply as
// Markdown. last says whether the entry is the newest, which is the one a reply may still be arriving in.
func renderMessage(c *ui.Context, deps Deps, m chat.Message, last bool) {
	switch m.Author {
	case chat.AuthorUser:
		if len(m.Images) > 0 {
			messagePictures(c, m.Images)
		}
		if m.Text != "" {
			design.UserBubble(c, m.Text)
		}
	case chat.AuthorNotice:
		noticeCard(c, *m.Notice)
	case chat.AuthorTool:
		if m.Tool != nil {
			toolRow(c, m)
		}
	default:
		reply(c, deps, m, last)
	}
}

// reply draws an assistant entry. A reply with no visible text yet shows the thinking state; a reply that stopped says
// so.
func reply(c *ui.Context, deps Deps, m chat.Message, last bool) {
	k := design.Tokens(c)
	streaming := m.Status == chat.StatusStreaming
	if strings.TrimSpace(m.Text) == "" {
		if streaming {
			thinking(c, k)
		}
		return
	}
	deps.Reply(c, m.Text, streaming && last)
	if m.Status == chat.StatusStopped {
		ui.Text(c, "Stopped").FontSize(11).TextColor(k.Tertiary)
	}
}

// thinking is the state of a reply that has no visible text yet, while the model works. It names the state only; the
// model's reasoning is not shown, as in the desktop app, which glows the same word in the accent color.
func thinking(c *ui.Context, k *design.TokenSet) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		ui.Text(c, "Thinking").FontSize(13).FontWeight(500).TextColor(k.Accent)
	})
}

// noticeCard draws a notice: its title, then the words that say what to do next.
func noticeCard(c *ui.Context, notice chat.Notice) {
	k := design.Tokens(c)
	ui.Column(c).Gap(2).Padding(8, 12).Radius(8).Background(k.Surface).Border(1, k.EdgeSubtle).Children(func() {
		ui.Text(c, notice.Title).FontSize(12).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, notice.Message).FontSize(12).TextColor(k.Muted)
	})
}
