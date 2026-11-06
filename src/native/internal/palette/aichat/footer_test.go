package aichat

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestChatFooterStopCancelsTheReply(t *testing.T) {
	stopped := false
	x := NewExtensionChat(func(c *ui.Context, text string, _ bool) { ui.Text(c, text) }, func() time.Time { return goldenNow })
	x.SetSpec(ExtensionChatSpec{
		Title:      "Chat",
		Messages:   []chat.Message{{Author: chat.AuthorAssistant, Text: "Working", Status: chat.StatusStreaming}},
		Stop:       func(search.Host) { stopped = true },
		ChooseTier: func(search.Host, chat.Tier) {},
		Tier:       chat.TierFast,
	})
	r := newRig(t, nil)
	r.open(x.View())
	tester := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tester.Frame()
	count := 0
	for _, text := range tester.Texts() {
		if text == "Stop" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d Stop controls, want one", count)
	}
	if err := tester.Click("Stop"); err != nil {
		t.Fatalf("click Stop: %v", err)
	}
	tester.Frame()
	if !stopped {
		t.Fatal("Stop did not cancel the reply")
	}
}
