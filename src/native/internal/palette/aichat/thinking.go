package aichat

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// shownReasoningBytes bounds the reasoning text the block draws: its newest part, so a long thought stays cheap to draw.
const shownReasoningBytes = 4000

// truncateBytesForShow is the longest prefix of text that is at most n bytes and ends on a whole rune.
func truncateBytesForShow(text string, n int) string {
	cut := n
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// progressRow is the line that says what a turn is doing, with the time since it began: a spinner, the label and the
// elapsed time. The model's name is never part of it.
func progressRow(c *ui.Context, k *design.TokenSet, label string, since, now time.Time) {
	progressRowFor(c, k, label, now.Sub(since))
}

// progressRowFor draws the progress line for a step that has run for elapsed.
func progressRowFor(c *ui.Context, k *design.TokenSet, label string, elapsed time.Duration) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		ui.Text(c, label).FontSize(13).FontWeight(500).TextColor(k.Accent)
		ui.Text(c, elapsedText(elapsed)).FontSize(12).TextColor(k.Muted)
	})
}

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

// elapsedText names a duration the way the progress row shows it: seconds, then minutes and seconds.
func elapsedText(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second) / time.Second)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
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
