package aichat

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// shownReasoningBytes bounds the reasoning text the block draws: its newest part, so a long thought stays cheap to draw.
const shownReasoningBytes = 4000

// reasoningBlock is the model's reasoning of entry index, a disclosure titled Thinking. It is closed until the user
// opens it, and while it is open it shows the newest part of the reasoning.
func reasoningBlock(c *ui.Context, p *page, index int, reasoning string) {
	k := design.Tokens(c)
	open := p.thinkingOpen[index]
	if ui.Collapsible(c, "Thinking", &open, func() {
		ui.Text(c, reasoningTail(reasoning)).FontSize(12).TextColor(k.Muted)
	}).Changed() {
		p.thinkingOpen[index] = open
	}
}

// reasoningTail is the newest part of reasoning, at most shownReasoningBytes, cut on a whole rune and marked when cut.
func reasoningTail(reasoning string) string {
	if len(reasoning) <= shownReasoningBytes {
		return reasoning
	}
	start := len(reasoning) - shownReasoningBytes
	for start < len(reasoning) && !utf8.RuneStart(reasoning[start]) {
		start++
	}
	return "…" + reasoning[start:]
}

// latestReasoning returns the index of the newest entry of the transcript that has reasoning, or -1.
func latestReasoning(messages []chat.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Reasoning != "" {
			return i
		}
	}
	return -1
}
