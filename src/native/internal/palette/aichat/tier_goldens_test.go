package aichat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// TestTierPickerGoldens renders the composer's tier picker naming Fast, then Smart, then disabled while a reply streams.
func TestTierPickerGoldens(t *testing.T) {
	r := newRig(t, nil)
	fast := r.chatWith("Picker fast", chat.TierFast, user("q"), assistant("a"))
	r.open(ChatView(r.deps(), fast.ID))
	golden.AssertMatrix(t, "tier-picker-fast", r.palette.View, pageGoldens)

	smart := r.chatWith("Picker smart", chat.TierSmart, user("q"), assistant("a"))
	r.open(ChatView(r.deps(), smart.ID))
	golden.AssertMatrix(t, "tier-picker-smart", r.palette.View, pageGoldens)

	busy := r.chatWith("Picker busy", chat.TierSmart, user("q"))
	if err := r.service.Send(busy.ID, "still going"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	r.open(ChatView(r.deps(), busy.ID))
	golden.AssertMatrix(t, "tier-picker-busy", r.palette.View, pageGoldens)
}
