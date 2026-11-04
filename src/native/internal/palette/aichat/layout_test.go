package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// TestEmptySignedInChatIsBlank checks a new chat of a signed-in account shows no hint text over the composer, as the
// desktop app's empty chat does.
func TestEmptySignedInChatIsBlank(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if tt.HasText("Ask anything. Replies stream in as Markdown.") {
		t.Fatal("an empty signed-in chat shows a hint, want it blank")
	}
	if tt.HasText("Not signed in: sign in from Nevermind Account to chat with AI.") {
		t.Fatal("a signed-in chat says it is not signed in")
	}
}

// TestSignedOutChatSaysSignInFirst checks an empty chat of an account that is not signed in says so.
func TestSignedOutChatSaysSignInFirst(t *testing.T) {
	r := newRig(t, nil)
	r.signed = false
	record := r.chatWith("", chat.TierFast)
	r.palette.Show()
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if !tt.HasText("Not signed in: sign in from Nevermind Account to chat with AI.") {
		t.Fatal("a signed-out chat does not say to sign in first")
	}
}
