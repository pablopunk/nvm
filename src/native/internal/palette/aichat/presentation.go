package aichat

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

type chatPresentation struct {
	visible        []int
	activity       int
	busy, awaiting bool
}

// presentChat separates conversation content from current turn activity without changing the model's transcript.
func presentChat(messages []chat.Message) chatPresentation {
	return presentChatInto(messages, nil)
}

func presentChatInto(messages []chat.Message, visible []int) chatPresentation {
	out := chatPresentation{activity: -1, visible: visible[:0]}
	awaitingIndex := -1
	for i, m := range messages {
		if m.Author == chat.AuthorUser {
			out.activity, out.busy, out.awaiting = -1, false, false
		}
		if m.Author == chat.AuthorTool {
			if m.Tool != nil {
				switch m.Status {
				case chat.StatusRunning:
					out.activity, out.busy = i, true
				case chat.StatusAwaiting:
					out.activity, out.awaiting = i, true
					awaitingIndex = i
				}
			}
			continue
		}
		if m.Author == chat.AuthorAssistant {
			if m.Status == chat.StatusStreaming {
				out.busy = true
				out.activity = -1
				if strings.TrimSpace(m.Text) == "" {
					out.activity = i
				}
			} else if m.Status == chat.StatusStopped || (m.Status == chat.StatusComplete && len(m.Calls) == 0) {
				out.activity, out.busy, out.awaiting = -1, false, false
			}
			if strings.TrimSpace(m.Text) == "" && m.Reasoning == "" && m.Status != chat.StatusStopped {
				continue
			}
		}
		if m.Author == chat.AuthorNotice {
			out.activity, out.busy, out.awaiting = -1, false, false
		}
		out.visible = append(out.visible, i)
	}
	if out.awaiting {
		out.activity = awaitingIndex
	}
	return out
}

func (x *ExtensionChat) transcript(c *ui.Context) {
	visible := x.presentation.visible
	if len(visible) == 0 {
		ui.Column(c).Grow(1)
		return
	}
	ui.List(c, &x.page.list, len(visible), func(i int) {
		index := visible[i]
		ui.Column(c).Key(index).Padding(6, 4).Children(func() {
			renderMessage(c, x.page, index, x.spec.Messages[index], index == len(x.spec.Messages)-1)
		})
	}).Grow(1)
}
