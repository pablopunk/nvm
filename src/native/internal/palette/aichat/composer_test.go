package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// TestComposerSendsTypedTextOnEnterAndClears checks the palette's input is the composer: typing and Enter send the
// text as the next message, and the composer clears.
func TestComposerSendsTypedTextOnEnterAndClears(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	p := newPage(r.deps(), record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("hello there")
	tt.Key(0, ui.KeyEnter)
	messages := mustChat(t, r, record.ID).Messages
	if len(messages) == 0 || messages[0].Author != chat.AuthorUser || messages[0].Text != "hello there" {
		t.Fatalf("messages = %+v, want the typed text sent as the user's message", messages)
	}
	tt.Frame()
	if draft := p.draft; draft != "" {
		t.Fatalf("composer = %q after it was sent, want it cleared", draft)
	}
}

// TestShiftEnterBreaksTheLineInsteadOfSending checks Shift+Enter adds a line to the composer, and that the next Enter
// sends both lines.
func TestShiftEnterBreaksTheLineInsteadOfSending(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	p := newPage(r.deps(), record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("first")
	tt.Key(ui.Shift, ui.KeyEnter)
	tt.Type("second")
	if n := len(mustChat(t, r, record.ID).Messages); n != 0 {
		t.Fatalf("Shift+Enter sent a message: %d messages", n)
	}
	tt.Key(0, ui.KeyEnter)
	messages := mustChat(t, r, record.ID).Messages
	if len(messages) == 0 || messages[0].Text != "first\nsecond" {
		t.Fatalf("messages = %+v, want both lines sent together", messages)
	}
}

// TestQueryTypedBeforeDoesNotLingerInTheComposer checks a query typed at the root does not show up in the chat's
// composer when the chat opens.
func TestQueryTypedBeforeDoesNotLingerInTheComposer(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast, user("hi"), assistant("Hi! How can I help?"))
	r.palette.SetStack(r.root)
	r.palette.SetQuery("asdf")
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if r.palette.Query() != "" {
		t.Fatalf("query = %q after the chat opened, want it cleared", r.palette.Query())
	}
	if tt.HasText("asdf") {
		t.Fatal("the query typed before still shows on the chat page")
	}
}

// TestRefusedSendKeepsTheTypedText checks a message sent while a reply streams is refused and its text stays in the
// composer, so nothing the user typed is lost.
func TestRefusedSendKeepsTheTypedText(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("", chat.TierFast)
	p := newPage(r.deps(), record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	tt.Type("one")
	tt.Key(0, ui.KeyEnter)
	reply := awaitReply(t, transport)
	writeFrame(t, reply, fixtures.StreamFrames("Counting", 24)[0])
	r.awaitReplyText(record.ID, "Counting")
	tt.Frame()

	tt.Type("two")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.draft != "two" {
		t.Fatalf("composer = %q after a refused send, want the text kept", p.draft)
	}
}

// TestEscapeKeepsTheComposerText checks Escape leaves the chat without clearing what is typed in the composer, as the
// desktop app's Escape does.
func TestEscapeKeepsTheComposerText(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	p := newPage(r.deps(), record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Type("keep me")
	tt.Key(0, ui.KeyEscape)
	if r.palette.Top() != r.root {
		t.Fatal("Escape should go back to the root")
	}
	if p.draft != "keep me" {
		t.Fatalf("composer = %q after Escape, want the text kept", p.draft)
	}
}
