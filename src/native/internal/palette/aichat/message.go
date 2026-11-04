package aichat

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// renderMessage draws one transcript entry: the user's line as a bubble, a notice as a card, and a reply as Markdown.
// index is the entry's place in the transcript and last says whether it is the newest, which is the one a reply may
// still be arriving in.
func renderMessage(c *ui.Context, p *page, index int, m chat.Message, last bool) {
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
			toolRow(c, p, index, m)
		}
	default:
		reply(c, p, index, m, last)
	}
}

// reply draws an assistant entry: its reasoning as a block that opens on request, then its text. A reply with no text
// yet shows the progress of the turn, and a reply that stopped says so.
func reply(c *ui.Context, p *page, index int, m chat.Message, last bool) {
	k := design.Tokens(c)
	streaming := m.Status == chat.StatusStreaming
	if m.Reasoning != "" {
		reasoningBlock(c, p, index, m.Reasoning)
	}
	if strings.TrimSpace(m.Text) == "" {
		if streaming {
			progressRow(c, k, "Thinking", m.At, p.deps.Now())
		}
		return
	}
	p.deps.Reply(c, m.Text, streaming && last)
	if m.Status == chat.StatusStopped {
		ui.Text(c, "Stopped").FontSize(11).TextColor(k.Tertiary)
	}
}

// noticeCard draws a notice: its title, then the words that say what to do next.
func noticeCard(c *ui.Context, notice chat.Notice) {
	k := design.Tokens(c)
	ui.Column(c).Gap(2).Padding(8, 12).Radius(8).Background(k.Surface).Border(1, k.EdgeSubtle).Children(func() {
		ui.Text(c, notice.Title).FontSize(12).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, notice.Message).FontSize(12).TextColor(k.Muted)
	})
}
